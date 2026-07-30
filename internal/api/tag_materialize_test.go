package api_test

import (
	"context"
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

// genresServer wires a real repo + a `genres` mapping (multi:true, sourced from
// tmdb:genres, mirroring metadata-mappings.yaml.example) over one seeded video, plus
// the Handlers under test — enough to drive MaterializeVideoTags (F50 P0-9) directly
// without a live provider, the same way studioServer drives RelinkVideoStudios.
func genresServer(t *testing.T) (h *api.Handlers, r *repo.Repo, vid int64) {
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
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: genres\n    label: Genres\n    multi: true\n    sources: [tmdb:genres]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h = api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	return h, r, vid
}

// TestMaterializeVideoTags covers P0-9 (F50, ADR-075 D4): each resolved `genres`
// value materializes as a tag with source='provider:<name>', idempotently, with the
// same alias-canonicalization and deny-list silent-skip behavior AttachMaterializedTags
// already guarantees at the repo layer -- this asserts the resolver wiring on top of
// it (reading the RESOLVED field, not the raw per-provider payload).
func TestMaterializeVideoTags(t *testing.T) {
	h, r, vid := genresServer(t)
	ctx := context.Background()

	// "azure" aliases to an existing "Blue" tag (the ADR's own example) -- seed that
	// tag + alias directly at the repo layer, ahead of materialization, on a
	// throwaway video so it doesn't count toward vid's own tag set.
	seedVid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/seed.mkv", FileSize: 1, Title: "Seed",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.AttachMaterializedTags(ctx, seedVid, []repo.MaterializedTag{{Name: "Blue", Source: "manual"}}); err != nil {
		t.Fatalf("seed blue tag: %v", err)
	}
	blueID, ok, err := r.TagIDByName(ctx, "Blue")
	if err != nil || !ok {
		t.Fatalf("blue tag id: ok=%v err=%v", ok, err)
	}
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, blueID, "azure"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb", "ext-1", map[string][]string{
		"genres": {"Action", "azure"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	if err := h.MaterializeVideoTags(ctx, vid); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	v, _, err := r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(v.Tags) != 2 {
		t.Fatalf("video.Tags = %+v, want 2 (Action, Blue)", v.Tags)
	}
	for _, tg := range v.Tags {
		if tg.Name == "azure" {
			t.Errorf("materialized a literal 'azure' tag, want it canonicalized to 'Blue': %+v", v.Tags)
		}
		if tg.Source != "provider:tmdb" {
			t.Errorf("tag %q source = %q, want provider:tmdb", tg.Name, tg.Source)
		}
	}

	// Idempotent: re-materializing an already-materialized video adds no rows.
	if err := h.MaterializeVideoTags(ctx, vid); err != nil {
		t.Fatalf("re-materialize: %v", err)
	}
	v, _, err = r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video after re-materialize: %v", err)
	}
	if len(v.Tags) != 2 {
		t.Errorf("video.Tags after re-materialize = %+v, want still 2", v.Tags)
	}

	// Deny-list: a denied genre value is silently skipped, not surfaced as an error.
	if err := r.DenyTag(ctx, "Horror"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb", "ext-1", map[string][]string{
		"genres": {"Action", "azure", "Horror"},
	}); err != nil {
		t.Fatalf("update enrichment: %v", err)
	}
	if err := h.MaterializeVideoTags(ctx, vid); err != nil {
		t.Fatalf("materialize after adding denied genre: %v", err)
	}
	v, _, err = r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video after denied genre: %v", err)
	}
	if len(v.Tags) != 2 {
		t.Errorf("video.Tags after denied genre = %+v, want still 2 (denied term skipped)", v.Tags)
	}
}

// TestMaterializeVideoTags_NoMappings covers the config-less no-op: an owner running
// without metadata-mappings.yaml gets no materialization, not a nil-pointer panic.
func TestMaterializeVideoTags_NoMappings(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)

	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := h.MaterializeVideoTags(ctx, vid); err != nil {
		t.Errorf("materialize with no mappings = %v, want nil (no-op)", err)
	}
}
