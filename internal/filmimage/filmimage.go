// Package filmimage is the film-facing entry point for a film's self-hosted images
// (F56/HOLODEX-280, ADR-086; poster/thumb roles). ImagePath/Store/Remove delegate to
// internal/entityimage (HOLODEX-286), which owns the actual disk layout shared with
// personimage/studioimage — this package exists so call sites keep asking "the film
// image package" for a film path, not a generic one, and so the disk-storage
// implementation detail can move without touching any caller.
//
// It deliberately does NOT reimplement the untrusted-bytes normalization — that
// security spine (sniff-decode, decompression-bomb guard, re-encode-to-JPEG metadata
// strip) lives once in personimage.Normalize/Hash and is reused here, so a film image
// gets byte-for-byte the same hardening as a person portrait or studio image.
package filmimage

import "holodex/internal/entityimage"

// ImagePath is the on-disk location for one of a film's images (ADR-014):
// {dir}/{filmID}/{imageID}.jpg. Both ids are server-assigned integers, never a
// request value, so path traversal is structurally impossible (the ADR-038 rule).
// The per-film subdir is NOT created here — Store creates it.
func ImagePath(dir string, filmID, imageID int64) string {
	return entityimage.Path(dir, filmID, imageID)
}

// Store writes normalized JPEG bytes to ImagePath via a temp file + rename so a
// reader never sees a torn file. The caller has already inserted the DB row, so
// imageID is the authoritative, server-assigned name.
func Store(dir string, filmID, imageID int64, data []byte) error {
	return entityimage.Store(dir, filmID, imageID, data)
}

// Remove deletes a stored image file. A missing file is not an error (the row may
// have outlived its bytes, or a prior delete was interrupted) — the DB row is the
// source of truth and is removed separately.
func Remove(dir string, filmID, imageID int64) error {
	return entityimage.Remove(dir, filmID, imageID)
}
