package personimage

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"holodex/internal/repo"
)

// fakeBackfillRepo drives Backfill with no DB: it serves a fixed missing-hash list,
// records SetPersonImageHash calls, and returns a fixed collapse victim list.
type fakeBackfillRepo struct {
	missing []repo.PersonImageRef
	hashed  map[int64]string
	victims []repo.PersonImageRef
}

func (f *fakeBackfillRepo) PersonImagesMissingHash(context.Context) ([]repo.PersonImageRef, error) {
	return f.missing, nil
}
func (f *fakeBackfillRepo) SetPersonImageHash(_ context.Context, id int64, hash string) error {
	if f.hashed == nil {
		f.hashed = map[int64]string{}
	}
	f.hashed[id] = hash
	return nil
}
func (f *fakeBackfillRepo) CollapseDuplicateGalleryExtras(context.Context) ([]repo.PersonImageRef, error) {
	return f.victims, nil
}

func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// TestBackfillHashesAndRemoves: rows with an on-disk file get hashed; a missing file is
// skipped (not fatal); collapse victims have their files removed (F34/ADR-050).
func TestBackfillHashesAndRemoves(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// person 1 image 10: real bytes on disk; image 11: file missing.
	want := []byte("normalized-jpeg-bytes")
	if err := Store(dir, 1, 10, want); err != nil {
		t.Fatal(err)
	}
	// A victim file to be removed by the collapse step.
	if err := Store(dir, 1, 12, []byte("dup")); err != nil {
		t.Fatal(err)
	}

	r := &fakeBackfillRepo{
		missing: []repo.PersonImageRef{{PersonID: 1, ID: 10}, {PersonID: 1, ID: 11}},
		victims: []repo.PersonImageRef{{PersonID: 1, ID: 12}},
	}

	hashed, removed, err := Backfill(ctx, r, dir, discardLog())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if hashed != 1 {
		t.Errorf("hashed = %d, want 1 (the missing file is skipped)", hashed)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if r.hashed[10] != Hash(want) {
		t.Errorf("image 10 hash = %q, want %q", r.hashed[10], Hash(want))
	}
	if _, ok := r.hashed[11]; ok {
		t.Error("missing-file row should not be hashed")
	}
	if _, err := os.Stat(ImagePath(dir, 1, 12)); !os.IsNotExist(err) {
		t.Error("collapse victim file should be removed")
	}
}
