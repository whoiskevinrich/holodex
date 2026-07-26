package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
)

// extractCoverArt writes a container's embedded cover image to dst via
// `exiftool -b` (Tier 1, ADR-009). For ordinary embedded art this stays the
// near-free byte copy Tier 1 has always been: DecodeConfig reads just the
// image header to check its width, and when it's already within Width the
// bytes go straight to disk. Only an oversized image — the case that matters,
// a provider's poster embedded at its original, often portrait, full
// resolution by a metadata writeback (ADR-039/ADR-041) — pays for the same
// ffmpeg scale pass Tier 2 uses (scaleToWidth), so it still ends up
// conforming to THUMBNAIL_WIDTH. DecodeConfig failing (a format Go's image
// package doesn't recognize) falls through to the scale path too, since
// ffmpeg reads far more formats than the stdlib decoders registered here. It
// tries the common cover-art tags in order and returns ok=false (no error)
// when none yield bytes. The scanner only flags HasCoverArt when one of these
// tags is present, so this is called just for files that actually have art.
func (m *Manager) extractCoverArt(ctx context.Context, path, dst string) (bool, error) {
	path = absPath(path)
	for _, tag := range []string{"-CoverArt", "-Artwork", "-Picture", "-AttachedFileData"} {
		data, err := exec.CommandContext(ctx, m.cfg.ExiftoolPath,
			"-b", tag, "-api", "largefilesupport=1", path).Output()
		if err != nil || len(data) == 0 {
			continue // tag absent for this file, or exiftool error — try the next
		}
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width <= m.cfg.Width {
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
		if err := m.scaleToWidth(ctx, []string{"-i", "pipe:0"}, bytes.NewReader(data), dst); err != nil {
			return false, fmt.Errorf("scale cover art: %w", err)
		}
		return true, nil
	}
	return false, nil
}
