package maintenance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Rebuild writes dst: src with op applied to its main video stream and everything else kept
// (file-maintenance.md, "The re-encode"). It does not verify or swap; the caller runs Verify
// on the result before anything touches src. On error dst and any intermediate are removed.
func Rebuild(ctx context.Context, t Tools, family string, op Op, available map[string]bool, src, dst string, p Probe) (err error) {
	intermediate := src + ".holodex-tmp" // MKV only; swept at boot like a tag write's temp
	for _, name := range []string{dst, intermediate} {
		if err := refuseSymlink(name); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			os.Remove(dst)
		}
	}()
	switch family {
	case FamilyMP4:
		format := "mp4"
		if strings.EqualFold(filepath.Ext(src), ".mov") {
			format = "mov"
		}
		args, err := MP4Args(p, op, available, format, src, dst)
		if err != nil {
			return err
		}
		if _, err := run(ctx, t.FFmpeg, args...); err != nil {
			return fmt.Errorf("re-encode: %w", err)
		}
		// Put the original's tags back. ffmpeg drops XMP (where MP4 edition lives), and with
		// +bitexact it writes no ©too, so the original's Encoder tag would vanish; restore
		// the iTunes item list and udta too. Not -all:all: that also turns the container's
		// brand fields into new Keys tags the original never had.
		if _, err := run(ctx, t.Exiftool, "-overwrite_original", "-TagsFromFile", toolArg(src),
			"-XMP:all", "-ItemList:all", "-UserData:all", toolArg(dst)); err != nil {
			return fmt.Errorf("restore tags: %w", err)
		}
		return nil
	case FamilyMKV:
		defer os.Remove(intermediate)
		args, err := MKVEncodeArgs(p, op, available, src, intermediate)
		if err != nil {
			return err
		}
		tags, err := PrepareTrackTags(ctx, t, src, p)
		if err != nil {
			return err
		}
		if tags != "" {
			defer os.Remove(tags)
		}
		if _, err := run(ctx, t.FFmpeg, args...); err != nil {
			return fmt.Errorf("re-encode: %w", err)
		}
		if err := runMkvmerge(ctx, t.Mkvmerge, MKVMergeArgs(p, src, intermediate, tags, dst)); err != nil {
			return fmt.Errorf("rebuild: %w", err)
		}
		// The mux date is part of the file as Matroska tools and libraries read it (often as
		// "date created"); keep the original's rather than mkvmerge's.
		if date := p.Format.Tags["creation_time"]; date != "" {
			// mkvpropedit takes YYYY-mm-ddTHH:MM:SSZ only, not ffprobe's six-decimal form.
			// mkvmerge stamps whole seconds; an original with sub-second precision fails the
			// verifier rather than being silently rounded.
			ts, err := time.Parse(time.RFC3339Nano, date)
			if err != nil {
				return fmt.Errorf("restore mux date: unreadable %q: %w", date, err)
			}
			if err := runMkvmerge(ctx, "mkvpropedit", []string{toolArg(dst), "--edit", "info",
				"--set", "date=" + ts.UTC().Format("2006-01-02T15:04:05Z")}); err != nil {
				return fmt.Errorf("restore mux date: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("maintenance: unsupported container family %q", family)
}

// ErrSymlinkedSibling: a name maintenance writes beside the media file is a symlink. ffmpeg -y
// and mkvmerge -o would write through it, possibly far outside the library, so the job stops.
var ErrSymlinkedSibling = errors.New("maintenance: a working file beside the media file is a symlink")

// refuseSymlink fails when name exists as a symlink. A missing name or a regular file (a
// stale temp the tools overwrite) is fine.
func refuseSymlink(name string) error {
	fi, err := os.Lstat(name)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkedSibling, filepath.Base(name))
	}
	return nil
}

// runMkvmerge treats exit 1 (warnings) as success, as mkvextract's callers do; 2 is an error.
func runMkvmerge(ctx context.Context, bin string, args []string) error {
	_, err := run(ctx, bin, args...)
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return nil
	}
	return err
}
