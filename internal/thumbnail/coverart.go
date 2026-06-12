package thumbnail

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// extractCoverArt writes a container's embedded cover image to dst as raw bytes
// via `exiftool -b` (Tier 1, ADR-009). It tries the common cover-art tags in
// order and returns ok=false (no error) when none yield bytes. The bytes are
// whatever the container holds (typically JPEG/PNG) and are served as-is; the
// scanner only flags HasCoverArt when one of these tags is present, so this is
// called just for files that actually have art.
func extractCoverArt(ctx context.Context, exiftoolPath, path, dst string) (bool, error) {
	path = absPath(path)
	for _, tag := range []string{"-CoverArt", "-Picture"} {
		data, err := exec.CommandContext(ctx, exiftoolPath,
			"-b", tag, "-api", "largefilesupport=1", path).Output()
		if err != nil || len(data) == 0 {
			continue // tag absent for this file, or exiftool error — try the next
		}
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return false, fmt.Errorf("write cover art: %w", err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			return false, fmt.Errorf("rename cover art: %w", err)
		}
		return true, nil
	}
	return false, nil
}
