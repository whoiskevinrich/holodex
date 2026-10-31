package writeback

import (
	"context"
	"os"
	"strings"
	"testing"

	"holodex/internal/fieldsource"
	"holodex/internal/thumbnail"
)

// withLocalPosterDir points the slot directory at a temp dir for one test.
func withLocalPosterDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := localPosterDir
	localPosterDir = dir
	t.Cleanup(func() { localPosterDir = prev })
	return dir
}

// noFetch fails the test if the guarded downloader is ever reached: a local
// poster must cross no network perimeter (security-perimeter.md).
func noFetch(t *testing.T) {
	withImageFetcher(t, func(context.Context, string) ([]byte, error) {
		t.Fatal("a local poster reached the image fetcher")
		return nil, nil
	})
}

func writeSlotFile(t *testing.T, dir string, id int64, slot string, data []byte) {
	t.Helper()
	if err := os.WriteFile(thumbnail.SlotPath(dir, id, slot), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// G10: a local token is copied from the job's own slot onto the cover path, the
// fetcher is never called, and the ledger token names the bytes copied.
func TestStageLocalPostersCopiesSlot(t *testing.T) {
	dir := withLocalPosterDir(t)
	noFetch(t)
	frame := []byte("\xff\xd8\xff frame bytes")
	writeSlotFile(t, dir, 7, thumbnail.SlotFrame, frame)
	token := fieldsource.LocalToken("frame", thumbnail.ContentHash(frame))
	batch := []FieldWrite{
		{TagName: "Title", Values: []string{"A title"}},
		{TagName: "cover.jpg", Values: []string{token}, IsImage: true},
	}

	written, cleanup, err := StageLocalPosters(7, batch)
	if err != nil {
		t.Fatal(err)
	}
	if written[1] != token || len(written) != 1 {
		t.Fatalf("written = %v, want the token at index 1", written)
	}
	path, done, err := imageFile(context.Background(), batch[1])
	if err != nil {
		t.Fatal(err)
	}
	done()
	if got, _ := os.ReadFile(path); string(got) != string(frame) {
		t.Fatalf("staged copy = %q", got)
	}
	if batch[0].Values[0] != "A title" || batch[0].staged {
		t.Fatal("a text field was touched")
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cleanup left the temp copy")
	}
}

// G11: the slot is replaced between enqueue and write — the ledger records the
// bytes copied, so the next resolve reads will-write, not in sync.
func TestStageLocalPostersRecordsCopiedHash(t *testing.T) {
	dir := withLocalPosterDir(t)
	noFetch(t)
	enqueued := fieldsource.LocalToken("upload", thumbnail.ContentHash([]byte("old upload")))
	writeSlotFile(t, dir, 3, thumbnail.SlotUpload, []byte("new upload"))
	batch := []FieldWrite{{TagName: "QuickTime:CoverArt", Values: []string{enqueued}, IsImage: true}}
	written, cleanup, err := StageLocalPosters(3, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if want := fieldsource.LocalToken("upload", thumbnail.ContentHash([]byte("new upload"))); written[0] != want {
		t.Fatalf("ledger token = %q, want %q", written[0], want)
	}
}

// G10: an unknown or path-shaped token is refused, and a missing slot fails the
// job with nothing substituted.
func TestStageLocalPostersRefuses(t *testing.T) {
	dir := withLocalPosterDir(t)
	noFetch(t)
	hash := strings.Repeat("a", 64)
	writeSlotFile(t, dir, 1, thumbnail.SlotFile, []byte("file art"))
	for _, v := range []string{"local:file#" + hash, "local:../1-file#" + hash, "local:frame", "local:upload#" + hash} {
		batch := []FieldWrite{{TagName: "cover.jpg", Values: []string{v}, IsImage: true}}
		if _, cleanup, err := StageLocalPosters(1, batch); err == nil {
			cleanup()
			t.Errorf("%q staged, want refused", v)
		} else if batch[0].staged {
			t.Errorf("%q marked staged on failure", v)
		}
	}
}

// An unstaged token never reaches a file: the downloader refuses a non-https value.
func TestUnstagedLocalTokenIsNotDownloaded(t *testing.T) {
	noFetch(t)
	f := FieldWrite{TagName: "cover.jpg", Values: []string{"local:frame#" + strings.Repeat("a", 64)}, IsImage: true}
	if _, _, err := imageFile(context.Background(), f); err == nil {
		t.Fatal("an unstaged token was embedded")
	}
}
