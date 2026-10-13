package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// moveFixture is a temp media root with a scanner over a fake repo
// (testing-strategy §26.3, "Scanner, fake repo").
type moveFixture struct {
	t   *testing.T
	dir string
	fr  *fakeRepo
	s   *Scanner
	ft  *fakeThumbnailer
}

func newMoveFixture(t *testing.T) *moveFixture {
	t.Helper()
	dir := t.TempDir()
	fr := newFakeRepo()
	s := New(Config{MediaPath: dir, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), fr, stubExtractor{})
	ft := &fakeThumbnailer{}
	s.SetThumbnailer(ft)
	return &moveFixture{t: t, dir: dir, fr: fr, s: s, ft: ft}
}

func (m *moveFixture) path(rel string) string { return filepath.Join(m.dir, filepath.FromSlash(rel)) }

// write puts content at rel with an mtime old enough to pass the MinAge guard.
func (m *moveFixture) write(rel string, b []byte) {
	m.t.Helper()
	p := m.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		m.t.Fatal(err)
	}
	writeAged(m.t, p, b, time.Hour)
}

func (m *moveFixture) move(from, to string) {
	m.t.Helper()
	if err := os.MkdirAll(filepath.Dir(m.path(to)), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.Rename(m.path(from), m.path(to)); err != nil {
		m.t.Fatal(err)
	}
}

func (m *moveFixture) copy(from, to string) {
	m.t.Helper()
	b, err := os.ReadFile(m.path(from))
	if err != nil {
		m.t.Fatal(err)
	}
	m.write(to, b)
}

func (m *moveFixture) remove(rel string) {
	m.t.Helper()
	if err := os.Remove(m.path(rel)); err != nil {
		m.t.Fatal(err)
	}
}

func (m *moveFixture) scan() {
	m.t.Helper()
	if err := m.s.ScanOnce(context.Background()); err != nil {
		m.t.Fatalf("scan: %v", err)
	}
}

// id returns the row id at rel, failing if there is none.
func (m *moveFixture) id(rel string) int64 {
	m.t.Helper()
	st, ok, _ := m.fr.StatByPath(context.Background(), m.path(rel))
	if !ok {
		m.t.Fatalf("no row at %s", rel)
	}
	return st.ID
}

func (m *moveFixture) hasRow(rel string) bool {
	_, ok, _ := m.fr.StatByPath(context.Background(), m.path(rel))
	return ok
}

func (m *moveFixture) active(id int64) bool {
	m.fr.mu.Lock()
	defer m.fr.mu.Unlock()
	return m.fr.active[id]
}

func (m *moveFixture) enqueued() int {
	m.ft.mu.Lock()
	defer m.ft.mu.Unlock()
	return len(m.ft.enqueued)
}

const moveSize = 1 << 20

func TestScanRepointsMovedFile(t *testing.T) {
	m := newMoveFixture(t)
	m.write("a.mp4", seededBytes(20, moveSize))
	m.scan()
	id, upserts, thumbs := m.id("a.mp4"), m.fr.uparts, m.enqueued()

	m.move("a.mp4", "sub/b.mp4")
	m.scan()

	if got := m.id("sub/b.mp4"); got != id {
		t.Fatalf("moved file id = %d, want %d", got, id)
	}
	if m.hasRow("a.mp4") {
		t.Error("old path still has a row")
	}
	if m.fr.uparts != upserts {
		t.Errorf("upserts = %d, want %d (a move inserts nothing)", m.fr.uparts, upserts)
	}
	if m.fr.repoints != 1 {
		t.Errorf("repoints = %d, want 1", m.fr.repoints)
	}
	if !m.active(id) {
		t.Error("repointed row is hidden")
	}
	if m.enqueued() != thumbs {
		t.Error("a move re-enqueued thumbnail work")
	}
}

func TestScanCopyIsNotAMove(t *testing.T) {
	m := newMoveFixture(t)
	m.write("a.mp4", seededBytes(21, moveSize))
	m.scan()
	id := m.id("a.mp4")

	m.copy("a.mp4", "b.mp4")
	m.scan()

	if m.id("a.mp4") != id || !m.active(id) {
		t.Error("original disturbed by a copy")
	}
	if m.id("b.mp4") == id {
		t.Error("copy took the original's id")
	}
	if m.fr.repoints != 0 {
		t.Errorf("repoints = %d, want 0", m.fr.repoints)
	}
}

func TestScanSizeTwinThatIsNotAMoveInserts(t *testing.T) {
	m := newMoveFixture(t)
	m.write("a.mp4", seededBytes(22, moveSize))
	m.scan()
	id := m.id("a.mp4")

	m.write("b.mp4", seededBytes(23, moveSize)) // same size, different content
	m.scan()

	if m.id("b.mp4") == id || m.id("a.mp4") != id {
		t.Error("size twin was treated as the same video")
	}
}

// One new file matches two missing rows: nothing is guessed (P0-4).
func TestScanAmbiguousTwoMissingRows(t *testing.T) {
	m := newMoveFixture(t)
	b := seededBytes(24, moveSize)
	m.write("x.mp4", b)
	m.write("y.mp4", b)
	m.scan()
	x, y := m.id("x.mp4"), m.id("y.mp4")

	m.remove("x.mp4")
	m.remove("y.mp4")
	m.write("z.mp4", b)
	m.scan()

	if z := m.id("z.mp4"); z == x || z == y {
		t.Fatalf("ambiguous file took id %d", z)
	}
	if m.active(x) || m.active(y) {
		t.Error("missing rows should end hidden")
	}
	if m.fr.repoints != 0 {
		t.Errorf("repoints = %d, want 0", m.fr.repoints)
	}
}

// One missing row is claimed by two new files (a move plus a copy): neither
// takes it, so worker order can't decide (P0-4).
func TestScanAmbiguousTwoNewFiles(t *testing.T) {
	m := newMoveFixture(t)
	m.write("a.mp4", seededBytes(25, moveSize))
	m.scan()
	id := m.id("a.mp4")

	m.move("a.mp4", "b.mp4")
	m.copy("b.mp4", "c.mp4")
	m.scan()

	if m.id("b.mp4") == id || m.id("c.mp4") == id {
		t.Fatal("a claimant took the contested id")
	}
	if m.active(id) {
		t.Error("contested row should end hidden")
	}
}

func TestScanRecognisesMoveAfterLongAbsence(t *testing.T) {
	m := newMoveFixture(t)
	b := seededBytes(26, moveSize)
	m.write("a.mp4", b)
	m.write("bystander.mp4", seededBytes(29, 2*moveSize)) // keeps the walk non-empty
	m.scan()
	id := m.id("a.mp4")

	m.remove("a.mp4")
	m.scan() // hidden as missing
	if m.active(id) {
		t.Fatal("row not hidden after its file went missing")
	}

	m.write("later/a.mp4", b)
	m.scan()
	if got := m.id("later/a.mp4"); got != id {
		t.Fatalf("returning file id = %d, want %d", got, id)
	}
	if !m.active(id) {
		t.Error("returning row still hidden")
	}
}

func TestScanNeverRepointsTrashedRow(t *testing.T) {
	m := newMoveFixture(t)
	m.write("a.mp4", seededBytes(27, moveSize))
	m.scan()
	id := m.id("a.mp4")
	m.fr.markDeleted(id)

	m.move("a.mp4", "b.mp4")
	m.scan()

	if m.id("b.mp4") == id {
		t.Fatal("a trashed row was repointed")
	}
}

// A moved file still inside the MinAge window is skipped, so its row is hidden
// for one pass; the next pass repoints it.
func TestScanMoveDuringMinAgeWait(t *testing.T) {
	m := newMoveFixture(t)
	m.write("a.mp4", seededBytes(28, moveSize))
	m.scan()
	id := m.id("a.mp4")

	m.move("a.mp4", "b.mp4")
	now := time.Now()
	if err := os.Chtimes(m.path("b.mp4"), now, now); err != nil {
		t.Fatal(err)
	}
	m.scan()
	if m.hasRow("b.mp4") {
		t.Fatal("young file indexed inside the MinAge window")
	}

	old := now.Add(-time.Hour)
	if err := os.Chtimes(m.path("b.mp4"), old, old); err != nil {
		t.Fatal(err)
	}
	m.scan()
	if got := m.id("b.mp4"); got != id || !m.active(id) {
		t.Fatalf("after the wait: id %d active %v, want %d active", got, m.active(id), id)
	}
}
