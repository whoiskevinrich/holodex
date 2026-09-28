package writeback

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrFragmentedMP4 refuses a write to a fragmented MP4 (moov followed by
// moof/mdat fragments — HLS/DASH downloads, some recorders). ExifTool's
// QuickTime writer cannot write these ("Can't yet handle movie fragments when
// writing"). Normally stageTemp remuxes such a file instead (ADR-116); this is
// the fallback when it can't — no ffmpeg, or the remux failed — so the file has
// to be remuxed out of band (HOLODEX-479). The message leads with a stable code
// the UI keys on and carries the remux command so the stored queue error is
// itself actionable.
var ErrFragmentedMP4 = errors.New("writeback_unsupported_container: fragmented MP4 — remux required " +
	"(ffmpeg -i in.mp4 -map 0 -c copy -movflags +faststart out.mp4, then replace the file and retry)")

// isoBMFFExts are the extensions routed to exiftool that carry an ISO BMFF
// (QuickTime/MP4) box structure, where fragmentation can occur.
var isoBMFFExts = map[string]bool{".mp4": true, ".m4v": true, ".mov": true}

// checkNotFragmented returns ErrFragmentedMP4 when path is an ISO BMFF file
// with a top-level moof box. Non-BMFF extensions, and files whose box structure
// doesn't parse, pass through untouched — exiftool remains the judge of those.
func checkNotFragmented(path string) error {
	if !isoBMFFExts[strings.ToLower(filepath.Ext(path))] {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("writeback open: %w", err)
	}
	defer f.Close()
	if hasTopLevelBox(f, "moof") {
		return ErrFragmentedMP4
	}
	return nil
}

// stageTemp is step 1 of ADR-041's copy → write → rename: it fills tmp with
// the file exiftool will write. Most files are byte-copied. A fragmented BMFF
// file is remuxed into a progressive one with a stream copy instead (ADR-116
// D1), and remuxed reports that so the caller restores the tags the remux drops
// (D2). The remux falls back to ErrFragmentedMP4 when it can't run or fails (D4).
func stageTemp(ctx context.Context, path, tmp string) (remuxed bool, err error) {
	switch err := checkNotFragmented(path); {
	case err == nil:
		if err := copyFile(path, tmp); err != nil {
			return false, fmt.Errorf("writeback copy: %w", err)
		}
		return false, nil
	case !errors.Is(err, ErrFragmentedMP4):
		return false, err
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false, fmt.Errorf("%w; ffmpeg not found to remux it", ErrFragmentedMP4)
	}
	format := "mp4"
	if strings.EqualFold(filepath.Ext(path), ".mov") {
		format = "mov" // keep the container matching the extension
	}
	// The input demuxer is forced (mov covers mp4/m4v/mov) and protocols are
	// limited to local files, so a crafted library file can't be probed as a
	// playlist and make ffmpeg fetch anything into the output.
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-y", "-loglevel", "error",
		"-protocol_whitelist", "file", "-f", "mov", "-i", ffmpegArg(path), "-map", "0", "-c", "copy", "-map_metadata", "0",
		"-movflags", "+faststart", "-f", format, ffmpegArg(tmp))
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		if ctx.Err() != nil {
			// Cancelled (e.g. shutdown), not a file the remux can't handle — a
			// retry will work, so don't send the owner off to remux by hand.
			return false, fmt.Errorf("writeback remux: %w", ctx.Err())
		}
		return false, fmt.Errorf("%w; ffmpeg remux failed: %v — %s", ErrFragmentedMP4, err, strings.TrimSpace(string(out)))
	}

	// D3: never hand exiftool a file it will refuse. tmp has no BMFF extension,
	// so this walks the boxes directly rather than via checkNotFragmented.
	f, err := os.Open(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("writeback open: %w", err)
	}
	stillFragmented := hasTopLevelBox(f, "moof")
	f.Close()
	if stillFragmented {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("%w; the remuxed copy is still fragmented", ErrFragmentedMP4)
	}
	return true, nil
}

// ffmpegArg keeps a relative path from being read as an option or a protocol
// (e.g. "-x.mp4", "concat:…"). Absolute paths pass through unchanged.
func ffmpegArg(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return "." + string(filepath.Separator) + p
}

// hasTopLevelBox walks the top-level ISO BMFF boxes of r, seeking over each
// body, and reports whether one of type want is present. Only box headers are
// read, so the scan is cheap on multi-gigabyte files. A malformed header ends
// the walk with false.
func hasTopLevelBox(r io.ReadSeeker, want string) bool {
	var hdr [16]byte
	for {
		if _, err := io.ReadFull(r, hdr[:8]); err != nil {
			return false
		}
		size := uint64(binary.BigEndian.Uint32(hdr[:4]))
		typ := string(hdr[4:8])
		if typ == want {
			return true
		}
		headerLen := uint64(8)
		switch size {
		case 0: // box extends to end of file — nothing follows it
			return false
		case 1: // 64-bit largesize follows the type
			if _, err := io.ReadFull(r, hdr[8:16]); err != nil {
				return false
			}
			size = binary.BigEndian.Uint64(hdr[8:16])
			headerLen = 16
		}
		if size < headerLen || size-headerLen > 1<<62 {
			return false
		}
		if _, err := r.Seek(int64(size-headerLen), io.SeekCurrent); err != nil {
			return false
		}
	}
}
