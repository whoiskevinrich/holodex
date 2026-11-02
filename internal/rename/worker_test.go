package rename

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// lib is a real temp library over real SQLite, named from one template, `{title}`.
type lib struct {
	t      *testing.T
	ctx    context.Context
	r      *repo.Repo
	db     *sql.DB
	root   string
	dir    string
	titles map[int64]string
	ex     *Executor
}

func newLib(t *testing.T) *lib {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	root := t.TempDir()
	l := &lib{t: t, ctx: context.Background(), r: repo.New(database), db: database, root: root,
		dir: filepath.Join(root, "films"), titles: map[int64]string{}}
	os.MkdirAll(l.dir, 0o755)
	l.ex = &Executor{Repo: l.r, Root: root, Plan: l.plan}
	return l
}

func (l *lib) plan(ctx context.Context, videoID int64, run []int64) (Row, error) {
	list, _ := ParseList([]string{"{title}"}, nil)
	paths, err := l.r.AllVideoPaths(ctx)
	if err != nil {
		return Row{}, err
	}
	disk := Disk{Paths: map[int64]string{}, ListDir: func(d string) []string {
		ents, _ := os.ReadDir(d)
		var n []string
		for _, e := range ents {
			n = append(n, e.Name())
		}
		return n
	}}
	inRun := map[int64]bool{videoID: true}
	for _, id := range run {
		inRun[id] = true
	}
	var inputs []Input
	for _, p := range paths {
		disk.Paths[p.ID] = p.FilePath
		if inRun[p.ID] {
			locked, _ := l.r.NameLocked(ctx, p.ID)
			inputs = append(inputs, Input{ID: p.ID, Path: p.FilePath, Values: Values{"title": {l.titles[p.ID]}}, Locked: locked})
		}
	}
	for _, row := range Plan(list, "", inputs, disk, func(id int64) bool { return inRun[id] }) {
		if row.VideoID == videoID {
			return row, nil
		}
	}
	return Row{}, errors.New("not found")
}

func (l *lib) video(file, title string) int64 {
	l.t.Helper()
	path := filepath.Join(l.dir, file)
	if err := os.WriteFile(path, []byte(file), 0o644); err != nil {
		l.t.Fatal(err)
	}
	id, err := l.r.UpsertVideo(l.ctx, &model.Video{FilePath: path, Title: title, FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
	if err != nil {
		l.t.Fatal(err)
	}
	l.titles[id] = title
	return id
}

func (l *lib) enqueue(batch string, ids ...int64) {
	l.t.Helper()
	var jobs []repo.RenameEnqueue
	for _, id := range ids {
		row, err := l.plan(l.ctx, id, ids)
		if err != nil {
			l.t.Fatal(err)
		}
		p, _ := json.Marshal(Payload{Name: row.Name, Template: "{title}", Written: row.Written})
		jobs = append(jobs, repo.RenameEnqueue{VideoID: id, Payload: string(p)})
	}
	if _, err := l.r.EnqueueRenames(l.ctx, jobs, batch); err != nil {
		l.t.Fatal(err)
	}
}

// drain runs the lane the way the queue does, returning each video's last error.
func (l *lib) drain() map[int64]error {
	out := map[int64]error{}
	for range 200 {
		job, err := l.r.ClaimNextWriteback(l.ctx)
		if err != nil {
			l.t.Fatal(err)
		}
		if job == nil {
			return out
		}
		err = l.ex.Run(l.ctx, job)
		out[job.VideoID] = err
		if errors.Is(err, ErrRequeue) {
			l.r.RequeueWriteback(l.ctx, job.ID)
			continue
		}
		ok := err == nil || errors.Is(err, ErrNoop)
		msg := ""
		if !ok {
			msg = err.Error()
		}
		l.r.FinishWriteback(l.ctx, job.ID, ok, msg)
	}
	l.t.Fatal("the queue never drained")
	return nil
}

func (l *lib) path(id int64) string {
	p, err := l.r.PathByID(l.ctx, id)
	if err != nil {
		l.t.Fatal(err)
	}
	return filepath.Base(p)
}

func (l *lib) scalar(q string, args ...any) string {
	var s sql.NullString
	if err := l.db.QueryRow(q, args...).Scan(&s); err != nil {
		l.t.Fatal(err)
	}
	return s.String
}

// R1 + R2: one rename writes a done row and changes only file_path; indexed_at stays.
func TestRunRenamesAndJournals(t *testing.T) {
	l := newLib(t)
	id := l.video("junk.mkv", "Ada")
	before := l.scalar(`SELECT indexed_at || file_mtime || file_size FROM videos WHERE id = ?`, id)
	l.enqueue("b1", id)
	if errs := l.drain(); errs[id] != nil {
		t.Fatal(errs[id])
	}
	if l.path(id) != "Ada.mkv" || read(t, l.dir, "Ada.mkv") != "junk.mkv" {
		t.Fatalf("path = %s", l.path(id))
	}
	if got := l.scalar(`SELECT old_name || '>' || new_name || ':' || state || ':' || trigger FROM file_renames WHERE video_id = ?`, id); got != "junk.mkv>Ada.mkv:done:manual" {
		t.Errorf("journal = %s", got)
	}
	if after := l.scalar(`SELECT indexed_at || file_mtime || file_size FROM videos WHERE id = ?`, id); after != before {
		t.Errorf("file layer changed: %s → %s", before, after)
	}
	// R25: applying again records nothing.
	l.enqueue("b2", id)
	l.drain()
	if n := l.scalar(`SELECT count(*) FROM file_renames WHERE video_id = ?`, id); n != "1" {
		t.Errorf("rows after a no-op = %s", n)
	}
}

// R3: step 3 accepts a row a scan already repointed.
func TestCompleteRenameAcceptsRepointed(t *testing.T) {
	l := newLib(t)
	id := l.video("junk.mkv", "Ada")
	fr := repo.FileRename{VideoID: id, OldName: "junk.mkv", NewName: "Ada.mkv", Trigger: repo.RenameTriggerManual}
	var err error
	if fr.ID, err = l.r.InsertPendingRename(l.ctx, fr); err != nil {
		t.Fatal(err)
	}
	if _, err := l.db.Exec(`UPDATE videos SET file_path = ? WHERE id = ?`, filepath.Join(l.dir, "Ada.mkv"), id); err != nil {
		t.Fatal(err)
	}
	if err := l.r.CompleteRename(l.ctx, fr, l.dir); err != nil {
		t.Fatal(err)
	}
}

func (l *lib) pending(id int64, old, new, via string) repo.FileRename {
	l.t.Helper()
	fr := repo.FileRename{VideoID: id, OldName: old, NewName: new, ViaName: via, Trigger: repo.RenameTriggerManual}
	var err error
	if fr.ID, err = l.r.InsertPendingRename(l.ctx, fr); err != nil {
		l.t.Fatal(err)
	}
	return fr
}

// R4–R7: boot recovery replays each pending row by where the file is.
func TestRecoverPending(t *testing.T) {
	t.Run("at old name", func(t *testing.T) { // R4
		l := newLib(t)
		id := l.video("junk.mkv", "Ada")
		l.pending(id, "junk.mkv", "Ada.mkv", "")
		l.enqueue("b", id)
		if _, err := RecoverPending(l.ctx, l.r, l.root); err != nil {
			t.Fatal(err)
		}
		l.drain()
		if n := l.scalar(`SELECT count(*) FROM file_renames WHERE video_id = ? AND state = 'done'`, id); n != "1" || l.path(id) != "Ada.mkv" {
			t.Errorf("done rows = %s, path = %s", n, l.path(id))
		}
	})
	t.Run("at new name", func(t *testing.T) { // R5
		l := newLib(t)
		id := l.video("junk.mkv", "Ada")
		l.pending(id, "junk.mkv", "Ada.mkv", "")
		os.Rename(filepath.Join(l.dir, "junk.mkv"), filepath.Join(l.dir, "Ada.mkv"))
		if _, err := RecoverPending(l.ctx, l.r, l.root); err != nil {
			t.Fatal(err)
		}
		if l.path(id) != "Ada.mkv" || l.scalar(`SELECT state FROM file_renames WHERE video_id = ?`, id) != "done" {
			t.Errorf("not finished: %s", l.path(id))
		}
		if locked, _ := l.r.NameLocked(l.ctx, id); locked {
			t.Error("recovery locked the video")
		}
	})
	t.Run("at via name, job requeued", func(t *testing.T) { // R6
		l := newLib(t)
		id := l.video("ada.mkv", "Ada")
		via := ViaName(id)
		l.pending(id, "ada.mkv", "Ada.mkv", via)
		os.Rename(filepath.Join(l.dir, "ada.mkv"), filepath.Join(l.dir, via))
		l.r.EnqueueRenames(l.ctx, []repo.RenameEnqueue{{VideoID: id, Payload: `{"name":"Ada.mkv"}`}}, "")
		if _, err := RecoverPending(l.ctx, l.r, l.root); err != nil {
			t.Fatal(err)
		}
		if errs := l.drain(); errs[id] != nil {
			t.Fatal(errs[id])
		}
		if got := names(t, l.dir); len(got) != 1 || got[0] != "Ada.mkv" || l.path(id) != "Ada.mkv" {
			t.Errorf("folder = %v, path = %s", got, l.path(id))
		}
	})
	t.Run("at via name, no job", func(t *testing.T) { // R6
		l := newLib(t)
		id := l.video("ada.mkv", "Ada")
		via := ViaName(id)
		l.pending(id, "ada.mkv", "Ada.mkv", via)
		os.Rename(filepath.Join(l.dir, "ada.mkv"), filepath.Join(l.dir, via))
		if _, err := RecoverPending(l.ctx, l.r, l.root); err != nil {
			t.Fatal(err)
		}
		if got := names(t, l.dir); len(got) != 1 || got[0] != "ada.mkv" {
			t.Errorf("folder = %v, want the old name back", got)
		}
	})
	t.Run("at both names", func(t *testing.T) { // R7
		l := newLib(t)
		id := l.video("junk.mkv", "Ada")
		write(t, l.dir, "Ada.mkv", "other")
		l.pending(id, "junk.mkv", "Ada.mkv", "")
		l.r.EnqueueRenames(l.ctx, []repo.RenameEnqueue{{VideoID: id, Payload: `{"name":"Ada.mkv"}`}}, "")
		if _, err := RecoverPending(l.ctx, l.r, l.root); err != nil {
			t.Fatal(err)
		}
		if read(t, l.dir, "Ada.mkv") != "other" || read(t, l.dir, "junk.mkv") != "junk.mkv" {
			t.Error("recovery touched a file")
		}
		if n := l.scalar(`SELECT count(*) FROM file_renames`); n != "0" {
			t.Errorf("rows = %s", n)
		}
		if _, failed, _ := l.r.FailedRename(l.ctx, id); !failed {
			t.Error("the job was not failed")
		}
	})
}

// A retried job whose rename already landed (step 3 failed last time) finishes it instead of
// abandoning the journal row; a stale row at the old name is dropped and the job plans afresh.
func TestRunSettlesLeftoverJournal(t *testing.T) {
	l := newLib(t)
	landed := l.video("a.mkv", "Ada")
	stale := l.video("b.mkv", "Bob")
	l.pending(landed, "a.mkv", "Ada.mkv", "")
	os.Rename(filepath.Join(l.dir, "a.mkv"), filepath.Join(l.dir, "Ada.mkv"))
	l.pending(stale, "b.mkv", "Old plan.mkv", "")
	l.enqueue("b", stale)
	l.r.EnqueueRenames(l.ctx, []repo.RenameEnqueue{{VideoID: landed, Payload: `{"name":"Ada.mkv"}`}}, "")
	if errs := l.drain(); errs[landed] != nil || errs[stale] != nil {
		t.Fatalf("errs = %v", errs)
	}
	if l.path(landed) != "Ada.mkv" || l.path(stale) != "Bob.mkv" {
		t.Errorf("landed = %s, stale = %s", l.path(landed), l.path(stale))
	}
}

// R19: the worker fails the job when the desired name moved since the plan.
func TestRunNameChanged(t *testing.T) {
	l := newLib(t)
	id := l.video("junk.mkv", "Ada")
	l.enqueue("b", id)
	l.titles[id] = "Bob"
	if errs := l.drain(); errs[id] == nil || l.path(id) != "junk.mkv" {
		t.Fatalf("err = %v, path = %s", errs[id], l.path(id))
	}
}

// R20: a two-video swap completes in one batch through a via name, with no disambiguator and no
// overwrite.
func TestRunSwap(t *testing.T) {
	l := newLib(t)
	a := l.video("x.mkv", "y")
	b := l.video("y.mkv", "x")
	l.enqueue("swap", a, b)
	if errs := l.drain(); errs[a] != nil || errs[b] != nil {
		t.Fatalf("errs = %v", errs)
	}
	if l.path(a) != "y.mkv" || l.path(b) != "x.mkv" || read(t, l.dir, "y.mkv") != "x.mkv" || read(t, l.dir, "x.mkv") != "y.mkv" {
		t.Errorf("a = %s, b = %s", l.path(a), l.path(b))
	}
}

// R20: a target held by another batch member that is leaving goes back to pending behind it.
func TestRunWaitsForBatchHolder(t *testing.T) {
	l := newLib(t)
	a := l.video("a.mkv", "b")
	b := l.video("b.mkv", "c")
	l.enqueue("chain", a, b)
	if errs := l.drain(); errs[a] != nil || errs[b] != nil {
		t.Fatalf("errs = %v", errs)
	}
	if l.path(a) != "b.mkv" || l.path(b) != "c.mkv" {
		t.Errorf("a = %s, b = %s", l.path(a), l.path(b))
	}
}

// R21 + R22: held and locked videos are never renamed.
func TestRunRefusesHeldAndLocked(t *testing.T) {
	l := newLib(t)
	held := l.video("h.mkv", "Held")
	locked := l.video("l.mkv", "Locked")
	l.enqueue("b", held, locked)
	now := time.Now()
	if err := l.r.BeginSwap(l.ctx, held, filepath.Join(l.dir, "h.mkv.holodex-orig"), "hflip", now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	l.r.SetNameLock(l.ctx, locked, true, repo.LockOwner, now)
	errs := l.drain()
	if errs[held] == nil || errs[locked] == nil || l.path(held) != "h.mkv" || l.path(locked) != "l.mkv" {
		t.Errorf("errs = %v", errs)
	}
}

// R15: undo restores the latest rename's old name, writes an undo row and locks; a taken old name
// refuses with no disambiguator.
func TestRunUndo(t *testing.T) {
	l := newLib(t)
	id := l.video("junk.mkv", "Ada")
	l.enqueue("b", id)
	l.drain()
	last, _ := l.r.LatestDoneRename(l.ctx, id)
	undo := func() error {
		p, _ := json.Marshal(Payload{Name: last.OldName, UndoOf: last.ID})
		l.r.EnqueueRenames(l.ctx, []repo.RenameEnqueue{{VideoID: id, Payload: string(p)}}, "")
		return l.drain()[id]
	}
	write(t, l.dir, "junk.mkv", "squatter")
	if err := undo(); err == nil || l.path(id) != "Ada.mkv" {
		t.Fatalf("undo onto a taken name: %v, path %s", err, l.path(id))
	}
	os.Remove(filepath.Join(l.dir, "junk.mkv"))
	l.r.DismissFailedRename(l.ctx, id)
	if err := undo(); err != nil || l.path(id) != "junk.mkv" {
		t.Fatalf("undo: %v, path %s", err, l.path(id))
	}
	if got := l.scalar(`SELECT trigger || ':' || undo_of FROM file_renames WHERE video_id = ? ORDER BY id DESC LIMIT 1`, id); got != "undo:"+strconv.FormatInt(last.ID, 10) {
		t.Errorf("undo row = %s", got)
	}
	st, _ := l.r.FileNameStates(l.ctx)
	if !st[id].Locked || st[id].LockSource != repo.LockUndo {
		t.Errorf("state = %+v", st[id])
	}

	// An undo is itself the latest rename, so it can be undone (§39.10): back to the plan's name.
	last, _ = l.r.LatestDoneRename(l.ctx, id)
	if err := undo(); err != nil || l.path(id) != "Ada.mkv" {
		t.Fatalf("undo of an undo: %v, path %s", err, l.path(id))
	}
	if got := l.scalar(`SELECT trigger || ':' || undo_of FROM file_renames WHERE video_id = ? ORDER BY id DESC LIMIT 1`, id); got != "undo:"+strconv.FormatInt(last.ID, 10) {
		t.Errorf("undo-of-undo row = %s", got)
	}
}
