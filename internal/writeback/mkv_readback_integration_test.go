//go:build integration

package writeback

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"holodex/internal/metadata"
)

// HOLODEX-505: on an ffmpeg-muxed MKV, a title edit plus a grown tag document
// leaves the Tags element where exiftool no longer finds it without -ee. Title
// still reads, so nothing looks wrong — but the post-write re-read saw no Genre
// and unlinked every file-sourced tag from the video. Every exiftool reader of
// a Matroska file must see the tags the write just put there.
func TestWriteMKV_TitleAndTagsReadBackEverywhere(t *testing.T) {
	requireCoverTools(t, "mkvpropedit", "mkvextract", "mkvmerge", "exiftool")
	clip := filepath.Join(t.TempDir(), "clip.mkv")
	// A clip long enough to have Clusters; untitled, with a smaller Genre than
	// the write — the shape of the reported file.
	if out, err := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=20:size=320x240:rate=25",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-metadata", "genre=alpha, hotel, golf, foxtrot, bravo", clip).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}

	genres := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	keywords := []string{"india", "juliett"}
	err := WriteBatch(context.Background(), clip, []FieldWrite{
		{TagName: "Title", Values: []string{"A Reasonably Long Title For The Video"}},
		{TagName: "Genre", Values: genres},
		{TagName: "Keywords", Values: keywords},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// The scanner's read (the post-write re-read that relinks file tags).
	ex, err := metadata.NewExtractor().Extract(context.Background(), clip)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, want := range append(slices.Clone(genres), keywords...) {
		if !slices.Contains(ex.Tags, want) {
			t.Errorf("scanner read-back missing %q (got %v)", want, ex.Tags)
		}
	}

	// The pre-write snapshot / Genre-delete check.
	cur, err := ReadCurrentValues(context.Background(), clip, []Mapped{{Field: "genres", TagName: "Genre"}})
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if want := fileValue(FieldWrite{Values: genres}); cur["genres"] != want {
		t.Errorf("ReadCurrentValues genres = %q, want %q", cur["genres"], want)
	}

	// The tag-key filter's read.
	keys, err := ReadTagKeys(context.Background(), clip, "Matroska")
	if err != nil {
		t.Fatalf("read tag keys: %v", err)
	}
	if got := keys["Keywords"]; !slices.Equal(got, keywords) {
		t.Errorf("ReadTagKeys Keywords = %v, want %v (all keys: %v)", got, keywords, keys)
	}
}
