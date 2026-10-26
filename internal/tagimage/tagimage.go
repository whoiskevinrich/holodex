// Package tagimage is the tag-facing entry point for a tag's owner-uploaded poster image
// (F82, list-views spec P0-10). Like studioimage it delegates to internal/entityimage,
// which owns the disk layout ({dir}/{tagID}/{imageID}.jpg|.png); this package exists so
// call sites ask "the tag image package" for a tag path. The untrusted-bytes
// normalization is personimage.Normalize, applied by the caller before Store.
package tagimage

import "holodex/internal/entityimage"

// Find is the on-disk location of a tag's image. Both ids are server-assigned
// integers, never a request value. A missing file errors with os.ErrNotExist.
func Find(dir string, tagID, imageID int64) (string, error) {
	return entityimage.Find(dir, tagID, imageID)
}

// Store writes normalized JPEG/PNG bytes to their id-named path atomically.
func Store(dir string, tagID, imageID int64, data []byte) error {
	return entityimage.Store(dir, tagID, imageID, data)
}

// Remove deletes a stored image file; a missing file is not an error.
func Remove(dir string, tagID, imageID int64) error {
	return entityimage.Remove(dir, tagID, imageID)
}

// Move re-files an image from a merged-away tag to the survivor that inherited it.
func Move(dir string, fromTagID, toTagID, imageID int64) error {
	return entityimage.Move(dir, fromTagID, toTagID, imageID)
}

// RemoveAll deletes a tag's whole image directory, once the tag itself is gone.
func RemoveAll(dir string, tagID int64) error {
	return entityimage.RemoveAll(dir, tagID)
}
