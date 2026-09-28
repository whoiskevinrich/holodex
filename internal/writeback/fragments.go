package writeback

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrFragmentedMP4 refuses a write to a fragmented MP4 (moov followed by
// moof/mdat fragments — HLS/DASH downloads, some recorders). ExifTool's
// QuickTime writer cannot write these ("Can't yet handle movie fragments when
// writing"), so every retry would fail the same way; the file has to be remuxed
// out of band first (HOLODEX-479). The message leads with a stable code the UI
// keys on and carries the remux command so the stored queue error is itself
// actionable.
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
