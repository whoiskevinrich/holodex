package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// seedTwoTags scans two videos so tags `a` and `b` exist (each with one video), and
// returns their ids. Creating them routes through resolveOrCreateByName, which also
// runs scan-time near-miss flagging (FlagNearMiss).
func seedTwoTags(t *testing.T, r *repo.Repo, a, b string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/"+a+".mkv", "A", nil, []string{a}), nil); err != nil {
		t.Fatalf("seed %q: %v", a, err)
	}
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/"+b+".mkv", "B", nil, []string{b}), nil); err != nil {
		t.Fatalf("seed %q: %v", b, err)
	}
	return tagIDByName(t, r, a), tagIDByName(t, r, b)
}

// TestReviewQueue_ScanFlagsAndList proves scan-time flagging (P1-2): creating a tag
// that is a loose-key near-miss of an existing one queues the pair, and the read
// surfaces both names + counts + the variation kind. "action" (no near-miss) does not.
func TestReviewQueue_ScanFlagsAndList(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	seedTwoTags(t, r, "sci-fi", "scifi")
	if _, err := r.UpsertVideo(ctx, sampleVideo("/m/x.mkv", "X", nil, []string{"action"}), nil); err != nil {
		t.Fatalf("seed action: %v", err)
	}

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1: %+v", len(pairs), pairs)
	}
	p := pairs[0]
	if p.EntityType != model.EntityTag || p.Variation != "punctuation" {
		t.Fatalf("pair = %+v, want tag/punctuation", p)
	}
	if p.A.Name == "" || p.B.Name == "" || p.A.VideoCount != 1 || p.B.VideoCount != 1 {
		t.Fatalf("pair names/counts = %+v", p)
	}
}

// TestReviewQueue_InternalWhitespaceVariation covers the person near-miss whose only
// difference is internal whitespace ("Mary Jane" vs "MaryJane") — flagged (person
// nameKey keeps internal whitespace) with the internal-whitespace variation.
func TestReviewQueue_InternalWhitespaceVariation(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Mary Jane"}, nil), nil)
	if err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	linkPeople(t, r, idA, "Mary Jane")
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"MaryJane"}, nil), nil)
	if err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	linkPeople(t, r, idB, "MaryJane")
	pairs, _ := r.ListReviewPairs(ctx)
	if len(pairs) != 1 || pairs[0].EntityType != model.EnrichEntityPerson || pairs[0].Variation != "internal-whitespace" {
		t.Fatalf("pairs = %+v, want one person/internal-whitespace", pairs)
	}
}

// TestReviewQueue_SeedBatch proves the batch seed (P1-1) re-detects near-misses over
// the whole library: after clearing the scan-flagged rows, SeedReviewQueue re-flags.
func TestReviewQueue_SeedBatch(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	seedTwoTags(t, r, "sci-fi", "scifi")

	// Clear the scan-flagged rows (no keep-separate marker), then re-seed from scratch.
	if _, err := db.ExecContext(ctx, `DELETE FROM identity_review_queue`); err != nil {
		t.Fatalf("clear: %v", err)
	}
	n, err := r.SeedIdentityReviewQueue(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 1 {
		t.Fatalf("seed flagged %d, want 1", n)
	}
	// Idempotent: a second seed adds nothing.
	if n2, _ := r.SeedIdentityReviewQueue(ctx); n2 != 0 {
		t.Fatalf("second seed flagged %d, want 0 (idempotent)", n2)
	}
}

// TestReviewQueue_DismissRecordsKeepSeparate proves dismiss removes the pair AND marks
// it keep-separate, so a re-seed never re-proposes it (P1-3, RD5, QA 2.10/2.11).
func TestReviewQueue_DismissRecordsKeepSeparate(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, b := seedTwoTags(t, r, "sci-fi", "scifi")

	if err := r.DismissReviewPair(ctx, model.EntityTag, a, b); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if pairs, _ := r.ListReviewPairs(ctx); len(pairs) != 0 {
		t.Fatalf("after dismiss: %+v, want empty", pairs)
	}
	if kept, _ := r.IsKeptSeparate(ctx, model.EntityTag, a, b); !kept {
		t.Fatal("dismiss did not record keep-separate")
	}
	// A re-seed must not re-surface the kept-separate pair.
	if n, _ := r.SeedIdentityReviewQueue(ctx); n != 0 {
		t.Fatalf("re-seed flagged %d, want 0 (kept-separate excluded)", n)
	}
}

// TestNearMiss covers the editor soft-warning lookup (P1-5): a candidate name finds an
// existing fuzzy near-miss (different nameKey, same loose key), excludes self, and is
// silenced by a keep-separate marker.
func TestNearMiss(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	scifi, drama := seedTwoTags(t, r, "scifi", "drama")

	// Typing "sci-fi" against the drama tag surfaces the "scifi" near-miss.
	nm, err := r.NearMiss(ctx, model.EntityTag, drama, "sci-fi")
	if err != nil {
		t.Fatalf("near-miss: %v", err)
	}
	if nm == nil || nm.ID != scifi {
		t.Fatalf("near-miss = %+v, want scifi (#%d)", nm, scifi)
	}
	// Excludes self: the scifi tag doesn't near-miss itself.
	if nm, _ := r.NearMiss(ctx, model.EntityTag, scifi, "sci-fi"); nm != nil {
		t.Fatalf("self near-miss = %+v, want nil", nm)
	}
	// An exact match (same nameKey) is a collision, not a near-miss → nil here.
	if nm, _ := r.NearMiss(ctx, model.EntityTag, drama, "SciFi"); nm != nil {
		t.Fatalf("exact-key near-miss = %+v, want nil (that's a 409 collision)", nm)
	}
	// Kept-separate silences it.
	if err := r.AddKeepSeparate(ctx, model.EntityTag, drama, scifi); err != nil {
		t.Fatalf("keep separate: %v", err)
	}
	if nm, _ := r.NearMiss(ctx, model.EntityTag, drama, "sci-fi"); nm != nil {
		t.Fatalf("kept-separate near-miss = %+v, want nil", nm)
	}
}

// TestReviewQueue_MergeDropsPair proves resolving a pair via merge leaves no stale
// queue row (the queue self-heals; QA "pair resolved elsewhere").
func TestReviewQueue_MergeDropsPair(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, b := seedTwoTags(t, r, "sci-fi", "scifi")

	if err := r.MergeEntities(ctx, model.EntityTag, a, b); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if pairs, _ := r.ListReviewPairs(ctx); len(pairs) != 0 {
		t.Fatalf("after merge: %+v, want empty", pairs)
	}
}
