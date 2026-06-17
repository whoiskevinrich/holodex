package personimage

import (
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/model"
)

// fakeImageRepo records InsertPersonImage/DeletePersonImage calls so a Sink test can
// assert provenance and the store-failure rollback without a real DB.
type fakeImageRepo struct {
	inserts []insertCall
	deletes []int64
	nextID  int64
}

type insertCall struct {
	personID                           int64
	role, source, provider, externalID string
	w, h, byteSize                     int
}

func (f *fakeImageRepo) InsertPersonImage(_ context.Context, personID int64, role, source, provider, externalID string, w, h, byteSize int) (int64, error) {
	f.inserts = append(f.inserts, insertCall{personID, role, source, provider, externalID, w, h, byteSize})
	f.nextID++
	return f.nextID, nil
}

func (f *fakeImageRepo) DeletePersonImage(_ context.Context, _ int64, imageID int64) error {
	f.deletes = append(f.deletes, imageID)
	return nil
}

func TestSinkStoreAssetNormalizes(t *testing.T) {
	dir := t.TempDir()
	fr := &fakeImageRepo{}
	sink := NewSink(fr, dir, 0)

	// A provider photo with a planted trailing "EXIF" marker: the enrichment path must
	// run the same metadata strip as an upload.
	src := jpegBytes(t, 80, 120)
	marker := []byte("EXIFGPS:secret-location")
	polluted := append(append([]byte{}, src...), marker...)

	if err := sink.StoreAsset(context.Background(), 7, model.PersonImageBanner, "tmdb", "tt42", polluted); err != nil {
		t.Fatalf("StoreAsset: %v", err)
	}

	if len(fr.inserts) != 1 {
		t.Fatalf("inserts = %d, want 1", len(fr.inserts))
	}
	c := fr.inserts[0]
	if c.personID != 7 || c.role != model.PersonImageBanner || c.source != model.PersonImageSourceEnrichment ||
		c.provider != "tmdb" || c.externalID != "tt42" || c.w != 80 || c.h != 120 {
		t.Errorf("insert provenance/dims = %+v", c)
	}
	if len(fr.deletes) != 0 {
		t.Errorf("unexpected rollback: %v", fr.deletes)
	}

	stored, err := os.ReadFile(ImagePath(dir, 7, fr.nextID))
	if err != nil {
		t.Fatalf("read stored asset: %v", err)
	}
	if bytes.Contains(stored, marker) {
		t.Error("stored asset still contains the trailing metadata marker")
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(stored)); err != nil || format != "jpeg" {
		t.Errorf("stored format = %q err=%v, want jpeg", format, err)
	}
}

func TestSinkRejectsBadAsset(t *testing.T) {
	fr := &fakeImageRepo{}
	sink := NewSink(fr, t.TempDir(), 0)
	if err := sink.StoreAsset(context.Background(), 1, model.PersonImageHeadshot, "p", "x", []byte("not an image")); err == nil {
		t.Error("expected error for a non-image asset")
	}
	if len(fr.inserts) != 0 {
		t.Errorf("nothing should be inserted for a bad asset; got %d", len(fr.inserts))
	}
}

func TestSinkRollsBackOnStoreFailure(t *testing.T) {
	dir := t.TempDir()
	// Block the per-person subdir by putting a FILE where Store needs a directory, so
	// Store's MkdirAll fails and the inserted row must be rolled back.
	if err := os.WriteFile(filepath.Join(dir, "9"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := &fakeImageRepo{}
	sink := NewSink(fr, dir, 0)

	if err := sink.StoreAsset(context.Background(), 9, model.PersonImageHeadshot, "p", "x", jpegBytes(t, 40, 40)); err == nil {
		t.Fatal("expected a store failure")
	}
	if len(fr.inserts) != 1 {
		t.Fatalf("row should be inserted before the store attempt; inserts = %d", len(fr.inserts))
	}
	if len(fr.deletes) != 1 || fr.deletes[0] != fr.nextID {
		t.Errorf("expected rollback delete of id %d; deletes = %v", fr.nextID, fr.deletes)
	}
}
