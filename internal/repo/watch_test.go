package repo_test

import (
	"context"
	"database/sql"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// F89 slice 4 (testing-strategy §40.4, watch-history.md).

func watchFixture(t *testing.T, n int) (*repo.Repo, *sql.DB, []int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	var ids []int64
	for i := range n {
		id, err := r.UpsertVideo(context.Background(), &model.Video{
			FilePath: "/m/" + string(rune('a'+i)) + ".mkv", FileSize: 1, Title: "v", Duration: 60,
			FileMtime: time.Unix(1_700_000_000, 0).UTC(),
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return r, database, ids
}

func watchCount(t *testing.T, d *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// R7: the summary rebuilds from the rows, and a mark never changes the row count.
func TestWatchSummaryRebuildsFromRows(t *testing.T) {
	r, d, ids := watchFixture(t, 4)
	ctx := context.Background()
	rng := rand.New(rand.NewSource(89))
	for range 200 {
		id := ids[rng.Intn(len(ids))]
		before := watchCount(t, d, `SELECT COUNT(*) FROM video_plays`)
		if rng.Intn(3) == 0 {
			if err := r.SetWatchedMark(ctx, id, rng.Intn(2) == 0); err != nil {
				t.Fatal(err)
			}
			if after := watchCount(t, d, `SELECT COUNT(*) FROM video_plays`); after != before {
				t.Fatalf("a mark changed the plays: %d to %d", before, after)
			}
		} else if err := r.RecordPlay(ctx, id, 0.9); err != nil {
			t.Fatal(err)
		}
		drift := watchCount(t, d, `
			SELECT COUNT(*) FROM video_watch w
			LEFT JOIN (SELECT video_id, COUNT(*) n, MAX(played_at) m FROM video_plays GROUP BY video_id) p ON p.video_id = w.video_id
			WHERE w.play_count != COALESCE(p.n, 0) OR COALESCE(w.last_played_at, '') != COALESCE(p.m, '')`)
		if drift != 0 {
			t.Fatalf("%d summaries drifted from the rows", drift)
		}
	}
}

// R8: the row and the summary land together or not at all.
func TestRecordPlayIsOneTransaction(t *testing.T) {
	r, d, ids := watchFixture(t, 1)
	mustRun(t, d, `CREATE TRIGGER fail_summary BEFORE INSERT ON video_watch BEGIN SELECT RAISE(ABORT, 'forced'); END`)
	if err := r.RecordPlay(context.Background(), ids[0], 0.9); err == nil {
		t.Fatal("want the forced error")
	}
	if n := watchCount(t, d, `SELECT COUNT(*) FROM video_plays`); n != 0 {
		t.Errorf("%d play rows survived the failed summary", n)
	}
}

// A play or a mark on a video that isn't there writes nothing.
func TestWatchWritesNeedAVideo(t *testing.T) {
	r, d, ids := watchFixture(t, 1)
	ctx := context.Background()
	mustRun(t, d, `UPDATE videos SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = ?`, ids[0])
	for name, err := range map[string]error{
		"play, deleted":  r.RecordPlay(ctx, ids[0], 0.9),
		"mark, deleted":  r.SetWatchedMark(ctx, ids[0], true),
		"play, no video": r.RecordPlay(ctx, 9999, 0.9),
		"mark, no video": r.SetWatchedMark(ctx, 9999, true),
	} {
		if err != repo.ErrNotFound {
			t.Errorf("%s: err = %v, want not found", name, err)
		}
	}
	if n := watchCount(t, d, `SELECT COUNT(*) FROM video_watch`); n != 0 {
		t.Errorf("%d summaries written", n)
	}
}

// A healed move folds the newer row away, and the history comes with it (SEC-10): the plays
// add up, and the later mark stands unless a later play cleared it.
func TestFoldKeepsTheWatchHistory(t *testing.T) {
	const t1, t2 = "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"
	for _, tc := range []struct {
		name         string
		older, newer [2]string // a play time and a mark time, "" for none
		wantCount    int
		wantMark     sql.NullInt64
	}{
		{name: "only newer played", newer: [2]string{t1, ""}, wantCount: 1},
		{name: "only older played", older: [2]string{t1, ""}, wantCount: 1},
		{name: "both played", older: [2]string{t1, ""}, newer: [2]string{t2, ""}, wantCount: 2},
		{name: "a later play clears older's mark", older: [2]string{"", t1}, newer: [2]string{t2, ""}, wantCount: 1},
		{name: "a later mark survives older's play", older: [2]string{t1, ""}, newer: [2]string{"", t2}, wantCount: 1, wantMark: sql.NullInt64{Int64: 1, Valid: true}},
		{name: "the later of two marks", older: [2]string{"", t2}, newer: [2]string{"", t1}, wantMark: sql.NullInt64{Int64: 1, Valid: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, d, _ := watchFixture(t, 0)
			ctx := context.Background()
			older, newer := seedMovePair(t, r, d, "/m/old.mkv", "/m/new.mkv", "fp")
			seed := func(id int64, h [2]string, mark bool) {
				if h[0] != "" {
					if err := r.RecordPlay(ctx, id, 0.9); err != nil {
						t.Fatal(err)
					}
					mustRun(t, d, `UPDATE video_plays SET played_at = ? WHERE video_id = ?`, h[0], id)
					mustRun(t, d, `UPDATE video_watch SET last_played_at = ? WHERE video_id = ?`, h[0], id)
				}
				if h[1] != "" {
					if err := r.SetWatchedMark(ctx, id, mark); err != nil {
						t.Fatal(err)
					}
					mustRun(t, d, `UPDATE video_watch SET manual_at = ? WHERE video_id = ?`, h[1], id)
				}
			}
			// In the two-mark case the earlier one is unwatched, so the winner shows.
			seed(older, tc.older, true)
			seed(newer, tc.newer, tc.older[1] == "")
			if err := r.FoldMovedVideo(ctx, older, newer); err != nil {
				t.Fatal(err)
			}
			if n := watchCount(t, d, `SELECT COUNT(*) FROM video_plays WHERE video_id = ?`, older); n != tc.wantCount {
				t.Errorf("play rows = %d, want %d", n, tc.wantCount)
			}
			var count int
			var mark sql.NullInt64
			if err := d.QueryRow(`SELECT play_count, manual_watched FROM video_watch WHERE video_id = ?`, older).Scan(&count, &mark); err != nil {
				t.Fatal(err)
			}
			if count != tc.wantCount || mark != tc.wantMark {
				t.Errorf("summary = %d plays, mark %v; want %d, %v", count, mark, tc.wantCount, tc.wantMark)
			}
			if n := watchCount(t, d, `SELECT COUNT(*) FROM video_watch WHERE video_id = ?`, newer); n != 0 {
				t.Errorf("newer kept a summary")
			}
		})
	}
}

// R10: a purge removes plays and summary, and it's the only path: the record holds no more
// than SEC-10 lists, and no repo code deletes from either table.
func TestPlaysLiveAsLongAsTheVideo(t *testing.T) {
	r, d, ids := watchFixture(t, 2)
	ctx := context.Background()
	for _, id := range ids {
		if err := r.RecordPlay(ctx, id, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	mustRun(t, d, `DELETE FROM videos WHERE id = ?`, ids[0])
	for _, table := range []string{"video_plays", "video_watch"} {
		if n := watchCount(t, d, `SELECT COUNT(*) FROM `+table+` WHERE video_id = ?`, ids[0]); n != 0 {
			t.Errorf("%s kept %d rows of the purged video", table, n)
		}
		if n := watchCount(t, d, `SELECT COUNT(*) FROM `+table+` WHERE video_id = ?`, ids[1]); n != 1 {
			t.Errorf("%s has %d rows of the other video, want 1", table, n)
		}
	}

	rows, err := d.Query(`SELECT name FROM pragma_table_info('video_plays') ORDER BY cid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, c)
	}
	if got := strings.Join(cols, ","); got != "id,video_id,played_at,progress" {
		t.Errorf("video_plays columns = %s", got)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	del := regexp.MustCompile(`(?i)(DELETE\s+FROM|DROP\s+TABLE)\s+video_(plays|watch)\b`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := del.Find(src); m != nil {
			t.Errorf("%s: %q; a purge is the only delete path", f, m)
		}
	}
}

func mustRun(t *testing.T, d *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := d.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}
