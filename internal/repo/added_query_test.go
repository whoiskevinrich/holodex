package repo_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"holodex/internal/query"
	"holodex/internal/repo"
)

// F89 P0-1 (testing-strategy §40 Q3 at the SQL end): `added:` reads added_at, which an
// insert sets and a re-upsert never moves, and a relative bound follows the clock.
func TestAddedCondition(t *testing.T) {
	r, database := newRepoWithDB(t)
	ctx := context.Background()
	oldID := upsert(t, r, sortVideo("/m/old.mkv", 1))
	newID := upsert(t, r, sortVideo("/m/new.mkv", 1))
	if _, err := database.Exec(`UPDATE videos SET added_at = ? WHERE id = ?`, "2026-01-10T12:00:00Z", oldID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE videos SET added_at = ? WHERE id = ?`, "2026-03-01T12:00:00Z", newID); err != nil {
		t.Fatal(err)
	}
	// A re-upsert with a changed file keeps added_at.
	upsert(t, r, sortVideo("/m/old.mkv", 2))
	var added string
	if err := database.QueryRow(`SELECT added_at FROM videos WHERE id = ?`, oldID).Scan(&added); err != nil || added != "2026-01-10T12:00:00Z" {
		t.Fatalf("added_at after re-upsert = %q %v, want it unchanged", added, err)
	}

	ids := func(src string, now time.Time) []int64 {
		t.Helper()
		q, err := query.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.ListVideoIDs(ctx, repo.VideoFilter{Expr: q.Root, Now: now, Sort: "added_asc"})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	march := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	for src, want := range map[string][]int64{
		"added:<30d":             {newID},
		"added:>30d":             {oldID},
		"-added:<30d":            {oldID},
		"added:2026-01":          {oldID},
		"added:2026-01..2026-03": {oldID, newID},
		"added:<2026-02":         {oldID},
		"added:>=2026-02":        {newID},
	} {
		if got := ids(src, march); !slices.Equal(got, want) {
			t.Errorf("%s at mid-March = %v, want %v", src, got, want)
		}
	}
	// 31 days on, the same stored condition has moved on.
	if got := ids("added:<30d", march.AddDate(0, 0, 31)); len(got) != 0 {
		t.Errorf("added:<30d a month later = %v, want none", got)
	}
}

// F89 P0-2 (§40 R11, repo end): the cap cuts the count, every page and the id list.
func TestVideoFilterCap(t *testing.T) {
	r, _ := newRepoWithDB(t)
	ctx := context.Background()
	var all []int64
	for _, p := range []string{"/m/a.mkv", "/m/b.mkv", "/m/c.mkv", "/m/d.mkv", "/m/e.mkv"} {
		all = append(all, upsert(t, r, sortVideo(p, 1)))
	}
	f := repo.VideoFilter{Sort: "added_asc", Cap: 3}
	if n, err := r.CountVideos(ctx, f); err != nil || n != 3 {
		t.Errorf("capped count = %d %v, want 3", n, err)
	}
	if n, err := r.MatchCount(ctx, f); err != nil || n != 5 {
		t.Errorf("match count = %d %v, want 5", n, err)
	}
	ids, err := r.ListVideoIDs(ctx, f)
	if err != nil || !slices.Equal(ids, all[:3]) {
		t.Errorf("capped ids = %v %v, want %v", ids, err, all[:3])
	}
	f.Limit, f.Offset = 2, 2
	page, total, err := r.ListVideos(ctx, f)
	if err != nil || total != 3 || len(page) != 1 || page[0].ID != all[2] {
		t.Errorf("last page = %d items (total %d) %v, want only the third", len(page), total, err)
	}
	f.Offset = 3
	if page, _, _ := r.ListVideos(ctx, f); len(page) != 0 {
		t.Errorf("page past the cap has %d items", len(page))
	}
}
