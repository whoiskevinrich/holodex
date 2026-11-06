package rename

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Filesystem refusals. None of them ever moves a file.
var (
	ErrExists     = errors.New("the target name is taken")
	ErrSymlink    = errors.New("a symlink is never renamed or renamed onto")
	ErrOutsideLib = errors.New("the folder is outside the library")
)

// ViaSuffix marks the half-way name of a two-step rename. It is not a media extension, so the
// scanner never ingests it, and the queue's orphan sweep never removes it.
const ViaSuffix = ".holodex-rename"

// ViaName is a video's half-way name. It is built from the id, so it stays short and carries no
// metadata (architecture "No-overwrite rename").
func ViaName(videoID int64) string { return "hx-" + strconv.FormatInt(videoID, 10) + ViaSuffix }

// ops are one folder's filesystem calls, every name relative to that folder. Linux binds them to
// a folder descriptor; other platforms to the folder's path.
type ops struct {
	// noReplace renames without ever replacing the target. It returns EINVAL or ENOSYS where
	// the mount can't, and the move falls back to checking the folder itself.
	noReplace func(old, new string) error
	rename    func(old, new string) error
	lstat     func(name string) (os.FileInfo, error)
	list      func() ([]string, error)
}

func unsupported(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOSYS)
}

// move renames src to dst in the folder, refusing a symlink on either side and never replacing an
// existing dst.
func (o ops) move(src, dst string) error {
	si, err := o.lstat(src)
	if err != nil {
		return err
	}
	if si.Mode()&os.ModeSymlink != 0 {
		return ErrSymlink
	}
	if di, err := o.lstat(dst); err == nil && di.Mode()&os.ModeSymlink != 0 {
		return ErrSymlink
	}
	err = o.noReplace(src, dst)
	switch {
	case err == nil:
		return nil
	case !unsupported(err):
		if errors.Is(err, os.ErrExist) {
			return ErrExists
		}
		return err
	}
	// The mount can't refuse for us. Compare every entry case-folded and in NFC, then let the
	// filesystem apply its own equivalence: anything that answers to dst and isn't src refuses.
	names, err := o.list()
	if err != nil {
		return err
	}
	key := FoldKey(dst)
	for _, n := range names {
		if n != src && FoldKey(n) == key {
			return ErrExists
		}
	}
	di, err := o.lstat(dst)
	switch {
	case err == nil && !os.SameFile(si, di):
		return ErrExists
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}
	return o.rename(src, dst)
}

// Folder is an opened folder inside the library. Every Lstat and rename for one job goes through it.
type Folder struct {
	Path string
	ops
	close func() error
}

// Close releases the folder.
func (f *Folder) Close() error {
	if f.close == nil {
		return nil
	}
	return f.close()
}

// Exists reports whether name answers in the folder, without following a symlink.
func (f *Folder) Exists(name string) (bool, error) {
	_, err := f.lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Move renames old to new in the folder without overwriting. A case- or composition-only change
// always takes two steps through the via name, and journal records the via name before the second
// step. A rename resumed from the via name passes old == ViaName(videoID).
func (f *Folder) Move(videoID int64, old, new string, journal func(via string) error) error {
	via := ViaName(videoID)
	if old != via && FoldKey(old) == FoldKey(new) {
		if err := f.move(old, via); err != nil {
			return err
		}
		if err := journal(via); err != nil {
			return err
		}
		old = via
	}
	return f.move(old, new)
}

// StepAside moves a file to its via name so a swap partner can take its name.
func (f *Folder) StepAside(videoID int64, old string) error { return f.move(old, ViaName(videoID)) }

// inside reports whether canonical path p is root itself or below it.
func inside(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

// canonicalRoot resolves the library root.
func canonicalRoot(root string) (string, error) {
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("library root: %w", err)
	}
	return filepath.Abs(r)
}
