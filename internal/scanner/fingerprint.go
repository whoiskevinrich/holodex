package scanner

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// fpSampleSize is the length of each of the three samples the content
// fingerprint reads (media-ingest.md, "Moves recognised by a sampled content
// hash").
const fpSampleSize = 64 << 10

// contentFingerprint returns the hex SHA-256 of a file's byte size followed by
// three fpSampleSize samples: the start, the middle (offset size/2) and the end.
// A file of at most three samples is hashed whole. It identifies a video's
// content across a move or rename (F79 P0-1) for 192 KiB of reads, and is
// recomputed whenever the file changes in place, so a writeback never leaves it
// stale.
func contentFingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("fingerprint open: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("fingerprint stat: %w", err)
	}
	size := info.Size()

	h := sha256.New()
	var sz [8]byte
	binary.BigEndian.PutUint64(sz[:], uint64(size))
	h.Write(sz[:])

	if size <= 3*fpSampleSize {
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("fingerprint read: %w", err)
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	buf := make([]byte, fpSampleSize)
	for _, off := range []int64{0, size / 2, size - fpSampleSize} {
		if _, err := f.ReadAt(buf, off); err != nil {
			return "", fmt.Errorf("fingerprint read at %d: %w", off, err)
		}
		h.Write(buf)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
