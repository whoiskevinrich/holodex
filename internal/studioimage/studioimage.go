// Package studioimage is the on-disk layout for a studio's self-hosted images
// (F51, ADR-079; generalizes HOLODEX-130/ADR-057's single logo cache to icon/logo/
// poster): path building, atomic write, and removal under
// DATA_PATH/studio-images/{studio_id}/{id}.jpg. The role isn't part of the path —
// each row's server-assigned id is already globally unique per studio, and the DB
// row is what maps an id back to its role.
//
// It deliberately does NOT reimplement the untrusted-bytes normalization — that
// security spine (sniff-decode, decompression-bomb guard, re-encode-to-JPEG metadata
// strip) lives once in personimage.Normalize/Hash and is reused here, so a studio
// image gets byte-for-byte the same hardening as a person portrait. This package owns
// only the disk concerns that differ (a studio-images root instead of person-images).
// If a third entity ever needs image storage, promote Normalize/Hash to a shared
// internal/imagenorm package; until then reuse keeps one source of truth.
package studioimage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ImagePath is the on-disk location for one of a studio's images (ADR-014):
// {dir}/{studioID}/{imageID}.jpg. Both ids are server-assigned integers, never a
// request value, so path traversal is structurally impossible (the ADR-038 rule).
// The per-studio subdir is NOT created here — Store creates it.
func ImagePath(dir string, studioID, imageID int64) string {
	return filepath.Join(dir, strconv.FormatInt(studioID, 10), strconv.FormatInt(imageID, 10)+".jpg")
}

// studioDir is the per-studio subdir under the images root.
func studioDir(dir string, studioID int64) string {
	return filepath.Join(dir, strconv.FormatInt(studioID, 10))
}

// Store writes normalized JPEG bytes to ImagePath via a temp file + rename so a
// reader never sees a torn file (mirrors personimage.Store / the thumbnail manager).
// The caller has already inserted the DB row, so imageID is the authoritative,
// server-assigned name.
func Store(dir string, studioID, imageID int64, data []byte) error {
	if err := os.MkdirAll(studioDir(dir, studioID), 0o755); err != nil {
		return fmt.Errorf("create studio image dir: %w", err)
	}
	dst := ImagePath(dir, studioID, imageID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write studio image: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename studio image: %w", err)
	}
	return nil
}

// Remove deletes a stored image file. A missing file is not an error (the row may have
// outlived its bytes, or a prior delete was interrupted) — the DB row is the source of
// truth and is removed separately.
func Remove(dir string, studioID, imageID int64) error {
	err := os.Remove(ImagePath(dir, studioID, imageID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove studio image: %w", err)
	}
	return nil
}
