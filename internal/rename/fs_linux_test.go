//go:build linux

package rename

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// F1: renameat2 with RENAME_NOREPLACE refuses an existing target itself (no fallback).
func TestRenameat2RefusesExisting(t *testing.T) {
	_, dir, f := openLib(t)
	write(t, dir, "a.mkv", "A")
	write(t, dir, "b.mkv", "B")
	if err := f.noReplace("a.mkv", "b.mkv"); err == nil {
		t.Fatal("renameat2 replaced an existing target")
	}
	if err := f.Move(1, "a.mkv", "b.mkv", noJournal); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if read(t, dir, "a.mkv") != "A" || read(t, dir, "b.mkv") != "B" {
		t.Fatal("a file changed")
	}
}

// F8: the folder is swapped for a symlink to a directory outside the library between the root
// check and the rename. The rename lands in the original folder, and nothing appears outside.
func TestFolderSwapCantRedirect(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(root, "films")
	os.MkdirAll(dir, 0o755)
	write(t, dir, "a.mkv", "A")
	moved := filepath.Join(root, "films-moved")
	afterRootCheck = func() {
		if err := os.Rename(dir, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, dir); err != nil {
			t.Fatal(err)
		}
		write(t, outside, "a.mkv", "decoy")
	}
	t.Cleanup(func() { afterRootCheck = func() {} })
	f, err := OpenFolder(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Move(1, "a.mkv", "b.mkv", noJournal); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if read(t, moved, "b.mkv") != "A" {
		t.Error("the rename did not land in the original folder")
	}
	if got := names(t, outside); len(got) != 1 || got[0] != "a.mkv" {
		t.Errorf("outside the library: %v", got)
	}
}
