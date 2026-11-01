//go:build !linux

package rename

import (
	"os"
	"path/filepath"
)

// afterRootCheck runs between the root check and the first rename (F8 runs on Linux only).
var afterRootCheck = func() {}

// OpenFolder checks dir against the library root and binds the folder's calls to its path. Off
// Linux this is the development primitive, path-based by design (security-perimeter.md).
func OpenFolder(root, dir string) (*Folder, error) {
	croot, err := canonicalRoot(root)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	if real, err = filepath.Abs(real); err != nil {
		return nil, err
	}
	if !inside(croot, real) {
		return nil, ErrOutsideLib
	}
	afterRootCheck()
	join := func(n string) string { return filepath.Join(real, n) }
	return &Folder{Path: real, ops: ops{
		noReplace: func(old, new string) error { return noReplace(join(old), join(new)) },
		rename:    func(old, new string) error { return os.Rename(join(old), join(new)) },
		lstat:     func(name string) (os.FileInfo, error) { return os.Lstat(join(name)) },
		list: func() ([]string, error) {
			d, err := os.Open(real)
			if err != nil {
				return nil, err
			}
			defer d.Close()
			return d.Readdirnames(-1)
		},
	}}, nil
}
