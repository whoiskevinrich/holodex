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

// ListPeople carries the headshot image id as the list avatar's ?v= cache-buster
// (F25.29): 0 when the person has no headshot, then the new image id once one is added —
// so the avatar URL changes (and the browser cache busts) after enrichment.
func TestListPeopleHeadshotVersion(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	people, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	if v := headshotVersionOf(people, pid); v != 0 {
		t.Fatalf("headshot_version with no headshot = %d, want 0", v)
	}

	hsID, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload, Width: 10, Height: 10, ByteSize: 1})
	if err != nil {
		t.Fatalf("insert headshot: %v", err)
	}

	people, err = r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	if v := headshotVersionOf(people, pid); v != hsID {
		t.Fatalf("headshot_version = %d, want the new image id %d", v, hsID)
	}
}

func headshotVersionOf(people []model.Person, id int64) int64 {
	for _, p := range people {
		if p.ID == id {
			return p.HeadshotVersion
		}
	}
	return -1
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

// TestLockedCoreRoles: only core slots the owner set by hand (upload/promoted) are
// reported as locked; a provider-set (enrichment) core slot and gallery uploads are
// not (F33, ADR-049).
func TestLockedCoreRoles(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	mk := func(role, source string) {
		if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: role, Source: source, Provider: "tmdb", Width: 1, Height: 1, ByteSize: 1}); err != nil {
			t.Fatalf("insert %s/%s: %v", role, source, err)
		}
	}
	mk(model.PersonImageHeadshot, model.PersonImageSourceUpload)   // owner-set → locked
	mk(model.PersonImageBanner, model.PersonImageSourcePromoted)   // owner-set → locked
	mk(model.PersonImagePoster, model.PersonImageSourceEnrichment) // provider-set → not locked
	mk(model.PersonImageExtra, model.PersonImageSourceUpload)      // gallery upload → not a core slot

	locked, err := r.LockedCoreRoles(ctx, pid)
	if err != nil {
		t.Fatalf("locked core roles: %v", err)
	}
	if _, ok := locked[model.PersonImageHeadshot]; !ok {
		t.Error("uploaded headshot should be locked")
	}
	if _, ok := locked[model.PersonImageBanner]; !ok {
		t.Error("promoted banner should be locked")
	}
	if _, ok := locked[model.PersonImagePoster]; ok {
		t.Error("enrichment poster should not be locked")
	}
	if len(locked) != 2 {
		t.Errorf("locked = %v, want exactly headshot+banner", locked)
	}
}

// TestGalleryDedupEnrichment: an enrichment gallery 'extra' whose content_hash already
// exists for the person — under any role — is rejected with ErrDuplicateImage, while
// owner uploads and core roles are never deduped (F34/ADR-050).
func TestGalleryDedupEnrichment(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	ins := func(role, source, hash, url string) (int64, error) {
		return r.InsertPersonImage(ctx, repo.PersonImageInsert{
			PersonID: pid, Role: role, Source: source, ContentHash: hash, SourceURL: url,
			Provider: "tmdb", Width: 1, Height: 1, ByteSize: 1,
		})
	}

	// A headshot with hash H.
	if _, err := ins(model.PersonImageHeadshot, model.PersonImageSourceEnrichment, "H", "https://cdn/head.jpg"); err != nil {
		t.Fatalf("insert headshot: %v", err)
	}
	// An enrichment extra duplicating the headshot's bytes (cross-role) → skipped.
	if _, err := ins(model.PersonImageExtra, model.PersonImageSourceEnrichment, "H", "https://cdn/dup.jpg"); !errors.Is(err, repo.ErrDuplicateImage) {
		t.Fatalf("cross-role dup extra = %v, want ErrDuplicateImage", err)
	}
	// A distinct enrichment extra (hash G) → stored.
	if _, err := ins(model.PersonImageExtra, model.PersonImageSourceEnrichment, "G", "https://cdn/g.jpg"); err != nil {
		t.Fatalf("distinct extra: %v", err)
	}
	// Re-enrich offering hash G again → skipped.
	if _, err := ins(model.PersonImageExtra, model.PersonImageSourceEnrichment, "G", "https://cdn/g2.jpg"); !errors.Is(err, repo.ErrDuplicateImage) {
		t.Fatalf("re-enrich dup extra = %v, want ErrDuplicateImage", err)
	}
	// An OWNER upload duplicating hash G → allowed (deliberate; never deduped).
	if _, err := ins(model.PersonImageExtra, model.PersonImageSourceUpload, "G", ""); err != nil {
		t.Fatalf("owner upload dup should be allowed: %v", err)
	}
	// A core role duplicating hash G (e.g. the F25.29 poster seed reusing headshot bytes)
	// → allowed; core inserts are never deduped.
	if _, err := ins(model.PersonImagePoster, model.PersonImageSourceEnrichment, "G", "https://cdn/g.jpg"); err != nil {
		t.Fatalf("core insert dup should be allowed: %v", err)
	}

	// Net gallery: G (enrichment) + G (owner upload) = 2 extras; the two H/G dup
	// enrichment extras were skipped.
	n, err := r.CountGalleryImages(ctx, pid)
	if err != nil {
		t.Fatalf("count gallery: %v", err)
	}
	if n != 2 {
		t.Errorf("gallery count = %d, want 2", n)
	}
}

// TestExistingPersonImageURLs returns every stored non-empty source_url for a person
// (any role), the input to the enrichment URL fast-path (F34/ADR-050).
func TestExistingPersonImageURLs(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	mk := func(role, source, url string) {
		if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{
			PersonID: pid, Role: role, Source: source, SourceURL: url, ContentHash: url,
			Provider: "tmdb", Width: 1, Height: 1, ByteSize: 1,
		}); err != nil {
			t.Fatalf("insert %s: %v", url, err)
		}
	}
	mk(model.PersonImageHeadshot, model.PersonImageSourceEnrichment, "https://cdn/head.jpg")
	mk(model.PersonImageExtra, model.PersonImageSourceEnrichment, "https://cdn/g.jpg")
	mk(model.PersonImageExtra, model.PersonImageSourceUpload, "") // upload, no url → not reported

	urls, err := r.ExistingPersonImageURLs(ctx, pid)
	if err != nil {
		t.Fatalf("existing urls: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("existing urls = %v, want 2", urls)
	}
	if _, ok := urls["https://cdn/head.jpg"]; !ok {
		t.Error("headshot url missing")
	}
	if _, ok := urls["https://cdn/g.jpg"]; !ok {
		t.Error("gallery url missing")
	}
}

// TestCollapseDuplicateGalleryExtras: the one-time backfill collapse keeps the earliest
// extra of a hash, drops an extra that duplicates a CORE image (core wins), never
// deletes a core image, and is idempotent (F34/ADR-050).
func TestCollapseDuplicateGalleryExtras(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	ins := func(role, hash string) int64 {
		// Pre-hashed rows simulate the post-backfill state; use upload source so the
		// insert-time dedup (enrichment-only) doesn't reject the planted duplicates.
		id, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{
			PersonID: pid, Role: role, Source: model.PersonImageSourceUpload, ContentHash: hash,
			Width: 1, Height: 1, ByteSize: 1,
		})
		if err != nil {
			t.Fatalf("insert %s/%s: %v", role, hash, err)
		}
		return id
	}

	head := ins(model.PersonImageHeadshot, "H") // core, hash H
	g1 := ins(model.PersonImageExtra, "A")      // earliest A → kept
	g2 := ins(model.PersonImageExtra, "A")      // dup A → dropped
	g3 := ins(model.PersonImageExtra, "A")      // dup A → dropped
	uniq := ins(model.PersonImageExtra, "B")    // unique → kept
	gh := ins(model.PersonImageExtra, "H")      // matches the core headshot → dropped (core wins)

	victims, err := r.CollapseDuplicateGalleryExtras(ctx)
	if err != nil {
		t.Fatalf("collapse: %v", err)
	}
	gotDeleted := map[int64]bool{}
	for _, v := range victims {
		gotDeleted[v.ID] = true
	}
	for _, id := range []int64{g2, g3, gh} {
		if !gotDeleted[id] {
			t.Errorf("expected %d collapsed", id)
		}
	}
	for _, id := range []int64{head, g1, uniq} {
		if gotDeleted[id] {
			t.Errorf("did not expect %d collapsed", id)
		}
	}

	// Survivors: headshot + g1(A) + uniq(B) — gallery has 2 extras, core intact.
	if n, _ := r.CountGalleryImages(ctx, pid); n != 2 {
		t.Errorf("post-collapse gallery = %d, want 2", n)
	}
	if _, err := r.GetPersonImage(ctx, pid, head); err != nil {
		t.Errorf("core headshot should survive: %v", err)
	}

	// Idempotent: a second pass collapses nothing.
	again, err := r.CollapseDuplicateGalleryExtras(ctx)
	if err != nil {
		t.Fatalf("collapse again: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second collapse removed %d, want 0", len(again))
	}
}

// TestPersonImagesMissingHashAndSet: the backfill's read/write of content_hash.
func TestPersonImagesMissingHashAndSet(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	pid := seedPerson(t, r, "Alice")

	// A row inserted with no ContentHash is "missing".
	id, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{PersonID: pid, Role: model.PersonImageExtra, Source: model.PersonImageSourceUpload, Width: 1, Height: 1, ByteSize: 1})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	missing, err := r.PersonImagesMissingHash(ctx)
	if err != nil {
		t.Fatalf("missing: %v", err)
	}
	if len(missing) != 1 || missing[0].ID != id {
		t.Fatalf("missing = %+v, want exactly id %d", missing, id)
	}
	if err := r.SetPersonImageHash(ctx, id, "deadbeef"); err != nil {
		t.Fatalf("set hash: %v", err)
	}
	missing, err = r.PersonImagesMissingHash(ctx)
	if err != nil {
		t.Fatalf("missing after set: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing after set = %d, want 0", len(missing))
	}
}
