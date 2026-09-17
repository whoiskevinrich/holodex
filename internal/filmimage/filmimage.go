// Package filmimage is the film-facing entry point for a film's self-hosted images
// (F56/HOLODEX-280, ADR-086; poster/thumb roles). Find/Store/Remove delegate to
// internal/entityimage (HOLODEX-286), which owns the actual disk layout shared with
// personimage/studioimage — this package exists so call sites keep asking "the film
// image package" for a film path, not a generic one, and so the disk-storage
// implementation detail can move without touching any caller.
//
// It deliberately does NOT reimplement the untrusted-bytes normalization — that
// security spine (sniff-decode, decompression-bomb guard, re-encode metadata
// strip) lives once in personimage.Normalize/Hash and is reused here, so a film image
// gets byte-for-byte the same hardening as a person portrait or studio image.
package filmimage

import (
	"fmt"

	"holodex/internal/entityimage"
	"holodex/internal/model"
)

// Find is the on-disk location for one of a film's images (ADR-014):
// {dir}/{filmID}/{imageID}.jpg or .png (ADR-097). Both ids are server-assigned integers,
// never a request value, so path traversal is structurally impossible (the ADR-038
// rule). A missing file errors with os.ErrNotExist.
func Find(dir string, filmID, imageID int64) (string, error) {
	return entityimage.Find(dir, filmID, imageID)
}

// Store writes normalized JPEG/PNG bytes to their id-named path via a temp file + rename so a
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

// PortraitBannerError reports a film banner refused because the image is not
// landscape (HOLODEX-386). The banner role renders `fit="cover"` into an 8:3 band, so
// a portrait (or square) image would show a cropped slice of poster art — the defect a
// sidecar emitting its poster URL under the `banner` kind produces. Both ingest paths
// (provider assets via imagesink, owner upload via api) refuse it at the door and each
// reports in its own idiom — an activity-log line vs. a 400 — so the type carries the
// dimensions rather than a fixed message.
type PortraitBannerError struct{ Width, Height int }

func (e *PortraitBannerError) Error() string {
	return fmt.Sprintf("%d×%d is portrait, banner role requires landscape", e.Width, e.Height)
}

// CheckRoleAspect enforces the per-role aspect rule on an already-decoded image before
// it is stored: the banner role must be strictly wider than tall ("frame follows source
// aspect, never config or role name" — web/src/lib/components/entity/CLAUDE.md). Every
// other role accepts any aspect.
func CheckRoleAspect(role string, width, height int) error {
	if role == model.FilmImageBanner && width <= height {
		return &PortraitBannerError{Width: width, Height: height}
	}
	return nil
}
