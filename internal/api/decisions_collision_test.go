package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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
