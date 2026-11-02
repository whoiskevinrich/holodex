package scanner

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/repo"
)

// renameScan is a real-repo scanner over a temp library holding one scanned file, for the F86
// journal cases (testing-strategy §39.4).
type renameScan struct {
	t   *testing.T
	ctx context.Context
	r   *repo.Repo
	db  *sql.DB
	s   *Scanner
	dir string
	id  int64
}

func newRenameScan(t *testing.T) *renameScan {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	rs := &renameScan{t: t, ctx: context.Background(), r: repo.New(database), db: database, dir: t.TempDir()}
	rs.s = New(Config{MediaPath: rs.dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), rs.r, stubExtractor{})
	os.MkdirAll(filepath.Join(rs.dir, "films"), 0o755)
	writeAged(t, rs.p("films/junk.mp4"), seededBytes(41, 1<<20), time.Hour)
	rs.scan()
	st, _, _ := rs.r.StatByPath(rs.ctx, rs.p("films/junk.mp4"))
	rs.id = st.ID
	return rs
}

func (rs *renameScan) p(rel string) string { return filepath.Join(rs.dir, filepath.FromSlash(rel)) }

func (rs *renameScan) scan() {
	rs.t.Helper()
	if err := rs.s.ScanOnce(rs.ctx); err != nil {
		rs.t.Fatal(err)
	}
}

func (rs *renameScan) mv(from, to string) {
	rs.t.Helper()
	os.MkdirAll(filepath.Dir(rs.p(to)), 0o755)
	if err := os.Rename(rs.p(from), rs.p(to)); err != nil {
		rs.t.Fatal(err)
	}
}

func (rs *renameScan) count(q string, args ...any) int {
	rs.t.Helper()
	var n int
	if err := rs.db.QueryRow(q, args...).Scan(&n); err != nil {
		rs.t.Fatal(err)
	}
	return n
}

func (rs *renameScan) pending(newName, via string) {
	rs.t.Helper()
	if _, err := rs.r.InsertPendingRename(rs.ctx, repo.FileRename{VideoID: rs.id, OldName: "junk.mp4",
		NewName: newName, ViaName: via, Trigger: repo.RenameTriggerManual}); err != nil {
		rs.t.Fatal(err)
	}
}

// done records that Holodex wrote the file's current name (a finished rename to it).
func (rs *renameScan) done(oldName, newName string) {
	rs.t.Helper()
	fr := repo.FileRename{VideoID: rs.id, OldName: oldName, NewName: newName, Trigger: repo.RenameTriggerManual}
	var err error
	if fr.ID, err = rs.r.InsertPendingRename(rs.ctx, fr); err != nil {
		rs.t.Fatal(err)
	}
	rs.mv("films/"+oldName, "films/"+newName)
	if err := rs.r.CompleteRename(rs.ctx, fr, rs.p("films")); err != nil {
		rs.t.Fatal(err)
	}
}

func (rs *renameScan) locked() bool {
	l, _ := rs.r.NameLocked(rs.ctx, rs.id)
	return l
}

// R10: a full pass mid-rename (file at the new or the via name) inserts, holds and hides nothing.
func TestScanDuringRename(t *testing.T) {
	for _, at := range []string{"new", "via"} {
		t.Run(at, func(t *testing.T) {
			rs := newRenameScan(t)
			via := "hx-1.holodex-rename"
			rs.pending("Ada.mp4", via)
			if at == "new" {
				rs.mv("films/junk.mp4", "films/Ada.mp4")
			} else {
				rs.mv("films/junk.mp4", "films/"+via)
			}
			rs.scan()
			if n := rs.count(`SELECT count(*) FROM videos`); n != 1 {
				t.Errorf("videos = %d, want 1", n)
			}
			if n := rs.count(`SELECT count(*) FROM videos WHERE id = ? AND active = 1`, rs.id); n != 1 {
				t.Error("the video was hidden mid-rename")
			}
		})
	}
}

// R9: the F79 net. With the journal row lost after step 2, the scan keeps the video's identity.
// (With the row present, the walk claims the new name for it: TestScanDuringRename.)
func TestScanLostJournalKeepsIdentity(t *testing.T) {
	rs := newRenameScan(t)
	// Crash after step 2 with the journal row lost (a restored DB): the file is at its new name and
	// nothing records why. The fingerprint repoints the same id.
	rs.mv("films/junk.mp4", "films/Bob.mp4")
	rs.scan()
	st, ok, _ := rs.r.StatByPath(rs.ctx, rs.p("films/Bob.mp4"))
	if !ok || st.ID != rs.id {
		t.Fatalf("the moved file did not keep its id: %+v %v", st, ok)
	}
	if rs.count(`SELECT count(*) FROM videos`) != 1 {
		t.Error("a new video was inserted")
	}
}

// R13: a Holodex-named file renamed by hand locks automatically; a move to another folder with the
// same name doesn't; a file never Holodex-named doesn't.
func TestScanHandEditLocks(t *testing.T) {
	rs := newRenameScan(t)
	rs.mv("films/junk.mp4", "films/junk2.mp4")
	rs.scan()
	if rs.locked() {
		t.Fatal("a never Holodex-named file was locked")
	}
	rs.mv("films/junk2.mp4", "films/junk.mp4")
	rs.scan()
	rs.done("junk.mp4", "Ada.mp4")
	rs.mv("films/Ada.mp4", "other/Ada.mp4")
	rs.scan()
	if rs.locked() {
		t.Fatal("a folder move locked the name")
	}
	rs.mv("other/Ada.mp4", "other/Ada (mine).mp4")
	rs.scan()
	st, _ := rs.r.FileNameStates(rs.ctx)
	if !st[rs.id].Locked || st[rs.id].LockSource != repo.LockAuto {
		t.Errorf("hand edit state = %+v", st[rs.id])
	}
}

// R13 through the fold: a hand edit the walk sees as a new file (the copy arrives while the
// Holodex-named file is still there) is healed by FoldMovedVideo, and the name still locks.
func TestScanHandEditLocksThroughFold(t *testing.T) {
	rs := newRenameScan(t)
	rs.done("junk.mp4", "Ada.mp4")
	data, err := os.ReadFile(rs.p("films/Ada.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	writeAged(t, rs.p("films/Ada (mine).mp4"), data, time.Hour)
	rs.scan() // both present: the copy is a new video
	if rs.count(`SELECT count(*) FROM videos`) != 2 {
		t.Fatal("the copy was not indexed as new")
	}
	os.Remove(rs.p("films/Ada.mp4"))
	rs.scan() // the original is gone: the fold heals the pair onto the older id
	st, ok, _ := rs.r.StatByPath(rs.ctx, rs.p("films/Ada (mine).mp4"))
	if !ok || st.ID != rs.id {
		t.Fatalf("the fold did not keep the older id: %+v %v", st, ok)
	}
	states, _ := rs.r.FileNameStates(rs.ctx)
	if !states[rs.id].Locked || states[rs.id].LockSource != repo.LockAuto {
		t.Errorf("hand edit through the fold: state = %+v", states[rs.id])
	}
}

// R11: the walk never ingests a via name.
func TestScanSkipsViaName(t *testing.T) {
	rs := newRenameScan(t)
	writeAged(t, rs.p("films/hx-9.holodex-rename"), seededBytes(43, 1<<20), time.Hour)
	rs.scan()
	if rs.count(`SELECT count(*) FROM videos`) != 1 {
		t.Error("a via name was ingested")
	}
}
