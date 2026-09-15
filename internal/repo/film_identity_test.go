package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Films on the ADR-061 identity spine (HOLODEX-376, ADR-096 D3, spec F60 RD4/RD5):
// composite key, year-aware alias routing, the ambiguous no-year case queued rather
// than routed, rename keeping the old title as an alias, merge, and the cleanup trigger.

func mustCreateFilm(t *testing.T, r *repo.Repo, name string, year int) int64 {
	t.Helper()
	id, err := r.CreateFilm(context.Background(), name, year)
	if err != nil {
		t.Fatalf("create film %q/%d: %v", name, year, err)
	}
	return id
}

// filmPairs returns the film review-queue pairs keyed "A|B" (names) → variation.
func filmPairs(t *testing.T, r *repo.Repo) map[string]string {
	t.Helper()
	pairs, err := r.ListReviewPairs(context.Background())
	if err != nil {
		t.Fatalf("list review pairs: %v", err)
	}
	got := map[string]string{}
	for _, p := range pairs {
		if p.EntityType == model.EnrichEntityFilm {
			got[p.A.Name+"|"+p.B.Name] = p.Variation
		}
	}
	return got
}

func filmAliases(t *testing.T, r *repo.Repo, filmID int64) []model.EntityAlias {
	t.Helper()
	al, err := r.AliasesForEntity(context.Background(), model.EnrichEntityFilm, filmID)
	if err != nil {
		t.Fatalf("aliases: %v", err)
	}
	return al
}

func TestFilmCompositeKey(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()

	s1980 := mustCreateFilm(t, r, "Superman II", 1980)
	s2006 := mustCreateFilm(t, r, "Superman II", 2006)
	if s1980 == s2006 {
		t.Fatal("same title, different years must be two films")
	}
	// Same title + same year folds case/whitespace (the nameKey), returning the
	// existing film rather than a duplicate.
	if id, err := r.CreateFilm(ctx, "  superman ii ", 1980); !errors.Is(err, repo.ErrFilmExists) || id != s1980 {
		t.Fatalf("filmKey create = (%d, %v), want (%d, ErrFilmExists)", id, err, s1980)
	}
	// ux_films_namekey backs the same rule at the schema layer.
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO films (name, year) VALUES ('SUPERMAN II ', 1980)`); err == nil {
		t.Fatal("ux_films_namekey did not reject a case/whitespace duplicate of (title, year)")
	}
	// Two legitimate same-title films are surfaced, never folded: a 'same-title'
	// review pair, the non-fuzzy kind that stays until the owner acts.
	if got := filmPairs(t, r); got["Superman II|Superman II"] != "same-title" {
		t.Fatalf("review pairs = %v, want the 1980/2006 pair flagged same-title", got)
	}
}

func TestFilmAliasRoutingByYear(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()

	s1980 := mustCreateFilm(t, r, "Superman II", 1980)
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityFilm, s1980, "Superman 2"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// Alias + matching year → routes to the film.
	if id, err := r.CreateFilm(ctx, "superman 2", 1980); !errors.Is(err, repo.ErrFilmExists) || id != s1980 {
		t.Fatalf("alias with matching year = (%d, %v), want (%d, ErrFilmExists)", id, err, s1980)
	}
	// Alias + no year, exactly one film answers to the title → routes.
	if id, err := r.CreateFilm(ctx, "Superman 2", 0); !errors.Is(err, repo.ErrFilmExists) || id != s1980 {
		t.Fatalf("alias with no year = (%d, %v), want (%d, ErrFilmExists)", id, err, s1980)
	}
	// Alias + a different year → a new film, and the pair is queued for the owner.
	s2006, err := r.CreateFilm(ctx, "Superman 2", 2006)
	if err != nil || s2006 == s1980 {
		t.Fatalf("alias with other year = (%d, %v), want a new film", s2006, err)
	}
	if got := filmPairs(t, r); got["Superman II|Superman 2"] != "same-title" {
		t.Fatalf("review pairs = %v, want the alias-mismatch pair queued", got)
	}
}

func TestFilmAmbiguousNoYearQueues(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()

	a := mustCreateFilm(t, r, "Nosferatu", 1922)
	b := mustCreateFilm(t, r, "Nosferatu", 1979)
	// The owner's standing verdict on a/b: kept separate, never re-queued.
	if err := r.DismissReviewPair(ctx, model.EnrichEntityFilm, a, b); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if len(filmPairs(t, r)) != 0 {
		t.Fatal("dismissed pair still queued")
	}

	// No year and two films answer to the title: nothing is auto-routed — a third,
	// year-less film is created and queued against BOTH; a/b stay dismissed.
	c, err := r.CreateFilm(ctx, "Nosferatu", 0)
	if err != nil || c == a || c == b {
		t.Fatalf("ambiguous no-year create = (%d, %v), want a new film", c, err)
	}
	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var withC int
	for _, p := range pairs {
		if p.EntityType != model.EnrichEntityFilm {
			continue
		}
		if p.A.ID != c && p.B.ID != c {
			t.Fatalf("pair %d/%d re-queued despite keep-separate", p.A.ID, p.B.ID)
		}
		if p.Variation != "same-title" {
			t.Fatalf("variation = %q, want same-title", p.Variation)
		}
		withC++
	}
	if withC != 2 {
		t.Fatalf("new film is in %d pairs, want 2 (one per same-title film)", withC)
	}
}

func TestRenameFilmKeepsOldTitleAsAlias(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()

	f := mustCreateFilm(t, r, "Akira", 1988)
	mustCreateFilm(t, r, "Akira", 2019)
	domu := mustCreateFilm(t, r, "Domu", 1988)
	tetsuo := mustCreateFilm(t, r, "Tetsuo", 1989)

	// The key is composite: a title already used under ANOTHER year is free — but the
	// pair is queued for the owner, exactly as a same-title create is.
	if cid, err := r.RenameEntity(ctx, model.EnrichEntityFilm, tetsuo, "Akira"); cid != 0 || err != nil {
		t.Fatalf("rename onto a same-title/other-year name = (%d, %v), want free", cid, err)
	}
	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("pairs: %v", err)
	}
	var withTetsuo int
	for _, p := range pairs {
		if p.EntityType == model.EnrichEntityFilm && (p.A.ID == tetsuo || p.B.ID == tetsuo) && p.Variation == "same-title" {
			withTetsuo++
		}
	}
	if withTetsuo != 2 {
		t.Fatalf("renamed film is in %d same-title pairs, want 2 (the 1988 and 2019 Akiras)", withTetsuo)
	}
	// … while the same title under the SAME year collides, naming the occupant.
	if cid, err := r.RenameEntity(ctx, model.EnrichEntityFilm, domu, "akira"); !errors.Is(err, repo.ErrNameTaken) || cid != f {
		t.Fatalf("rename onto (Akira, 1988) = (%d, %v), want (%d, ErrNameTaken)", cid, err, f)
	}

	if cid, err := r.RenameEntity(ctx, model.EnrichEntityFilm, f, "AKIRA (Akira)"); cid != 0 || err != nil {
		t.Fatalf("rename: (%d, %v)", cid, err)
	}
	got, err := r.GetFilm(ctx, f)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "AKIRA (Akira)" || got.Year != 1988 {
		t.Fatalf("after rename = %q/%d, want the new title with the year untouched", got.Name, got.Year)
	}
	if len(got.Aliases) != 1 || got.Aliases[0].Alias != "Akira" {
		t.Fatalf("aliases after rename = %+v, want the old title kept (and carried on GetFilm)", got.Aliases)
	}
	// The old title still routes (year rule) to the renamed film.
	if id, err := r.CreateFilm(ctx, "Akira", 1988); !errors.Is(err, repo.ErrFilmExists) || id != f {
		t.Fatalf("old title after rename = (%d, %v), want (%d, ErrFilmExists)", id, err, f)
	}
}

func TestFilmDeleteCleansSpine(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()

	a := mustCreateFilm(t, r, "Solaris", 1972)
	b := mustCreateFilm(t, r, "Solaris", 2002)
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityFilm, a, "Solyaris"); err != nil {
		t.Fatalf("alias: %v", err)
	}
	// A provider alias the owner removed leaves a suppression row behind.
	if skipped, err := r.ApplyProviderAliases(ctx, model.EnrichEntityFilm, a, tmdb, []string{"Солярис"}); err != nil || len(skipped) != 0 {
		t.Fatalf("provider alias apply = (%v, %v)", skipped, err)
	}
	for _, al := range filmAliases(t, r, a) {
		if al.Alias == "Солярис" {
			if err := r.DeleteEntityAlias(ctx, model.EnrichEntityFilm, a, al.ID); err != nil {
				t.Fatalf("delete provider alias: %v", err)
			}
		}
	}
	if err := r.AddKeepSeparate(ctx, model.EnrichEntityFilm, a, b); err != nil {
		t.Fatalf("keep separate: %v", err)
	}
	// a/b were queued as same-title on create, too.

	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM films WHERE id = ?`, a); err != nil {
		t.Fatalf("delete film: %v", err)
	}
	for table, where := range map[string]string{
		"entity_aliases":            "entity_id = ?1",
		"entity_alias_suppressions": "entity_id = ?1",
		"entity_keep_separate":      "id_lo = ?1 OR id_hi = ?1",
		"identity_review_queue":     "id_lo = ?1 OR id_hi = ?1",
	} {
		var n int
		if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE entity_type = 'film' AND (`+where+`)`, a).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s still holds %d row(s) for the deleted film", table, n)
		}
	}
}

func TestApplyProviderAliases_Film(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()
	f := mustCreateFilm(t, r, "Superman II", 1980)
	mustCreateFilm(t, r, "Superman 2", 2006)

	// "Superman 2" is another film's title: a year-blind exact conflict, so it is
	// skipped and queued (never auto-attached), while the free title lands as a
	// provider-sourced alias.
	skipped, err := r.ApplyProviderAliases(ctx, model.EnrichEntityFilm, f, tmdb,
		[]string{"Superman II: The Adventure Continues", "Superman 2"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(skipped) != 1 || skipped[0].Alias != "Superman 2" {
		t.Fatalf("skipped = %+v, want just the colliding title", skipped)
	}
	got := filmAliases(t, r, f)
	if len(got) != 1 || got[0].Alias != "Superman II: The Adventure Continues" || got[0].Source != tmdb {
		t.Fatalf("film aliases = %+v, want the free title with source %q", got, tmdb)
	}
}

func TestMergeFilms(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()

	winner := mustCreateFilm(t, r, "Akira", 1988)
	loser := mustCreateFilm(t, r, "AKIRA (1988)", 1988)
	otomo := seedPerson(t, r, "Katsuhiro Otomo")
	v1, _ := r.UpsertVideo(ctx, sampleVideo("/m/1.mkv", "One", nil, nil), nil)
	v2, _ := r.UpsertVideo(ctx, sampleVideo("/m/2.mkv", "Two", nil, nil), nil)
	v3, _ := r.UpsertVideo(ctx, sampleVideo("/m/3.mkv", "Three", nil, nil), nil)
	one, two := int64(1), int64(2)
	if _, err := r.AttachFilmVideo(ctx, winner, v1, &one, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	// The loser's scene 1 collides with the winner's; its scene 2 does not.
	if _, err := r.AttachFilmVideo(ctx, loser, v2, &one, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, loser, v3, &two, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := r.AddFilmPersonRole(ctx, loser, otomo, "director", nil); err != nil {
		t.Fatalf("role: %v", err)
	}

	affected, err := r.MergeEntitiesWithAffectedVideos(ctx, model.EnrichEntityFilm, winner, loser)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if len(affected) != 2 {
		t.Fatalf("affected = %v, want the loser's two videos", affected)
	}
	fvs, err := r.FilmVideos(ctx, winner)
	if err != nil {
		t.Fatalf("film videos: %v", err)
	}
	scenes := map[int64]*int64{}
	for _, fv := range fvs {
		scenes[fv.Video.ID] = fv.SceneNumber
	}
	if len(scenes) != 3 {
		t.Fatalf("winner has %d videos after merge, want all 3 (no link dropped)", len(scenes))
	}
	if scenes[v1] == nil || *scenes[v1] != 1 || scenes[v2] != nil || scenes[v3] == nil || *scenes[v3] != 2 {
		t.Fatalf("scene numbers after merge = v1:%v v2:%v v3:%v, want 1 / NULL (collided) / 2",
			scenes[v1], scenes[v2], scenes[v3])
	}
	roles, err := r.FilmPeopleRoles(ctx, winner)
	if err != nil || len(roles) != 1 || roles[0].Role != "director" {
		t.Fatalf("roles after merge = %+v, %v; want the loser's director carried over", roles, err)
	}
	if got := filmAliases(t, r, winner); len(got) != 1 || got[0].Alias != "AKIRA (1988)" {
		t.Fatalf("aliases after merge = %+v, want the loser's title", got)
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM films WHERE id = ?`, loser).Scan(&n); err != nil || n != 0 {
		t.Fatalf("loser still present (%d, %v)", n, err)
	}
}
