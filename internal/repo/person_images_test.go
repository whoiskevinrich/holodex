package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// seedPerson upserts a video to create a person and returns the person id.
func seedPerson(t *testing.T, r *repo.Repo, name string) int64 {
	t.Helper()
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/"+name+".mkv", "T", []string{name}, nil), nil); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return personIDByName(t, r, name)
}

func TestPersonImagesCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	id, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 100, Height: 80, ByteSize: 1234})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := r.GetPersonImage(ctx, pid, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Role != model.PersonImageExtra || got.Width != 100 || got.Height != 80 {
		t.Errorf("got = %+v", got)
	}
	if got.Version != id {
		t.Errorf("version = %d, want %d (== id)", got.Version, id)
	}

	// Scoped get: another person's id can't read this image.
	other := seedPerson(t, r, "Bob")
	if _, err := r.GetPersonImage(ctx, other, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("cross-person get = %v, want ErrNotFound", err)
	}

	list, err := r.ListPersonImages(ctx, pid)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v len=%d err=%v", list, len(list), err)
	}

	// Delete is scoped too.
	if err := r.DeletePersonImage(ctx, other, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("cross-person delete = %v, want ErrNotFound", err)
	}
	if err := r.DeletePersonImage(ctx, pid, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := r.ListPersonImages(ctx, pid); len(list) != 0 {
		t.Errorf("after delete len = %d, want 0", len(list))
	}
}

func TestCoreSlotReplace(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	first, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1})
	if err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	second, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload, Width: 20, Height: 20, ByteSize: 2})
	if err != nil {
		t.Fatalf("insert 2 (replace): %v", err)
	}

	// The core slot holds exactly one image — the newest.
	list, _ := r.ListPersonImages(ctx, pid)
	headshots := 0
	for _, pi := range list {
		if pi.Role == model.PersonImageHeadshot {
			headshots++
		}
	}
	if headshots != 1 {
		t.Fatalf("headshot count = %d, want 1 (single-slot)", headshots)
	}
	cur, err := r.CorePersonImage(ctx, pid, model.PersonImageHeadshot)
	if err != nil {
		t.Fatalf("core image: %v", err)
	}
	if cur.ID != second {
		t.Errorf("core image id = %d, want %d (replaced)", cur.ID, second)
	}
	if _, err := r.GetPersonImage(ctx, pid, first); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("old core image still present: %v", err)
	}
}

func TestGalleryCap(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	for i := 0; i < repo.GalleryCap; i++ {
		if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	if n, _ := r.CountGalleryImages(ctx, pid); n != repo.GalleryCap {
		t.Fatalf("gallery count = %d, want %d", n, repo.GalleryCap)
	}
	// The 21st is refused with the typed error.
	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1}); !errors.Is(err, repo.ErrGalleryFull) {
		t.Errorf("over-cap insert = %v, want ErrGalleryFull", err)
	}
	// A core role is NOT bounded by the gallery cap.
	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1}); err != nil {
		t.Errorf("core insert blocked by gallery cap: %v", err)
	}
}

func TestReorderGallery(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	var ids []int64
	for i := 0; i < 3; i++ {
		id, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1})
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		ids = append(ids, id)
	}
	// Reverse the order.
	reversed := []int64{ids[2], ids[1], ids[0]}
	if err := r.ReorderGallery(ctx, pid, reversed); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	set, err := r.PersonImageSet(ctx, pid)
	if err != nil {
		t.Fatalf("image set: %v", err)
	}
	if len(set.Gallery) != 3 {
		t.Fatalf("gallery len = %d", len(set.Gallery))
	}
	// Gallery is ordered by sort_order; the first should now be the previously-last id.
	if set.Gallery[0].ID != ids[2] {
		t.Errorf("reordered first = %d, want %d", set.Gallery[0].ID, ids[2])
	}
}

func TestPersonImageSet(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	hs, _ := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1})
	_, _ = r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1})

	set, err := r.PersonImageSet(ctx, pid)
	if err != nil {
		t.Fatalf("image set: %v", err)
	}
	slot, ok := set.Roles[model.PersonImageHeadshot]
	if !ok || !slot.Present || slot.Version != hs {
		t.Errorf("headshot slot = %+v ok=%v, want present version %d", slot, ok, hs)
	}
	if _, ok := set.Roles[model.PersonImageBanner]; ok {
		t.Error("banner slot should be absent")
	}
	if len(set.Gallery) != 1 {
		t.Errorf("gallery len = %d, want 1", len(set.Gallery))
	}
}

func TestPersonImagesCascade(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")
	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Deleting the person cascades to its images (FK ON DELETE CASCADE).
	if _, err := database.ExecContext(ctx, `DELETE FROM people WHERE id = ?`, pid); err != nil {
		t.Fatalf("delete person: %v", err)
	}
	if list, _ := r.ListPersonImages(ctx, pid); len(list) != 0 {
		t.Errorf("images survived person delete (no cascade): %+v", list)
	}
}

func TestInsertPersonImageRejectsBadRole(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")
	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: "avatar", Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1}); err == nil {
		t.Error("expected error for invalid role")
	}
}

// TestGalleryCapConfigurable: SetGalleryCap changes the effective bound, and an
// explicit OverCap insert bypasses it (F25).
func TestGalleryCapConfigurable(t *testing.T) {
	r := newRepo(t)
	r.SetGalleryCap(2)
	if r.GalleryCapValue() != 2 {
		t.Fatalf("cap value = %d, want 2", r.GalleryCapValue())
	}
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	for i := 0; i < 2; i++ {
		if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 1, Height: 1, ByteSize: 1}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	// The 3rd is refused at the configured cap.
	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 1, Height: 1, ByteSize: 1}); !errors.Is(err, repo.ErrGalleryFull) {
		t.Errorf("over-cap insert = %v, want ErrGalleryFull", err)
	}
	// OverCap bypasses the cap.
	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 1, Height: 1, ByteSize: 1, OverCap: true}); err != nil {
		t.Errorf("over-cap insert with OverCap = %v, want success", err)
	}
	if n, _ := r.CountGalleryImages(ctx, pid); n != 3 {
		t.Errorf("gallery count = %d, want 3 (cap bypassed)", n)
	}
}

// TestDeleteSuppressesEnrichmentURL: deleting an enrichment-sourced gallery image
// records its URL for suppression; deleting an upload (no URL) or a core role does
// not (F25, ADR-043).
func TestDeleteSuppressesEnrichmentURL(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	// An enrichment gallery image carrying a source URL.
	enr, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceEnrichment, Provider: "tmdb", SourceURL: "https://cdn/a.jpg", Width: 1, Height: 1, ByteSize: 1})
	if err != nil {
		t.Fatalf("insert enrichment extra: %v", err)
	}
	// An owner upload with no source URL.
	up, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 1, Height: 1, ByteSize: 1})
	if err != nil {
		t.Fatalf("insert upload extra: %v", err)
	}
	// A core role with a source URL (must NOT suppress on delete).
	core, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceEnrichment, Provider: "tmdb", SourceURL: "https://cdn/head.jpg", Width: 1, Height: 1, ByteSize: 1})
	if err != nil {
		t.Fatalf("insert core: %v", err)
	}

	for _, id := range []int64{enr, up, core} {
		if err := r.DeletePersonImage(ctx, pid, id); err != nil {
			t.Fatalf("delete %d: %v", id, err)
		}
	}

	sup, err := r.SuppressedPersonImageURLs(ctx, pid)
	if err != nil {
		t.Fatalf("suppressed urls: %v", err)
	}
	if _, ok := sup["https://cdn/a.jpg"]; !ok {
		t.Errorf("enrichment extra url not suppressed; set = %v", sup)
	}
	if _, ok := sup["https://cdn/head.jpg"]; ok {
		t.Error("core-role url should not be suppressed")
	}
	if len(sup) != 1 {
		t.Errorf("suppressed count = %d, want 1 (upload had no url)", len(sup))
	}
}
