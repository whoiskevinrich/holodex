// Package providericon is the on-disk layout for a metadata provider's single
// self-hosted brand icon (HOLODEX-134, ADR-059): path building, atomic write, and
// removal under DATA_PATH/provider-icons/{id}.jpg or .png (ADR-097).
//
// Like internal/studioimage, it deliberately does NOT reimplement the untrusted-bytes
// normalization — that security spine (sniff-decode, decompression-bomb guard,
// re-encode metadata strip) lives once in personimage.Normalize/Hash and is
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

	"holodex/internal/entityimage"
)

// exts lists the on-disk extensions in lookup order (.jpg first: the common case,
// and the only one that existed before ADR-097).
var exts = []string{".jpg", ".png"}

// Path is the on-disk location for a provider icon (ADR-059/ADR-014):
// {dir}/{iconID}{ext}. iconID is the server-assigned provider_icons row id, never a
// request value (the provider NAME never touches the path), so path traversal is
// structurally impossible (the ADR-038 rule).
func Path(dir string, iconID int64, ext string) string {
	return filepath.Join(dir, strconv.FormatInt(iconID, 10)+ext)
}

// Find returns the stored file for an icon id, whichever extension it was written
// with (entityimage.Ext picks .png for a transparent icon, .jpg otherwise). A missing
// file errors with os.ErrNotExist so the serving handler 404s as before.
func Find(dir string, iconID int64) (string, error) {
	var err error
	for _, ext := range exts {
		p := Path(dir, iconID, ext)
		if _, err = os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("find provider icon: %w", err)
}

// Store writes normalized JPEG/PNG bytes to Path via a temp file + rename so a reader
// never sees a torn file (mirrors studioimage.Store / personimage.Store). The caller
// has already inserted the DB row, so iconID is the authoritative, server-assigned name.
func Store(dir string, iconID int64, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create provider icon dir: %w", err)
	}
	dst := Path(dir, iconID, entityimage.Ext(data))
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
	for _, ext := range exts {
		err := os.Remove(Path(dir, iconID, ext))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove provider icon: %w", err)
		}
	}
	return nil
}
