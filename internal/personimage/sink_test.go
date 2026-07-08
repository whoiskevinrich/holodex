package personimage

import (
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// fakeImageRepo records InsertPersonImage/DeletePersonImage calls so a Sink test can
// assert provenance and the store-failure rollback without a real DB.
type fakeImageRepo struct {
	inserts    []repo.PersonImageInsert
	deletes    []int64
	suppressed map[string]struct{}
	core       map[string]bool     // core roles reported as already filled
	locked     map[string]struct{} // core roles the owner set by hand (F33, ADR-049)
	existing   map[string]struct{} // asset URLs already stored (F34/ADR-050)
	insertErr  error               // when set, InsertPersonImage returns it (e.g. ErrDuplicateImage)
	nextID     int64
}

func (f *fakeImageRepo) InsertPersonImage(_ context.Context, in repo.PersonImageInsert) (int64, error) {
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.inserts = append(f.inserts, in)
	f.nextID++
	return f.nextID, nil
}

func (f *fakeImageRepo) DeletePersonImage(_ context.Context, _ int64, imageID int64) error {
	f.deletes = append(f.deletes, imageID)
	return nil
}

func (f *fakeImageRepo) SuppressedPersonImageURLs(_ context.Context, _ int64) (map[string]struct{}, error) {
	return f.suppressed, nil
}

func (f *fakeImageRepo) CorePersonImage(_ context.Context, _ int64, role string) (model.PersonImage, error) {
	if f.core[role] {
		return model.PersonImage{Role: role}, nil
	}
	return model.PersonImage{}, repo.ErrNotFound
}

func (f *fakeImageRepo) LockedCoreRoles(_ context.Context, _ int64) (map[string]struct{}, error) {
	return f.locked, nil
}

func (f *fakeImageRepo) ExistingPersonImageURLs(_ context.Context, _ int64) (map[string]struct{}, error) {
	return f.existing, nil
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

	if err := sink.StoreAsset(context.Background(), 7, model.PersonImageBanner, "tmdb", "tt42", "https://cdn/x.jpg", polluted, false); err != nil {
		t.Fatalf("StoreAsset: %v", err)
	}

	if len(fr.inserts) != 1 {
		t.Fatalf("inserts = %d, want 1", len(fr.inserts))
	}
	c := fr.inserts[0]
	if c.PersonID != 7 || c.Role != model.PersonImageBanner || c.Source != model.PersonImageSourceEnrichment ||
		c.Provider != "tmdb" || c.ExternalID != "tt42" || c.SourceURL != "https://cdn/x.jpg" || c.Width != 80 || c.Height != 120 {
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

// TestSinkSkipsDuplicate: when the repo reports the asset duplicates one the person
// already has (ErrDuplicateImage), StoreAsset is a silent no-op — no error to the
// caller and no file written to disk (F34/ADR-050). It also threads a content hash.
func TestSinkSkipsDuplicate(t *testing.T) {
	dir := t.TempDir()
	fr := &fakeImageRepo{insertErr: repo.ErrDuplicateImage}
	sink := NewSink(fr, dir, 0)

	if err := sink.StoreAsset(context.Background(), 5, model.PersonImageExtra, "tmdb", "x", "https://cdn/dup.jpg", jpegBytes(t, 30, 30), false); err != nil {
		t.Fatalf("duplicate StoreAsset should be a silent skip, got %v", err)
	}
	// Nothing on disk (the id was never assigned, but assert the person dir stayed empty).
	if entries, _ := os.ReadDir(filepath.Join(dir, "5")); len(entries) != 0 {
		t.Errorf("duplicate skip wrote files: %v", entries)
	}
}

// TestSinkThreadsContentHash: a stored asset carries the hash of its NORMALIZED bytes,
// so the dedup key matches what's on disk regardless of the source encoding.
func TestSinkThreadsContentHash(t *testing.T) {
	fr := &fakeImageRepo{}
	sink := NewSink(fr, t.TempDir(), 0)
	raw := jpegBytes(t, 50, 50)
	if err := sink.StoreAsset(context.Background(), 3, model.PersonImageExtra, "tmdb", "x", "https://cdn/a.jpg", raw, false); err != nil {
		t.Fatalf("StoreAsset: %v", err)
	}
	if len(fr.inserts) != 1 || fr.inserts[0].ContentHash == "" {
		t.Fatalf("insert content hash not set: %+v", fr.inserts)
	}
	// The hash is over the normalized output, not the raw input.
	norm, _, _, err := Normalize(raw, 0)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if fr.inserts[0].ContentHash != Hash(norm) {
		t.Errorf("content hash = %q, want hash of normalized bytes", fr.inserts[0].ContentHash)
	}
}

// TestSinkThreadsOverCap: overCap=true (an owner/admin enrichment run, HOLODEX-174)
// threads through to PersonImageInsert.OverCap, so the repo's gallery-cap check is
// bypassed the same way an owner's manual "Add anyway" upload bypasses it.
func TestSinkThreadsOverCap(t *testing.T) {
	fr := &fakeImageRepo{}
	sink := NewSink(fr, t.TempDir(), 0)
	if err := sink.StoreAsset(context.Background(), 3, model.PersonImageExtra, "tmdb", "x", "https://cdn/a.jpg", jpegBytes(t, 30, 30), true); err != nil {
		t.Fatalf("StoreAsset: %v", err)
	}
	if len(fr.inserts) != 1 || !fr.inserts[0].OverCap {
		t.Fatalf("insert OverCap = %+v, want true", fr.inserts)
	}
}

func TestSinkRejectsBadAsset(t *testing.T) {
	fr := &fakeImageRepo{}
	sink := NewSink(fr, t.TempDir(), 0)
	if err := sink.StoreAsset(context.Background(), 1, model.PersonImageHeadshot, "p", "x", "", []byte("not an image"), false); err == nil {
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

	if err := sink.StoreAsset(context.Background(), 9, model.PersonImageHeadshot, "p", "x", "", jpegBytes(t, 40, 40), false); err == nil {
		t.Fatal("expected a store failure")
	}
	if len(fr.inserts) != 1 {
		t.Fatalf("row should be inserted before the store attempt; inserts = %d", len(fr.inserts))
	}
	if len(fr.deletes) != 1 || fr.deletes[0] != fr.nextID {
		t.Errorf("expected rollback delete of id %d; deletes = %v", fr.nextID, fr.deletes)
	}
}
