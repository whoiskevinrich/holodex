package repo_test

import (
	"context"
	"testing"
	"time"
)

func TestFindTitleCollision(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	when := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

	a := sampleVideo("/m/a.mkv", "Session One", []string{"Alice", "Bob"}, nil)
	a.RecordedAt = &when
	aID, err := r.UpsertVideo(ctx, a, nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	linkPeople(t, r, aID, "Alice", "Bob")
	if err := r.ReconcileVideoStudios(ctx, aID, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio a: %v", err)
	}

	b := sampleVideo("/m/b.mkv", "Session Two", []string{"Alice", "Bob"}, nil)
	b.RecordedAt = &when
	bID, err := r.UpsertVideo(ctx, b, nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}
	linkPeople(t, r, bID, "Alice", "Bob")
	if err := r.ReconcileVideoStudios(ctx, bID, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio b: %v", err)
	}

	// Renaming b to a's exact normalized title (case/whitespace-insensitive) with an
	// otherwise identical composite key collides with a.
	collision, err := r.FindTitleCollision(ctx, bID, "  session ONE  ")
	if err != nil {
		t.Fatalf("find collision: %v", err)
	}
	if collision == nil {
		t.Fatal("want a collision, got none")
	}
	if collision.ID != aID {
		t.Errorf("collision id = %d, want %d", collision.ID, aID)
	}
	if collision.Title != "Session One" {
		t.Errorf("collision title = %q", collision.Title)
	}
	wantPeople := map[string]bool{"Alice": true, "Bob": true}
	if len(collision.People) != 2 || !wantPeople[collision.People[0]] || !wantPeople[collision.People[1]] {
		t.Errorf("collision people = %v", collision.People)
	}
	if collision.Studio == nil || *collision.Studio != "Acme" {
		t.Errorf("collision studio = %v", collision.Studio)
	}
	if collision.RecordedAt == nil || *collision.RecordedAt != when.Format(time.RFC3339) {
		t.Errorf("collision recorded_at = %v", collision.RecordedAt)
	}

	// A genuinely distinct title produces no collision.
	if collision, err := r.FindTitleCollision(ctx, bID, "Session Three"); err != nil || collision != nil {
		t.Errorf("distinct title: collision=%v err=%v", collision, err)
	}

	// Same normalized title but a different people set is not a collision.
	c := sampleVideo("/m/c.mkv", "Session Four", []string{"Carol"}, nil)
	c.RecordedAt = &when
	cID, err := r.UpsertVideo(ctx, c, nil)
	if err != nil {
		t.Fatalf("seed c: %v", err)
	}
	linkPeople(t, r, cID, "Carol")
	if collision, err := r.FindTitleCollision(ctx, cID, "Session One"); err != nil || collision != nil {
		t.Errorf("different people: collision=%v err=%v", collision, err)
	}

	// A soft-deleted video with an otherwise-matching key is not reported.
	if err := r.SoftDelete(ctx, aID); err != nil {
		t.Fatalf("soft delete a: %v", err)
	}
	if collision, err := r.FindTitleCollision(ctx, bID, "Session One"); err != nil || collision != nil {
		t.Errorf("soft-deleted source excluded: collision=%v err=%v", collision, err)
	}
}

// TestFindTitleCollision_NoPeople confirms a colliding video with zero linked
// people reports People as an empty (non-nil) slice, never nil. Caught live: a
// nil slice marshals to JSON `null`, and CollisionOfferCard.svelte reads
// video.people.length unconditionally, crashing the page (HOLODEX-270).
func TestFindTitleCollision_NoPeople(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a := sampleVideo("/m/a.mkv", "Solo Session", nil, nil)
	aID, err := r.UpsertVideo(ctx, a, nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	b := sampleVideo("/m/b.mkv", "Other Session", nil, nil)
	bID, err := r.UpsertVideo(ctx, b, nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}

	collision, err := r.FindTitleCollision(ctx, bID, "Solo Session")
	if err != nil {
		t.Fatalf("find collision: %v", err)
	}
	if collision == nil {
		t.Fatal("want a collision, got none")
	}
	if collision.ID != aID {
		t.Errorf("collision id = %d, want %d", collision.ID, aID)
	}
	if collision.People == nil {
		t.Error("collision.People is nil, want an empty non-nil slice")
	}
	if len(collision.People) != 0 {
		t.Errorf("collision.People = %v, want empty", collision.People)
	}
}

func TestFindStudioCollision(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	when := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

	a := sampleVideo("/m/a.mkv", "Session One", []string{"Alice", "Bob"}, nil)
	a.RecordedAt = &when
	aID, err := r.UpsertVideo(ctx, a, nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	linkPeople(t, r, aID, "Alice", "Bob")
	if err := r.ReconcileVideoStudios(ctx, aID, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio a: %v", err)
	}

	// b shares a's exact title/date/people but a different studio — reassigning
	// b's studio to "Acme" (any case/whitespace) should collide with a.
	b := sampleVideo("/m/b.mkv", "Session One", []string{"Alice", "Bob"}, nil)
	b.RecordedAt = &when
	bID, err := r.UpsertVideo(ctx, b, nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}
	linkPeople(t, r, bID, "Alice", "Bob")
	if err := r.ReconcileVideoStudios(ctx, bID, []string{"Other"}, nil); err != nil {
		t.Fatalf("link studio b: %v", err)
	}

	// Reassigning b's studio to a's studio name (by name, not id — the create-new
	// path has no id yet) collides.
	collision, err := r.FindStudioCollision(ctx, bID, []string{"  ACME  "})
	if err != nil {
		t.Fatalf("find collision: %v", err)
	}
	if collision == nil {
		t.Fatal("want a collision, got none")
	}
	if collision.ID != aID {
		t.Errorf("collision id = %d, want %d", collision.ID, aID)
	}
	if collision.Studio == nil || *collision.Studio != "Acme" {
		t.Errorf("collision studio = %v", collision.Studio)
	}

	// A distinct proposed studio name produces no collision.
	if collision, err := r.FindStudioCollision(ctx, bID, []string{"Nonexistent Studio"}); err != nil || collision != nil {
		t.Errorf("distinct studio: collision=%v err=%v", collision, err)
	}

	// Same title/date/studio but a different people set is not a collision.
	c := sampleVideo("/m/c.mkv", "Session One", []string{"Carol"}, nil)
	c.RecordedAt = &when
	cID, err := r.UpsertVideo(ctx, c, nil)
	if err != nil {
		t.Fatalf("seed c: %v", err)
	}
	linkPeople(t, r, cID, "Carol")
	if err := r.ReconcileVideoStudios(ctx, cID, []string{"Other"}, nil); err != nil {
		t.Fatalf("link studio c: %v", err)
	}
	if collision, err := r.FindStudioCollision(ctx, cID, []string{"Acme"}); err != nil || collision != nil {
		t.Errorf("different people: collision=%v err=%v", collision, err)
	}

	// A soft-deleted video with an otherwise-matching key is not reported.
	if err := r.SoftDelete(ctx, aID); err != nil {
		t.Fatalf("soft delete a: %v", err)
	}
	if collision, err := r.FindStudioCollision(ctx, bID, []string{"Acme"}); err != nil || collision != nil {
		t.Errorf("soft-deleted source excluded: collision=%v err=%v", collision, err)
	}
}
