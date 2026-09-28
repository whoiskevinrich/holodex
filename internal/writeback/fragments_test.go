package writeback

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// box builds one ISO BMFF box with a 32-bit size header and bodyLen zero bytes.
func box(typ string, bodyLen int) []byte {
	b := make([]byte, 8+bodyLen)
	binary.BigEndian.PutUint32(b, uint32(8+bodyLen))
	copy(b[4:8], typ)
	return b
}

// largeBox builds a box using the 64-bit largesize header form (size field = 1).
func largeBox(typ string, bodyLen int) []byte {
	b := make([]byte, 16+bodyLen)
	binary.BigEndian.PutUint32(b, 1)
	copy(b[4:8], typ)
	binary.BigEndian.PutUint64(b[8:16], uint64(16+bodyLen))
	return b
}

// withBody overwrites the start of b's body (after the 8-byte header) with s.
func withBody(b []byte, s string) []byte {
	copy(b[8:], s)
	return b
}

func TestHasTopLevelBox(t *testing.T) {
	cat := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"progressive mp4", cat(box("ftyp", 16), box("moov", 40), box("mdat", 100)), false},
		{"fragmented mp4", cat(box("ftyp", 16), box("moov", 40), box("moof", 24), box("mdat", 100)), true},
		{"moof after largesize mdat", cat(box("ftyp", 16), largeBox("mdat", 64), box("moof", 8)), true},
		// "moof" bytes inside a box body must not count — only headers are read.
		{"moof inside a body", cat(box("ftyp", 16), withBody(box("moov", 8), "\x00\x00\x00\x08moof")), false},
		{"size 0 runs to EOF", cat(box("ftyp", 16), []byte{0, 0, 0, 0, 'm', 'd', 'a', 't'}, box("moof", 8)), false},
		{"malformed size", cat(box("ftyp", 16), []byte{0, 0, 0, 4, 'j', 'u', 'n', 'k'}), false},
		{"truncated", []byte{0, 0, 0}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasTopLevelBox(bytes.NewReader(c.data), "moof"); got != c.want {
				t.Errorf("hasTopLevelBox = %v, want %v", got, c.want)
			}
		})
	}
}

// TestWriteBatch_FragmentedMP4Refused covers HOLODEX-479: a fragmented MP4 is
// refused with ErrFragmentedMP4 before exiftool runs (and before the temp copy
// is made), so the original is untouched and no temp file is left behind. The
// check runs ahead of any tool, so this needs no exiftool on PATH.
func TestWriteBatch_FragmentedMP4Refused(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".mp4", ".M4V", ".mov"} {
		path := filepath.Join(dir, "clip"+ext)
		orig := bytes.Join([][]byte{box("ftyp", 16), box("moov", 40), box("moof", 24), box("mdat", 100)}, nil)
		if err := os.WriteFile(path, orig, 0o644); err != nil {
			t.Fatal(err)
		}
		err := WriteBatch(context.Background(), path, []FieldWrite{{TagName: "Title", Values: []string{"x"}}})
		if !errors.Is(err, ErrFragmentedMP4) {
			t.Fatalf("%s: err = %v, want ErrFragmentedMP4", ext, err)
		}
		if got, _ := os.ReadFile(path); !bytes.Equal(got, orig) {
			t.Errorf("%s: original modified", ext)
		}
		if _, err := os.Stat(path + ".holodex-tmp"); !os.IsNotExist(err) {
			t.Errorf("%s: temp file left behind", ext)
		}
	}
}

// TestWriteBatch_FragmentedMP4Integration reproduces the reported file with
// ffmpeg's fragmenting muxer flags and confirms the refusal, then confirms the
// documented remux produces a file that is no longer refused.
func TestWriteBatch_FragmentedMP4Integration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	frag := filepath.Join(dir, "frag.mp4")
	out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=10",
		"-c:v", "mpeg4", "-movflags", "frag_keyframe+empty_moov", frag).CombinedOutput()
	if err != nil {
		t.Fatalf("generate fragmented mp4: %v — %s", err, out)
	}
	if err := checkNotFragmented(frag); !errors.Is(err, ErrFragmentedMP4) {
		t.Fatalf("fragmented file: err = %v, want ErrFragmentedMP4", err)
	}

	remuxed := filepath.Join(dir, "out.mp4")
	out, err = exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", frag, "-map", "0", "-c", "copy", "-movflags", "+faststart", remuxed).CombinedOutput()
	if err != nil {
		t.Fatalf("remux: %v — %s", err, out)
	}
	if err := checkNotFragmented(remuxed); err != nil {
		t.Fatalf("remuxed file still refused: %v", err)
	}
}
