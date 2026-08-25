// Package filmimage is the on-disk layout for a film's self-hosted images
// (F56/HOLODEX-280, ADR-086; poster/thumb roles): path building, atomic write, and
// removal under DATA_PATH/film-images/{film_id}/{id}.jpg. The role isn't part of the
// path — each row's server-assigned id is already globally unique per film, and the
// DB row is what maps an id back to its role.
//
// It deliberately does NOT reimplement the untrusted-bytes normalization — that
// security spine (sniff-decode, decompression-bomb guard, re-encode-to-JPEG metadata
// strip) lives once in personimage.Normalize/Hash and is reused here, so a film image
// gets byte-for-byte the same hardening as a person portrait or studio image. This
// package owns only the disk concerns that differ (a film-images root instead of
// studio-images), mirroring internal/studioimage (F51, ADR-079) exactly.
package filmimage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ImagePath is the on-disk location for one of a film's images (ADR-014):
// {dir}/{filmID}/{imageID}.jpg. Both ids are server-assigned integers, never a
// request value, so path traversal is structurally impossible (the ADR-038 rule).
// The per-film subdir is NOT created here — Store creates it.
func ImagePath(dir string, filmID, imageID int64) string {
	return filepath.Join(dir, strconv.FormatInt(filmID, 10), strconv.FormatInt(imageID, 10)+".jpg")
}

// filmDir is the per-film subdir under the images root.
func filmDir(dir string, filmID int64) string {
	return filepath.Join(dir, strconv.FormatInt(filmID, 10))
}

// Store writes normalized JPEG bytes to ImagePath via a temp file + rename so a
// reader never sees a torn file (mirrors studioimage.Store). The caller has already
// inserted the DB row, so imageID is the authoritative, server-assigned name.
func Store(dir string, filmID, imageID int64, data []byte) error {
	if err := os.MkdirAll(filmDir(dir, filmID), 0o755); err != nil {
		return fmt.Errorf("create film image dir: %w", err)
	}
	dst := ImagePath(dir, filmID, imageID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write film image: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename film image: %w", err)
	}
	return nil
}

// Remove deletes a stored image file. A missing file is not an error (the row may have
// outlived its bytes, or a prior delete was interrupted) — the DB row is the source of
// truth and is removed separately.
func Remove(dir string, filmID, imageID int64) error {
	err := os.Remove(ImagePath(dir, filmID, imageID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove film image: %w", err)
	}
	return nil
}
