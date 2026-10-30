package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// --- Merge carry-over (F23.9, HOLODEX-607) -------------------------------------------
//
// A merge keeps what only the merged-away entity had: its data from each provider the
// survivor has none from, and its photos in slots the survivor leaves empty. Where both
// hold data from one provider the survivor's wins whole, so two provider records never
// mix field by field.

func mergePair(t *testing.T, r *repo.Repo) (survivor, merged int64) {
	t.Helper()
	ctx := context.Background()
	idA, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	linkPeople(t, r, idA, "Alice")
	idB, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Ally"}, nil), nil)
	linkPeople(t, r, idB, "Ally")
	return personIDByName(t, r, "Alice"), personIDByName(t, r, "Ally")
}

func TestMergePersons_CarriesEnrichmentPerProvider(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	alice, ally := mergePair(t, r)
	person := model.EnrichEntityPerson

	// Both matched provider-1 (to different records); only the loser matched provider-2.
	if err := r.UpsertEnrichment(ctx, person, alice, "provider-1", "p1:a", map[string][]string{"name": {"Alice A"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, person, ally, "provider-1", "p1:b", map[string][]string{"name": {"Ally B"}, "aliases": {"Al"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, person, ally, "provider-2", "p2:b", map[string][]string{"name": {"Ally Two"}, "aliases": {"A2"}}); err != nil {
		t.Fatal(err)
	}

	if err := r.MergePersons(ctx, alice, ally); err != nil {
		t.Fatalf("merge: %v", err)
	}

	rows, err := r.EnrichmentForEntity(ctx, person, alice)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, row := range rows {
		got[row.Provider+"/"+row.FieldKey] = row.ExternalID
	}
	want := map[string]string{"provider-1/name": "p1:a", "provider-2/aliases": "p2:b", "provider-2/name": "p2:b"}
	if len(got) != len(want) {
		t.Fatalf("survivor enrichment = %v, want %v (provider-1 is the survivor's alone; provider-2 carried over)", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("survivor enrichment %s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	if id, ok, _ := r.MatchExternalID(ctx, person, alice, "provider-2"); !ok || id != "p2:b" {
		t.Errorf("provider-2 match after merge = %q, %v; want p2:b, true", id, ok)
	}
	if rows, _ := r.EnrichmentForEntity(ctx, person, ally); len(rows) != 0 {
		t.Errorf("merged-away enrichment left behind: %+v", rows)
	}
}

// A provider the survivor dismissed is its own verdict for that provider: the merged
// person's data from it does not come over.
func TestMergePersons_RespectsSurvivorDismissal(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	alice, ally := mergePair(t, r)
	person := model.EnrichEntityPerson
	if err := r.DismissEnrichment(ctx, person, alice, "provider-1"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, person, ally, "provider-1", "p1:b", map[string][]string{"name": {"Ally B"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.MergePersons(ctx, alice, ally); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if rows, _ := r.EnrichmentForEntity(ctx, person, alice); len(rows) != 0 {
		t.Errorf("dismissed provider's data carried over: %+v", rows)
	}
}

// The merged person's removed-photo list follows its provider match, so a re-enrich of
// the survivor does not bring those photos back.
func TestMergePersons_CarriesPhotoSuppressions(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	alice, ally := mergePair(t, r)
	id, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{
		PersonID: ally, Role: model.PersonImageExtra, Source: model.PersonImageSourceEnrichment,
		Provider: "provider-2", SourceURL: "https://img.example/1.jpg", ContentHash: "x1",
		Width: 10, Height: 10, ByteSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DeletePersonImage(ctx, ally, id); err != nil {
		t.Fatal(err)
	}
	if err := r.MergePersons(ctx, alice, ally); err != nil {
		t.Fatalf("merge: %v", err)
	}
	got, err := r.SuppressedPersonImageURLs(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["https://img.example/1.jpg"]; !ok {
		t.Errorf("survivor suppressions = %v, want the merged person's removed photo", got)
	}
}

func insertPersonImage(t *testing.T, r *repo.Repo, personID int64, role, hash string) int64 {
	t.Helper()
	id, err := r.InsertPersonImage(context.Background(), repo.PersonImageInsert{
		PersonID: personID, Role: role, Source: model.PersonImageSourceUpload,
		ContentHash: hash, Width: 10, Height: 10, ByteSize: 100,
	})
	if err != nil {
		t.Fatalf("insert %s image: %v", role, err)
	}
	return id
}

func TestMergePersons_CarriesPhotosIntoEmptySlots(t *testing.T) {
	r := newRepo(t)
	r.SetGalleryCap(3)
	ctx := context.Background()
	alice, ally := mergePair(t, r)

	keptHeadshot := insertPersonImage(t, r, alice, model.PersonImageHeadshot, "h-alice")
	insertPersonImage(t, r, alice, model.PersonImageExtra, "x-shared")
	insertPersonImage(t, r, alice, model.PersonImageExtra, "x-alice")

	insertPersonImage(t, r, ally, model.PersonImageHeadshot, "h-ally") // slot taken: dropped
	banner := insertPersonImage(t, r, ally, model.PersonImageBanner, "b-ally")
	insertPersonImage(t, r, ally, model.PersonImageExtra, "x-shared") // same photo: dropped
	firstExtra := insertPersonImage(t, r, ally, model.PersonImageExtra, "x-ally-1")
	insertPersonImage(t, r, ally, model.PersonImageExtra, "x-ally-2") // over the cap: dropped

	if err := r.MergePersons(ctx, alice, ally); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if got, err := r.CorePersonImage(ctx, alice, model.PersonImageHeadshot); err != nil || got.ID != keptHeadshot {
		t.Errorf("headshot = %+v, %v; want the survivor's own (%d)", got, err, keptHeadshot)
	}
	if got, err := r.CorePersonImage(ctx, alice, model.PersonImageBanner); err != nil || got.ID != banner {
		t.Errorf("banner = %+v, %v; want the merged person's (%d) in the empty slot", got, err, banner)
	}
	images, err := r.ListPersonImages(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	var gallery []model.PersonImage
	for _, im := range images {
		if im.Role == model.PersonImageExtra {
			gallery = append(gallery, im)
		}
	}
	if len(gallery) != 3 {
		t.Fatalf("gallery = %d images, want 3 (cap): %+v", len(gallery), gallery)
	}
	if last := gallery[2]; last.ID != firstExtra {
		t.Errorf("last gallery image = %d, want the merged person's first new extra (%d) appended after the survivor's", last.ID, firstExtra)
	}
}

func TestMergeStudios_CarriesImagesIntoEmptySlots(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	acme := seedStudio(t, r, "Acme")
	acmeCo := seedStudio(t, r, "Acme Co")

	put := func(studioID int64, role string) int64 {
		t.Helper()
		id, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
			StudioID: studioID, Role: role, Source: "upload", Width: 10, Height: 10, ByteSize: 100,
		})
		if err != nil {
			t.Fatalf("put %s: %v", role, err)
		}
		return id
	}
	keptLogo := put(acme, model.StudioImageLogo)
	put(acmeCo, model.StudioImageLogo)
	icon := put(acmeCo, model.StudioImageIcon)

	if err := r.MergeEntities(ctx, model.EnrichEntityStudio, acme, acmeCo); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got, err := r.GetStudioImage(ctx, acme, model.StudioImageLogo); err != nil || got.ID != keptLogo {
		t.Errorf("logo = %+v, %v; want the survivor's own (%d)", got, err, keptLogo)
	}
	if got, err := r.GetStudioImage(ctx, acme, model.StudioImageIcon); err != nil || got.ID != icon {
		t.Errorf("icon = %+v, %v; want the merged studio's (%d) in the empty slot", got, err, icon)
	}
}
