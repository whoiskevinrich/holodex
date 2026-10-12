package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestMediaBrief_RequestOrderAndGating: POST /media/brief returns each listed video's
// title and length in the order asked (the run's order, not a sort), leaves out an id
// that isn't an openable video — soft-deleted or unknown — and lists a repeated id once
// (spec RD14 / P0-13). The requests are a visitor's (no token).
func TestMediaBrief_RequestOrderAndGating(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	a := seedPlainVideo(t, r, "Brief A")
	b := seedPlainVideo(t, r, "Brief B")
	c := seedPlainVideo(t, r, "Brief C")
	gone := seedPlainVideo(t, r, "Brief Deleted")
	if _, err := sqlDB.Exec(`UPDATE videos SET duration_sec = 90 WHERE id = ?`, b); err != nil {
		t.Fatalf("set duration: %v", err)
	}
	if _, err := sqlDB.Exec(`UPDATE videos SET deleted_at = '2026-10-01T00:00:00Z' WHERE id = ?`, gone); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	code, got := postBrief(t, srv.URL, []int64{c, gone, a, 999999, b, a})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	var ids []int64
	for _, it := range got {
		ids = append(ids, it.ID)
	}
	if want := fmt.Sprint([]int64{c, a, b}); fmt.Sprint(ids) != want {
		t.Errorf("ids = %v, want %s (request order, deleted/unknown dropped, repeat once)", ids, want)
	}
	if got[0].Title != "Brief C" || got[2].Duration != 90 {
		t.Errorf("rows = %+v, want titles and lengths carried", got)
	}
}

// TestMediaBrief_Bounds: no ids is an empty list, not an error; more than one chunk's
// worth is refused, so the client has to ask in chunks.
func TestMediaBrief_Bounds(t *testing.T) {
	srv, _, _ := filmEntityServer(t)
	if code, got := postBrief(t, srv.URL, nil); code != http.StatusOK || len(got) != 0 {
		t.Errorf("empty: status %d, %d items; want 200 and none", code, len(got))
	}
	many := make([]int64, 1001)
	for i := range many {
		many[i] = int64(i + 1)
	}
	if code, _ := postBrief(t, srv.URL, many); code != http.StatusBadRequest {
		t.Errorf("1001 ids: status %d, want 400", code)
	}
}

type briefRow struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Duration int    `json:"duration_sec"`
}

func postBrief(t *testing.T, base string, ids []int64) (int, []briefRow) {
	t.Helper()
	buf, _ := json.Marshal(map[string]any{"ids": ids})
	resp, err := http.Post(base+"/api/v1/media/brief", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST brief: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Items []briefRow `json:"items"`
	}
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode brief: %v", err)
		}
	}
	return resp.StatusCode, body.Items
}
