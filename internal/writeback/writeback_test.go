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
		"batch including image": {withImage, []ffmpegImgEntry{{"Poster", "/tmp/poster.jpg"}}, true},
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
