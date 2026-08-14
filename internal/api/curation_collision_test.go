package api_test

import (
	"context"
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
)

// TestCurationAPI_PeopleCollision exercises the HOLODEX-272 people leg of the
// HOLODEX-270/271 composite-key collision gate: attaching a person via curation
// add that would produce a {title, people, date, studio} match against another
// active video is rejected with 409 + the colliding video, and never persists —
// unless the caller sets override.
func TestCurationAPI_PeopleCollision(t *testing.T) {
	srv, r, id := peopleDecisionServer(t)
	ctx := context.Background()

	otherID, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/b.mkv", FileSize: 1, Title: "File Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, otherID, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio (other): %v", err)
	}

	base := srv.URL + "/api/v1/media/" + itoa(otherID) + "/curation"
	baseA := srv.URL + "/api/v1/media/" + itoa(id) + "/curation"

	// Seed people through curation adds (the only path that actually populates
	// video_people — it's derived from resolved actors/director, not writable
	// directly), in an order that doesn't collide along the way: a reaches
	// {Alice, Bob} while b sits at {Alice} only.
	for _, add := range []struct {
		url, value string
	}{{baseA, "Alice"}, {baseA, "Bob"}, {base, "Alice"}} {
		if code := sendDecision(t, http.MethodPost, add.url, "", map[string]any{"field": "actors", "value": add.value, "action": "add"}); code != http.StatusNoContent {
			t.Fatalf("seed add %q: want 204, got %d", add.value, code)
		}
	}

	// Attaching Bob to b collides with a.
	code, body := rawRequest(t, http.MethodPost, base, map[string]any{"field": "actors", "value": "Bob", "action": "add"})
	if code != http.StatusConflict {
		t.Fatalf("collision: want 409, got %d", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if conflict == nil || int64(conflict["id"].(float64)) != id {
		t.Fatalf("409 conflict payload = %v, want video #%d", body["conflict"], id)
	}

	// The rejected attach must not have persisted.
	people, err := r.PeopleForVideos(ctx, []int64{otherID})
	if err != nil {
		t.Fatalf("people for video: %v", err)
	}
	if len(people[otherID]) != 1 || people[otherID][0].Name != "Alice" {
		t.Errorf("collision must not persist the attach, got %v", people[otherID])
	}

	// Override bypasses the check and commits, relinking video_people.
	if code := sendDecision(t, http.MethodPost, base, "", map[string]any{
		"field": "actors", "value": "Bob", "action": "add", "override": true,
	}); code != http.StatusNoContent {
		t.Fatalf("override: want 204, got %d", code)
	}
	people, err = r.PeopleForVideos(ctx, []int64{otherID})
	if err != nil {
		t.Fatalf("people for video: %v", err)
	}
	names := map[string]bool{}
	for _, p := range people[otherID] {
		names[p.Name] = true
	}
	if !names["Alice"] || !names["Bob"] {
		t.Errorf("override should persist the attach, got %v", people[otherID])
	}
}

// TestCurationAPI_PeopleCollision_Suppress confirms the same gate fires on a
// detach (action=suppress), not just an attach — removing a person changes
// video_people's composite-key dimension exactly as adding one does.
func TestCurationAPI_PeopleCollision_Suppress(t *testing.T) {
	srv, r, id := peopleDecisionServer(t)
	ctx := context.Background()

	otherID, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/b.mkv", FileSize: 1, Title: "File Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, otherID, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio (other): %v", err)
	}

	base := srv.URL + "/api/v1/media/" + itoa(otherID) + "/curation"
	baseA := srv.URL + "/api/v1/media/" + itoa(id) + "/curation"

	// Seed people through curation adds, in an order that doesn't collide along
	// the way: a reaches {Alice} while b reaches {Alice, Bob} (added Bob first so
	// the intermediate {Bob} state can't match a's eventual {Alice}).
	for _, add := range []struct {
		url, value string
	}{{baseA, "Alice"}, {base, "Bob"}, {base, "Alice"}} {
		if code := sendDecision(t, http.MethodPost, add.url, "", map[string]any{"field": "actors", "value": add.value, "action": "add"}); code != http.StatusNoContent {
			t.Fatalf("seed add %q: want 204, got %d", add.value, code)
		}
	}

	// Removing Bob from b collides with a.
	code, body := rawRequest(t, http.MethodPost, base, map[string]any{"field": "actors", "value": "Bob", "action": "suppress"})
	if code != http.StatusConflict {
		t.Fatalf("collision: want 409, got %d", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if conflict == nil || int64(conflict["id"].(float64)) != id {
		t.Fatalf("409 conflict payload = %v, want video #%d", body["conflict"], id)
	}

	// The rejected suppress must not have persisted.
	people, err := r.PeopleForVideos(ctx, []int64{otherID})
	if err != nil {
		t.Fatalf("people for video: %v", err)
	}
	if len(people[otherID]) != 2 {
		t.Errorf("collision must not persist the suppress, got %v", people[otherID])
	}

	// Override bypasses the check and commits.
	if code := sendDecision(t, http.MethodPost, base, "", map[string]any{
		"field": "actors", "value": "Bob", "action": "suppress", "override": true,
	}); code != http.StatusNoContent {
		t.Fatalf("override: want 204, got %d", code)
	}
	people, err = r.PeopleForVideos(ctx, []int64{otherID})
	if err != nil {
		t.Fatalf("people for video: %v", err)
	}
	if len(people[otherID]) != 1 || people[otherID][0].Name != "Alice" {
		t.Errorf("override should persist the suppress, got %v", people[otherID])
	}
}

// TestCurationAPI_NonPersonFieldSkipsCollisionGate confirms the gate is
// registry-driven (ADR-072 §3), not a hardcoded field list: a curation add on a
// non-person-typed field commits normally, unaffected by the People gate.
func TestCurationAPI_NonPersonFieldSkipsCollisionGate(t *testing.T) {
	srv, r, _ := peopleDecisionServer(t)
	ctx := context.Background()

	otherID, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/b.mkv", FileSize: 1, Title: "Other Title",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}

	base := srv.URL + "/api/v1/media/" + itoa(otherID) + "/curation"
	if code := sendDecision(t, http.MethodPost, base, "", map[string]any{"field": "genres", "value": "Action", "action": "add"}); code != http.StatusNoContent {
		t.Fatalf("non-person field: want 204, got %d", code)
	}
}

// TestCurationAPI_PersonFieldNotMapped confirms setCuration's People collision
// gate and relinkPeopleWithContext fast path only engage for a person-typed field
// that's actually mapped in metadata-mappings.yaml (HOLODEX-274 review fix) — an
// unmapped field (director, in this fixture, which maps only actors) must fall
// back to the guarded relinkIfEntity path and leave video_people untouched, same
// as before HOLODEX-274 (HOLODEX-256's "unconfigured means no opinion" invariant).
func TestCurationAPI_PersonFieldNotMapped(t *testing.T) {
	srv, r, id := peopleDecisionServer(t)
	ctx := context.Background()

	base := srv.URL + "/api/v1/media/" + itoa(id) + "/curation"
	if code := sendDecision(t, http.MethodPost, base, "", map[string]any{"field": "director", "value": "Carol", "action": "add"}); code != http.StatusNoContent {
		t.Fatalf("unmapped field add: want 204, got %d", code)
	}

	people, err := r.PeopleForVideos(ctx, []int64{id})
	if err != nil {
		t.Fatalf("people for video: %v", err)
	}
	if len(people[id]) != 0 {
		t.Errorf("director isn't mapped in this fixture; curation add must not link it, got %v", people[id])
	}
}

// peopleDecisionServer is decisionServer's sibling with an "actors" field added to
// the mapping (multi-valued, matching real config) so the HOLODEX-272 people
// collision gate — and the relink that follows a commit — can be exercised.
func peopleDecisionServer(t *testing.T) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	return peopleDecisionServerWithFields(t, "fields:\n"+
		"  - canonical: title\n    label: Title\n    sources: [tmdb:title, file:title]\n"+
		"  - canonical: actors\n    label: Actors\n    multi: true\n    sources: [tmdb:actors, file:Artist]\n"+
		"  - canonical: genres\n    label: Genres\n    merge: true\n    sources: [tmdb:genres, file:genres]\n")
}

// peopleDecisionServerWithFields is peopleDecisionServer's parameterized core —
// fieldsYAML is the full metadata-mappings.yaml body, so a test that needs a
// different set of mapped fields (e.g. curation_concurrency_test.go's
// actors+director fixture) can reuse this scaffolding instead of forking it.
func peopleDecisionServerWithFields(t *testing.T, fieldsYAML string) (*httptest.Server, *repo.Repo, int64) {
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
	if err := os.WriteFile(mpath, []byte(fieldsYAML), 0o644); err != nil {
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
