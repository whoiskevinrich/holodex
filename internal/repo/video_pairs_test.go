package repo_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Duplicate videos (F76): detection, keep both, keep one with carry-over, labels.

func matchVideo(t *testing.T, r *repo.Repo, id int64, provider, ext string) {
	t.Helper()
	if err := r.UpsertEnrichment(context.Background(), model.EnrichEntityVideo, id, provider, ext,
		map[string][]string{"title": {"Harbor Lights"}}); err != nil {
		t.Fatalf("match %d: %v", id, err)
	}
}

func pairsOf(t *testing.T, r *repo.Repo) []repo.VideoPair {
	t.Helper()
	pairs, err := r.ListVideoPairs(context.Background())
	if err != nil {
		t.Fatalf("list video pairs: %v", err)
	}
	return pairs
}

func countT(t *testing.T, d *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", q, err)
	}
	return n
}

// P0-1: a pair exists while both files are live and share one provider's current match.
func TestVideoPairs_Detection(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	other := seedVideo(t, r, "/m/other.mkv")
	seedVideo(t, r, "/m/unmatched.mkv")
	matchVideo(t, r, a, "tmdb", "tmdb:1")
	matchVideo(t, r, b, "tmdb", "tmdb:1")
	matchVideo(t, r, other, "otherdb", "tmdb:1") // same id string, different provider

	if got := pairsOf(t, r); len(got) != 1 || got[0].A != a || got[0].B != b {
		t.Fatalf("pairs = %+v, want only {%d %d}", got, a, b)
	}
	// The row is labeled with the shared provider item's title (F76).
	if got := pairsOf(t, r); got[0].Title != "Harbor Lights" {
		t.Fatalf("pair title = %q, want the provider's title", got[0].Title)
	}

	if err := r.SoftDelete(ctx, b); err != nil {
		t.Fatal(err)
	}
	if got := pairsOf(t, r); len(got) != 0 {
		t.Fatalf("trashed side still pairs: %+v", got)
	}
	if err := r.Restore(ctx, b); err != nil {
		t.Fatal(err)
	}
	if got := pairsOf(t, r); len(got) != 1 {
		t.Fatalf("restored pair did not return: %+v", got)
	}

	matchVideo(t, r, b, "tmdb", "tmdb:2") // re-matched to a different item
	if got := pairsOf(t, r); len(got) != 0 {
		t.Fatalf("re-matched side still pairs: %+v", got)
	}
}

func TestVideoPairs_InactiveFileLeaves(t *testing.T) {
	r, d := newRepoDB(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	matchVideo(t, r, a, "tmdb", "tmdb:1")
	matchVideo(t, r, b, "tmdb", "tmdb:1")
	mustExec(t, d, `UPDATE videos SET active = 0 WHERE id = ?`, b)
	if got := pairsOf(t, r); len(got) != 0 {
		t.Fatalf("missing-from-disk side still pairs: %+v", got)
	}
}

// The current match is the newest non-empty memo: an older row left behind by a
// narrower re-enrich must not keep a stale pair alive.
func TestVideoPairs_NewestMemoWins(t *testing.T) {
	ctx := context.Background()
	r, d := newRepoDB(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	matchVideo(t, r, a, "tmdb", "tmdb:1")
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, b, "tmdb", "tmdb:1",
		map[string][]string{"title": {"x"}, "release_date": {"2019"}}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, d, `UPDATE entity_enrichment SET fetched_at = '2020-01-01T00:00:00Z' WHERE entity_id = ?`, b)
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, b, "tmdb", "tmdb:9",
		map[string][]string{"title": {"y"}}); err != nil {
		t.Fatal(err)
	}
	if got := pairsOf(t, r); len(got) != 0 {
		t.Fatalf("stale release_date row kept the pair: %+v", got)
	}
}

func TestVideoPairs_ThreeFilesThreePairs(t *testing.T) {
	r := newRepo(t)
	for _, p := range []string{"/m/a.mkv", "/m/b.mkv", "/m/c.mkv"} {
		matchVideo(t, r, seedVideo(t, r, p), "tmdb", "tmdb:1")
	}
	if got := pairsOf(t, r); len(got) != 3 {
		t.Fatalf("pairs = %d, want 3", len(got))
	}
}

// P0-6: keep both is durable across a re-match.
func TestVideoPairs_KeepBothDurable(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	matchVideo(t, r, a, "tmdb", "tmdb:1")
	matchVideo(t, r, b, "tmdb", "tmdb:1")
	if err := r.DismissVideoPair(ctx, b, a); err != nil {
		t.Fatal(err)
	}
	matchVideo(t, r, b, "tmdb", "tmdb:1")
	if got := pairsOf(t, r); len(got) != 0 {
		t.Fatalf("kept-both pair returned: %+v", got)
	}
}

// carryFixture builds kept K and trashed T with owner work on both sides.
type carryFixture struct {
	k, t, onlyT, both, filmT, filmBoth int64
}

func seedCarry(t *testing.T, r *repo.Repo) carryFixture {
	t.Helper()
	ctx := context.Background()
	var f carryFixture
	kv := sampleVideo("/m/k.mkv", "K", nil, []string{"shared"})
	k, err := r.UpsertVideo(ctx, kv, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.k = k
	f.t = seedVideo(t, r, "/m/t.mkv")
	x := seedVideo(t, r, "/m/x.mkv")
	matchVideo(t, r, f.k, "tmdb", "tmdb:1")
	matchVideo(t, r, f.t, "tmdb", "tmdb:1")

	pOnly, err := r.CreatePlaylist(ctx, "only T", "manual", "private", nil, []int64{x, f.t})
	if err != nil {
		t.Fatal(err)
	}
	f.onlyT = pOnly.ID
	pBoth, err := r.CreatePlaylist(ctx, "both", "manual", "private", nil, []int64{f.k, f.t})
	if err != nil {
		t.Fatal(err)
	}
	f.both = pBoth.ID

	if f.filmT, err = r.CreateFilm(ctx, "Film T", 2019); err != nil {
		t.Fatal(err)
	}
	scene := int64(3)
	if _, err := r.AttachFilmVideo(ctx, f.filmT, f.t, &scene, false); err != nil {
		t.Fatal(err)
	}
	if f.filmBoth, err = r.CreateFilm(ctx, "Film Both", 2019); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AttachFilmVideo(ctx, f.filmBoth, f.k, nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AttachFilmVideo(ctx, f.filmBoth, f.t, nil, true); err != nil {
		t.Fatal(err)
	}

	for _, d := range []struct {
		id         int64
		field, val string
	}{
		{f.k, "title", "Kept title"},
		{f.t, "title", "Trashed title"},
		{f.t, "edition", "Director's Cut"},
		{f.t, "part", "2"},
		{f.t, "collection", "Harbor Series"},
	} {
		if err := r.SetDecision(ctx, model.EnrichEntityVideo, d.id, d.field, "manual", d.val); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.SetCuration(ctx, model.EnrichEntityVideo, f.t, "genres", "noir", "add"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFacetNotApplicable(ctx, model.EnrichEntityVideo, f.t, "studio"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shared", "solo"} {
		if _, err := r.AttachTagToVideo(ctx, f.t, name); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// P0-4/P0-5: keep one trashes the other and carries work additively; the preview the
// confirm shows equals what moved.
func TestVideoPairs_KeepOneCarriesWork(t *testing.T) {
	ctx := context.Background()
	r, d := newRepoDB(t)
	f := seedCarry(t, r)

	preview, err := r.PreviewCarry(ctx, f.k, f.t)
	if err != nil {
		t.Fatal(err)
	}
	// edits: collection, genres, studio (title is the kept copy's own; edition and part never carry)
	if preview.Playlists != 1 || len(preview.Films) != 1 || preview.Films[0].FilmID != f.filmT ||
		preview.Edits != 3 || preview.ManualTags != 2 {
		t.Fatalf("preview = %+v", preview)
	}

	tPos := countT(t, d, `SELECT position FROM playlist_videos WHERE playlist_id = ? AND video_id = ?`, f.onlyT, f.t)

	moved, err := r.KeepOneVideo(ctx, f.k, f.t)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Playlists != preview.Playlists || len(moved.Films) != len(preview.Films) ||
		moved.Edits != preview.Edits || moved.ManualTags != preview.ManualTags {
		t.Fatalf("moved %+v != preview %+v", moved, preview)
	}

	if n := countT(t, d, `SELECT count(*) FROM videos WHERE id = ? AND deleted_at IS NOT NULL`, f.t); n != 1 {
		t.Fatal("trashed copy is not in Trash")
	}
	// Playlists: K took T's place, at T's position, in "only T"; "both" is unchanged.
	if n := countT(t, d, `SELECT count(*) FROM playlist_videos WHERE playlist_id = ? AND video_id = ? AND position = ?`, f.onlyT, f.k, tPos); n != 1 {
		t.Fatal("kept copy did not take the trashed copy's playlist place")
	}
	if n := countT(t, d, `SELECT count(*) FROM playlist_videos WHERE playlist_id = ? AND video_id = ?`, f.onlyT, f.t); n != 0 {
		t.Fatal("trashed copy still holds the moved playlist place")
	}
	if n := countT(t, d, `SELECT count(*) FROM playlist_videos WHERE playlist_id = ?`, f.both); n != 2 {
		t.Fatal("a playlist holding both copies changed")
	}
	// Films: the scene link moved with its number; the shared film keeps both links.
	if n := countT(t, d, `SELECT count(*) FROM film_videos WHERE film_id = ? AND video_id = ? AND scene_number = 3`, f.filmT, f.k); n != 1 {
		t.Fatal("film link did not move with its scene number")
	}
	if n := countT(t, d, `SELECT count(*) FROM film_videos WHERE film_id = ?`, f.filmBoth); n != 2 {
		t.Fatal("the kept copy's own film link changed")
	}
	// Decisions: K keeps its title; collection carried; part and edition never carry.
	var title string
	if err := d.QueryRow(`SELECT manual_value FROM field_source_decisions WHERE entity_type='video' AND entity_id=? AND field_key='title'`, f.k).Scan(&title); err != nil || title != "Kept title" {
		t.Fatalf("kept title = %q, %v", title, err)
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=? AND field_key='collection'`, f.k); n != 1 {
		t.Fatal("collection decision did not carry")
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=? AND field_key IN ('part','edition')`, f.k); n != 0 {
		t.Fatal("part or edition carried")
	}
	if n := countT(t, d, `SELECT count(*) FROM metadata_curation WHERE entity_type='video' AND entity_id=? AND field_key='genres'`, f.k); n != 1 {
		t.Fatal("curation did not carry")
	}
	if n := countT(t, d, `SELECT count(*) FROM facet_not_applicable WHERE entity_type='video' AND entity_id=? AND canonical_field='studio'`, f.k); n != 1 {
		t.Fatal("not-applicable mark did not carry")
	}
	// Tags: "solo" added, the file-sourced "shared" upgraded to manual.
	if n := countT(t, d, `SELECT count(*) FROM video_tags vt JOIN tags g ON g.id = vt.tag_id WHERE vt.video_id=? AND vt.source='manual' AND g.name IN ('shared','solo')`, f.k); n != 2 {
		t.Fatalf("manual tags on kept copy = %d, want 2", n)
	}
	// The trashed copy keeps its own edits, so a restore loses nothing but what moved.
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=?`, f.t); n != 4 {
		t.Fatalf("trashed copy decisions = %d, want 4", n)
	}
}

// A decision pinned to a source the kept copy can't read would blank the field there, so
// it stays behind; a film-pinned decision carries when its film link moves with it.
func TestVideoPairs_KeepOneSkipsUnreadableDecisions(t *testing.T) {
	ctx := context.Background()
	r, d := newRepoDB(t)
	k := seedVideo(t, r, "/m/k.mkv")
	tr := seedVideo(t, r, "/m/t.mkv")
	matchVideo(t, r, k, "tmdb", "tmdb:1")
	matchVideo(t, r, tr, "tmdb", "tmdb:1")
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, tr, "otherdb", "otherdb:7",
		map[string][]string{"title": {"Other"}}); err != nil {
		t.Fatal(err)
	}
	film, err := r.CreateFilm(ctx, "Film", 2019)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AttachFilmVideo(ctx, film, tr, nil, true); err != nil {
		t.Fatal(err)
	}
	for field, src := range map[string]string{
		"title":      "provider:otherdb",              // kept copy not matched to otherdb
		"tagline":    "provider:tmdb",                 // kept copy matched: carries
		"collection": "provider:film:" + itoa64(film), // link moves with it: carries
	} {
		if err := r.SetDecision(ctx, model.EnrichEntityVideo, tr, field, src, ""); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := r.PreviewCarry(ctx, k, tr)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Edits != 2 {
		t.Fatalf("preview edits = %d, want 2 (tagline, collection)", preview.Edits)
	}
	if _, err := r.KeepOneVideo(ctx, k, tr); err != nil {
		t.Fatal(err)
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=? AND field_key='title'`, k); n != 0 {
		t.Fatal("a decision pinned to an unmatched provider carried")
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=? AND field_key IN ('tagline','collection')`, k); n != 2 {
		t.Fatal("readable provider and film decisions did not carry")
	}
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// P0-5: if any step fails, nothing changes and neither file is trashed.
func TestVideoPairs_KeepOneAtomic(t *testing.T) {
	ctx := context.Background()
	r, d := newRepoDB(t)
	f := seedCarry(t, r)
	mustExec(t, d, `CREATE TRIGGER boom BEFORE UPDATE OF deleted_at ON videos
		WHEN NEW.deleted_at IS NOT NULL BEGIN SELECT RAISE(ABORT, 'boom'); END`)

	if _, err := r.KeepOneVideo(ctx, f.k, f.t); err == nil {
		t.Fatal("keep one succeeded through a failing trash step")
	}
	if n := countT(t, d, `SELECT count(*) FROM playlist_videos WHERE playlist_id = ? AND video_id = ?`, f.onlyT, f.t); n != 1 {
		t.Fatal("playlist place moved despite the rollback")
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=? AND field_key='collection'`, f.k); n != 0 {
		t.Fatal("decision carried despite the rollback")
	}
	if n := countT(t, d, `SELECT count(*) FROM videos WHERE id = ? AND deleted_at IS NULL`, f.t); n != 1 {
		t.Fatal("trashed copy left the library despite the rollback")
	}
}

func TestVideoPairs_KeepOneNotLive(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	matchVideo(t, r, a, "tmdb", "tmdb:1")
	matchVideo(t, r, b, "tmdb", "tmdb:2")
	if _, err := r.KeepOneVideo(ctx, a, b); !errors.Is(err, repo.ErrVideoPairNotLive) {
		t.Fatalf("err = %v, want ErrVideoPairNotLive", err)
	}
	if err := r.LabelVideoPair(ctx, "edition", [2]repo.VideoLabel{{ID: a, Value: "Cut"}, {ID: b}}); !errors.Is(err, repo.ErrVideoPairNotLive) {
		t.Fatalf("label err = %v, want ErrVideoPairNotLive", err)
	}
}

// P0-7/8: labeling sets the values and resolves the pair; a blank side is untouched.
func TestVideoPairs_LabelResolves(t *testing.T) {
	ctx := context.Background()
	r, d := newRepoDB(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	matchVideo(t, r, a, "tmdb", "tmdb:1")
	matchVideo(t, r, b, "tmdb", "tmdb:1")
	if err := r.LabelVideoPair(ctx, "edition", [2]repo.VideoLabel{{ID: a, Value: "Director's Cut"}, {ID: b}}); err != nil {
		t.Fatal(err)
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND field_key='edition' AND entity_id=? AND manual_value='Director''s Cut'`, a); n != 1 {
		t.Fatal("edition not set")
	}
	if n := countT(t, d, `SELECT count(*) FROM field_source_decisions WHERE entity_type='video' AND entity_id=?`, b); n != 0 {
		t.Fatal("blank side was written")
	}
	if got := pairsOf(t, r); len(got) != 0 {
		t.Fatalf("labeled pair still listed: %+v", got)
	}
}

func TestVideoPairs_HardDeleteDropsKeepBoth(t *testing.T) {
	ctx := context.Background()
	r, d := newRepoDB(t)
	a := seedVideo(t, r, "/m/a.mkv")
	b := seedVideo(t, r, "/m/b.mkv")
	if err := r.DismissVideoPair(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	if err := r.HardDelete(ctx, b); err != nil {
		t.Fatal(err)
	}
	if n := countT(t, d, `SELECT count(*) FROM entity_keep_separate WHERE entity_type = 'video'`); n != 0 {
		t.Fatal("keep-both row survived a permanent delete")
	}
}
