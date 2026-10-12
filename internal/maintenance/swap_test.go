package maintenance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/repo"
)

// memStore is an in-memory held_originals journal.
type memStore struct{ rows map[int64]*repo.HeldOriginal }

func newMemStore() *memStore { return &memStore{rows: map[int64]*repo.HeldOriginal{}} }

func (m *memStore) GetHeldOriginal(_ context.Context, id int64) (*repo.HeldOriginal, error) {
	if h, ok := m.rows[id]; ok {
		c := *h
		return &c, nil
	}
	return nil, nil
}

func (m *memStore) ListHeldOriginals(_ context.Context, state string) ([]repo.HeldOriginal, error) {
	var out []repo.HeldOriginal
	for _, h := range m.rows {
		if h.State == state {
			out = append(out, *h)
		}
	}
	return out, nil
}

func (m *memStore) BeginSwap(_ context.Context, id int64, held, op string, now, exp time.Time) error {
	if _, ok := m.rows[id]; ok {
		return errors.New("row exists")
	}
	m.rows[id] = &repo.HeldOriginal{VideoID: id, HeldPath: held, Op: op, State: repo.HeldSwapping, CreatedAt: now, ExpiresAt: exp}
	return nil
}

func (m *memStore) SetHeldState(_ context.Context, id int64, state string) error {
	h, ok := m.rows[id]
	if !ok {
		return errors.New("no row")
	}
	h.State = state
	return nil
}

func (m *memStore) DeleteHeldOriginal(_ context.Context, id int64) error {
	delete(m.rows, id)
	return nil
}

var t0 = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

const week = 7 * 24 * time.Hour

// rig is a library file with a verified candidate beside it.
type rig struct {
	path  string
	store *memStore
	s     *Swapper
}

func newRig(t *testing.T) *rig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.mkv")
	write(t, path, "original")
	write(t, NewPath(path), "flipped")
	st := newMemStore()
	s := NewSwapper(st, week)
	s.now = func() time.Time { return t0 }
	return &rig{path: path, store: st, s: s}
}

func write(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(name string) string {
	b, err := os.ReadFile(name)
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

// failNth makes the nth rename (1-based) fail.
func failNth(s *Swapper, n int) {
	calls := 0
	s.rename = func(a, b string) error {
		calls++
		if calls == n {
			return errors.New("injected rename failure")
		}
		return os.Rename(a, b)
	}
}

func TestSwap_HappyPath(t *testing.T) {
	r := newRig(t)
	if err := r.s.Swap(context.Background(), 1, r.path, "hflip"); err != nil {
		t.Fatal(err)
	}
	if read(r.path) != "flipped" || read(HeldPath(r.path)) != "original" || read(NewPath(r.path)) != "<missing>" {
		t.Errorf("disk: path=%q held=%q new=%q", read(r.path), read(HeldPath(r.path)), read(NewPath(r.path)))
	}
	h := r.store.rows[1]
	if h == nil || h.State != repo.HeldHeld || h.HeldPath != HeldPath(r.path) || !h.ExpiresAt.Equal(t0.Add(week)) {
		t.Errorf("journal row = %+v", h)
	}
}

func TestSwap_FirstRenameFails_NothingMoves(t *testing.T) {
	r := newRig(t)
	failNth(r.s, 1)
	if err := r.s.Swap(context.Background(), 1, r.path, "hflip"); err == nil {
		t.Fatal("swap succeeded")
	}
	if read(r.path) != "original" || r.store.rows[1] != nil {
		t.Errorf("path=%q row=%+v", read(r.path), r.store.rows[1])
	}
}

func TestSwap_SecondRenameFails_RolledBack(t *testing.T) {
	r := newRig(t)
	failNth(r.s, 2)
	if err := r.s.Swap(context.Background(), 1, r.path, "hflip"); err == nil {
		t.Fatal("swap succeeded")
	}
	if read(r.path) != "original" || read(HeldPath(r.path)) != "<missing>" || r.store.rows[1] != nil {
		t.Errorf("path=%q held=%q row=%+v", read(r.path), read(HeldPath(r.path)), r.store.rows[1])
	}
}

// Rename 2 and its rollback both fail: the row stays swapping, and boot recovery finishes the
// swap because the candidate is still there.
func TestSwap_RollbackFails_RecoveryFinishes(t *testing.T) {
	r := newRig(t)
	calls := 0
	r.s.rename = func(a, b string) error {
		calls++
		if calls >= 2 {
			return errors.New("injected")
		}
		return os.Rename(a, b)
	}
	if err := r.s.Swap(context.Background(), 1, r.path, "hflip"); err == nil {
		t.Fatal("swap succeeded")
	}
	if r.store.rows[1] == nil || r.store.rows[1].State != repo.HeldSwapping {
		t.Fatalf("row = %+v, want swapping", r.store.rows[1])
	}
	r.s.rename = os.Rename
	if n, err := r.s.RecoverSwaps(context.Background()); err != nil || n != 1 {
		t.Fatalf("recovered %d, err %v", n, err)
	}
	if read(r.path) != "flipped" || read(HeldPath(r.path)) != "original" || r.store.rows[1].State != repo.HeldHeld {
		t.Errorf("path=%q held=%q row=%+v", read(r.path), read(HeldPath(r.path)), r.store.rows[1])
	}
}

// Each crash point, reconstructed on disk with a swapping row, settles to one whole version
// of the file and loses no bytes.
func TestRecoverSwaps_EveryCrashPoint(t *testing.T) {
	cases := []struct {
		name               string
		path, held, newp   string // "" = absent
		wantPath, wantHeld string
		wantState          string // "" = row deleted
	}{
		{"before the first rename", "original", "", "flipped", "original", "<missing>", ""},
		{"between the renames", "", "original", "flipped", "flipped", "original", repo.HeldHeld},
		{"between, candidate swept", "", "original", "", "original", "<missing>", ""},
		{"after both, state write lost", "flipped", "original", "", "flipped", "original", repo.HeldHeld},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.mkv")
			for name, body := range map[string]string{path: c.path, HeldPath(path): c.held, NewPath(path): c.newp} {
				if body != "" {
					write(t, name, body)
				}
			}
			st := newMemStore()
			st.rows[1] = &repo.HeldOriginal{VideoID: 1, HeldPath: HeldPath(path), State: repo.HeldSwapping}
			if _, err := NewSwapper(st, week).RecoverSwaps(context.Background()); err != nil {
				t.Fatal(err)
			}
			if read(path) != c.wantPath || read(HeldPath(path)) != c.wantHeld {
				t.Errorf("path=%q held=%q, want %q/%q", read(path), read(HeldPath(path)), c.wantPath, c.wantHeld)
			}
			got := ""
			if h := st.rows[1]; h != nil {
				got = h.State
			}
			if got != c.wantState {
				t.Errorf("row state %q, want %q", got, c.wantState)
			}
		})
	}
}

func TestRecoverSwaps_NothingOnDiskKeepsTheRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.mkv")
	st := newMemStore()
	st.rows[1] = &repo.HeldOriginal{VideoID: 1, HeldPath: HeldPath(path), State: repo.HeldSwapping}
	if _, err := NewSwapper(st, week).RecoverSwaps(context.Background()); err == nil {
		t.Error("recovery claimed success with neither file present")
	}
	if st.rows[1] == nil {
		t.Error("the only record of the lost file was deleted")
	}
}

func TestRecoverSwaps_InterruptedRestoreAndDiscard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.mkv")
	write(t, path, "flipped")
	write(t, HeldPath(path), "original")
	st := newMemStore()
	st.rows[1] = &repo.HeldOriginal{VideoID: 1, HeldPath: HeldPath(path), State: repo.HeldRestoring}
	if _, err := NewSwapper(st, week).RecoverSwaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	if read(path) != "original" || st.rows[1] != nil {
		t.Errorf("restore not finished: path=%q row=%+v", read(path), st.rows[1])
	}

	write(t, HeldPath(path), "original")
	st.rows[2] = &repo.HeldOriginal{VideoID: 2, HeldPath: HeldPath(path), State: repo.HeldDiscarding}
	if _, err := NewSwapper(st, week).RecoverSwaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	if read(HeldPath(path)) != "<missing>" || st.rows[2] != nil {
		t.Errorf("discard not finished: held=%q row=%+v", read(HeldPath(path)), st.rows[2])
	}
}

// A second operation while a hold stands keeps the FIRST original and the first expiry.
func TestSwap_SecondOperationKeepsTheFirstOriginal(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if err := r.s.Swap(ctx, 1, r.path, "hflip"); err != nil {
		t.Fatal(err)
	}
	write(t, NewPath(r.path), "flipped twice")
	r.s.now = func() time.Time { return t0.Add(48 * time.Hour) }
	if err := r.s.Swap(ctx, 1, r.path, "hflip"); err != nil {
		t.Fatal(err)
	}
	if read(r.path) != "flipped twice" || read(HeldPath(r.path)) != "original" {
		t.Errorf("path=%q held=%q", read(r.path), read(HeldPath(r.path)))
	}
	if !r.store.rows[1].ExpiresAt.Equal(t0.Add(week)) {
		t.Errorf("window restarted: expires %v", r.store.rows[1].ExpiresAt)
	}
}

func TestSwap_UnjournaledHeldFileIsNeverOverwritten(t *testing.T) {
	r := newRig(t)
	write(t, HeldPath(r.path), "someone's earlier original")
	if err := r.s.Swap(context.Background(), 1, r.path, "hflip"); !errors.Is(err, ErrHeldMismatch) {
		t.Fatalf("err = %v, want ErrHeldMismatch", err)
	}
	if read(HeldPath(r.path)) != "someone's earlier original" || read(r.path) != "original" {
		t.Error("files moved")
	}
}

func TestRestoreAndDiscard(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	if err := r.s.Swap(ctx, 1, r.path, "hflip"); err != nil {
		t.Fatal(err)
	}
	if err := r.s.Restore(ctx, 1, r.path); err != nil {
		t.Fatal(err)
	}
	if read(r.path) != "original" || read(HeldPath(r.path)) != "<missing>" || r.store.rows[1] != nil {
		t.Errorf("after restore: path=%q held=%q row=%+v", read(r.path), read(HeldPath(r.path)), r.store.rows[1])
	}

	write(t, NewPath(r.path), "flipped")
	if err := r.s.Swap(ctx, 1, r.path, "hflip"); err != nil {
		t.Fatal(err)
	}
	if err := r.s.Discard(ctx, 1, r.path); err != nil {
		t.Fatal(err)
	}
	if read(r.path) != "flipped" || read(HeldPath(r.path)) != "<missing>" || r.store.rows[1] != nil {
		t.Errorf("after discard: path=%q held=%q row=%+v", read(r.path), read(HeldPath(r.path)), r.store.rows[1])
	}
}

// A failed restore or discard leaves the hold standing, so the owner can retry (and the expiry
// pass keeps trying) without waiting for a reboot.
func TestRestoreAndDiscard_FailureKeepsTheHold(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	if err := r.s.Swap(ctx, 1, r.path, "hflip"); err != nil {
		t.Fatal(err)
	}
	failNth(r.s, 1)
	if err := r.s.Restore(ctx, 1, r.path); err == nil {
		t.Fatal("restore succeeded")
	}
	if st := r.store.rows[1].State; st != repo.HeldHeld {
		t.Fatalf("after a failed restore the row is %q", st)
	}
	r.s.remove = func(string) error { return errors.New("injected remove failure") }
	if err := r.s.Discard(ctx, 1, r.path); err == nil {
		t.Fatal("discard succeeded")
	}
	if st := r.store.rows[1].State; st != repo.HeldHeld || read(HeldPath(r.path)) != "original" {
		t.Fatalf("after a failed discard: row %q, held %q", st, read(HeldPath(r.path)))
	}
	r.s.rename = os.Rename
	if err := r.s.Restore(ctx, 1, r.path); err != nil || read(r.path) != "original" {
		t.Errorf("retry of restore: %v, path %q", err, read(r.path))
	}
}

// A journal row whose held_path disagrees with the video's current path steers nothing.
func TestRestoreAndDiscard_RefuseAMismatchedRow(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	elsewhere := filepath.Join(t.TempDir(), "victim")
	write(t, elsewhere, "do not touch")
	r.store.rows[1] = &repo.HeldOriginal{VideoID: 1, HeldPath: elsewhere, State: repo.HeldHeld}
	if err := r.s.Restore(ctx, 1, r.path); !errors.Is(err, ErrHeldMismatch) {
		t.Errorf("restore err = %v", err)
	}
	if err := r.s.Discard(ctx, 1, r.path); !errors.Is(err, ErrHeldMismatch) {
		t.Errorf("discard err = %v", err)
	}
	if read(elsewhere) != "do not touch" || read(r.path) != "original" {
		t.Error("a mismatched row moved or deleted a file")
	}
}

func TestSwap_RefusesASymlinkedHeldPath(t *testing.T) {
	r := newRig(t)
	target := filepath.Join(t.TempDir(), "precious")
	write(t, target, "keep")
	if err := os.Symlink(target, HeldPath(r.path)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := r.s.Swap(context.Background(), 1, r.path, "hflip"); !errors.Is(err, ErrSymlinkedSibling) {
		t.Fatalf("err = %v", err)
	}
	if read(target) != "keep" || read(r.path) != "original" {
		t.Error("files moved through the link")
	}
}
