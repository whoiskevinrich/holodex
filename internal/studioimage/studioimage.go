// Package studioimage is the studio-facing entry point for a studio's self-hosted
// images (F51, ADR-079; generalizes HOLODEX-130/ADR-057's single logo cache to
// icon/logo/poster). Find/Store/Remove delegate to internal/entityimage
// (HOLODEX-286), which owns the actual disk layout shared with
// personimage/filmimage — this package exists so call sites keep asking "the studio
// image package" for a studio path, not a generic one, and so the disk-storage
// implementation detail can move without touching any caller.
//
// It deliberately does NOT reimplement the untrusted-bytes normalization — that
// security spine (sniff-decode, decompression-bomb guard, re-encode metadata
// strip) lives once in personimage.Normalize/Hash and is reused here, so a studio
// image gets byte-for-byte the same hardening as a person portrait.
package studioimage

import "holodex/internal/entityimage"

// Find is the on-disk location for one of a studio's images (ADR-014):
// {dir}/{studioID}/{imageID}.jpg or .png (ADR-097). Both ids are server-assigned integers,
// never a request value, so path traversal is structurally impossible (the ADR-038
// rule). A missing file errors with os.ErrNotExist.
func Find(dir string, studioID, imageID int64) (string, error) {
	return entityimage.Find(dir, studioID, imageID)
}

// Store writes normalized JPEG/PNG bytes to their id-named path via a temp file + rename so a
// reader never sees a torn file. The caller has already inserted the DB row, so
// imageID is the authoritative, server-assigned name.
func Store(dir string, studioID, imageID int64, data []byte) error {
	return entityimage.Store(dir, studioID, imageID, data)
}

// Remove deletes a stored image file. A missing file is not an error (the row may
// have outlived its bytes, or a prior delete was interrupted) — the DB row is the
// source of truth and is removed separately.
func Remove(dir string, studioID, imageID int64) error {
	return entityimage.Remove(dir, studioID, imageID)
}
