package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestMediaIDs_OwnerGated: GET /media/ids is built and gated by the same mediaFilterFor
// as /media (ADR-121 D3/D7), so the owner-only inputs are refused for a visitor.
func TestMediaIDs_OwnerGated(t *testing.T) {
	srv := completenessBrowseServer(t, "secret")
	for _, q := range []string{"sort=completeness_desc", "sort=completeness_asc", "missing_facet=poster_url"} {
		if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/ids?"+q, ""); code != http.StatusUnauthorized {
			t.Errorf("visitor %s: want 401, got %d", q, code)
		}
		if code, _ := getJSONTok(t, srv.URL+"/api/v1/media/ids?"+q, "secret"); code != http.StatusOK {
			t.Errorf("owner %s: want 200, got %d", q, code)
		}
	}
}

// TestMediaIDs_MatchesMedia: for every browse sort the id list is exactly the
// concatenated /media pages, a full-film video /media hides is absent, and limit/offset
// don't page it (testing-strategy §23.3).
func TestMediaIDs_MatchesMedia(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	ctx := context.Background()

	var ids []int64
	for i := 0; i < 12; i++ {
		id := seedPlainVideo(t, r, fmt.Sprintf("Ids%02d", (i*5)%12))
		ids = append(ids, id)
		if _, err := sqlDB.Exec(`UPDATE videos SET indexed_at = ?, duration_sec = ?, width = ?, height = ?
			WHERE id = ?`, fmt.Sprintf("2026-02-%02dT00:00:00Z", 1+(i*7)%12), 60+(i*13)%7, 640+(i%3)*640, 360+(i%3)*360, id); err != nil {
			t.Fatalf("spread columns: %v", err)
		}
	}
	tagID := seedTag(t, sqlDB, ids[0], "ids-tag")
	for _, id := range ids[1:] {
		sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, tagID)
	}
	full := seedPlainVideo(t, r, "IdsFullFilm")
	filmID, err := r.CreateFilm(ctx, "Ids Film", 2025)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, full, nil, true); err != nil {
		t.Fatalf("attach full film: %v", err)
	}
	sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, full, tagID)

	for _, sort := range []string{"added_desc", "added_asc", "title_asc", "title_desc",
		"duration_desc", "duration_asc", "resolution_desc", "resolution_asc", "random&seed=7"} {
		q := fmt.Sprintf("tag=%d&sort=%s", tagID, sort)
		t.Run(sort, func(t *testing.T) {
			var paged []int64
			for offset := 0; ; offset += 5 {
				page, total := gridIDs(t, fmt.Sprintf("%s/api/v1/media?%s&limit=5&offset=%d", srv.URL, q, offset))
				paged = append(paged, page...)
				if len(paged) >= total {
					break
				}
			}
			got := mediaIDs(t, srv.URL+"/api/v1/media/ids?"+q+"&limit=2&offset=3")
			if fmt.Sprint(got.IDs) != fmt.Sprint(paged) {
				t.Errorf("ids differ from /media\nids   %v\nmedia %v", got.IDs, paged)
			}
			if len(got.IDs) != len(ids) {
				t.Errorf("got %d ids, want %d (limit/offset must not page it)", len(got.IDs), len(ids))
			}
			for _, id := range got.IDs {
				if id == full {
					t.Errorf("full-film video %d present, want hidden as on /media", full)
				}
			}
		})
	}

	// random with no seed mints one and echoes it; sending it back reproduces the order.
	first := mediaIDs(t, fmt.Sprintf("%s/api/v1/media/ids?tag=%d&sort=random", srv.URL, tagID))
	if first.Seed == nil {
		t.Fatalf("random without seed: no seed echoed")
	}
	again := mediaIDs(t, fmt.Sprintf("%s/api/v1/media/ids?tag=%d&sort=random&seed=%d", srv.URL, tagID, *first.Seed))
	if fmt.Sprint(first.IDs) != fmt.Sprint(again.IDs) {
		t.Errorf("echoed seed doesn't reproduce the order:\n%v\n%v", first.IDs, again.IDs)
	}
	if plain := mediaIDs(t, fmt.Sprintf("%s/api/v1/media/ids?tag=%d", srv.URL, tagID)); plain.Seed != nil {
		t.Errorf("non-random sort echoed a seed %d", *plain.Seed)
	}
}

type mediaIDsBody struct {
	IDs  []int64 `json:"ids"`
	Seed *int64  `json:"seed"`
}

func mediaIDs(t *testing.T, url string) mediaIDsBody {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	var body mediaIDsBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return body
}
