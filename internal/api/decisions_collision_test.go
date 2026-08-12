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
)

// TestDecisionAPI_TitleCollision exercises the HOLODEX-270 composite-key collision gate:
// a manual title edit that would produce a {title, people, date, studio} match against
// another active video is rejected with 409 + the colliding video, and never persists —
// unless the caller sets override, which bypasses the check and commits normally.
func TestDecisionAPI_TitleCollision(t *testing.T) {
	srv, r, id := decisionServer(t, "")
	ctx := context.Background()

	// Give the seeded video ("File Title") and a second video the exact same
	// composite key so renaming the second into a collision is unambiguous.
	if err := r.ReconcileVideoPeople(ctx, id, []repo.PersonRoleName{{Name: "Alice", Role: "actor"}}, nil); err != nil {
		t.Fatalf("link people: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}

	// decisionServer's seeded video has no RecordedAt (nil) — match that here so the
	// composite key aligns on every axis except title.
	otherID, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/b.mkv", FileSize: 1, Title: "Other Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, otherID, []repo.PersonRoleName{{Name: "Alice", Role: "actor"}}, nil); err != nil {
		t.Fatalf("link people (other): %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, otherID, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio (other): %v", err)
	}

	base := srv.URL + "/api/v1/media/" + itoa(otherID) + "/fields/title/decision"

	// Renaming the second video into a collision with the first is blocked.
	code, body := putDecisionRaw(t, base, map[string]any{"source": "manual", "manual_value": "  FILE title  "})
	if code != http.StatusConflict {
		t.Fatalf("collision: want 409, got %d", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if conflict == nil || int64(conflict["id"].(float64)) != id {
		t.Fatalf("409 conflict payload = %v, want video #%d", body["conflict"], id)
	}
	if conflict["title"] != "File Title" {
		t.Errorf("conflict title = %v", conflict["title"])
	}

	// The rejected edit must not have persisted.
	f := resolvedField(t, srv, otherID, "title")
	if f["values"].([]any)[0] == "FILE title" {
		t.Error("collision must not persist the edit")
	}

	// Override bypasses the check and commits.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]any{
		"source": "manual", "manual_value": "FILE title", "override": true,
	}); code != 204 {
		t.Fatalf("override: want 204, got %d", code)
	}
	f = resolvedField(t, srv, otherID, "title")
	if f["values"].([]any)[0] != "FILE title" {
		t.Errorf("override should persist, got %v", f["values"])
	}
}

// TestDecisionAPI_StudioCollision exercises the HOLODEX-271 studio leg of the same gate:
// reassigning a video's studio to a name that would produce a {title, people, date,
// studio} match against another active video is rejected with 409 + the colliding
// video, and never persists — unless the caller sets override. Unlike Title, this gate
// fires on any manual studio pick, not just a free-text edit, since a picker chip/search
// selection changes the composite key exactly the same way (internal/api/decisions.go's
// studioCollision comment).
func TestDecisionAPI_StudioCollision(t *testing.T) {
	srv, r, id := studioDecisionServer(t)
	ctx := context.Background()

	// Give the seeded video ("File Title") and a second video the same title/date/
	// people, but different studios, so reassigning the second's studio to the
	// first's is the only axis that changes.
	if err := r.ReconcileVideoPeople(ctx, id, []repo.PersonRoleName{{Name: "Alice", Role: "actor"}}, nil); err != nil {
		t.Fatalf("link people: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}

	otherID, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/b.mkv", FileSize: 1, Title: "File Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, otherID, []repo.PersonRoleName{{Name: "Alice", Role: "actor"}}, nil); err != nil {
		t.Fatalf("link people (other): %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, otherID, []string{"Other"}, nil); err != nil {
		t.Fatalf("link studio (other): %v", err)
	}

	base := srv.URL + "/api/v1/media/" + itoa(otherID) + "/fields/studio/decision"

	// Reassigning the second video's studio into a collision with the first is blocked.
	code, body := putDecisionRaw(t, base, map[string]any{"source": "manual", "manual_value": "  ACME  "})
	if code != http.StatusConflict {
		t.Fatalf("collision: want 409, got %d", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if conflict == nil || int64(conflict["id"].(float64)) != id {
		t.Fatalf("409 conflict payload = %v, want video #%d", body["conflict"], id)
	}
	conflictStudios, _ := conflict["studios"].([]any)
	if len(conflictStudios) != 1 || conflictStudios[0] != "Acme" {
		t.Errorf("conflict studios = %v", conflict["studios"])
	}

	// The rejected edit must not have persisted.
	studios, err := r.StudiosForVideos(ctx, []int64{otherID})
	if err != nil {
		t.Fatalf("studios for video: %v", err)
	}
	if len(studios[otherID]) != 1 || studios[otherID][0].Name != "Other" {
		t.Errorf("collision must not persist the edit, got %v", studios[otherID])
	}

	// Override bypasses the check and commits. resolveOrCreateByName matches the
	// existing "Acme" studio case-insensitively rather than creating a duplicate
	// "ACME" row, so the persisted name keeps the existing casing.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]any{
		"source": "manual", "manual_value": "ACME", "override": true,
	}); code != 204 {
		t.Fatalf("override: want 204, got %d", code)
	}
	studios, err = r.StudiosForVideos(ctx, []int64{otherID})
	if err != nil {
		t.Fatalf("studios for video: %v", err)
	}
	if len(studios[otherID]) != 1 || studios[otherID][0].Name != "Acme" {
		t.Errorf("override should persist, got %v", studios[otherID])
	}
}

// studioDecisionServer is decisionServer's sibling with a studio field added to the
// mapping (decisionServer's own mapping has no studio field, since none of its other
// callers need one), so the HOLODEX-271 studio collision gate can be exercised.
func studioDecisionServer(t *testing.T) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "File Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

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
	h.SetAuth(api.NewAuth(""), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, id
}

// putDecisionRaw is sendDecision's sibling for the 409 case, returning the decoded body.
func putDecisionRaw(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, url, strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return resp.StatusCode, decoded
}
