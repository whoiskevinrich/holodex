package writeback

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestWriteBatch_FragmentedFallbackWhenRemuxFails covers ADR-116 D4: when the
// remux can't produce a file — here the "fragmented MP4" is bare box headers
// ffmpeg can't parse, or ffmpeg is absent — the write falls back to HOLODEX-479's
// ErrFragmentedMP4, the original is untouched, and no temp file is left behind.
// Needs no tool on PATH: both branches land on the same error.
func TestWriteBatch_FragmentedFallbackWhenRemuxFails(t *testing.T) {
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
		if !strings.HasPrefix(err.Error(), "writeback_unsupported_container") {
			t.Errorf("%s: message %q lost the code prefix the UI keys on", ext, err)
		}
		// The base message already names ffmpeg (the hint); the wrapped reason
		// is what must be present.
		if !strings.Contains(err.Error(), "ffmpeg remux failed") && !strings.Contains(err.Error(), "ffmpeg not found") {
			t.Errorf("%s: message %q doesn't say why the remux didn't apply", ext, err)
		}
		if got, _ := os.ReadFile(path); !bytes.Equal(got, orig) {
			t.Errorf("%s: original modified", ext)
		}
		if _, err := os.Stat(path + ".holodex-tmp"); !os.IsNotExist(err) {
			t.Errorf("%s: temp file left behind", ext)
		}
	}
}

// TestWriteBatch_RemuxesFragmentedOnWrite covers ADR-116 D1–D3 end to end: a
// fragmented file carrying QuickTime tags and an XMP Edition is written to
// through WriteBatch. The write succeeds, the batch's field lands, every other
// tag survives (XMP included — ffmpeg's remux drops it, -TagsFromFile restores
// it), the result is no longer fragmented, and the container still matches the
// extension.
func TestWriteBatch_RemuxesFragmentedOnWrite(t *testing.T) {
	requireExiftool(t)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	// .mov carries no XMP here: exiftool keeps a MOV's XMP inside moov
	// (udta/XMP_), so there is no top-level box to graft. The restore argv is
	// the same for both.
	for _, c := range []struct {
		ext, format, brand string
		xmp                bool
	}{
		{".mp4", "mp4", "", true}, // any brand but QuickTime's
		{".mov", "mov", "qt  ", false},
	} {
		t.Run(c.ext, func(t *testing.T) {
			dir := t.TempDir()
			path := fragmentedFixture(t, dir, c.ext, c.format, c.xmp)

			err := WriteBatch(context.Background(), path, []FieldWrite{{TagName: "QuickTime:Title", Values: []string{"New Title"}}})
			if err != nil {
				t.Fatalf("WriteBatch: %v", err)
			}

			got := map[string]string{}
			for _, tag := range []string{"Title", "Comment", "Genre", "Edition"} {
				out, err := exec.Command("exiftool", "-s3", "-"+tag, path).Output()
				if err != nil {
					t.Fatalf("read %s: %v", tag, err)
				}
				got[tag] = strings.TrimSpace(string(out))
			}
			want := map[string]string{"Title": "New Title", "Comment": "keep me", "Genre": "Drama"}
			if c.xmp {
				want["Edition"] = "Director's Cut"
			}
			for tag, w := range want {
				if got[tag] != w {
					t.Errorf("%s = %q, want %q", tag, got[tag], w)
				}
			}

			if err := checkNotFragmented(path); err != nil {
				t.Errorf("file still fragmented after write: %v", err)
			}
			brand := majorBrand(t, path)
			if c.brand != "" && brand != c.brand {
				t.Errorf("major brand = %q, want %q", brand, c.brand)
			}
			if c.brand == "" && brand == "qt  " {
				t.Errorf("an .mp4 came out as a QuickTime movie")
			}
			if _, err := os.Stat(path + ".holodex-tmp"); !os.IsNotExist(err) {
				t.Errorf("temp file left behind")
			}
		})
	}
}

// fragmentedFixture builds clip<ext>: a fragmented file with QuickTime title,
// comment and genre, plus (withXMP) an XMP Edition. ffmpeg can't fragment a
// file and keep its XMP, so the XMP uuid box is grafted from an exiftool-tagged
// progressive copy — the shape a recorder that writes XMP would produce.
func fragmentedFixture(t *testing.T, dir, ext, format string, withXMP bool) string {
	t.Helper()
	run := func(name string, args ...string) {
		t.Helper()
		if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v — %s", name, args, err, out)
		}
	}
	prog := filepath.Join(dir, "prog"+ext)
	run("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=10",
		"-c:v", "mpeg4", "-metadata", "title=Orig Title", "-metadata", "comment=keep me",
		"-metadata", "genre=Drama", "-f", format, prog)
	if withXMP {
		run("exiftool", "-q", "-m", "-overwrite_original", "-XMP-prism:Edition=Director's Cut", prog)
	}

	path := filepath.Join(dir, "clip"+ext)
	run("ffmpeg", "-y", "-loglevel", "error", "-i", prog, "-map", "0", "-c", "copy", "-map_metadata", "0",
		"-movflags", "frag_keyframe+empty_moov", "-f", format, path)

	if err := checkNotFragmented(path); !errors.Is(err, ErrFragmentedMP4) {
		t.Fatalf("fixture is not fragmented: %v", err)
	}
	if !withXMP {
		return path
	}

	xmp := topLevelBox(t, prog, "uuid")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(xmp); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if out, _ := exec.Command("exiftool", "-s3", "-Edition", path).Output(); strings.TrimSpace(string(out)) != "Director's Cut" {
		t.Fatalf("fixture's grafted XMP isn't readable: Edition = %q", out)
	}
	return path
}

// topLevelBox returns the bytes of the first top-level box of type typ in path.
// Fixture files are small and use 32-bit box sizes only.
func topLevelBox(t *testing.T, path, typ string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[i:]))
		if size < 8 || i+size > len(data) {
			break
		}
		if string(data[i+4:i+8]) == typ {
			return data[i : i+size]
		}
		i += size
	}
	t.Fatalf("no top-level %q box in %s", typ, path)
	return nil
}

// majorBrand reads the ftyp major brand ("isom", "qt  ", …).
func majorBrand(t *testing.T, path string) string {
	t.Helper()
	b := topLevelBox(t, path, "ftyp")
	return string(b[8:12])
}
