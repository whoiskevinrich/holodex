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

// extractCoverArt writes a container's embedded cover image to the thumbnail-
// and poster-tier destinations via a single `exiftool -b` read (Tier 1,
// ADR-009; two-tier output added by F53/HOLODEX-253 — see writeCoverArtTiers).
// It tries the common cover-art tags in order and returns ok=false (no error)
// when none yield bytes. The scanner only flags HasCoverArt when one of these
// tags is present, so this is called just for files that actually have art.
func (m *Manager) extractCoverArt(ctx context.Context, path, thumbDst, posterDst string) (bool, error) {
	path = absPath(path)
	for _, tag := range []string{"-CoverArt", "-Artwork", "-Picture", "-AttachedFileData"} {
		data, err := exec.CommandContext(ctx, m.cfg.ExiftoolPath,
			"-b", tag, "-api", "largefilesupport=1", path).Output()
		if err != nil || len(data) == 0 {
			continue // tag absent for this file, or exiftool error — try the next
		}
		if err := m.writeCoverArtTiers(ctx, data, thumbDst, posterDst); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// writeCoverArtTiers derives both the thumbnail- and poster-tier outputs from
// one already-extracted embedded-art byte buffer (P0-3): at most two ffmpeg
// scale passes, never a second exiftool/network/disk read of the source.
// DecodeConfig reads just the image header to check its width; when it fails
// (a format Go's image package doesn't recognize) both tiers fall through to
// the scale path, since ffmpeg reads far more formats than the stdlib
// decoders registered here. Each tier's within-cap-or-scale decision is
// independent of the other's — both read from the same in-memory buffer, so
// there's nothing to gain by computing a combined threshold up front.
func (m *Manager) writeCoverArtTiers(ctx context.Context, data []byte, thumbDst, posterDst string) error {
	width, known := 0, false
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width, known = cfg.Width, true
	}

	if known && width <= m.cfg.PosterWidth {
		if err := writeAtomic(posterDst, data); err != nil {
			return fmt.Errorf("write cover art poster: %w", err)
		}
	} else if err := m.scaleToWidth(ctx, []string{"-i", "pipe:0"}, bytes.NewReader(data), posterDst, m.cfg.PosterWidth); err != nil {
		return fmt.Errorf("scale cover art poster: %w", err)
	}

	if known && width <= m.cfg.Width {
		if err := writeAtomic(thumbDst, data); err != nil {
			return fmt.Errorf("write cover art thumb: %w", err)
		}
		return nil
	}
	if err := m.scaleToWidth(ctx, []string{"-i", "pipe:0"}, bytes.NewReader(data), thumbDst, m.cfg.Width); err != nil {
		return fmt.Errorf("scale cover art thumb: %w", err)
	}
	return nil
}

// writeAtomic writes data to dst via a temp file + rename so a concurrent
// reader (the serving handler) never observes a partial file.
func writeAtomic(dst string, data []byte) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
