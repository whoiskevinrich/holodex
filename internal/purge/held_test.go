package purge

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"holodex/internal/repo"
)

// A video with a held original (F77, spec R8): purge follows delete_remove_files for it too.

func heldRig(t *testing.T, state string) (string, *fakeRepo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.mkv")
	if err := os.WriteFile(path, []byte("flipped"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+repo.HeldPathSuffix, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, &fakeRepo{
		expired: []repo.TrashItem{{ID: 1, FilePath: path}},
		paths:   map[int64]string{1: path},
		held:    map[int64]*repo.HeldOriginal{1: {VideoID: 1, HeldPath: path + repo.HeldPathSuffix, State: state}},
	}
}

func readOr(name string) string {
	b, err := os.ReadFile(name)
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

func TestPurge_HeldOriginal_RemoveFilesOnDeletesBoth(t *testing.T) {
	path, fr := heldRig(t, repo.HeldHeld)
	New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil).Sweep(context.Background())
	if readOr(path) != "<missing>" || readOr(path+repo.HeldPathSuffix) != "<missing>" {
		t.Errorf("left on disk: path=%q held=%q", readOr(path), readOr(path+repo.HeldPathSuffix))
	}
	if !slices.Contains(fr.hardDeleted, 1) {
		t.Error("row not purged")
	}
}

func TestPurge_HeldOriginal_RemoveFilesOffPutsTheOriginalBack(t *testing.T) {
	path, fr := heldRig(t, repo.HeldHeld)
	New(fr, Config{Grace: time.Hour, RemoveFiles: false}, nil).Sweep(context.Background())
	if readOr(path) != "original" || readOr(path+repo.HeldPathSuffix) != "<missing>" {
		t.Errorf("path=%q held=%q, want the original back in place", readOr(path), readOr(path+repo.HeldPathSuffix))
	}
	if !slices.Contains(fr.hardDeleted, 1) {
		t.Error("row not purged")
	}
}

// Mid-swap, mid-restore or mid-discard: purge waits rather than guess which file is which.
func TestPurge_HeldOriginal_UnsettledWaits(t *testing.T) {
	for _, state := range []string{repo.HeldSwapping, repo.HeldRestoring, repo.HeldDiscarding} {
		path, fr := heldRig(t, state)
		New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil).Sweep(context.Background())
		if readOr(path) != "flipped" || readOr(path+repo.HeldPathSuffix) != "original" {
			t.Errorf("%s: files touched", state)
		}
		if slices.Contains(fr.hardDeleted, 1) {
			t.Errorf("%s: row purged while the hold is unsettled", state)
		}
		if len(fr.runs) != 1 || fr.runs[0].Errors != 1 {
			t.Errorf("%s: the wait isn't visible in activity: %+v", state, fr.runs)
		}
	}
}

// A flip queued or running on the file: purge waits for the lane rather than delete under it.
func TestPurge_WaitsForAMaintenanceJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.mkv")
	os.WriteFile(path, []byte("x"), 0o644)
	fr := &fakeRepo{
		expired:   []repo.TrashItem{{ID: 1, FilePath: path}},
		paths:     map[int64]string{1: path},
		maintBusy: map[int64]bool{1: true},
	}
	New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil).Sweep(context.Background())
	if readOr(path) != "x" || slices.Contains(fr.hardDeleted, 1) {
		t.Error("purged a file a maintenance job is working on")
	}
}

// A journal row that names some other file steers nothing.
func TestPurge_HeldOriginal_MismatchedRowWaits(t *testing.T) {
	path, fr := heldRig(t, repo.HeldHeld)
	victim := filepath.Join(t.TempDir(), "victim")
	os.WriteFile(victim, []byte("keep"), 0o644)
	fr.held[1].HeldPath = victim
	New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil).Sweep(context.Background())
	if readOr(victim) != "keep" || readOr(path) != "flipped" || slices.Contains(fr.hardDeleted, 1) {
		t.Error("a mismatched row steered purge")
	}
}

func TestPurge_HeldOriginal_SymlinkNotFollowed(t *testing.T) {
	path, fr := heldRig(t, repo.HeldHeld)
	os.Remove(path + repo.HeldPathSuffix)
	target := filepath.Join(t.TempDir(), "precious")
	os.WriteFile(target, []byte("keep"), 0o644)
	if err := os.Symlink(target, path+repo.HeldPathSuffix); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	New(fr, Config{Grace: time.Hour, RemoveFiles: false}, nil).Sweep(context.Background())
	if readOr(target) != "keep" || readOr(path) != "flipped" || slices.Contains(fr.hardDeleted, 1) {
		t.Error("purge followed or moved a symlinked held original")
	}
}
