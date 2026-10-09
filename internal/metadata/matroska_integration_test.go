//go:build integration

package metadata

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// HOLODEX-536: exiftool flattens Matroska tags with the first occurrence of a
// key winning, whatever its target level. A track-level and a collection-level
// tag ahead of the episode's own must not become the episode's title or cast.
func TestExtract_MatroskaTargetLevels(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not on PATH")
	}
	path := filepath.Join(t.TempDir(), "episode.mkv")
	file := mkvFile(
		ebmlEl(ebmlIDInfo, ebmlUint(0x2AD7B1, 1000000)),
		ebmlTags(
			testTag{level: 30, uidID: ebmlIDTrackUID, uid: 9, pairs: [][2]string{{"TITLE", "Audio Track"}}},
			testTag{level: 70, pairs: [][2]string{{"TITLE", "The Series"}, {"ACTOR", "Series Regular"}}},
			testTag{level: 50, pairs: [][2]string{{"TITLE", "The Episode"}, {"ACTOR", "Guest"}, {"ACTOR", "Lead"}}},
		),
	)
	if err := os.WriteFile(path, file, 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewExtractor()
	raw, err := e.runExiftool(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if naive := mapExiftool(raw); naive.Title == "The Episode" {
		t.Fatalf("exiftool alone already picks the episode title; this test no longer exercises HOLODEX-536")
	}

	ex, err := e.Extract(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Title != "The Episode" {
		t.Errorf("Title = %q, want %q", ex.Title, "The Episode")
	}
	if want := []string{"Guest", "Lead"}; !slices.Equal(ex.People, want) {
		t.Errorf("People = %v, want %v", ex.People, want)
	}
}
