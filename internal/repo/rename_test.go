package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// --- RenamePerson (F37 P0-5 / RD1) ---------------------------------------------------

func TestRenamePerson(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	alice := personIDByName(t, r, "Alice")

	if cid, err := r.RenamePerson(ctx, alice, "Alicia"); err != nil || cid != 0 {
		t.Fatalf("rename: cid=%d err=%v", cid, err)
	}
	p, err := r.GetPerson(ctx, alice)
	if err != nil || p.Name != "Alicia" {
		t.Fatalf("renamed person = %+v (%v)", p, err)
	}
	// The old name is an alias — search (FTS) and scan routing keep matching it.
	if len(p.Aliases) != 1 || p.Aliases[0].Alias != "Alice" {
		t.Fatalf("old name should be an alias: %+v", p.Aliases)
	}
	res, err := r.Search(ctx, "Alice", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if countPeople(res.People, alice) != 1 {
		t.Error("old name must stay search-matchable after rename")
	}
	// A new file crediting the old name links to the renamed person (F23 routing).
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Alice"}, nil), nil); err != nil {
		t.Fatalf("upsert old-name file: %v", err)
	}
	if people, _ := r.ListPeople(ctx, false); len(people) != 1 {
		t.Errorf("old-name scan must route to the renamed person: %+v", people)
	}
}

func TestRenamePerson_ConflictLeavesEverythingUntouched(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Bob"}, nil), nil)
	alice := personIDByName(t, r, "Alice")
	bob := personIDByName(t, r, "Bob")

	cid, err := r.RenamePerson(ctx, alice, "Bob")
	if !errors.Is(err, repo.ErrNameTaken) || cid != bob {
		t.Fatalf("colliding rename: cid=%d err=%v, want (%d, ErrNameTaken)", cid, err, bob)
	}
	p, _ := r.GetPerson(ctx, alice)
	if p.Name != "Alice" || len(p.Aliases) != 0 {
		t.Errorf("failed rename must not mutate: %+v", p)
	}
}

func TestRenamePerson_NoOpAndNotFound(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	alice := personIDByName(t, r, "Alice")

	if cid, err := r.RenamePerson(ctx, alice, "Alice"); err != nil || cid != 0 {
		t.Fatalf("no-op rename: cid=%d err=%v", cid, err)
	}
	p, _ := r.GetPerson(ctx, alice)
	if len(p.Aliases) != 0 {
		t.Errorf("no-op rename must not add a self-alias: %+v", p.Aliases)
	}
	if _, err := r.RenamePerson(ctx, 99999, "Zed"); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("unknown person rename = %v, want ErrNotFound", err)
	}
}

func TestRenamePerson_ToOwnAliasTidiesSelfAlias(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	alice := personIDByName(t, r, "Alice")
	if _, err := r.AddPersonAlias(ctx, alice, "Ally"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// Renaming onto one of the person's own aliases is allowed; the now-redundant
	// alias row (equal to the canonical name) is tidied, mirroring the merge
	// transaction's self-alias cleanup.
	if cid, err := r.RenamePerson(ctx, alice, "Ally"); err != nil || cid != 0 {
		t.Fatalf("rename to own alias: cid=%d err=%v", cid, err)
	}
	p, _ := r.GetPerson(ctx, alice)
	if p.Name != "Ally" {
		t.Fatalf("renamed person = %+v", p)
	}
	if len(p.Aliases) != 1 || p.Aliases[0].Alias != "Alice" {
		t.Errorf("want the old name as the only alias (self-alias tidied): %+v", p.Aliases)
	}
}

// --- Merge cleanup (F37 P0-6 / RD5) ---------------------------------------------------

func TestMergePersons_DropsDecisionsAndCuration(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	_, _ = r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Bob"}, nil), nil)
	alice := personIDByName(t, r, "Alice")
	bob := personIDByName(t, r, "Bob")

	for _, pid := range []int64{alice, bob} {
		if err := r.SetDecision(ctx, model.EnrichEntityPerson, pid, "bio", "manual", "words"); err != nil {
			t.Fatalf("seed decision: %v", err)
		}
		if err := r.SetCuration(ctx, model.EnrichEntityPerson, pid, "aliases", "Zed", repo.CurationSuppress); err != nil {
			t.Fatalf("seed curation: %v", err)
		}
	}

	if err := r.MergePersons(ctx, alice, bob); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// The merged-away person's rows are gone; the canonical's are untouched.
	if rows, _ := r.DecisionsForEntity(ctx, model.EnrichEntityPerson, bob); len(rows) != 0 {
		t.Errorf("merged-away decisions must be dropped: %+v", rows)
	}
	if rows, _ := r.CurationForEntity(ctx, model.EnrichEntityPerson, bob); len(rows) != 0 {
		t.Errorf("merged-away curation must be dropped: %+v", rows)
	}
	if rows, _ := r.DecisionsForEntity(ctx, model.EnrichEntityPerson, alice); len(rows) != 1 {
		t.Errorf("canonical decisions must survive: %+v", rows)
	}
	if rows, _ := r.CurationForEntity(ctx, model.EnrichEntityPerson, alice); len(rows) != 1 {
		t.Errorf("canonical curation must survive: %+v", rows)
	}
}
