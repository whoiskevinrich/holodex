// Package entityimage is the shared on-disk layout for a self-hosted entity image
// (person/studio/film, HOLODEX-286): path building, atomic write, and removal under
// {dir}/{entityID}/{imageID}.jpg. It deliberately does NOT reimplement the
// untrusted-bytes normalization — that security spine (sniff-decode,
// decompression-bomb guard, re-encode-to-JPEG metadata strip) lives once in
// personimage.Normalize/Hash and is reused by every caller, so every entity image
// gets byte-for-byte the same hardening. The per-entity packages
// (personimage/studioimage/filmimage) each keep their own ImagePath/Store/Remove as
// thin wrappers delegating here — this package is an implementation detail, not a
// new call surface; nothing above those wrappers changes.
package entityimage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Path is the on-disk location for one of an entity's images (ADR-014):
// {dir}/{entityID}/{imageID}.jpg. Both ids are server-assigned integers, never a
// request value, so path traversal is structurally impossible (the ADR-038 rule).
// The per-entity subdir is NOT created here — Store creates it.
func Path(dir string, entityID, imageID int64) string {
	return filepath.Join(dir, strconv.FormatInt(entityID, 10), strconv.FormatInt(imageID, 10)+".jpg")
}

// entityDir is the per-entity subdir under the images root.
func entityDir(dir string, entityID int64) string {
	return filepath.Join(dir, strconv.FormatInt(entityID, 10))
}

// Store writes normalized JPEG bytes to Path via a temp file + rename so a reader
// never sees a torn file (mirrors the thumbnail manager's atomic write). The caller
// has already inserted the DB row, so imageID is the authoritative, server-assigned
// name.
func Store(dir string, entityID, imageID int64, data []byte) error {
	if err := os.MkdirAll(entityDir(dir, entityID), 0o755); err != nil {
		return fmt.Errorf("create entity image dir: %w", err)
	}
	dst := Path(dir, entityID, imageID)
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

// Remove deletes a stored image file. A missing file is not an error (the row may
// have outlived its bytes, or a prior delete was interrupted) — the DB row is the
// source of truth and is removed separately.
func Remove(dir string, entityID, imageID int64) error {
	err := os.Remove(Path(dir, entityID, imageID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove entity image: %w", err)
	}
	return nil
}
