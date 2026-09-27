//go:build integration

// Integration test for the multi-value genre writeback round trip (HOLODEX-464)
// against the real ffmpeg + exiftool binaries. Run with:
//
//	go test -tags integration ./internal/writeback/
//
// A container genre tag is ONE comma-delimited string. exiftool keeps only the
// last of repeated -QuickTime:Genre= assignments, so writing tags as separate
// values cut every MP4 to its last tag — and the next rescan then dropped the
// file-sourced tags from the video. This pins "write 3, read 3" on both
// containers, and that a write replaces an existing genre rather than appending.
package writeback_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"holodex/internal/metadata"
	"holodex/internal/writeback"
)

func TestGenreRoundTrip_MultiValue(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	want := []string{"Drama", "Crime", "Neo-Noir"}
	for _, tc := range []struct{ container, ext string }{
		{"Matroska", "mkv"},
		{"MP4", "mp4"},
	} {
		t.Run(tc.container, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "clip."+tc.ext)
			// Seed a prior genre so the test also proves the write replaces it.
			gen := exec.Command("ffmpeg", "-y", "-f", "lavfi",
				"-i", "testsrc=duration=1:size=64x64:rate=5",
				"-metadata", "genre=Horror, Thriller", "-loglevel", "error", path)
			if out, err := gen.CombinedOutput(); err != nil {
				t.Skipf("could not synthesize clip: %v: %s", err, out)
			}

			tag, ok := writeback.TagForField("genres", tc.container)
			if !ok {
				t.Fatalf("no genres write target for %s", tc.container)
			}
			if err := writeback.Write(context.Background(), path, tag, want); err != nil {
				t.Fatalf("write %s=%q: %v", tag, want, err)
			}

			ex, err := metadata.NewExtractor().Extract(context.Background(), path)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if !slices.Equal(ex.Tags, want) {
				t.Errorf("read back tags %q, want %q", ex.Tags, want)
			}
		})
	}
}

// requireTools skips unless every media binary the round trips shell out to is present.
func requireTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "ffprobe", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
}

// seedTaggedClip makes a 1 s clip whose tag keys another tool filled in:
// Genre "Drama", Keywords "Drama, Heist", Category "Heist". MKV takes them from
// ffmpeg; MP4 from exiftool, which files Keywords under the Keys group.
func seedTaggedClip(t *testing.T, ext string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip."+ext)
	args := []string{"-y", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5"}
	if ext == "mkv" {
		args = append(args, "-metadata", "genre=Drama", "-metadata", "keywords=Drama, Heist", "-metadata", "category=Heist")
	}
	if out, err := exec.Command("ffmpeg", append(args, path)...).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}
	if ext == "mp4" {
		if out, err := exec.Command("exiftool", "-m", "-overwrite_original", "-QuickTime:Genre=Drama",
			"-QuickTime:Keywords=Drama, Heist", "-QuickTime:Category=Heist", path).CombinedOutput(); err != nil {
			t.Fatalf("seed tags: %v: %s", err, out)
		}
	}
	return path
}

func tagSet(t *testing.T, path string) map[string]bool {
	t.Helper()
	ex, err := metadata.NewExtractor().Extract(context.Background(), path)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	set := map[string]bool{}
	for _, tag := range ex.Tags {
		set[tag] = true
	}
	return set
}

// ADR-110 D2 end to end: after a genres write plus its filtered tag-key writes,
// the scanner reads back only the written set — Heist, removed in the UI, no
// longer returns from Keywords or Category.
func TestTagKeyFilterRoundTrip_BothContainers(t *testing.T) {
	requireTools(t)
	for _, tc := range []struct{ container, ext string }{{"Matroska", "mkv"}, {"MP4", "mp4"}} {
		t.Run(tc.container, func(t *testing.T) {
			ctx := context.Background()
			path := seedTaggedClip(t, tc.ext)
			if got := tagSet(t, path); !got["Heist"] {
				t.Fatalf("seed not read as tags: %v", got)
			}

			present, err := writeback.ReadTagKeys(ctx, path, tc.container)
			if err != nil {
				t.Fatal(err)
			}
			genre, _ := writeback.TagForField("genres", tc.container)
			batch := []writeback.FieldWrite{{TagName: genre, Values: []string{"Drama"}}}
			for _, m := range writeback.FilterTagKeys(present, func(v string) bool { return v == "Drama" }, "manual") {
				batch = append(batch, writeback.FieldWrite{TagName: m.TagName, Values: m.Values, Delete: m.Delete})
			}
			if err := writeback.WriteBatch(ctx, path, batch); err != nil {
				t.Fatalf("write %+v: %v", batch, err)
			}

			if got := tagSet(t, path); len(got) != 1 || !got["Drama"] {
				t.Errorf("tags after write = %v, want only Drama", got)
			}
			after, err := writeback.ReadTagKeys(ctx, path, tc.container)
			if err != nil {
				t.Fatal(err)
			}
			for name := range after {
				if strings.HasSuffix(name, "Category") {
					t.Errorf("Category left on file (%v); a key filtered to nothing is deleted", after)
				}
			}
		})
	}
}

// An empty written set deletes Genre itself (ADR-110): the file holds no tags.
func TestGenreDelete_BothContainers(t *testing.T) {
	requireTools(t)
	for _, tc := range []struct{ container, ext string }{{"Matroska", "mkv"}, {"MP4", "mp4"}} {
		t.Run(tc.container, func(t *testing.T) {
			path := seedTaggedClip(t, tc.ext)
			genre, _ := writeback.TagForField("genres", tc.container)
			if err := writeback.WriteBatch(context.Background(), path,
				[]writeback.FieldWrite{{TagName: genre, Delete: true}}); err != nil {
				t.Fatal(err)
			}
			ex, err := exec.Command("exiftool", "-s3", "-Genre", path).Output()
			if err != nil || strings.TrimSpace(string(ex)) != "" {
				t.Errorf("Genre still on file: %q (err %v)", ex, err)
			}
		})
	}
}

// Every ExtraTagKeys key is one the scanner reads as tags — the premise of
// filtering them. A key the scanner ignored would be filtered for nothing; one
// it reads but this list lacks would let a removed tag come back.
func TestExtraTagKeysAreReadAsTags(t *testing.T) {
	requireTools(t)
	path := filepath.Join(t.TempDir(), "clip.mkv")
	args := []string{"-y", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5"}
	for _, k := range writeback.ExtraTagKeys {
		args = append(args, "-metadata", k+"=from-"+strings.ToLower(k))
	}
	if out, err := exec.Command("ffmpeg", append(args, path)...).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}
	got := tagSet(t, path)
	for _, k := range writeback.ExtraTagKeys {
		if !got["from-"+strings.ToLower(k)] {
			t.Errorf("scanner did not read %s as a tag (tags %v)", k, got)
		}
	}
}

// HOLODEX-466 / ADR-110 D5: an MP4 tagline no longer lands in a key the scanner
// reads as tags, so a comma in it can't split into tags on rescan.
func TestMP4TaglineIsNotReadAsTags(t *testing.T) {
	requireTools(t)
	path := filepath.Join(t.TempDir(), "clip.mp4")
	if out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=64x64:rate=5", path).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}
	tag, _ := writeback.TagForField("tagline", "MP4")
	if err := writeback.Write(context.Background(), path, tag, []string{"One heist, one night"}); err != nil {
		t.Fatal(err)
	}
	if got := tagSet(t, path); len(got) != 0 {
		t.Errorf("tagline read back as tags: %v", got)
	}
}
