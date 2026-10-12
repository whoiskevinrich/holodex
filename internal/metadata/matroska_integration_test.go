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

// HOLODEX-536: exiftool flattens Matroska tags into one value per key, whatever
// the target level. Which occurrence wins has changed between versions: first in
// older ones, last in 13.25 and 13.59 (HOLODEX-565). The episode's own (level 50) tag
// therefore sits between a track-level and a collection-level one, so neither
// rule lands on it by position, and only Holodex's level selection can produce
// the episode's title and cast.
func TestExtract_MatroskaTargetLevels(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not on PATH")
	}
	path := filepath.Join(t.TempDir(), "episode.mkv")
	file := mkvFile(
		ebmlEl(ebmlIDInfo, ebmlUint(0x2AD7B1, 1000000)),
		ebmlTags(
			testTag{level: 30, uidID: ebmlIDTrackUID, uid: 9, pairs: [][2]string{{"TITLE", "Audio Track"}}},
			testTag{level: 50, pairs: [][2]string{{"TITLE", "The Episode"}, {"ACTOR", "Guest"}, {"ACTOR", "Lead"}}},
			testTag{level: 70, pairs: [][2]string{{"TITLE", "The Series"}, {"ACTOR", "Series Regular"}}},
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
