//go:build linux

package rename

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// renameat2 is the no-replace call; a test swaps it to drive the fallback.
var renameat2 = unix.Renameat2

// afterRootCheck runs between the root check and the first rename; a test swaps the folder there
// (F8).
var afterRootCheck = func() {}

// OpenFolder opens dir once and checks the library bound on the descriptor's own path, so a folder
// swapped for a symlink afterwards can't redirect the rename (security-perimeter.md).
func OpenFolder(root, dir string) (*Folder, error) {
	croot, err := canonicalRoot(root)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: dir, Err: err}
	}
	fdPath := "/proc/self/fd/" + strconv.Itoa(fd)
	real, err := os.Readlink(fdPath)
	if err != nil || !inside(croot, real) {
		unix.Close(fd)
		if err != nil {
			return nil, fmt.Errorf("resolve folder: %w", err)
		}
		return nil, ErrOutsideLib
	}
	afterRootCheck()
	f := &Folder{Path: real, close: func() error { return unix.Close(fd) }}
	f.ops = ops{
		noReplace: func(old, new string) error {
			return renameat2(fd, old, fd, new, unix.RENAME_NOREPLACE)
		},
		rename: func(old, new string) error { return unix.Renameat(fd, old, fd, new) },
		// The descriptor's /proc path resolves to the opened folder itself, whatever now sits at
		// its old path, and gives os.FileInfo values os.SameFile can compare.
		lstat: func(name string) (os.FileInfo, error) { return os.Lstat(fdPath + "/" + name) },
		list: func() ([]string, error) {
			d, err := os.Open(fdPath)
			if err != nil {
				return nil, err
			}
			defer d.Close()
			return d.Readdirnames(-1)
		},
	}
	return f, nil
}
