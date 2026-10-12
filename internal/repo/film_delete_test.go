package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// filmFixture is one film with a numbered scene and a full-film file, the scene
// carrying a person so the person-page Films row and count have something to find.
type filmFixture struct {
	film, scene, full int64
	person            int64
}

func seedTrashableFilm(t *testing.T, r *repo.Repo) filmFixture {
	t.Helper()
	ctx := context.Background()
	var fx filmFixture
	var err error
	if fx.film, err = r.CreateFilm(ctx, "Heat", 1995); err != nil {
		t.Fatalf("create film: %v", err)
	}
	if fx.scene, err = r.UpsertVideo(ctx, sampleVideo("/m/heat-01.mkv", "Heat scene", []string{"Alice"}, nil), nil); err != nil {
		t.Fatalf("seed scene: %v", err)
	}
	linkPeople(t, r, fx.scene, "Alice")
	if fx.full, err = r.UpsertVideo(ctx, sampleVideo("/m/heat-full.mkv", "Heat full", nil, nil), nil); err != nil {
		t.Fatalf("seed full: %v", err)
	}
	one := int64(1)
	if _, err := r.AttachFilmVideo(ctx, fx.film, fx.scene, &one, false); err != nil {
		t.Fatalf("attach scene: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, fx.film, fx.full, nil, true); err != nil {
		t.Fatalf("attach full: %v", err)
	}
	fx.person = personIDByName(t, r, "Alice")
	return fx
}

// TestSoftDeleteFilmHidesFromEverySurface is F78's cardinal invariant (spec RD5/P0-5):
// a trashed film disappears from every read at once, its videos stay, and its
// full-film files show again because it stands in for nothing.
func TestSoftDeleteFilmHidesFromEverySurface(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fx := seedTrashableFilm(t, r)

	if err := r.SoftDeleteFilm(ctx, fx.film); err != nil {
		t.Fatalf("soft delete film: %v", err)
	}

	if _, err := r.GetFilm(ctx, fx.film); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("GetFilm = %v, want ErrNotFound", err)
	}
	if vis, err := r.FilmVisible(ctx, fx.film); err != nil || vis {
		t.Errorf("FilmVisible = %v,%v want false,nil", vis, err)
	}
	if films, err := r.ListFilms(ctx); err != nil || len(films) != 0 {
		t.Errorf("ListFilms = %d films, %v; want none", len(films), err)
	}
	if films, err := r.SearchFilms(ctx, "Heat", 10); err != nil || len(films) != 0 {
		t.Errorf("SearchFilms = %d films, %v; want none", len(films), err)
	}
	if films, err := r.ListFilmsForEntity(ctx, fx.person, 0, 0); err != nil || len(films) != 0 {
		t.Errorf("ListFilmsForEntity(person) = %d films, %v; want none", len(films), err)
	}
	if n, err := r.CountFilmsForPerson(ctx, fx.person); err != nil || n != 0 {
		t.Errorf("CountFilmsForPerson = %d, %v; want 0", n, err)
	}
	if byVideo, err := r.FilmsForVideos(ctx, []int64{fx.scene, fx.full}); err != nil || len(byVideo) != 0 {
		t.Errorf("FilmsForVideos = %v, %v; want no attachments", byVideo, err)
	}
	if err := r.EntityExists(ctx, model.EnrichEntityFilm, fx.film); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("EntityExists(film) = %v, want ErrNotFound (no identity ops on a trashed film)", err)
	}
	res, err := r.Search(ctx, "Heat", 10, true)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Films) != 0 {
		t.Errorf("global search films = %d, want 0", len(res.Films))
	}
	// Videos stay (RD2); the full-film file stops being hidden.
	if !containsVideoID(res.Videos, fx.full) || !containsVideoID(res.Videos, fx.scene) {
		t.Errorf("search videos should still include both linked videos: %+v", res.Videos)
	}
	vids, _, err := r.ListVideos(ctx, repo.VideoFilter{HideFullFilmVideos: true})
	if err != nil {
		t.Fatalf("list videos: %v", err)
	}
	if !containsVideoID(vids, fx.full) {
		t.Errorf("a trashed film's full-film file should show in browse again")
	}
	unattached, _, err := r.ListVideos(ctx, repo.VideoFilter{UnattachedToAnyFilm: true})
	if err != nil {
		t.Fatalf("list unattached: %v", err)
	}
	if !containsVideoID(unattached, fx.scene) {
		t.Errorf("a video linked only to a trashed film counts as unattached in the picker")
	}
}

// Restore is lossless (RD5): scenes, numbering and the full-film flag come back.
func TestRestoreFilmIsLossless(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fx := seedTrashableFilm(t, r)

	if err := r.SoftDeleteFilm(ctx, fx.film); err != nil {
		t.Fatal(err)
	}
	if err := r.SoftDeleteFilm(ctx, fx.film); err != nil {
		t.Errorf("second soft delete should be idempotent, got %v", err)
	}
	if err := r.RestoreFilm(ctx, fx.film); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := r.RestoreFilm(ctx, fx.film); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("restore of a live film = %v, want ErrNotFound", err)
	}
	fvs, err := r.FilmVideos(ctx, fx.film)
	if err != nil {
		t.Fatal(err)
	}
	if len(fvs) != 2 || fvs[0].Video.ID != fx.scene || fvs[0].SceneNumber == nil || *fvs[0].SceneNumber != 1 ||
		fvs[1].Video.ID != fx.full || !fvs[1].IsFullFilm {
		t.Errorf("restored film videos = %+v", fvs)
	}
	if _, err := r.GetFilm(ctx, fx.film); err != nil {
		t.Errorf("GetFilm after restore = %v", err)
	}
}

func TestSoftDeleteFilmNotFound(t *testing.T) {
	r := newRepo(t)
	if err := r.SoftDeleteFilm(context.Background(), 999); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("SoftDeleteFilm(missing) = %v, want ErrNotFound", err)
	}
}

// RD7: one Trash, newest first, each row labelled with its kind; a film reads
// "Name (Year)".
func TestTrashListsFilmsAndMedia(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fx := seedTrashableFilm(t, r)

	if err := r.SoftDelete(ctx, fx.scene); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // deleted_at has second resolution
	if err := r.SoftDeleteFilm(ctx, fx.film); err != nil {
		t.Fatal(err)
	}
	items, err := r.Trash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("trash = %+v, want 2 rows", items)
	}
	if items[0].Kind != repo.TrashKindFilm || items[0].ID != fx.film || items[0].Title != "Heat (1995)" {
		t.Errorf("newest row = %+v, want the film titled Heat (1995)", items[0])
	}
	if items[1].Kind != repo.TrashKindMedia || items[1].ID != fx.scene {
		t.Errorf("older row = %+v, want the media item", items[1])
	}
}

// RD6: a trashed film keeps its (name, year); create and year edits onto it are
// refused naming the trashed film, and it is never an alias-routing target.
func TestTrashedFilmHoldsItsKey(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fx := seedTrashableFilm(t, r)
	if err := r.SoftDeleteFilm(ctx, fx.film); err != nil {
		t.Fatal(err)
	}

	id, err := r.CreateFilm(ctx, "heat", 1995)
	if !errors.Is(err, repo.ErrFilmTrashed) || id != fx.film {
		t.Errorf("CreateFilm onto trashed key = %d, %v; want %d, ErrFilmTrashed", id, err, fx.film)
	}
	c, trashed, err := r.TrashedFilm(ctx, fx.film)
	if err != nil || !trashed || c.FilmName != "Heat" || c.Year != 1995 || !c.Trashed {
		t.Errorf("TrashedFilm = %+v, %v, %v", c, trashed, err)
	}

	// No year given: the trashed same-title film must not be the routing target.
	other, err := r.CreateFilm(ctx, "Heat", 0)
	if err != nil || other == fx.film {
		t.Errorf("CreateFilm(Heat, no year) = %d, %v; want a new film, not the trashed one", other, err)
	}

	newer, err := r.CreateFilm(ctx, "Heat", 2023)
	if err != nil {
		t.Fatal(err)
	}
	col, err := r.SetFilmYear(ctx, newer, 1995)
	if err != nil {
		t.Fatal(err)
	}
	if col == nil || col.FilmID != fx.film || !col.Trashed {
		t.Errorf("SetFilmYear onto trashed key = %+v, want a trashed collision", col)
	}
	fire, err := r.CreateFilm(ctx, "Fire", 1995)
	if err != nil {
		t.Fatal(err)
	}
	conflict, err := r.RenameEntity(ctx, model.EnrichEntityFilm, fire, "Heat")
	if !errors.Is(err, repo.ErrNameTaken) || conflict != fx.film {
		t.Errorf("rename onto trashed key = %d, %v; want %d, ErrNameTaken", conflict, err, fx.film)
	}
}

// RD3/RD10: a hard delete leaves the videos, drops the film's own rows, and clears
// every video's standing choice of the film as a source.
func TestHardDeleteFilmCleansUp(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()
	fx := seedTrashableFilm(t, r)

	source := "provider:film:" + itoa64(fx.film)
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, fx.scene, "collection", source, ""); err != nil {
		t.Fatalf("set video decision: %v", err)
	}
	if err := r.SetDecision(ctx, model.EnrichEntityFilm, fx.film, "name", "manual", "Heat!"); err != nil {
		t.Fatalf("set film decision: %v", err)
	}
	if err := r.HardDeleteFilm(ctx, fx.film); err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	if err := r.FilmExistsAny(ctx, fx.film); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("film row survived: %v", err)
	}
	for _, vid := range []int64{fx.scene, fx.full} {
		if _, _, err := r.GetVideo(ctx, vid); err != nil {
			t.Errorf("video %d should survive a film delete: %v", vid, err)
		}
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM field_source_decisions WHERE (entity_type = 'video' AND source = ?) OR (entity_type = 'film' AND entity_id = ?)`,
		source, fx.film).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d decisions survived the film's purge, want 0", n)
	}
	if err := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM film_videos WHERE film_id = ?`, fx.film).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d film_videos rows survived, want 0", n)
	}
}

func TestExpiredSoftDeletedFilms(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	fx := seedTrashableFilm(t, r)
	if err := r.SoftDeleteFilm(ctx, fx.film); err != nil {
		t.Fatal(err)
	}
	if got, err := r.ExpiredSoftDeletedFilms(ctx, time.Now().Add(-time.Hour)); err != nil || len(got) != 0 {
		t.Errorf("not yet expired: %+v, %v", got, err)
	}
	got, err := r.ExpiredSoftDeletedFilms(ctx, time.Now().Add(time.Hour))
	if err != nil || len(got) != 1 || got[0].ID != fx.film {
		t.Errorf("expired = %+v, %v; want the film", got, err)
	}
}
