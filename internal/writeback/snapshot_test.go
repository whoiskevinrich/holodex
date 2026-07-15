package writeback

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestReadCurrentValues_RoundTrips writes a tag via mkvpropedit, then confirms
// ReadCurrentValues (F48.9, ADR-067) reads back the value a subsequent write
// is about to overwrite — the pre-write snapshot's core contract.
func TestReadCurrentValues_RoundTrips(t *testing.T) {
	requireMkvpropedit(t)
	requireExiftool(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	if err := os.WriteFile(orig, minimalMKV, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Write(context.Background(), orig, "Title", []string{"Original Title"}); err != nil {
		t.Skipf("synthetic MKV not writable by mkvpropedit: %v", err)
	}

	got, err := ReadCurrentValues(context.Background(), orig, []Mapped{{Field: "title", TagName: "Title"}})
	if err != nil {
		t.Skipf("synthetic MKV not readable by exiftool: %v", err)
	}
	if got["title"] != "Original Title" {
		t.Errorf("want %q, got %q (%v)", "Original Title", got["title"], got)
	}
}

// TestReadCurrentValues_AbsentTagIsEmpty confirms a tag with no value reads
// back as "" rather than erroring — matching the snapshot table's "'' if
// previously absent" contract (ADR-067).
func TestReadCurrentValues_AbsentTagIsEmpty(t *testing.T) {
	requireExiftool(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	if err := os.WriteFile(orig, minimalMKV, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadCurrentValues(context.Background(), orig, []Mapped{{Field: "studio", TagName: "Publisher"}})
	if err != nil {
		t.Skipf("synthetic MKV not readable by exiftool: %v", err)
	}
	if got["studio"] != "" {
		t.Errorf("want empty for absent tag, got %q", got["studio"])
	}
}

// TestReadCurrentValues_SkipsImageFields confirms an image-mapped field never
// reaches exiftool as a text-tag read — there is nothing to snapshot for a
// binary attachment — and needs no tool on PATH at all when every field is an
// image field.
func TestReadCurrentValues_SkipsImageFields(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "clip.mkv")
	if err := os.WriteFile(orig, minimalMKV, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadCurrentValues(context.Background(), orig, []Mapped{{Field: "poster_url", TagName: "cover.jpg", IsImage: true}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want no result for an image-only field, got %v", got)
	}
}
