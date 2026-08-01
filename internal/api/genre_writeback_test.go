package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
)

// genreWritebackServer wires a real repo + a `genres` mapping (mirroring
// genresServer, tag_materialize_test.go) plus a capturing WriteBatchFunc, so
// tests can drive GenreWritebackValues directly or the actual
// POST /media/{id}/writeback path (F50 S6, ADR-075 RD9) end to end.
func genreWritebackServer(t *testing.T) (h *api.Handlers, srv *httptest.Server, r *repo.Repo, vid int64, written *[]writeback.FieldWrite) {
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
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A", Container: "Matroska",
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
	written = &[]writeback.FieldWrite{}
	h.SetWriteback(func(_ context.Context, _ string, fields []writeback.FieldWrite) error {
		*written = append(*written, fields...)
		return nil
	})
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return h, srv, r, vid, written
}

// TestGenreWritebackValues covers P0-10 (F50, ADR-075 RD9): the value union
// fed to a "genres" writeback is the video's attached tags (ancestor-expanded)
// plus the raw resolved genres union, with the raw side deny-list-filtered,
// deduplicated case-insensitively.
func TestGenreWritebackValues(t *testing.T) {
	h, _, r, vid, _ := genreWritebackServer(t)
	ctx := context.Background()

	// Hierarchy: German Shepherd (child of Dog, child of Animal), attached to vid.
	if _, err := r.AttachTagToVideo(ctx, vid, "Animal"); err != nil {
		t.Fatalf("attach Animal: %v", err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "Dog"); err != nil {
		t.Fatalf("attach Dog: %v", err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "German Shepherd"); err != nil {
		t.Fatalf("attach German Shepherd: %v", err)
	}
	dogID, ok, err := r.TagIDByName(ctx, "Dog")
	if err != nil || !ok {
		t.Fatalf("dog id: ok=%v err=%v", ok, err)
	}
	animalID, ok, err := r.TagIDByName(ctx, "Animal")
	if err != nil || !ok {
		t.Fatalf("animal id: ok=%v err=%v", ok, err)
	}
	shepherdID, ok, err := r.TagIDByName(ctx, "German Shepherd")
	if err != nil || !ok {
		t.Fatalf("shepherd id: ok=%v err=%v", ok, err)
	}
	if _, err := r.SetTagParent(ctx, dogID, &animalID); err != nil {
		t.Fatalf("set dog parent: %v", err)
	}
	if _, err := r.SetTagParent(ctx, shepherdID, &dogID); err != nil {
		t.Fatalf("set shepherd parent: %v", err)
	}

	// "TV Movie" is denied and present in the raw resolved genres union, but
	// never attached as a Tag — must not appear in the writeback values.
	if _, err := r.DenyTag(ctx, "TV Movie"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb", "ext-1", map[string][]string{
		"genres": {"Action", "TV Movie"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	values, err := h.GenreWritebackValues(ctx, vid)
	if err != nil {
		t.Fatalf("genre writeback values: %v", err)
	}
	got := make(map[string]bool, len(values))
	for _, v := range values {
		got[strings.ToLower(v)] = true
	}
	for _, want := range []string{"animal", "dog", "german shepherd", "action"} {
		if !got[want] {
			t.Errorf("values = %v, missing %q", values, want)
		}
	}
	if got["tv movie"] {
		t.Errorf("values = %v, denied term must not appear", values)
	}
	if len(values) != 4 {
		t.Errorf("values = %v, want exactly 4 (Animal, Dog, German Shepherd, Action)", values)
	}
}

// TestGenreWritebackValues_NoMappings covers the config-less no-op — same
// posture as MaterializeVideoTags_NoMappings (S5): tags still union in, the
// raw-resolved side is simply empty.
func TestGenreWritebackValues_NoMappings(t *testing.T) {
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
	if _, err := r.AttachTagToVideo(ctx, vid, "Action"); err != nil {
		t.Fatalf("attach tag: %v", err)
	}

	values, err := h.GenreWritebackValues(ctx, vid)
	if err != nil {
		t.Fatalf("genre writeback values with no mappings: %v", err)
	}
	if len(values) != 1 || values[0] != "action" {
		t.Errorf("values = %v, want just the attached tag [action]", values)
	}
}

// TestGenreWritebackEndpoint covers the actual wiring (F50 S6): a "genres"
// field submitted to POST /media/{id}/writeback is overridden with the
// computed union regardless of what values the client sent — the file's
// Genre tag deterministically reflects DB state, not an ad hoc client edit.
func TestGenreWritebackEndpoint(t *testing.T) {
	_, srv, r, vid, written := genreWritebackServer(t)
	ctx := context.Background()

	if _, err := r.AttachTagToVideo(ctx, vid, "Comedy"); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, vid, "tmdb", "ext-1", map[string][]string{
		"genres": {"Action"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"fields": []map[string]any{
			{"field": "genres", "values": []string{"Whatever The Client Typed"}, "source": "tmdb:genres"},
		},
	})
	resp, err := http.Post(srv.URL+"/api/v1/media/"+itoa(vid)+"/writeback", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST writeback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("writeback status = %d, want 204", resp.StatusCode)
	}

	if len(*written) != 1 {
		t.Fatalf("written fields = %+v, want exactly 1", *written)
	}
	got := make(map[string]bool, len((*written)[0].Values))
	for _, v := range (*written)[0].Values {
		got[v] = true
	}
	if got["Whatever The Client Typed"] {
		t.Errorf("client-submitted value leaked through unoverridden: %+v", (*written)[0].Values)
	}
	// "Comedy" is a tag entity, so it's lower-cased in storage/writeback; "Action" is
	// a raw resolved genre from provider enrichment (not a tag), so its casing is untouched.
	if !got["comedy"] || !got["Action"] {
		t.Errorf("written genres = %+v, want the computed union (comedy, Action)", (*written)[0].Values)
	}
}
