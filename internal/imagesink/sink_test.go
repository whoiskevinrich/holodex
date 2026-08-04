package imagesink

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/studioimage"
)

// jpegBytes encodes a solid w×h JPEG with a fake "EXIF-ish" trailing marker so a
// test can assert re-encoding drops anything that isn't pixels.
func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0x40, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode source jpeg: %v", err)
	}
	return buf.Bytes()
}

// fakePersonRepo records InsertPersonImage/DeletePersonImage calls so a Sink test
// can assert provenance and the store-failure rollback without a real DB.
type fakePersonRepo struct {
	inserts    []repo.PersonImageInsert
	deletes    []int64
	suppressed map[string]struct{}
	core       map[string]bool
	locked     map[string]struct{}
	existing   map[string]struct{}
	insertErr  error
	nextID     int64
}

func (f *fakePersonRepo) InsertPersonImage(_ context.Context, in repo.PersonImageInsert) (int64, error) {
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.inserts = append(f.inserts, in)
	f.nextID++
	return f.nextID, nil
}
func (f *fakePersonRepo) DeletePersonImage(_ context.Context, _ int64, imageID int64) error {
	f.deletes = append(f.deletes, imageID)
	return nil
}
func (f *fakePersonRepo) SuppressedPersonImageURLs(_ context.Context, _ int64) (map[string]struct{}, error) {
	return f.suppressed, nil
}
func (f *fakePersonRepo) CorePersonImage(_ context.Context, _ int64, role string) (model.PersonImage, error) {
	if f.core[role] {
		return model.PersonImage{Role: role}, nil
	}
	return model.PersonImage{}, repo.ErrNotFound
}
func (f *fakePersonRepo) LockedCoreRoles(_ context.Context, _ int64) (map[string]struct{}, error) {
	return f.locked, nil
}
func (f *fakePersonRepo) ExistingPersonImageURLs(_ context.Context, _ int64) (map[string]struct{}, error) {
	return f.existing, nil
}

// fakeStudioRepo mirrors fakePersonRepo for the studio side.
type fakeStudioRepo struct {
	inserts   []repo.StudioImageInsert
	deletes   []string // roles deleted
	existing  map[string]repo.StudioImage
	insertErr error
	nextID    int64
}

func (f *fakeStudioRepo) ReplaceStudioImage(_ context.Context, in repo.StudioImageInsert) (int64, error) {
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.inserts = append(f.inserts, in)
	f.nextID++
	return f.nextID, nil
}
func (f *fakeStudioRepo) GetStudioImage(_ context.Context, _ int64, role string) (repo.StudioImage, error) {
	if img, ok := f.existing[role]; ok {
		return img, nil
	}
	return repo.StudioImage{}, repo.ErrNotFound
}
func (f *fakeStudioRepo) DeleteStudioImage(_ context.Context, _ int64, role string) error {
	f.deletes = append(f.deletes, role)
	return nil
}
func (f *fakeStudioRepo) LockedStudioImageRoles(_ context.Context, _ int64) (map[string]struct{}, error) {
	return nil, nil
}

func TestSinkStoreAsset_Person_Normalizes(t *testing.T) {
	dir := t.TempDir()
	fr := &fakePersonRepo{}
	sink := New(fr, dir, 0, &fakeStudioRepo{}, t.TempDir(), 0)

	// A provider photo with a planted trailing "EXIF" marker: the enrichment path must
	// run the same metadata strip as an upload.
	src := jpegBytes(t, 80, 120)
	marker := []byte("EXIFGPS:secret-location")
	polluted := append(append([]byte{}, src...), marker...)

	if err := sink.StoreAsset(context.Background(), model.EnrichEntityPerson, 7, model.PersonImageBanner, "tmdb", "tt42", "https://cdn/x.jpg", polluted, false); err != nil {
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

	stored, err := os.ReadFile(personimage.ImagePath(dir, 7, fr.nextID))
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
// already has (ErrDuplicateImage), StoreAsset is a silent no-op (F34/ADR-050).
func TestSinkSkipsDuplicate(t *testing.T) {
	dir := t.TempDir()
	fr := &fakePersonRepo{insertErr: repo.ErrDuplicateImage}
	sink := New(fr, dir, 0, &fakeStudioRepo{}, t.TempDir(), 0)

	if err := sink.StoreAsset(context.Background(), model.EnrichEntityPerson, 5, model.PersonImageExtra, "tmdb", "x", "https://cdn/dup.jpg", jpegBytes(t, 30, 30), false); err != nil {
		t.Fatalf("duplicate StoreAsset should be a silent skip, got %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "5")); len(entries) != 0 {
		t.Errorf("duplicate skip wrote files: %v", entries)
	}
}

func TestSinkRollsBackOnStoreFailure_Person(t *testing.T) {
	dir := t.TempDir()
	// Block the per-person subdir by putting a FILE where Store needs a directory, so
	// Store's MkdirAll fails and the inserted row must be rolled back.
	if err := os.WriteFile(filepath.Join(dir, "9"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := &fakePersonRepo{}
	sink := New(fr, dir, 0, &fakeStudioRepo{}, t.TempDir(), 0)

	if err := sink.StoreAsset(context.Background(), model.EnrichEntityPerson, 9, model.PersonImageHeadshot, "p", "x", "", jpegBytes(t, 40, 40), false); err == nil {
		t.Fatal("expected a store failure")
	}
	if len(fr.inserts) != 1 {
		t.Fatalf("row should be inserted before the store attempt; inserts = %d", len(fr.inserts))
	}
	if len(fr.deletes) != 1 || fr.deletes[0] != fr.nextID {
		t.Errorf("expected rollback delete of id %d; deletes = %v", fr.nextID, fr.deletes)
	}
}

// TestSinkStoreAsset_Studio_Normalizes mirrors the person test for the studio side
// (F51, ADR-079): normalize runs, the row records enrichment provenance, and the
// superseded file (if any) is removed after a successful replace.
func TestSinkStoreAsset_Studio_Normalizes(t *testing.T) {
	dir := t.TempDir()
	sr := &fakeStudioRepo{}
	sink := New(&fakePersonRepo{}, t.TempDir(), 0, sr, dir, 0)

	if err := sink.StoreAsset(context.Background(), model.EnrichEntityStudio, 3, model.StudioImageLogo, "tmdb", "tmdb:10342", "https://cdn/logo.jpg", jpegBytes(t, 100, 40), false); err != nil {
		t.Fatalf("StoreAsset: %v", err)
	}
	if len(sr.inserts) != 1 {
		t.Fatalf("inserts = %d, want 1", len(sr.inserts))
	}
	c := sr.inserts[0]
	if c.StudioID != 3 || c.Role != model.StudioImageLogo || c.Source != model.StudioImageSourceEnrichment ||
		c.Provider != "tmdb" || c.ExternalID != "tmdb:10342" || c.Width != 100 || c.Height != 40 {
		t.Errorf("insert provenance/dims = %+v", c)
	}
	stored, err := os.ReadFile(studioimage.ImagePath(dir, 3, sr.nextID))
	if err != nil {
		t.Fatalf("read stored asset: %v", err)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(stored)); err != nil || format != "jpeg" {
		t.Errorf("stored format = %q err=%v, want jpeg", format, err)
	}
}

// TestSinkRollsBackOnStoreFailure_Studio: a disk-store failure rolls back the just-
// inserted studio_images row.
func TestSinkRollsBackOnStoreFailure_Studio(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sr := &fakeStudioRepo{}
	sink := New(&fakePersonRepo{}, t.TempDir(), 0, sr, dir, 0)

	if err := sink.StoreAsset(context.Background(), model.EnrichEntityStudio, 4, model.StudioImageLogo, "p", "x", "", jpegBytes(t, 40, 40), false); err == nil {
		t.Fatal("expected a store failure")
	}
	if len(sr.inserts) != 1 {
		t.Fatalf("row should be inserted before the store attempt; inserts = %d", len(sr.inserts))
	}
	if len(sr.deletes) != 1 || sr.deletes[0] != model.StudioImageLogo {
		t.Errorf("expected rollback delete of role logo; deletes = %v", sr.deletes)
	}
}

// TestSinkUnsupportedEntityType: StoreAsset errors for an entity type neither engine
// backs, rather than silently doing nothing.
func TestSinkUnsupportedEntityType(t *testing.T) {
	sink := New(&fakePersonRepo{}, t.TempDir(), 0, &fakeStudioRepo{}, t.TempDir(), 0)
	if err := sink.StoreAsset(context.Background(), "video", 1, "poster", "p", "x", "", jpegBytes(t, 10, 10), false); err == nil {
		t.Fatal("expected an error for an unsupported entity type")
	}
}
