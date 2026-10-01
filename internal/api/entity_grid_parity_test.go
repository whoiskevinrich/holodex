package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"holodex/internal/model"
)

// TestEntityGridMatchesMedia is the HOLODEX-501 characterisation test (testing-strategy
// §23.3): the person, tag and studio detail grids show the same videos, in the same
// order, as GET /media with the matching facet. It pins the owner's 2026-09-30 parity
// check before the bespoke embedded lists are removed (ADR-121 D6).
func TestEntityGridMatchesMedia(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	ctx := context.Background()

	const n = 30
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = seedPlainVideo(t, r, fmt.Sprintf("Grid%02d", i))
	}
	// Spread indexed_at so the default added_desc order isn't just id order, with a
	// tie every third video so the id tiebreak is exercised too.
	for i, id := range ids {
		ts := fmt.Sprintf("2026-01-%02dT00:00:00Z", 1+(i*7)%28)
		if i%3 == 0 && i > 0 {
			ts = fmt.Sprintf("2026-01-%02dT00:00:00Z", 1+((i-1)*7)%28)
		}
		if _, err := sqlDB.Exec(`UPDATE videos SET indexed_at = ? WHERE id = ?`, ts, id); err != nil {
			t.Fatalf("set indexed_at: %v", err)
		}
	}

	tagID := seedTag(t, sqlDB, ids[0], "grid-tag")
	studioID := seedStudio(t, sqlDB, ids[0], "Grid Studio")
	for _, id := range ids[1:] {
		if _, err := sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, tagID); err != nil {
			t.Fatalf("tag: %v", err)
		}
		if _, err := sqlDB.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, id, studioID); err != nil {
			t.Fatalf("studio: %v", err)
		}
	}
	for _, id := range ids {
		linkPeople(t, r, id, "Grid Person")
	}
	people, err := r.PeopleForVideos(ctx, []int64{ids[0]})
	if err != nil || len(people[ids[0]]) == 0 {
		t.Fatalf("lookup person: %v", err)
	}
	personID := people[ids[0]][0].ID

	// A full-film video carrying all three facets: hidden on both paths.
	full := seedPlainVideo(t, r, "GridFullFilm")
	filmID, err := r.CreateFilm(ctx, "Grid Film", 2025)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, full, nil, true); err != nil {
		t.Fatalf("attach full film: %v", err)
	}
	sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, full, tagID)
	sqlDB.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, full, studioID)
	linkPeople(t, r, full, "Grid Person")

	for _, c := range []struct{ name, entityPath, facet string }{
		{"person", fmt.Sprintf("/people/%d", personID), fmt.Sprintf("person=%d", personID)},
		{"tag", fmt.Sprintf("/tags/%d", tagID), fmt.Sprintf("tag=%d", tagID)},
		{"studio", fmt.Sprintf("/studios/%d", studioID), fmt.Sprintf("studio_id=%d", studioID)},
	} {
		t.Run(c.name, func(t *testing.T) {
			entityIDs, entityTotal := gridIDs(t, srv.URL+"/api/v1"+c.entityPath)
			mediaIDs, mediaTotal := gridIDs(t, srv.URL+"/api/v1/media?limit=1000&"+c.facet)
			if entityTotal != n || mediaTotal != n {
				t.Fatalf("totals: entity %d, media %d, want %d", entityTotal, mediaTotal, n)
			}
			if fmt.Sprint(entityIDs) != fmt.Sprint(mediaIDs) {
				t.Errorf("order differs\nentity %v\nmedia  %v", entityIDs, mediaIDs)
			}
		})
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
