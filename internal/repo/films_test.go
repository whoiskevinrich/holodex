package repo_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

func TestFilmsForVideo(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	other, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed other video: %v", err)
	}

	// No attachments yet.
	got, err := r.FilmsForVideo(ctx, id)
	if err != nil {
		t.Fatalf("films for video (empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("unattached video: got %v, want empty", got)
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	exec(`INSERT INTO films (id, name, year) VALUES (1, 'Zoo Film', 2020)`)
	exec(`INSERT INTO films (id, name, year) VALUES (2, 'Alpha Film', 2019)`)
	exec(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (1, ?, 6, 0, '2026-01-01T00:00:00Z')`, id)
	exec(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (2, ?, NULL, 1, '2026-01-01T00:00:00Z')`, id)
	exec(`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (1, ?, NULL, 0, '2026-01-01T00:00:00Z')`, other)

	got, err = r.FilmsForVideo(ctx, id)
	if err != nil {
		t.Fatalf("films for video: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d attachments, want 2: %+v", len(got), got)
	}
	// Ordered by film name COLLATE NOCASE: "Alpha Film" before "Zoo Film".
	if got[0].FilmID != 2 || got[0].FilmName != "Alpha Film" || !got[0].IsFullFilm {
		t.Errorf("attachment[0] = %+v, want {2 Alpha Film true}", got[0])
	}
	if got[1].FilmID != 1 || got[1].FilmName != "Zoo Film" || got[1].IsFullFilm {
		t.Errorf("attachment[1] = %+v, want {1 Zoo Film false}", got[1])
	}

	// The other video's attachment must not leak in.
	gotOther, err := r.FilmsForVideo(ctx, other)
	if err != nil {
		t.Fatalf("films for other video: %v", err)
	}
	if len(gotOther) != 1 || gotOther[0].FilmID != 1 {
		t.Fatalf("other video attachments = %+v, want [film 1]", gotOther)
	}
}

func TestCreateFilm(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	id, err := r.CreateFilm(ctx, "  My Film  ", 2020)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	f, err := r.GetFilm(ctx, id)
	if err != nil {
		t.Fatalf("get film: %v", err)
	}
	if f.Name != "My Film" || f.Year != 2020 {
		t.Errorf("film = %+v, want name %q trimmed, year 2020", f, "My Film")
	}

	// Same name+year (case-folded) is get-or-create, not a duplicate row.
	again, err := r.CreateFilm(ctx, "my film", 2020)
	if !errors.Is(err, repo.ErrFilmExists) || again != id {
		t.Errorf("create duplicate: id=%d err=%v, want id=%d ErrFilmExists", again, err, id)
	}

	// Same name, different year is a distinct film -- the common legitimate case.
	otherYear, err := r.CreateFilm(ctx, "My Film", 2021)
	if err != nil {
		t.Fatalf("create same-name-different-year: %v", err)
	}
	if otherYear == id {
		t.Errorf("same name different year got same id %d", id)
	}

	// No year is legal (nullableYear).
	noYear, err := r.CreateFilm(ctx, "Undated Film", 0)
	if err != nil {
		t.Fatalf("create undated film: %v", err)
	}
	nf, err := r.GetFilm(ctx, noYear)
	if err != nil || nf.Year != 0 {
		t.Errorf("undated film year = %d (err=%v), want 0", nf.Year, err)
	}
}

func TestListAndSearchFilms(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	zoo, err := r.CreateFilm(ctx, "Zoo Adventure", 2018)
	if err != nil {
		t.Fatalf("create zoo film: %v", err)
	}
	if _, err := r.CreateFilm(ctx, "Alpha Quest", 2019); err != nil {
		t.Fatalf("create alpha film: %v", err)
	}

	// Empty (zero-attachment) films still list -- no prune-on-empty for an
	// asserted link, unlike ListStudios.
	all, err := r.ListFilms(ctx)
	if err != nil {
		t.Fatalf("list films: %v", err)
	}
	if len(all) != 2 || all[0].Name != "Alpha Quest" || all[1].Name != "Zoo Adventure" {
		t.Fatalf("list films = %+v, want [Alpha Quest, Zoo Adventure] name-sorted", all)
	}
	if all[1].VideoCount != 0 {
		t.Errorf("zoo video count = %d, want 0 (no attachments yet)", all[1].VideoCount)
	}

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/z.mkv", "Z", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, zoo, vid, nil, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	got, err := r.GetFilm(ctx, zoo)
	if err != nil || got.VideoCount != 1 {
		t.Errorf("zoo video count after attach = %d (err=%v), want 1", got.VideoCount, err)
	}

	found, err := r.SearchFilms(ctx, "zoo", 10)
	if err != nil {
		t.Fatalf("search films: %v", err)
	}
	if len(found) != 1 || found[0].ID != zoo {
		t.Fatalf("search %q = %+v, want [zoo]", "zoo", found)
	}
	noMatch, err := r.SearchFilms(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("search no-match: %v", err)
	}
	if len(noMatch) != 0 {
		t.Fatalf("search no-match = %+v, want empty", noMatch)
	}

	// Global Search() (HOLODEX-283): films appear as their own group when
	// filmsEnabled, and are omitted (empty, non-nil) when it's off.
	all2, err := r.Search(ctx, "zoo", 10, true)
	if err != nil {
		t.Fatalf("global search (filmsEnabled=true): %v", err)
	}
	if len(all2.Films) != 1 || all2.Films[0].ID != zoo {
		t.Fatalf("global search films = %+v, want [zoo]", all2.Films)
	}
	disabled, err := r.Search(ctx, "zoo", 10, false)
	if err != nil {
		t.Fatalf("global search (filmsEnabled=false): %v", err)
	}
	if disabled.Films == nil || len(disabled.Films) != 0 {
		t.Fatalf("global search films (disabled) = %+v, want non-nil empty", disabled.Films)
	}
}

func TestAttachFilmVideoCollisions(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	filmID, err := r.CreateFilm(ctx, "Collision Film", 2022)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	v1, err := r.UpsertVideo(ctx, sampleVideo("/m/one.mkv", "One", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed v1: %v", err)
	}
	v2, err := r.UpsertVideo(ctx, sampleVideo("/m/two.mkv", "Two", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed v2: %v", err)
	}

	six := int64(6)
	if _, err := r.AttachFilmVideo(ctx, filmID, v1, &six, false); err != nil {
		t.Fatalf("attach v1 at scene 6: %v", err)
	}

	// Re-attaching the same pair is rejected, not upserted.
	if _, err := r.AttachFilmVideo(ctx, filmID, v1, &six, false); !errors.Is(err, repo.ErrFilmVideoAlreadyAttached) {
		t.Errorf("re-attach same pair: err=%v, want ErrFilmVideoAlreadyAttached", err)
	}

	// A different video at the same scene number is a named collision, not a swap.
	occ, err := r.AttachFilmVideo(ctx, filmID, v2, &six, false)
	if !errors.Is(err, repo.ErrSceneNumberTaken) {
		t.Fatalf("attach v2 at taken scene 6: err=%v, want ErrSceneNumberTaken", err)
	}
	if occ == nil || occ.VideoID != v1 || occ.VideoTitle != "One" {
		t.Errorf("collision occupant = %+v, want video %d %q", occ, v1, "One")
	}

	// Unnumbered (nil) never collides -- many unnumbered scenes coexist.
	if _, err := r.AttachFilmVideo(ctx, filmID, v2, nil, false); err != nil {
		t.Fatalf("attach v2 unnumbered: %v", err)
	}

	// Detach then re-attach is fine; detaching an unattached pair is ErrNotFound.
	if err := r.DetachFilmVideo(ctx, filmID, v2); err != nil {
		t.Fatalf("detach v2: %v", err)
	}
	if err := r.DetachFilmVideo(ctx, filmID, v2); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("detach already-detached: err=%v, want ErrNotFound", err)
	}
}

func TestBulkAttachFilmVideos(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	filmID, err := r.CreateFilm(ctx, "Bulk Film", 2023)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	var vids []int64
	for i := 0; i < 3; i++ {
		vid, err := r.UpsertVideo(ctx, sampleVideo(fmt.Sprintf("/m/bulk%d.mkv", i), fmt.Sprintf("Bulk %d", i), nil, nil), nil)
		if err != nil {
			t.Fatalf("seed video %d: %v", i, err)
		}
		vids = append(vids, vid)
	}

	if _, err := r.BulkAttachFilmVideos(ctx, filmID, vids, 10); err != nil {
		t.Fatalf("bulk attach: %v", err)
	}
	fvs, err := r.FilmVideos(ctx, filmID)
	if err != nil {
		t.Fatalf("film videos: %v", err)
	}
	if len(fvs) != 3 {
		t.Fatalf("film videos = %+v, want 3", fvs)
	}
	for i, fv := range fvs {
		want := int64(10 + i)
		if fv.SceneNumber == nil || *fv.SceneNumber != want {
			t.Errorf("scene[%d] = %v, want %d", i, fv.SceneNumber, want)
		}
	}

	// A mid-batch collision (scene 11 already taken by vids[1]) rolls back the
	// whole batch -- vids[3] below must NOT have been attached either.
	vid4, err := r.UpsertVideo(ctx, sampleVideo("/m/bulk3.mkv", "Bulk 3", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video 3: %v", err)
	}
	occ, err := r.BulkAttachFilmVideos(ctx, filmID, []int64{vid4}, 11)
	if !errors.Is(err, repo.ErrSceneNumberTaken) {
		t.Fatalf("bulk attach onto taken scene: err=%v, want ErrSceneNumberTaken", err)
	}
	if occ == nil || occ.VideoID != vids[1] {
		t.Errorf("collision occupant = %+v, want video %d", occ, vids[1])
	}
	stillAttached, err := r.FilmsForVideo(ctx, vid4)
	if err != nil {
		t.Fatalf("films for vid4: %v", err)
	}
	if len(stillAttached) != 0 {
		t.Errorf("vid4 attachments after failed bulk attach = %+v, want none", stillAttached)
	}
}

// TestHideFullFilmVideos covers RD6/HOLODEX-282: VideoFilter.HideFullFilmVideos
// (browse/search) and Search/Related's hideFullFilmVideos param (RelatedShelf)
// must exclude a video marked is_full_film, while leaving it visible when the
// caller passes false (e.g. films_enabled=false, or the film-video-candidates
// picker's videoFilterFromQuery reuse — see handlers.go doc comment) and never
// affecting GetVideo (the detail page, always reachable by direct URL).
func TestHideFullFilmVideos(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	r := repo.New(sqlDB)
	ctx := context.Background()

	filmID, err := r.CreateFilm(ctx, "Hidden Film", 2024)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	full, err := r.UpsertVideo(ctx, sampleVideo("/m/full.mkv", "Full Film", nil, []string{"shared-tag"}), nil)
	if err != nil {
		t.Fatalf("seed full-film video: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, full, nil, true); err != nil {
		t.Fatalf("attach full-film video: %v", err)
	}
	scene, err := r.UpsertVideo(ctx, sampleVideo("/m/scene.mkv", "A Scene", nil, []string{"shared-tag"}), nil)
	if err != nil {
		t.Fatalf("seed scene video: %v", err)
	}
	one := int64(1)
	if _, err := r.AttachFilmVideo(ctx, filmID, scene, &one, false); err != nil {
		t.Fatalf("attach scene video: %v", err)
	}

	// ListVideos: hidden only when the flag is set.
	shown, _, err := r.ListVideos(ctx, repo.VideoFilter{})
	if err != nil {
		t.Fatalf("list videos (flag off): %v", err)
	}
	if !containsVideoID(shown, full) {
		t.Errorf("HideFullFilmVideos=false: full-film video missing from list, want present")
	}
	hidden, _, err := r.ListVideos(ctx, repo.VideoFilter{HideFullFilmVideos: true})
	if err != nil {
		t.Fatalf("list videos (flag on): %v", err)
	}
	if containsVideoID(hidden, full) {
		t.Errorf("HideFullFilmVideos=true: full-film video present in list, want hidden")
	}
	if !containsVideoID(hidden, scene) {
		t.Errorf("HideFullFilmVideos=true: non-full-film scene video missing, want present")
	}

	// GetVideo (detail page): always reachable regardless of the flag.
	if _, _, err := r.GetVideo(ctx, full); err != nil {
		t.Errorf("GetVideo(full-film video) = %v, want reachable by direct id", err)
	}

	// Search: hidden from video results only when filmsEnabled=true.
	res, err := r.Search(ctx, "Full Film", 10, false)
	if err != nil {
		t.Fatalf("search (flag off): %v", err)
	}
	if !containsVideoID(res.Videos, full) {
		t.Errorf("search flag off: full-film video missing from results, want present")
	}
	res, err = r.Search(ctx, "Full Film", 10, true)
	if err != nil {
		t.Fatalf("search (flag on): %v", err)
	}
	if containsVideoID(res.Videos, full) {
		t.Errorf("search flag on: full-film video present in results, want hidden")
	}

	// Related/randomSiblings (RelatedShelf): both videos share "shared-tag", making
	// `full` a sibling candidate for `scene`'s tag shelf; it must drop out only
	// when hideFullFilmVideos is true.
	related, err := r.Related(ctx, scene, 5, false)
	if err != nil {
		t.Fatalf("related (flag off): %v", err)
	}
	if related.Tag == nil || !containsVideoID(related.Tag.Items, full) {
		t.Errorf("related flag off: full-film video missing from tag shelf, want present: %+v", related.Tag)
	}
	related, err = r.Related(ctx, scene, 5, true)
	if err != nil {
		t.Fatalf("related (flag on): %v", err)
	}
	if related.Tag != nil && containsVideoID(related.Tag.Items, full) {
		t.Errorf("related flag on: full-film video present in tag shelf, want hidden: %+v", related.Tag)
	}
}

func containsVideoID(vids []model.Video, id int64) bool {
	for _, v := range vids {
		if v.ID == id {
			return true
		}
	}
	return false
}
