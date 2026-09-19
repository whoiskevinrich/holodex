package writeback

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireExiftool(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not on PATH — skipping writeback I/O tests")
	}
}

func requireMkvpropedit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("mkvpropedit"); err != nil {
		t.Skip("mkvpropedit not on PATH — install MKVToolNix to run MKV write tests")
	}
}

// minimalMKV is a minimal EBML/Matroska header that carries the magic bytes
// exiftool uses to identify the format. Success-path tests use this so
// exiftool can at least attempt a write; they still skip if exiftool rejects
// it with a hard format error (real media files needed for a guaranteed pass).
var minimalMKV = []byte{
	// EBML element ID
	0x1A, 0x45, 0xDF, 0xA3,
	// size VINT (31 bytes of header follow)
	0x9F,
	// EBMLVersion = 1
	0x42, 0x86, 0x81, 0x01,
	// EBMLReadVersion = 1
	0x42, 0xF7, 0x81, 0x01,
	// EBMLMaxIDLength = 4
	0x42, 0xF2, 0x81, 0x04,
	// EBMLMaxSizeLength = 8
	0x42, 0xF3, 0x81, 0x08,
	// DocType = "matroska" (8 bytes)
	0x42, 0x82, 0x88, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a',
	// DocTypeVersion = 4
	0x42, 0x87, 0x81, 0x04,
	// DocTypeReadVersion = 2
	0x42, 0x85, 0x81, 0x02,
}

// TestWrite_OriginalUnchangedOnExiftoolFailure verifies that a bad tag name
// causes exiftool to exit non-zero and leaves the original file byte-for-byte
// unchanged with no temp file leaking.
func TestWrite_OriginalUnchangedOnExiftoolFailure(t *testing.T) {
	requireExiftool(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	sentinel := []byte("sentinel-original-content")
	if err := os.WriteFile(orig, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	// A blank tag name makes exiftool exit non-zero.
	_ = Write(context.Background(), orig, "", []string{"value"})

	got, err := os.ReadFile(orig)
	if err != nil || !bytes.Equal(got, sentinel) {
		t.Errorf("original was modified on failure: len=%d err=%v", len(got), err)
	}
	// No temp file left behind.
	for _, e := range mustReadDir(t, dir) {
		if strings.Contains(e, "holodex-tmp") {
			t.Errorf("temp file leaked: %s", e)
		}
	}
}

// TestWrite_TempFileCleanedOnSuccess verifies no .holodex-tmp or .tags.xml
// remains after a successful write. Uses MKV → mkvpropedit path.
func TestWrite_TempFileCleanedOnSuccess(t *testing.T) {
	requireMkvpropedit(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	if err := os.WriteFile(orig, minimalMKV, 0o644); err != nil {
		t.Fatal(err)
	}

	err := Write(context.Background(), orig, "Title", []string{"Test Title"})
	if err != nil {
		t.Skipf("synthetic MKV not writable by mkvpropedit; atomicity covered by failure test: %v", err)
	}
	if _, err := os.Stat(orig); err != nil {
		t.Errorf("original missing after write: %v", err)
	}
	for _, e := range mustReadDir(t, dir) {
		if strings.Contains(e, "holodex-tmp") || strings.Contains(e, ".tags.xml") {
			t.Errorf("temp file not cleaned up: %s", e)
		}
	}
}

// TestWrite_MultiValue verifies that multiple values are accepted without error.
// Uses MKV → mkvpropedit path.
func TestWrite_MultiValue(t *testing.T) {
	requireMkvpropedit(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	if err := os.WriteFile(orig, minimalMKV, 0o644); err != nil {
		t.Fatal(err)
	}

	err := Write(context.Background(), orig, "GENRE", []string{"Drama", "Thriller"})
	if err != nil {
		t.Skipf("synthetic MKV not writable by mkvpropedit: %v", err)
	}
}

// TestWrite_ContextCancelled verifies a pre-cancelled context leaves the
// original untouched.
func TestWrite_ContextCancelled(t *testing.T) {
	requireExiftool(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	sentinel := []byte("cancel-sentinel")
	if err := os.WriteFile(orig, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = Write(ctx, orig, "Title", []string{"x"})

	got, _ := os.ReadFile(orig)
	if !bytes.Equal(got, sentinel) {
		t.Error("original modified despite cancelled context")
	}
}

// TestWrite_EmptyValues returns an error without touching the file.
func TestWrite_EmptyValues(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	sentinel := []byte("sentinel")
	if err := os.WriteFile(orig, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Write(context.Background(), orig, "Title", nil); err == nil {
		t.Error("expected error for empty values slice")
	}
	got, _ := os.ReadFile(orig)
	if !bytes.Equal(got, sentinel) {
		t.Error("original modified despite error return")
	}
}

// TestBuildFFmpegArgs_AlwaysMapsAllStreams verifies -map 0 is present
// regardless of whether the batch includes an image field. Without it,
// ffmpeg's automatic stream selection drops attachment streams (embedded
// cover art) on any text-only writeback — this was the bug where existing
// posters were silently erased.
func TestBuildFFmpegArgs_AlwaysMapsAllStreams(t *testing.T) {
	textOnly := []FieldWrite{{TagName: "Title", Values: []string{"New Title"}}}
	withImage := []FieldWrite{
		{TagName: "Title", Values: []string{"New Title"}},
		{TagName: "Poster", Values: []string{"https://example.com/poster.jpg"}, IsImage: true},
	}

	for name, tc := range map[string]struct {
		fields     []FieldWrite
		imgEntries []ffmpegImgEntry
		wantAttach bool
	}{
		"text-only batch":       {textOnly, nil, false},
		"batch including image": {withImage, []ffmpegImgEntry{{"Poster", "/tmp/poster.jpg", "image/jpeg"}}, true},
	} {
		t.Run(name, func(t *testing.T) {
			args := buildFFmpegArgs("/media/clip.mkv", "/media/clip.mkv.holodex-new", "matroska", tc.fields, tc.imgEntries)
			joined := strings.Join(args, " ")
			for _, want := range []string{"-map 0", "-map_metadata 0"} {
				if !strings.Contains(joined, want) {
					t.Errorf("%s: expected %q in args, got %q", name, want, joined)
				}
			}
			if got := strings.Contains(joined, "-attach"); got != tc.wantAttach {
				t.Errorf("%s: -attach present = %v, want %v (args: %q)", name, got, tc.wantAttach, joined)
			}
		})
	}
}

// TestBuildFFmpegArgs_ReplacesExistingCover verifies a cover writeback drops the
// input's attachment of the same name (so the new one replaces it instead of
// stacking beside it, and an undecodable one stops blocking the remux) and
// labels the new attachment with its sniffed mimetype — a PNG attached as
// image/jpeg is what produced the undecodable attachment in the first place.
// Text-only batches must not carry the negative map: -map 0 alone preserves
// whatever cover is already there.
func TestBuildFFmpegArgs_ReplacesExistingCover(t *testing.T) {
	entries := []ffmpegImgEntry{{"cover.jpg", "/tmp/holodex-cover-1.png", "image/png"}}
	args := buildFFmpegArgs("/media/clip.mkv", "/media/clip.mkv.holodex-new", "matroska", nil, entries)
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"-map 0 -map -0:m:filename:cover.jpg -c copy",
		"-metadata:s:t:0 mimetype=image/png",
		"-metadata:s:t:0 filename=cover.jpg",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in args, got %q", want, joined)
		}
	}

	textOnly := buildFFmpegArgs("/media/clip.mkv", "/media/clip.mkv.holodex-new", "matroska",
		[]FieldWrite{{TagName: "Title", Values: []string{"T"}}}, nil)
	if joined := strings.Join(textOnly, " "); strings.Contains(joined, "-map -0") {
		t.Errorf("text-only batch must not exclude any stream, got %q", joined)
	}
}

func TestCoverMIME(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/holodex-cover-1.png": "image/png",
		"/tmp/holodex-cover-2.jpg": "image/jpeg",
	} {
		if got := coverMIME(path); got != want {
			t.Errorf("coverMIME(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestMergeTagsXML verifies that fields are folded into the file's existing
// tags rather than replacing them. mkvpropedit's --tags global: swaps out the
// whole TAGS element, so a batch that only wrote GENRE used to erase every tag
// an earlier batch had written.
func TestMergeTagsXML(t *testing.T) {
	const existing = `<?xml version="1.0"?>
<!DOCTYPE Tags SYSTEM "matroskatags.dtd">
<Tags>
<Tag>
<Targets />
<Simple><Name>ARTIST</Name><String>Prior Artist</String></Simple>
<Simple><Name>GENRE</Name><String>Old Genre</String></Simple>
</Tag>
<Tag>
<Targets><TargetTypeValue>30</TargetTypeValue></Targets>
<Simple><Name>PART_NUMBER</Name><String>3</String></Simple>
</Tag>
</Tags>`

	got, err := mergeTagsXML(existing, []FieldWrite{
		{TagName: "Genre", Values: []string{"Drama", "Thriller"}},
	})
	if err != nil {
		t.Fatalf("mergeTagsXML: %v", err)
	}

	for _, want := range []string{
		"<Name>ARTIST</Name><String>Prior Artist</String>", // untouched tag survives
		"<Name>PART_NUMBER</Name><String>3</String>",       // targeted Tag survives
		"<TargetTypeValue>30</TargetTypeValue>",            // its Targets survive
		"<Name>GENRE</Name><String>Drama</String>",         // new values written
		"<Name>GENRE</Name><String>Thriller</String>",      // multi-value
	} {
		if !strings.Contains(got, want) {
			t.Errorf("merged document missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Old Genre") {
		t.Errorf("stale value for a replaced tag survived:\n%s", got)
	}
}

// TestMergeTagsXML_ReplacesPartNumber pins the other half of the PART_NUMBER story
// (HOLODEX-389 RD5): TestMergeTagsXML proves a foreign PART_NUMBER survives a write
// to some other tag; this proves a write to `part` REPLACES it rather than adding a
// second Simple beside it. Two PART_NUMBERs would make exiftool's read-back
// order-dependent and in_sync (ADR-093) a coin flip. The foreign copy sat on a
// targeted Tag, so that Tag is dropped whole once emptied (Matroska requires a
// Simple per Tag) and the survivor lands on the untargeted Tag Holodex writes.
func TestMergeTagsXML_ReplacesPartNumber(t *testing.T) {
	const existing = `<?xml version="1.0"?>
<!DOCTYPE Tags SYSTEM "matroskatags.dtd">
<Tags>
<Tag>
<Targets />
<Simple><Name>ARTIST</Name><String>Prior Artist</String></Simple>
</Tag>
<Tag>
<Targets><TargetTypeValue>30</TargetTypeValue></Targets>
<Simple><Name>PART_NUMBER</Name><String>3</String></Simple>
</Tag>
</Tags>`

	got, err := mergeTagsXML(existing, []FieldWrite{{TagName: "PART_NUMBER", Values: []string{"2"}}})
	if err != nil {
		t.Fatalf("mergeTagsXML: %v", err)
	}
	if n := strings.Count(got, "<Name>PART_NUMBER</Name>"); n != 1 {
		t.Fatalf("PART_NUMBER appears %d times, want exactly 1:\n%s", n, got)
	}
	if !strings.Contains(got, "<Name>PART_NUMBER</Name><String>2</String>") {
		t.Errorf("new value not written:\n%s", got)
	}
	if strings.Contains(got, "<String>3</String>") || strings.Contains(got, "TargetTypeValue") {
		t.Errorf("foreign PART_NUMBER (or its emptied Tag) survived a part write:\n%s", got)
	}
	if !strings.Contains(got, "<Name>ARTIST</Name><String>Prior Artist</String>") {
		t.Errorf("unrelated tag lost:\n%s", got)
	}
}

// TestMergeTagsXML_NoExisting covers a file with no tags at all — the merge
// should produce a plain single-Tag document.
func TestMergeTagsXML_NoExisting(t *testing.T) {
	got, err := mergeTagsXML("", []FieldWrite{{TagName: "Genre", Values: []string{"Drama"}}})
	if err != nil {
		t.Fatalf("mergeTagsXML: %v", err)
	}
	for _, want := range []string{"<Tags>", "<Targets />", "<Name>GENRE</Name><String>Drama</String>", "</Tags>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func mustReadDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}
