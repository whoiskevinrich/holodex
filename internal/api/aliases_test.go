package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// aliasServer wires the person-alias surface with an optional owner token. A
// person ("Alice") is seeded; its id is returned. token="" leaves the gate open.
func aliasServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	if _, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/x.mkv", Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Alice"}},
	}, nil); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	pid, _, err := r.PersonIDByName(context.Background(), "Alice")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	return srv, r, pid
}

func sendTok(t *testing.T, method, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestAliasEndpointsGatedAndValidated(t *testing.T) {
	srv, _, pid := aliasServer(t, "s3cret")
	aliases := srv.URL + "/api/v1/people/" + itoa(pid) + "/aliases"

	// Gating: no token → 401, nothing created.
	if code, _ := postTok(t, aliases, "", map[string]string{"alias": "Rob"}); code != http.StatusUnauthorized {
		t.Errorf("no-token add = %d, want 401", code)
	}

	// Validation.
	if code, _ := postTok(t, aliases, "s3cret", map[string]string{"alias": "   "}); code != http.StatusBadRequest {
		t.Errorf("empty alias = %d, want 400", code)
	}
	if code, _ := postTok(t, aliases, "s3cret", map[string]string{"alias": strings.Repeat("a", 201)}); code != http.StatusBadRequest {
		t.Errorf("over-long alias = %d, want 400", code)
	}

	// Happy path: trimmed + returned.
	code, body := postTok(t, aliases, "s3cret", map[string]string{"alias": "  Rob  "})
	if code != http.StatusOK {
		t.Fatalf("add = %d, want 200", code)
	}
	list := aliasList(t, body)
	if len(list) != 1 || list[0]["alias"] != "Rob" {
		t.Fatalf("after add = %v, want one trimmed 'Rob'", body["aliases"])
	}
	aliasID := int64(list[0]["id"].(float64))

	// Idempotent re-add (case-insensitive).
	_, body = postTok(t, aliases, "s3cret", map[string]string{"alias": "rob"})
	if l := aliasList(t, body); len(l) != 1 {
		t.Errorf("idempotent re-add grew the list: %v", body["aliases"])
	}

	// Delete gating + 404 + happy path.
	del := aliases + "/" + itoa(aliasID)
	if code := sendTok(t, http.MethodDelete, del, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token delete = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodDelete, aliases+"/99999", "s3cret"); code != http.StatusNotFound {
		t.Errorf("unknown delete = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodDelete, del, "s3cret"); code != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", code)
	}
}

func TestGetPersonIncludesAliases(t *testing.T) {
	srv, _, pid := aliasServer(t, "") // open gate
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	if code, _ := postTok(t, base+"/aliases", "", map[string]string{"alias": "Al"}); code != http.StatusOK {
		t.Fatalf("add: %d", code)
	}
	code, body := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("get person = %d", code)
	}
	person, _ := body["person"].(map[string]any)
	got, _ := person["aliases"].([]any)
	if len(got) != 1 {
		t.Fatalf("person.aliases = %v, want one", person["aliases"])
	}
}

func TestAddAliasConflict409(t *testing.T) {
	srv, r, jen := aliasServer(t, "") // open gate; jen = "Alice" seeded
	// Seed a second, distinct person whose name we'll try to add as Alice's alias.
	if _, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/y.mkv", Title: "Y", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Bob"}},
	}, nil); err != nil {
		t.Fatalf("seed second person: %v", err)
	}
	bob, _, _ := r.PersonIDByName(context.Background(), "Bob")

	code, body := postTok(t, srv.URL+"/api/v1/people/"+itoa(jen)+"/aliases", "", map[string]string{"alias": "Bob"})
	if code != http.StatusConflict {
		t.Fatalf("add colliding alias = %d, want 409", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if conflict == nil || int64(conflict["id"].(float64)) != bob {
		t.Errorf("409 conflict payload = %v, want Bob (#%d)", body["conflict"], bob)
	}
}

func TestMergeEndpoint(t *testing.T) {
	srv, r, jen := aliasServer(t, "s3cret") // jen = "Alice"
	if _, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/y.mkv", Title: "Y", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Bob"}},
	}, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	bob, _, _ := r.PersonIDByName(context.Background(), "Bob")
	mergeURL := srv.URL + "/api/v1/people/" + itoa(jen) + "/merge"

	// Gated.
	if code, _ := postTok(t, mergeURL, "", map[string]int64{"from_id": bob}); code != http.StatusUnauthorized {
		t.Errorf("no-token merge = %d, want 401", code)
	}
	// Self-merge rejected.
	if code, _ := postTok(t, mergeURL, "s3cret", map[string]int64{"from_id": jen}); code != http.StatusBadRequest {
		t.Errorf("self-merge = %d, want 400", code)
	}
	// Happy path: Bob folds into Alice.
	code, body := postTok(t, mergeURL, "s3cret", map[string]int64{"from_id": bob})
	if code != http.StatusOK {
		t.Fatalf("merge = %d, want 200", code)
	}
	person, _ := body["person"].(map[string]any)
	aliases, _ := person["aliases"].([]any)
	if len(aliases) != 1 {
		t.Errorf("merged person aliases = %v, want [Bob]", person["aliases"])
	}
	// Bob is gone.
	if code, _ := getJSON(t, srv.URL+"/api/v1/people/"+itoa(bob)); code != http.StatusNotFound {
		t.Errorf("merged person GET = %d, want 404", code)
	}
}

// TestMergeEndpoint_PropagatesWritebackToAffectedVideos is F48.8a end to end
// through the HTTP handler: merging Bob into Jenny enqueues one writeback job
// per video Bob was linked to, and a video where Bob co-starred with someone
// else gets that other person's name preserved alongside Jenny's — not
// overwritten with just the merge survivor's name.
func TestMergeEndpoint_PropagatesWritebackToAffectedVideos(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)

	var mu sync.Mutex
	written := map[string][]string{} // file path -> Artist tag values written
	q := writequeue.New(r, func(_ context.Context, path string, fields []writeback.FieldWrite) error {
		mu.Lock()
		defer mu.Unlock()
		for _, f := range fields {
			if f.TagName == "Artist" {
				written[path] = f.Values
			}
		}
		return nil
	}, log, 1, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)
	h.SetWriteQueue(q)
	h.SetAuth(api.NewAuth(""), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	seed := func(path, title string, people ...string) {
		v := &model.Video{
			FilePath: path, Title: title, Duration: 60, Width: 1920, Height: 1080,
			Container: "Matroska", FileMtime: time.Now().UTC().Truncate(time.Second),
		}
		for _, p := range people {
			v.People = append(v.People, model.Person{Name: p})
		}
		if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}
	seed("/m/solo.mkv", "Solo", "Bob")
	seed("/m/together.mkv", "Together", "Bob", "Carol")
	seed("/m/jenny.mkv", "Jenny", "Jenny")

	bob, _, _ := r.PersonIDByName(ctx, "Bob")
	jenny, _, _ := r.PersonIDByName(ctx, "Jenny")

	code, _ := postTok(t, srv.URL+"/api/v1/people/"+itoa(jenny)+"/merge", "", map[string]int64{"from_id": bob})
	if code != http.StatusOK {
		t.Fatalf("merge = %d, want 200", code)
	}

	// Two videos were linked to Bob (solo.mkv, together.mkv); jenny.mkv never was.
	if depth, err := q.Depth(ctx); err != nil || depth != 2 {
		t.Fatalf("queue depth = %d err=%v, want 2 (one writeback job per affected video)", depth, err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := r.PendingWritebackCount(ctx); n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writeback queue did not drain")
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if got := written["/m/solo.mkv"]; len(got) != 1 || got[0] != "Jenny" {
		t.Errorf("solo.mkv written = %v, want [Jenny]", got)
	}
	if got := written["/m/together.mkv"]; len(got) != 2 || got[0] != "Carol" || got[1] != "Jenny" {
		t.Errorf("together.mkv written = %v, want [Carol Jenny] (Carol preserved, Bob → Jenny)", got)
	}
	if _, wrote := written["/m/jenny.mkv"]; wrote {
		t.Error("jenny.mkv was never linked to Bob, should not have been written")
	}
}

func aliasList(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, _ := body["aliases"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, a := range raw {
		m, _ := a.(map[string]any)
		out = append(out, m)
	}
	return out
}
