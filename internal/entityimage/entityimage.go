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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// Remove deletes a stored image file under either extension. A missing file is not
// an error (the row may have outlived its bytes, or a prior delete was interrupted)
// — the DB row is the source of truth and is removed separately.
func Remove(dir string, entityID, imageID int64) error {
	for _, ext := range exts {
		err := os.Remove(Path(dir, entityID, imageID, ext))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove entity image: %w", err)
		}
	}
	return nil
}
