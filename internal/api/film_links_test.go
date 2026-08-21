package api_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// filmsServer wires a real repo + Handlers with `studio` and `actors` mapped (so
// RelinkVideoEntity's studio and person branches both do real work) over one seeded
// video, plus the raw *sql.DB -- films/film_videos (migration 0043) intentionally
// have no repo methods yet (ADR-085: film_videos is an ASSERTED owner link with no
// reconciler), so seeding/reading it here goes directly through SQL, the same way
// a future attach endpoint eventually will.
func filmsServer(t *testing.T) (h *api.Handlers, r *repo.Repo, sqlDB *sql.DB, vid int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	vid, err = r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{{SourceKey: "Publisher", Value: "Acme"}})
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: studio\n    label: Studio\n    filterable: true\n    sources: [file:Publisher, tmdb:studio]\n" +
		"  - canonical: actors\n    label: Actors\n    multi: true\n    sources: [tmdb:actors]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h = api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	return h, r, database, vid
}

// filmVideoRow snapshots one film_videos row for before/after comparison.
type filmVideoRow struct {
	filmID, videoID int64
	sceneNumber     sql.NullInt64
	isFullFilm      bool
	createdAt       string
}

// seedFilmVideo directly inserts a film + a film_videos link (scene 6), standing in
// for the not-yet-built attach endpoint.
func seedFilmVideo(t *testing.T, sqlDB *sql.DB, videoID int64) filmVideoRow {
	t.Helper()
	ctx := context.Background()
	res, err := sqlDB.ExecContext(ctx, `INSERT INTO films (name, year) VALUES (?, ?)`, "Scene Test Film", 2020)
	if err != nil {
		t.Fatalf("seed film: %v", err)
	}
	filmID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("film id: %v", err)
	}
	createdAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO film_videos (film_id, video_id, scene_number, is_full_film, created_at) VALUES (?, ?, ?, 0, ?)`,
		filmID, videoID, 6, createdAt,
	); err != nil {
		t.Fatalf("seed film_videos: %v", err)
	}
	return filmVideoRow{filmID: filmID, videoID: videoID, sceneNumber: sql.NullInt64{Int64: 6, Valid: true}, isFullFilm: false, createdAt: createdAt}
}

func readFilmVideos(t *testing.T, sqlDB *sql.DB) []filmVideoRow {
	t.Helper()
	rows, err := sqlDB.QueryContext(context.Background(),
		`SELECT film_id, video_id, scene_number, is_full_film, created_at FROM film_videos ORDER BY film_id, video_id`)
	if err != nil {
		t.Fatalf("query film_videos: %v", err)
	}
	defer rows.Close()
	var out []filmVideoRow
	for rows.Next() {
		var fv filmVideoRow
		var isFull int
		if err := rows.Scan(&fv.filmID, &fv.videoID, &fv.sceneNumber, &isFull, &fv.createdAt); err != nil {
			t.Fatalf("scan film_videos: %v", err)
		}
		fv.isFullFilm = isFull != 0
		out = append(out, fv)
	}
	return out
}

// TestFilmVideosSurviveFullRelinkCycle is ADR-085 action item 6. film_videos is an
// ASSERTED owner link (spec RD1) with no reconciler at all -- structurally the
// opposite of video_studios/video_people (RelinkVideoEntity, ADR-053/072), which
// are DERIVED and pruned-on-empty on every relink trigger. This asserts that
// guarantee end-to-end: a directly seeded film_videos row must survive,
// byte-for-byte, every relink trigger a real scan/enrich/decision/curation cycle
// fires -- because unlike Studio/Person there is and must never be a
// RelinkFilmVideos function wired into any of those paths.
func TestFilmVideosSurviveFullRelinkCycle(t *testing.T) {
	h, r, sqlDB, vid := filmsServer(t)
	ctx := context.Background()

	want := seedFilmVideo(t, sqlDB, vid)
	assertUnchanged := func(step string) {
		t.Helper()
		got := readFilmVideos(t, sqlDB)
		if len(got) != 1 || got[0] != want {
			t.Fatalf("film_videos mutated after %s: got %+v, want [%+v]", step, got, want)
		}
	}

	// "scan": RelinkVideoEntity is the single entry point every scanned/refreshed
	// video runs through (person_links.go), re-deriving video_studios/video_people.
	if err := h.RelinkVideoEntity(ctx, vid); err != nil {
		t.Fatalf("relink (scan): %v", err)
	}
	assertUnchanged("scan relink")

	// "enrich": seed a provider payload for both mapped entity fields, then re-run
	// the same relink a completed enrich run triggers.
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb", "ext-1", map[string][]string{
		"studio": {"Acme Films"},
		"actors": {"Someone"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}
	if err := h.RelinkVideoEntity(ctx, vid); err != nil {
		t.Fatalf("relink (enrich): %v", err)
	}
	assertUnchanged("enrich relink")

	// "decision": set a per-field source decision the way the decisions API does at
	// the repo layer, then re-relink.
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, vid, "studio", "tmdb", ""); err != nil {
		t.Fatalf("set decision: %v", err)
	}
	if err := h.RelinkVideoEntity(ctx, vid); err != nil {
		t.Fatalf("relink (decision): %v", err)
	}
	assertUnchanged("decision relink")

	// "curation": set a curation override the way the curation API does at the
	// repo layer, then re-relink.
	if err := r.SetCuration(ctx, model.EnrichEntityVideo, vid, "studio", "Curated Studio", "override"); err != nil {
		t.Fatalf("set curation: %v", err)
	}
	if err := h.RelinkVideoEntity(ctx, vid); err != nil {
		t.Fatalf("relink (curation): %v", err)
	}
	assertUnchanged("curation relink")
}
