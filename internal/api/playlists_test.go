package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// seedPlaylistVideos indexes n videos with distinct titles/durations/tags so every
// browse sort produces a distinct order, and returns their ids in insertion order.
func seedPlaylistVideos(t *testing.T, r *repo.Repo, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	titles := []string{"Delta", "Alpha", "Echo", "Bravo", "Charlie", "Foxtrot"}
	for i := 0; i < n; i++ {
		v := &model.Video{
			FilePath: "/m/pl" + itoa(int64(i)) + ".mkv", FileSize: 100, Title: titles[i%len(titles)],
			Duration: 60 + 30*i, Width: 1920, Height: 1080,
			FileMtime: time.Now().UTC().Truncate(time.Second),
			Tags:      []model.Tag{{Name: "amv"}},
		}
		if i%2 == 0 {
			v.Tags = append(v.Tags, model.Tag{Name: "even"})
		}
		id, err := r.UpsertVideo(context.Background(), v, nil)
		if err != nil {
			t.Fatalf("seed video %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	return ids
}

func itemIDs(items any) []int64 {
	var out []int64
	for _, it := range items.([]any) {
		out = append(out, int64(it.(map[string]any)["id"].(float64)))
	}
	return out
}

func sameOrder(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPlaylistEndpoints covers F69 S1 (ADR-104): gating, validation, the ref
// handle, CRUD, idempotent membership, the visitor visibility gate (private =
// 404, never 403), the trash seam, delete's cascade, and /capabilities'
// public_playlists count.
func TestPlaylistEndpoints(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	vids := seedPlaylistVideos(t, r, 4)
	base := srv.URL + "/api/v1"
	pls := base + "/playlists"

	// Gating: writes are owner-only; reads are open.
	if code, _ := postTok(t, pls, "", map[string]any{"name": "x"}); code != http.StatusUnauthorized {
		t.Errorf("no-token create = %d, want 401", code)
	}
	if code, body := getJSONTok(t, pls, ""); code != http.StatusOK || len(body["items"].([]any)) != 0 {
		t.Errorf("visitor empty list = %d %v, want 200 []", code, body["items"])
	}

	// Validation.
	for name, body := range map[string]map[string]any{
		"empty name": {"name": "  "},
		"bad sort":   {"name": "x", "sort": "sideways"},
		"bad vis":    {"name": "x", "visibility": "friends"},
	} {
		if code, _ := postTok(t, pls, "s3cret", body); code != http.StatusBadRequest {
			t.Errorf("%s create = %d, want 400", name, code)
		}
	}

	// Create empty (defaults: added_desc, private) and read the ref.
	code, body := postTok(t, pls, "s3cret", map[string]any{"name": " Saturday shorts "})
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, body)
	}
	p := body["playlist"].(map[string]any)
	pid := int64(p["id"].(float64))
	if p["name"] != "Saturday shorts" || p["sort"] != "added_desc" || p["visibility"] != "private" ||
		p["item_count"].(float64) != 0 || p["ref"] != "playlist:"+itoa(pid) {
		t.Errorf("created playlist = %v", p)
	}
	plURL := pls + "/" + itoa(pid)

	// The ref handle is accepted where an id is; a wrong kind is a 400.
	if code := sendTok(t, http.MethodGet, pls+"/playlist:"+itoa(pid), "s3cret"); code != http.StatusOK {
		t.Errorf("GET by ref = %d, want 200", code)
	}
	if code := sendTok(t, http.MethodGet, pls+"/video:"+itoa(pid), "s3cret"); code != http.StatusBadRequest {
		t.Errorf("GET by wrong-kind ref = %d, want 400", code)
	}

	// Visitor + private → 404, identical to an unknown id; owner → 200.
	if code := sendTok(t, http.MethodGet, plURL, ""); code != http.StatusNotFound {
		t.Errorf("visitor private GET = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodGet, pls+"/999", ""); code != http.StatusNotFound {
		t.Errorf("visitor unknown GET = %d, want 404", code)
	}
	if code, _ := getJSONTok(t, plURL, "s3cret"); code != http.StatusOK {
		t.Errorf("owner private GET = %d, want 200", code)
	}

	// Membership: add is idempotent; remove 404s when not a member; unknown video 404s.
	vURL := func(v int64) string { return plURL + "/videos/" + itoa(v) }
	for i := 0; i < 2; i++ {
		if code, body := reqTokBody(t, http.MethodPut, vURL(vids[0]), "s3cret", nil); code != http.StatusOK ||
			body["playlist"].(map[string]any)["item_count"].(float64) != 1 {
			t.Errorf("add #%d = %d %v, want 200 item_count 1", i, code, body)
		}
	}
	if code := sendTok(t, http.MethodPut, vURL(999), "s3cret"); code != http.StatusNotFound {
		t.Errorf("add unknown video = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodPut, vURL(vids[0]), ""); code != http.StatusUnauthorized {
		t.Errorf("visitor add = %d, want 401", code)
	}
	if code := sendTok(t, http.MethodDelete, vURL(vids[1]), "s3cret"); code != http.StatusNotFound {
		t.Errorf("remove non-member = %d, want 404", code)
	}
	if code := sendTok(t, http.MethodPut, vURL(vids[1]), "s3cret"); code != http.StatusOK {
		t.Errorf("add second = %d", code)
	}

	// Manual order = insertion order; items carry the browse tile shape (ref, title).
	_, body = getJSONTok(t, plURL, "s3cret")
	if got := itemIDs(body["items"]); !sameOrder(got, []int64{vids[1], vids[0]}) {
		t.Errorf("added_desc order = %v, want newest first %v", got, []int64{vids[1], vids[0]})
	}
	if code, body := reqTokBody(t, http.MethodPatch, plURL, "s3cret", map[string]any{"sort": "manual"}); code != http.StatusOK ||
		body["playlist"].(map[string]any)["sort"] != "manual" {
		t.Fatalf("patch sort = %d %v", code, body)
	}
	_, body = getJSONTok(t, plURL, "s3cret")
	if got := itemIDs(body["items"]); !sameOrder(got, []int64{vids[0], vids[1]}) {
		t.Errorf("manual order = %v, want insertion %v", got, []int64{vids[0], vids[1]})
	}
	if body["items"].([]any)[0].(map[string]any)["ref"] != "video:"+itoa(vids[0]) {
		t.Errorf("item lacks the video ref: %v", body["items"].([]any)[0])
	}

	// The media detail carries the video's playlists, visibility-filtered (spec P0-7).
	if _, detail := getJSONTok(t, base+"/media/"+itoa(vids[0]), "s3cret"); len(detail["playlists"].([]any)) != 1 {
		t.Errorf("owner detail playlists = %v, want 1", detail["playlists"])
	}
	if _, detail := getJSONTok(t, base+"/media/"+itoa(vids[0]), ""); len(detail["playlists"].([]any)) != 0 {
		t.Errorf("visitor detail playlists (private) = %v, want 0", detail["playlists"])
	}

	// Trash seam: a trashed member vanishes from items and item_count, returns on restore.
	if err := r.SoftDelete(context.Background(), vids[0]); err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, plURL, "s3cret")
	if got := itemIDs(body["items"]); !sameOrder(got, []int64{vids[1]}) ||
		body["playlist"].(map[string]any)["item_count"].(float64) != 1 {
		t.Errorf("after trash: items %v count %v", got, body["playlist"].(map[string]any)["item_count"])
	}
	if err := r.Restore(context.Background(), vids[0]); err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, plURL, "s3cret")
	if got := itemIDs(body["items"]); len(got) != 2 {
		t.Errorf("after restore: items %v, want 2", got)
	}

	// Visibility: flip to public → visitor lists and opens it; capabilities counts it.
	if code, capsBody := getJSONTok(t, base+"/capabilities", ""); code != http.StatusOK || capsBody["public_playlists"].(float64) != 0 {
		t.Errorf("capabilities before = %d %v", code, capsBody["public_playlists"])
	}
	if code, _ := reqTokBody(t, http.MethodPatch, plURL, "s3cret", map[string]any{"visibility": "public"}); code != http.StatusOK {
		t.Fatalf("patch visibility = %d", code)
	}
	if code, body := getJSONTok(t, pls, ""); code != http.StatusOK || len(body["items"].([]any)) != 1 {
		t.Errorf("visitor list after public = %d %v", code, body["items"])
	}
	if code := sendTok(t, http.MethodGet, plURL, ""); code != http.StatusOK {
		t.Errorf("visitor public GET = %d, want 200", code)
	}
	if _, capsBody := getJSONTok(t, base+"/capabilities", ""); capsBody["public_playlists"].(float64) != 1 {
		t.Errorf("capabilities after = %v, want 1", capsBody["public_playlists"])
	}
	// Visitors never see owner controls' data: PATCH/DELETE are 401.
	if code, _ := reqTokBody(t, http.MethodPatch, plURL, "", map[string]any{"name": "x"}); code != http.StatusUnauthorized {
		t.Errorf("visitor patch = %d, want 401", code)
	}

	// Delete cascades membership and leaves the videos alone.
	if code := sendTok(t, http.MethodDelete, plURL, "s3cret"); code != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", code)
	}
	if code := sendTok(t, http.MethodGet, plURL, "s3cret"); code != http.StatusNotFound {
		t.Errorf("GET after delete = %d, want 404", code)
	}
	if code, body := getJSONTok(t, base+"/media", "s3cret"); code != http.StatusOK || body["total"].(float64) != 4 {
		t.Errorf("videos after playlist delete = %d total %v, want 4", code, body["total"])
	}
}

// TestPlaylistSnapshot covers the from_query producer (ADR-104 D3): membership and
// order equal the browse list for the same filter across sorts, the client's
// paging is ignored, and a random filter snapshots as 'manual' in the seeded
// order on screen.
func TestPlaylistSnapshot(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	seedPlaylistVideos(t, r, 6)
	base := srv.URL + "/api/v1"
	evenTag := tagID(t, r, "even")

	for _, sort := range []string{"added_desc", "title_asc", "duration_desc", "resolution_asc"} {
		q := "tag=" + itoa(evenTag) + "&sort=" + sort + "&limit=1&offset=1"
		_, browse := getJSONTok(t, base+"/media?tag="+itoa(evenTag)+"&sort="+sort+"&limit=50", "s3cret")
		code, body := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": sort, "from_query": q})
		if code != http.StatusCreated {
			t.Fatalf("%s: create = %d %v", sort, code, body)
		}
		p := body["playlist"].(map[string]any)
		if p["sort"] != sort || p["item_count"].(float64) != browse["total"].(float64) {
			t.Errorf("%s: playlist %v vs browse total %v", sort, p, browse["total"])
		}
		_, got := getJSONTok(t, base+"/playlists/"+itoa(int64(p["id"].(float64))), "s3cret")
		if !sameOrder(itemIDs(got["items"]), itemIDs(browse["items"])) {
			t.Errorf("%s: order %v, browse %v", sort, itemIDs(got["items"]), itemIDs(browse["items"]))
		}
	}

	// random → stored as manual, in the order the seed produced on browse.
	_, browse := getJSONTok(t, base+"/media?sort=random&seed=42&limit=50", "s3cret")
	code, body := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "shuffle", "from_query": "?sort=random&seed=42"})
	if code != http.StatusCreated {
		t.Fatalf("random create = %d %v", code, body)
	}
	p := body["playlist"].(map[string]any)
	if p["sort"] != "manual" {
		t.Errorf("random snapshot sort = %v, want manual", p["sort"])
	}
	_, got := getJSONTok(t, base+"/playlists/"+itoa(int64(p["id"].(float64))), "s3cret")
	if !sameOrder(itemIDs(got["items"]), itemIDs(browse["items"])) {
		t.Errorf("random snapshot order %v, browse %v", itemIDs(got["items"]), itemIDs(browse["items"]))
	}

	// No filter at all = the whole browse-visible library.
	code, body = postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "all", "from_query": ""})
	if code != http.StatusCreated || body["playlist"].(map[string]any)["item_count"].(float64) != 6 {
		t.Errorf("whole-library snapshot = %d %v, want 6", code, body)
	}
	// An explicit body sort wins over the filter's.
	code, body = postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "override", "sort": "title_desc", "from_query": "sort=added_asc"})
	if code != http.StatusCreated || body["playlist"].(map[string]any)["sort"] != "title_desc" {
		t.Errorf("explicit sort = %d %v, want title_desc", code, body)
	}

	// A 'random' playlist read without a seed gets one minted and echoed; the SPA
	// carries it back as a JSON number, so it must survive a float64 round trip
	// (≤ 2^53) and reproduce the same order when sent back.
	code, body = postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "shuffled", "sort": "random", "from_query": ""})
	if code != http.StatusCreated {
		t.Fatalf("random playlist create = %d %v", code, body)
	}
	rid := itoa(int64(body["playlist"].(map[string]any)["id"].(float64)))
	_, first := getJSONTok(t, base+"/playlists/"+rid, "s3cret")
	seed, ok := first["seed"].(float64)
	if !ok || seed <= 0 || seed > 1<<53-1 {
		t.Fatalf("echoed seed = %v, want a positive JSON-safe integer", first["seed"])
	}
	_, again := getJSONTok(t, base+"/playlists/"+rid+"?seed="+itoa(int64(seed)), "s3cret")
	if !sameOrder(itemIDs(again["items"]), itemIDs(first["items"])) || again["seed"].(float64) != seed {
		t.Errorf("seed %v round trip: %v then %v", seed, itemIDs(first["items"]), itemIDs(again["items"]))
	}
}
