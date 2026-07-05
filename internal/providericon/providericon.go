// Package providericon is the on-disk layout for a metadata provider's single
// self-hosted brand icon (HOLODEX-134, ADR-059): path building, atomic write, and
// removal under DATA_PATH/provider-icons/{id}.jpg.
//
// Like internal/studioimage, it deliberately does NOT reimplement the untrusted-bytes
// normalization — that security spine (sniff-decode, decompression-bomb guard,
// re-encode-to-JPEG metadata strip) lives once in personimage.Normalize/Hash and is
// reused by the caller, so a provider icon gets byte-for-byte the same hardening as a
// person portrait. This package owns only the disk concerns that differ: a
// provider-icons root, keyed by the server-assigned row id (there is no per-entity
// subdir — a provider icon is a flat, one-per-provider cache).
package providericon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ImagePath is the on-disk location for a provider icon (ADR-059/ADR-014):
// {dir}/{iconID}.jpg. iconID is the server-assigned provider_icons row id, never a
// request value (the provider NAME never touches the path), so path traversal is
// structurally impossible (the ADR-038 rule).
func ImagePath(dir string, iconID int64) string {
	return filepath.Join(dir, strconv.FormatInt(iconID, 10)+".jpg")
}

// Store writes normalized JPEG bytes to ImagePath via a temp file + rename so a reader
// never sees a torn file (mirrors studioimage.Store / personimage.Store). The caller
// has already inserted the DB row, so iconID is the authoritative, server-assigned name.
func Store(dir string, iconID int64, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create provider icon dir: %w", err)
	}
	dst := ImagePath(dir, iconID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write provider icon: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename provider icon: %w", err)
	}
	return nil
}

// Remove deletes a stored icon file. A missing file is not an error (the row may have
// outlived its bytes, or a prior delete was interrupted) — the DB row is the source of
// truth and is removed separately.
func Remove(dir string, iconID int64) error {
	err := os.Remove(ImagePath(dir, iconID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove provider icon: %w", err)
	}
	return nil
}
