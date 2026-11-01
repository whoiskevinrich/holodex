package rename

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	d, _ := os.Open(dir)
	defer d.Close()
	n, _ := d.Readdirnames(-1)
	return n
}

func openLib(t *testing.T) (root, dir string, f *Folder) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "films")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFolder(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return root, dir, f
}

func noJournal(string) error { return nil }

// F1/F3: the platform primitive refuses an existing target; both files are untouched.
func TestMoveRefusesExisting(t *testing.T) {
	_, dir, f := openLib(t)
	write(t, dir, "a.mkv", "A")
	write(t, dir, "b.mkv", "B")
	if err := f.Move(1, "a.mkv", "b.mkv", noJournal); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if read(t, dir, "a.mkv") != "A" || read(t, dir, "b.mkv") != "B" {
		t.Fatal("a file changed")
	}
}

// F2: with the primitive unsupported (EINVAL, then ENOSYS), the listing check refuses an existing
// target compared case-folded and in NFC, and a free target is renamed.
func TestMoveFallback(t *testing.T) {
	for _, errno := range []error{syscall.EINVAL, syscall.ENOSYS} {
		_, dir, f := openLib(t)
		f.noReplace = func(string, string) error { return errno }
		write(t, dir, "a.mkv", "A")
		write(t, dir, "café.mkv", "C")
		write(t, dir, "ada.mkv", "D")
		f.list = func() ([]string, error) { return []string{"a.mkv", "café.mkv", "ada.mkv"}, nil }
		for _, dst := range []string{"Café.mkv", "ADA.mkv"} {
			if err := f.Move(1, "a.mkv", dst, noJournal); !errors.Is(err, ErrExists) {
				t.Errorf("%v: Move to %q = %v, want ErrExists", errno, dst, err)
			}
		}
		if err := f.Move(1, "a.mkv", "free.mkv", noJournal); err != nil {
			t.Fatalf("%v: free target: %v", errno, err)
		}
		if read(t, dir, "free.mkv") != "A" || read(t, dir, "ada.mkv") != "D" {
			t.Errorf("%v: wrong bytes after the fallback", errno)
		}
	}
}

// F7: the fallback defers to the filesystem: a holder the listing misses but Lstat finds refuses.
func TestMoveFallbackLstatsTarget(t *testing.T) {
	_, dir, f := openLib(t)
	f.noReplace = func(string, string) error { return syscall.EINVAL }
	write(t, dir, "a.mkv", "A")
	write(t, dir, "Ada.mkv", "H")
	f.list = func() ([]string, error) { return []string{"a.mkv"}, nil }
	if err := f.Move(1, "a.mkv", "Ada.mkv", noJournal); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if read(t, dir, "Ada.mkv") != "H" {
		t.Fatal("the holder changed")
	}
}

// F4 (+F3's case half): a case-only or composition-only change takes two steps through the via
// name, journalled before the second step; a 250-byte old name still gets a short via name.
func TestMoveCaseOnlyTwoSteps(t *testing.T) {
	for _, c := range []struct{ old, new string }{
		{"ada.mkv", "Ada.mkv"},
		{"Café.mkv", "Café.mkv"},
		{strings.Repeat("a", 246) + ".mkv", strings.Repeat("A", 246) + ".mkv"},
	} {
		_, dir, f := openLib(t)
		write(t, dir, c.old, "X")
		var journalled string
		err := f.Move(412, c.old, c.new, func(via string) error {
			journalled = via
			if !bytes.Equal([]byte(read(t, dir, via)), []byte("X")) {
				t.Error("the file isn't at the via name when it is journalled")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%q: %v", c.new, err)
		}
		if journalled != "hx-412.holodex-rename" || len(journalled) >= MaxNameBytes {
			t.Errorf("via = %q", journalled)
		}
		got := names(t, dir)
		if len(got) != 1 || got[0] != c.new {
			t.Errorf("folder = %q, want [%q]", got, c.new)
		}
	}
}

// F5: a symlinked current file, target or via name is refused and nothing moves.
func TestMoveRefusesSymlinks(t *testing.T) {
	_, dir, f := openLib(t)
	write(t, dir, "real.mkv", "R")
	if err := os.Symlink(filepath.Join(dir, "real.mkv"), filepath.Join(dir, "link.mkv")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := f.Move(1, "link.mkv", "x.mkv", noJournal); !errors.Is(err, ErrSymlink) {
		t.Errorf("symlinked source: %v", err)
	}
	if err := f.Move(1, "real.mkv", "link.mkv", noJournal); !errors.Is(err, ErrSymlink) && !errors.Is(err, ErrExists) {
		t.Errorf("symlinked target: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "real.mkv"), filepath.Join(dir, ViaName(1))); err != nil {
		t.Fatal(err)
	}
	if err := f.Move(1, "real.mkv", "REAL.mkv", noJournal); err == nil {
		t.Error("a symlinked via name was used")
	}
	if read(t, dir, "real.mkv") != "R" {
		t.Error("the file moved")
	}
}

// F6: a folder resolving outside the library is refused; one resolving inside is allowed.
func TestOpenFolderRootBound(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	inner := filepath.Join(root, "inner")
	os.MkdirAll(inner, 0o755)
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := os.Symlink(inner, filepath.Join(root, "in")); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFolder(root, filepath.Join(root, "out")); !errors.Is(err, ErrOutsideLib) {
		t.Errorf("outside: %v", err)
	}
	f, err := OpenFolder(root, filepath.Join(root, "in"))
	if err != nil {
		t.Fatalf("inside: %v", err)
	}
	f.Close()
	if _, err := OpenFolder(root, outside); !errors.Is(err, ErrOutsideLib) {
		t.Errorf("a plain outside folder: %v", err)
	}
	_ = runtime.GOOS
}
