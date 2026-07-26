package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// extractCoverArt writes a container's embedded cover image to dst as a JPEG
// scaled to Width, via `exiftool -b` (Tier 1, ADR-009) followed by the same
// ffmpeg scale pass Tier 2 uses (scaleToWidth). It tries the common cover-art
// tags in order and returns ok=false (no error) when none yield bytes. The
// scale pass matters because embedded art can come from a metadata-writeback
// (a provider's poster image embedded at its original, often portrait, full
// resolution — see ADR-039/ADR-041) rather than from the source file itself;
// without it, Tier 1 thumbnails would ignore THUMBNAIL_WIDTH entirely. The
// scanner only flags HasCoverArt when one of these tags is present, so this is
// called just for files that actually have art.
func (m *Manager) extractCoverArt(ctx context.Context, path, dst string) (bool, error) {
	path = absPath(path)
	for _, tag := range []string{"-CoverArt", "-Artwork", "-Picture", "-AttachedFileData"} {
		data, err := exec.CommandContext(ctx, m.cfg.ExiftoolPath,
			"-b", tag, "-api", "largefilesupport=1", path).Output()
		if err != nil || len(data) == 0 {
			continue // tag absent for this file, or exiftool error — try the next
		}
		if err := m.scaleToWidth(ctx, []string{"-i", "pipe:0"}, bytes.NewReader(data), dst); err != nil {
			return false, fmt.Errorf("scale cover art: %w", err)
		}
		return true, nil
	}
	return false, nil
}
