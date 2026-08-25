package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// cascadeServer wires a real repo with films_enabled on, a studio mapping field (for
// the HOLODEX-270/271 collision gate), and a live writeQueue -- for exercising
// cascadeFilmStudio/cascadeFilmStudioHandler end to end (F57, ADR-086).
func cascadeServer(t *testing.T, token string) (*httptest.Server, *repo.Repo) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [tmdb:title, file:title]\n" +
		"  - canonical: studio\n    label: Studio\n    sources: [file:Publisher, tmdb:studio]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetFilmsEnabled(true)
	h.SetAuth(api.NewAuth(token), false)

	q := writequeue.New(r, func(context.Context, string, []writeback.FieldWrite) error { return nil }, log, 1, "")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	q.Start(ctx)
	h.SetWriteQueue(q)

	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r
}

// seedCascadeVideo creates a video with the given title/people/recordedAt, the fixed
// composite-key axis FindStudioCollision holds constant while only studio varies.
func seedCascadeVideo(t *testing.T, r *repo.Repo, path, title string, people []string, when time.Time) int64 {
	t.Helper()
	v := &model.Video{
		FilePath: path, FileSize: 1, Title: title,
		FileMtime: time.Now().UTC().Truncate(time.Second), RecordedAt: &when,
	}
	id, err := r.UpsertVideo(context.Background(), v, nil)
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
	if len(people) > 0 {
		linkPeople(t, r, id, people...)
	}
	return id
}

func cascadePost(t *testing.T, srv *httptest.Server, token string, filmID int64, source, manualValue string) (int, map[string]any) {
	t.Helper()
	body := map[string]string{"source": source, "manual_value": manualValue}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/films/"+itoa(filmID)+"/studio/cascade", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post cascade: %v", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	_ = json.Unmarshal(rawBody, &decoded)
	return resp.StatusCode, decoded
}

// TestCascadeFilmStudio_PartialCollision_BestEffort covers ADR-086 D2's best-effort
// failure posture: a collision on one video must not abort the others, unlike
// ADR-077's syncTagWriteback (which aborts on a read failure before anything is
// committed -- this cascade's per-video decision-set IS the commit).
func TestCascadeFilmStudio_PartialCollision_BestEffort(t *testing.T) {
	srv, r := cascadeServer(t, "tok")
	when := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

	filmID, err := r.CreateFilm(t.Context(), "Partial Collision", 2020)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	v1 := seedCascadeVideo(t, r, "/m/v1.mkv", "Scene One", []string{"Alice"}, when)
	v2 := seedCascadeVideo(t, r, "/m/v2.mkv", "Scene Two", []string{"Bob"}, when)
	v3 := seedCascadeVideo(t, r, "/m/v3.mkv", "Scene Three", []string{"Carol"}, when)
	for _, vid := range []int64{v1, v2, v3} {
		if _, err := r.AttachFilmVideo(t.Context(), filmID, vid, nil, false); err != nil {
			t.Fatalf("attach %d: %v", vid, err)
		}
	}

	// An unrelated video sharing v2's exact title/people/date, already on "Acme" --
	// reassigning v2 to "Acme" will collide with it.
	outside := seedCascadeVideo(t, r, "/m/outside.mkv", "Scene Two", []string{"Bob"}, when)
	if err := r.ReconcileVideoStudios(t.Context(), outside, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link outside studio: %v", err)
	}

	code, body := cascadePost(t, srv, "tok", filmID, "manual", "Acme")
	if code != http.StatusAccepted {
		t.Fatalf("cascade status = %d, body = %v", code, body)
	}
	if body["batch_id"] == "" || body["batch_id"] == nil {
		t.Errorf("batch_id = %v, want non-empty (2 videos should enqueue)", body["batch_id"])
	}
	results, ok := body["results"].([]any)
	if !ok || len(results) != 3 {
		t.Fatalf("results = %v, want 3 entries", body["results"])
	}
	status := map[int64]string{}
	for _, raw := range results {
		row := raw.(map[string]any)
		status[int64(row["video_id"].(float64))] = row["status"].(string)
	}
	if status[v1] != "enqueued" || status[v3] != "enqueued" {
		t.Errorf("status = %v, want v1 and v3 enqueued", status)
	}
	if status[v2] != "collision" {
		t.Errorf("status[v2] = %q, want collision", status[v2])
	}

	waitQueueDrained(t, r)
	done, failed := 0, 0
	if _, running, d, f, err := r.GetWritebackBatchStatus(t.Context(), body["batch_id"].(string)); err == nil {
		done, failed = d, f
		_ = running
	}
	if done != 2 || failed != 0 {
		t.Errorf("batch done=%d failed=%d, want 2/0", done, failed)
	}

	studios, err := r.StudiosForVideos(t.Context(), []int64{v1, v2, v3})
	if err != nil {
		t.Fatalf("video studios: %v", err)
	}
	if len(studios[v1]) != 1 || studios[v1][0].Name != "Acme" {
		t.Errorf("v1 studios = %v, want [Acme]", studios[v1])
	}
	if len(studios[v3]) != 1 || studios[v3][0].Name != "Acme" {
		t.Errorf("v3 studios = %v, want [Acme]", studios[v3])
	}
	if len(studios[v2]) != 0 {
		t.Errorf("v2 studios = %v, want none (collision, no decision set)", studios[v2])
	}
}

// TestCascadeFilmStudio_SameValueRedecide_NotACollision guards the HOLODEX-270 gate's
// scope: it compares a proposed value against OTHER videos, never against the video's
// own current decision, so re-running the cascade with the same studio must not
// spuriously report every video as colliding with itself.
func TestCascadeFilmStudio_SameValueRedecide_NotACollision(t *testing.T) {
	srv, r := cascadeServer(t, "tok")
	when := time.Date(2021, 3, 4, 0, 0, 0, 0, time.UTC)

	filmID, err := r.CreateFilm(t.Context(), "Redecide", 2021)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	v1 := seedCascadeVideo(t, r, "/m/rd1.mkv", "Redecide One", []string{"Alice"}, when)
	v2 := seedCascadeVideo(t, r, "/m/rd2.mkv", "Redecide Two", []string{"Bob"}, when)
	for _, vid := range []int64{v1, v2} {
		if _, err := r.AttachFilmVideo(t.Context(), filmID, vid, nil, false); err != nil {
			t.Fatalf("attach %d: %v", vid, err)
		}
		if err := r.ReconcileVideoStudios(t.Context(), vid, []string{"Acme"}, nil); err != nil {
			t.Fatalf("seed studio %d: %v", vid, err)
		}
	}

	code, body := cascadePost(t, srv, "tok", filmID, "manual", "Acme")
	if code != http.StatusAccepted {
		t.Fatalf("cascade status = %d, body = %v", code, body)
	}
	results := body["results"].([]any)
	for _, raw := range results {
		row := raw.(map[string]any)
		if row["status"] != "enqueued" {
			t.Errorf("video %v status = %v, want enqueued (same-value redecide)", row["video_id"], row["status"])
		}
	}
	if body["batch_id"] == "" {
		t.Error("batch_id is empty, want non-empty")
	}
}

// TestCascadeFilmStudio_AllCollide_EmptyBatch covers ADR-086 D2's clean-no-op case:
// when every video collides, batch_id must be "" and nothing gets enqueued, with no
// error surfaced -- the frontend uses an empty batch_id to omit the progress link.
func TestCascadeFilmStudio_AllCollide_EmptyBatch(t *testing.T) {
	srv, r := cascadeServer(t, "tok")
	when := time.Date(2022, 5, 6, 0, 0, 0, 0, time.UTC)

	filmID, err := r.CreateFilm(t.Context(), "All Collide", 2022)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	v1 := seedCascadeVideo(t, r, "/m/ac1.mkv", "Shared Scene", []string{"Alice"}, when)
	if _, err := r.AttachFilmVideo(t.Context(), filmID, v1, nil, false); err != nil {
		t.Fatalf("attach v1: %v", err)
	}
	outside := seedCascadeVideo(t, r, "/m/ac-outside.mkv", "Shared Scene", []string{"Alice"}, when)
	if err := r.ReconcileVideoStudios(t.Context(), outside, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link outside studio: %v", err)
	}

	code, body := cascadePost(t, srv, "tok", filmID, "manual", "Acme")
	if code != http.StatusAccepted {
		t.Fatalf("cascade status = %d, body = %v", code, body)
	}
	if v, ok := body["batch_id"].(string); !ok || v != "" {
		t.Errorf("batch_id = %v, want empty string", body["batch_id"])
	}
	results := body["results"].([]any)
	for _, raw := range results {
		row := raw.(map[string]any)
		if row["status"] == "enqueued" {
			t.Errorf("video %v status = enqueued, want none enqueued", row["video_id"])
		}
	}
}

// TestCascadeFilmStudio_ZeroVideos_EmptyBatch covers the other half of D2's no-op
// case: a film with no attached videos at all.
func TestCascadeFilmStudio_ZeroVideos_EmptyBatch(t *testing.T) {
	srv, r := cascadeServer(t, "tok")
	filmID, err := r.CreateFilm(t.Context(), "Empty Film", 2023)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}

	code, body := cascadePost(t, srv, "tok", filmID, "manual", "Acme")
	if code != http.StatusAccepted {
		t.Fatalf("cascade status = %d, body = %v", code, body)
	}
	if v, ok := body["batch_id"].(string); !ok || v != "" {
		t.Errorf("batch_id = %v, want empty string", body["batch_id"])
	}
	if results, ok := body["results"].([]any); ok && len(results) != 0 {
		t.Errorf("results = %v, want empty", results)
	}
}

// TestCascadeFilmStudioHandler_OwnerGated covers RD1: the cascade endpoint is an
// owner-only mutation, matching the pencil affordance being owner-view-gated in the UI.
func TestCascadeFilmStudioHandler_OwnerGated(t *testing.T) {
	srv, r := cascadeServer(t, "tok")
	filmID, err := r.CreateFilm(t.Context(), "Gated", 2024)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}

	code, _ := cascadePost(t, srv, "", filmID, "manual", "Acme")
	if code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Errorf("unauthenticated cascade status = %d, want 401 or 403", code)
	}
}
