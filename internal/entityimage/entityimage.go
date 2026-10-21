// Package entityimage is the shared on-disk layout for a self-hosted entity image
// (person/studio/film, HOLODEX-286): path building, atomic write, and removal under
// {dir}/{entityID}/{imageID}.jpg or .png. It deliberately does NOT reimplement the
// untrusted-bytes normalization — that security spine (sniff-decode,
// decompression-bomb guard, re-encode metadata strip) lives once in
// personimage.Normalize/Hash and is reused by every caller, so every entity image
// gets byte-for-byte the same hardening. The per-entity packages
// (personimage/studioimage/filmimage) each keep their own Find/Store/Remove as
// thin wrappers delegating here — this package is an implementation detail, not a
// new call surface; nothing above those wrappers changes.
//
// The extension is derived from the normalized bytes, never stored (ADR-097): a
// non-opaque image is re-encoded to PNG so a logo's transparency survives, an opaque
// one to JPEG. Since an image id is server-assigned and never reused, exactly one of
// the two files exists for an id — Find stats for it, Remove clears both.
package entityimage

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// exts lists the on-disk extensions in lookup order (.jpg first: the overwhelmingly
// common case, and the only one that existed before ADR-097).
var exts = []string{".jpg", ".png"}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// Ext is the on-disk extension for normalized bytes: ".png" for a PNG (the
// alpha-preserving path), ".jpg" otherwise. Only ever fed personimage.Normalize's own
// output, so an 8-byte signature check is exact — this is not a sniff of untrusted
// input.
func Ext(data []byte) string {
	if bytes.HasPrefix(data, pngSignature) {
		return ".png"
	}
	return ".jpg"
}

// ContentType is the MIME type to serve a stored image with, from its extension.
// Explicit (not mime.TypeByExtension, which reads the OS registry on Windows) because
// the response also sets X-Content-Type-Options: nosniff.
func ContentType(path string) string {
	if filepath.Ext(path) == ".png" {
		return "image/png"
	}
	return "image/jpeg"
}

// Path is the on-disk location for one of an entity's images (ADR-014):
// {dir}/{entityID}/{imageID}{ext}. Both ids are server-assigned integers, never a
// request value, so path traversal is structurally impossible (the ADR-038 rule).
// The per-entity subdir is NOT created here — Store creates it.
func Path(dir string, entityID, imageID int64, ext string) string {
	return filepath.Join(dir, strconv.FormatInt(entityID, 10), strconv.FormatInt(imageID, 10)+ext)
}

// Find returns the path of the stored file for an image id, whichever extension it
// was written with. A missing file returns an error wrapping os.ErrNotExist so a
// serving handler can 404 it exactly as it did a missing .jpg.
func Find(dir string, entityID, imageID int64) (string, error) {
	var err error
	for _, ext := range exts {
		p := Path(dir, entityID, imageID, ext)
		if _, err = os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("find entity image: %w", err)
}

func entityDir(dir string, entityID int64) string {
	return filepath.Join(dir, strconv.FormatInt(entityID, 10))
}

// Store writes normalized bytes to Path(dir, entityID, imageID, Ext(data)),
// creating the per-entity subdir, via a temp file + rename so a reader never sees a
// torn file (mirrors the thumbnail manager's atomic write). The caller has already
// inserted the DB row, so imageID is the authoritative, server-assigned name.
func Store(dir string, entityID, imageID int64, data []byte) error {
	if err := os.MkdirAll(entityDir(dir, entityID), 0o755); err != nil {
		return fmt.Errorf("create entity image dir: %w", err)
	}
	dst := Path(dir, entityID, imageID, Ext(data))
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write entity image: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename entity image: %w", err)
	}
	return nil
}

// VariantPath is the on-disk location of a width variant of a stored original
// (HOLODEX-332): {dir}/{entityID}/{imageID}-w{width}{ext}, a sibling of the original
// carrying its extension. original is a path Find returned, so it is built from
// server-assigned ids only; Find never resolves a -w file, it builds exact names.
func VariantPath(original string, width int) string {
	ext := filepath.Ext(original)
	return strings.TrimSuffix(original, ext) + "-w" + strconv.Itoa(width) + ext
}

// StoreVariant writes a width variant atomically to path. Unlike Store's fixed .tmp
// name, each writer gets its own temp file, so two concurrent first requests for the
// same tier never write into one file. The whole temp is then published with a hard
// link, which never replaces an existing file: the first writer's variant stands and
// a later racer drops its identical copy. A replacing rename would do on Linux, but on
// Windows it locks the name while it runs, so a request opening the variant in that
// moment fails.
func StoreVariant(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create entity image variant temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		return fmt.Errorf("write entity image variant: %w", errors.Join(werr, cerr))
	}
	err = os.Link(tmp.Name(), path)
	if err == nil || errors.Is(err, fs.ErrExist) {
		return nil
	}
	// A filesystem without hard links: fall back to the replacing rename.
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("publish entity image variant: %w", err)
	}
	return nil
}

// RemoveAll deletes an entity's whole image directory — the purge of an entity whose
// rows are going too (F78 RD3). A missing directory is not an error. entityID is a
// server-assigned integer, so the path can't escape dir.
func RemoveAll(dir string, entityID int64) error {
	if err := os.RemoveAll(entityDir(dir, entityID)); err != nil {
		return fmt.Errorf("remove entity image dir: %w", err)
	}
	return nil
}

// Remove deletes a stored image file under either extension, with its width variants
// (and any temp a crashed variant write left). A missing file is not an error (the
// row may have outlived its bytes, or a prior delete was interrupted) — the DB row is
// the source of truth and is removed separately.
func Remove(dir string, entityID, imageID int64) error {
	// A prefix match over the listing, not filepath.Glob: the data dir is config and may
	// hold glob metacharacters.
	// A failure on one file never stops the rest: a variant held open by a reader on
	// Windows must not leave the original behind.
	sub := entityDir(dir, entityID)
	var paths []string
	for _, ext := range exts {
		paths = append(paths, Path(dir, entityID, imageID, ext))
	}
	var errs error
	entries, err := os.ReadDir(sub)
	if err != nil && !os.IsNotExist(err) {
		errs = fmt.Errorf("list entity images: %w", err)
	}
	prefix := strconv.FormatInt(imageID, 10) + "-w"
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			paths = append(paths, filepath.Join(sub, e.Name()))
		}
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			errs = errors.Join(errs, fmt.Errorf("remove entity image: %w", err))
		}
	}
	return errs
}
