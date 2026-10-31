package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"holodex/internal/model"
)

// num reads a JSON number field as an int.
func num(body map[string]any, key string) int {
	f, _ := body[key].(float64)
	return int(f)
}

// TestOwnerQueuePagingContract pins the owner-queue list envelope (HOLODEX-587) on
// Trash, the plainest queue: {items, total, limit, offset}, pages that tile, and the
// clamped limit/offset echoed rather than the requested ones. The handlers all share
// pageFromQuery/pageOf, so the clamp itself is unit-tested in paging_internal_test.go.
func TestOwnerQueuePagingContract(t *testing.T) {
	srv, r, first := deleteServer(t, "s3cret")
	ctx := context.Background()
	ids := []int64{first}
	for _, p := range []string{"/m/b.mkv", "/m/c.mkv"} {
		id, err := r.UpsertVideo(ctx, &model.Video{FilePath: p, Title: p, Duration: 60,
			FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := r.SoftDelete(ctx, id); err != nil {
			t.Fatalf("soft delete: %v", err)
		}
	}
	trash := srv.URL + "/api/v1/admin/trash"

	if code := sendTok(t, http.MethodGet, trash+"?limit=1", ""); code != http.StatusUnauthorized {
		t.Errorf("no-token paged trash = %d, want 401", code)
	}

	seen := map[float64]bool{}
	for _, off := range []string{"0", "2"} {
		code, body := getJSONTok(t, trash+"?limit=2&offset="+off, "s3cret")
		if code != http.StatusOK {
			t.Fatalf("trash offset %s = %d", off, code)
		}
		if num(body, "total") != 3 || num(body, "limit") != 2 || body["offset"] == nil {
			t.Errorf("trash offset %s envelope = %v", off, body)
		}
		for _, it := range body["items"].([]any) {
			id := it.(map[string]any)["id"].(float64)
			if seen[id] {
				t.Errorf("item %v on two pages", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 3 {
		t.Errorf("two pages covered %d items, want 3", len(seen))
	}

	_, body := getJSONTok(t, trash+"?limit=999999&offset=-4", "s3cret")
	if num(body, "limit") != 1000 || num(body, "offset") != 0 || len(body["items"].([]any)) != 3 {
		t.Errorf("clamped trash = %v, want limit 1000 offset 0 and all 3 items", body)
	}
	_, body = getJSONTok(t, trash+"?offset=50", "s3cret")
	if items := body["items"].([]any); len(items) != 0 || num(body, "total") != 3 {
		t.Errorf("past-the-end trash = %v, want [] with total 3", body)
	}
}

// TestDuplicatesPagingByType: ?type= narrows the pairs and their total, while counts
// stays every type's unfiltered count, for the headings and list-page banners.
func TestDuplicatesPagingByType(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	seedTagVideo(t, r, "/m/a.mkv", "sci-fi")
	seedTagVideo(t, r, "/m/b.mkv", "scifi")
	list := srv.URL + "/api/v1/owner/duplicates"

	_, body := getJSONTok(t, list+"?type=tag&limit=1", "s3cret")
	counts, _ := body["counts"].(map[string]any)
	if num(body, "total") != 1 || len(body["items"].([]any)) != 1 || counts["tag"] != float64(1) {
		t.Errorf("type=tag = %v, want the one tag pair", body)
	}
	_, body = getJSONTok(t, list+"?type=person", "s3cret")
	counts, _ = body["counts"].(map[string]any)
	if num(body, "total") != 0 || len(body["items"].([]any)) != 0 || counts["tag"] != float64(1) {
		t.Errorf("type=person = %v, want no items, tag count still 1", body)
	}
	// Every lane has a count, 0 included, so the tabs never shift (F70 P0-8); the
	// Videos list carries the same counts.
	for _, lane := range []string{"person", "studio", "tag", "film", "video"} {
		if _, ok := counts[lane]; !ok {
			t.Errorf("counts = %v, missing lane %q", counts, lane)
		}
	}
	_, body = getJSONTok(t, list+"/videos", "s3cret")
	if counts, _ := body["counts"].(map[string]any); counts["tag"] != float64(1) || counts["video"] != float64(0) {
		t.Errorf("videos list counts = %v, want tag 1 and video 0", body["counts"])
	}
	if code, _ := getJSONTok(t, list+"?type=bogus", "s3cret"); code != http.StatusBadRequest {
		t.Errorf("type=bogus = %d, want 400", code)
	}
}
