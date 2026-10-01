package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"holodex/internal/model"
)

// TestEntityGridUncapped covers HOLODEX-501 (spec F75 P0-1, ADR-121 D6). The person,
// tag and studio pages load their grids through GET /media with the facet and paging,
// so a tag with more than 500 videos shows all of them, and the entity detail payloads
// no longer embed a (formerly 500-capped) video list.
//
// Before the switch, this file pinned that the embedded lists equalled /media in set
// and order for every entity (commit 8b62ebca), so the switch changes no grid's
// contents below the old cap.
func TestEntityGridUncapped(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)

	const n = 600
	first := seedPlainVideo(t, r, "Uncapped000")
	tagID := seedTag(t, sqlDB, first, "big-tag")
	studioID := seedStudio(t, sqlDB, first, "Big Studio")
	linkPeople(t, r, first, "Big Person")
	personID, ok, err := r.PersonIDByName(context.Background(), "Big Person")
	if err != nil || !ok {
		t.Fatalf("person id: %v (found=%v)", err, ok)
	}
	for i := 1; i < n; i++ {
		id := seedPlainVideo(t, r, fmt.Sprintf("Uncapped%03d", i))
		if _, err := sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, tagID); err != nil {
			t.Fatalf("tag: %v", err)
		}
	}

	// Every page of /media?tag= together is the whole set, once each.
	seen := map[int64]bool{}
	for offset := 0; offset < n; offset += 50 {
		ids, total := gridIDs(t, fmt.Sprintf("%s/api/v1/media?tag=%d&limit=50&offset=%d", srv.URL, tagID, offset))
		if total != n {
			t.Fatalf("offset %d: total = %d, want %d", offset, total, n)
		}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("video %d on two pages", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != n {
		t.Errorf("paged through %d videos, want %d", len(seen), n)
	}
	// A Play all run over the same grid gets all of them in one list (ADR-121 D7).
	if got := mediaIDs(t, fmt.Sprintf("%s/api/v1/media/ids?tag=%d", srv.URL, tagID)); len(got.IDs) != n {
		t.Errorf("/media/ids returned %d ids, want %d", len(got.IDs), n)
	}

	// The detail payloads carry the entity, not a video list.
	for _, path := range []string{
		fmt.Sprintf("/people/%d", personID), fmt.Sprintf("/tags/%d", tagID), fmt.Sprintf("/studios/%d", studioID),
	} {
		resp, err := http.Get(srv.URL + "/api/v1" + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		var body map[string]json.RawMessage
		json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, resp.StatusCode)
		}
		for _, k := range []string{"items", "total"} {
			if _, ok := body[k]; ok {
				t.Errorf("GET %s still embeds %q", path, k)
			}
		}
	}
}

func gridIDs(t *testing.T, url string) ([]int64, int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	var body struct {
		Items []model.Video `json:"items"`
		Total int           `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	out := make([]int64, len(body.Items))
	for i, v := range body.Items {
		out[i] = v.ID
	}
	return out, body.Total
}
