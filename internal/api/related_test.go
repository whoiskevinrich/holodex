package api_test

import (
	"strconv"
	"testing"
)

// TestRelatedEndpoint checks the GET /media/{id}/related contract: both blocks
// present, items exclude the current item. seedVideo shares person "Alice" and tag
// "nature" across items, so two seeded videos are related.
func TestRelatedEndpoint(t *testing.T) {
	srv, r, _ := newServer(t)
	v1 := seedVideo(t, r, "/m/a.mp4", "A")
	v2 := seedVideo(t, r, "/m/b.mp4", "B")

	code, body := getJSON(t, srv.URL+"/api/v1/media/"+strconv.FormatInt(v1, 10)+"/related")
	if code != 200 {
		t.Fatalf("related code = %d", code)
	}

	person, _ := body["person"].(map[string]any)
	if person == nil {
		t.Fatal("person block missing")
	}
	items, _ := person["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("person items = %d, want 1", len(items))
	}
	for _, it := range items {
		m, _ := it.(map[string]any)
		if int64(m["id"].(float64)) == v1 {
			t.Error("related items must exclude the current item")
		}
		if int64(m["id"].(float64)) != v2 {
			t.Errorf("related item id = %v, want %d", m["id"], v2)
		}
	}

	if _, ok := body["tag"].(map[string]any); !ok {
		t.Error("tag block missing (item has a tag)")
	}
}

func TestRelatedEndpointNotFound(t *testing.T) {
	srv, _, _ := newServer(t)
	if code, _ := getJSON(t, srv.URL+"/api/v1/media/99999/related"); code != 404 {
		t.Errorf("missing id code = %d, want 404", code)
	}
}
