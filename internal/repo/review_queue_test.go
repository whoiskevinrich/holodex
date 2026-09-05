package repo_test

import (
	"context"
	"slices"
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

// TestReviewQueue_MatchKindClassification proves ListReviewPairs classifies each pair
// by its STRONGEST live evidence: canonical-vs-canonical is "canonical" (the real
// "same entity typo'd twice" signal), one side needing an alias is "mixed", and both
// sides matching ONLY via an alias is "alias" — the weakest signal, since aliases on
// genuinely distinct people collide far more often than canonical names do (the
// private-media investigation this fixes found 206/207 flagged person pairs involved
// an alias on at least one side). Rows sort strongest-evidence-first.
func TestReviewQueue_MatchKindClassification(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// canonical vs canonical.
	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Mary Jane"}, nil), nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	linkPeople(t, r, idA, "Mary Jane")
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"MaryJane"}, nil), nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}
	linkPeople(t, r, idB, "MaryJane")

	// mixed: one person's canonical name collides only with an alias manually added to
	// a second, otherwise-unrelated person.
	idC, err := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "C", []string{"John Q Public"}, nil), nil)
	if err != nil {
		t.Fatalf("seed c: %v", err)
	}
	linkPeople(t, r, idC, "John Q Public")
	idD, err := r.UpsertVideo(ctx, sampleVideo("/m/d.mkv", "D", []string{"Someone Else"}, nil), nil)
	if err != nil {
		t.Fatalf("seed d: %v", err)
	}
	linkPeople(t, r, idD, "Someone Else")
	dID := personIDByName(t, r, "Someone Else")
	if _, err := r.AddPersonAlias(ctx, dID, "John-Q-Public"); err != nil {
		t.Fatalf("add alias d: %v", err)
	}

	// alias vs alias: two unrelated people whose only collision is between an alias on
	// each — never auto-flagged at scan time (FlagNearMiss only fires for a
	// newly-created entity, exactly like a real merge/rename-derived alias); only a
	// re-seed catches it.
	idE, err := r.UpsertVideo(ctx, sampleVideo("/m/e.mkv", "E", []string{"First Person"}, nil), nil)
	if err != nil {
		t.Fatalf("seed e: %v", err)
	}
	linkPeople(t, r, idE, "First Person")
	idF, err := r.UpsertVideo(ctx, sampleVideo("/m/f.mkv", "F", []string{"Second Person"}, nil), nil)
	if err != nil {
		t.Fatalf("seed f: %v", err)
	}
	linkPeople(t, r, idF, "Second Person")
	eID := personIDByName(t, r, "First Person")
	fID := personIDByName(t, r, "Second Person")
	if _, err := r.AddPersonAlias(ctx, eID, "Alpha-One"); err != nil {
		t.Fatalf("add alias e: %v", err)
	}
	if _, err := r.AddPersonAlias(ctx, fID, "AlphaOne"); err != nil {
		t.Fatalf("add alias f: %v", err)
	}
	if _, err := r.SeedIdentityReviewQueue(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var got []string
	for _, p := range pairs {
		if p.EntityType == model.EnrichEntityPerson {
			got = append(got, p.MatchKind)
		}
	}
	want := []string{"canonical", "mixed", "alias"} // strongest-first
	if !slices.Equal(got, want) {
		t.Fatalf("match kinds (in order) = %v, want %v: %+v", got, want, pairs)
	}
}

// TestReviewQueue_StaleRowDropped proves a queued pair vanishes on its own once
// nothing currently connects the two entities — not just on merge/dismiss. This
// reproduces the private-media staleness bug directly: 203 of 207 live pairs no
// longer collided under current names/aliases AT ALL, because until now only merge
// and dismiss ever pruned a row — deleting (or editing) the alias that justified a
// match left the row behind forever. Deleting the one alias responsible for this
// match reproduces that.
func TestReviewQueue_StaleRowDropped(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Foo Bar"}, nil), nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	linkPeople(t, r, idA, "Foo Bar")
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Someone Else"}, nil), nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}
	linkPeople(t, r, idB, "Someone Else")
	bID := personIDByName(t, r, "Someone Else")
	alias, err := r.AddPersonAlias(ctx, bID, "FooBar")
	if err != nil {
		t.Fatalf("add alias: %v", err)
	}
	if _, err := r.SeedIdentityReviewQueue(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pairs) != 1 || pairs[0].MatchKind != "mixed" {
		t.Fatalf("pairs = %+v, want one mixed-kind pair", pairs)
	}

	// The alias is deleted (an owner correcting a bad merge/typo, say). Nothing
	// currently connects A and B any more, but DeleteEntityAlias doesn't touch
	// identity_review_queue — only merge/dismiss ever did, before this fix.
	if err := r.DeletePersonAlias(ctx, bID, alias.ID); err != nil {
		t.Fatalf("delete alias: %v", err)
	}

	pairs, err = r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("after alias delete: %+v, want empty (stale row should self-heal)", pairs)
	}
}

// TestReviewQueue_ProviderAliasRowsAlwaysSurface proves a non-fuzzy queue row (F58/
// ADR-088's 'provider-alias' variation: an EXACT alias conflict between two entities
// whose actual names/aliases are NOT required to resemble each other at all — the
// collision is about one specific candidate string, not name similarity) is never
// dropped by the live-revalidation added for near-miss staleness. That join only
// makes sense for the fuzzy variations it understands (internal-whitespace/
// punctuation); this regression-tests that anything else always passes through, since
// the two entities here would never survive a loose-key recheck on their own names.
func TestReviewQueue_ProviderAliasRowsAlwaysSurface(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()

	idA, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"John Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	linkPeople(t, r, idA, "John Smith")
	idB, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Robert Jones"}, nil), nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}
	linkPeople(t, r, idB, "Robert Jones")
	a := personIDByName(t, r, "John Smith")
	b := personIDByName(t, r, "Robert Jones")
	lo, hi := a, b
	if b < a {
		lo, hi = b, a
	}

	// Queued the way ApplyProviderAliases does (queueProviderAliasPair): the two
	// canonical names are nothing alike, and the skipped candidate ("Bob") belongs to
	// neither of them -- the row itself, not a live loose-key match, is what says
	// there's still something to resolve.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation, detail)
		 VALUES ('person', ?, ?, 'provider-alias', 'Bob')`, lo, hi); err != nil {
		t.Fatalf("seed provider-alias row: %v", err)
	}

	pairs, err := r.ListReviewPairs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pairs) != 1 || pairs[0].Variation != "provider-alias" || pairs[0].MatchKind != "" {
		t.Fatalf("pairs = %+v, want one provider-alias row with empty match_kind", pairs)
	}
}
