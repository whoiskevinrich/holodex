package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

func TestExtractionReview_UpsertUpdatesPendingInPlace(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "Film", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := r.UpsertExtractionReview(ctx, videoID, "title", "Filename Title", "Tag Title", 0.55, 0); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}
	rows, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(rows) != 1 {
		t.Fatalf("want 1 pending row, got %d err=%v", len(rows), err)
	}
	first := rows[0]

	// Re-running extraction on the same (video, field) while still pending
	// updates the row in place (F48.4b) — no duplicate.
	if err := r.UpsertExtractionReview(ctx, videoID, "title", "New Filename Title", "Tag Title", 0.62, 0); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	rows, err = r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(rows) != 1 {
		t.Fatalf("want still exactly 1 pending row, got %d err=%v", len(rows), err)
	}
	if rows[0].ID != first.ID {
		t.Fatalf("expected the same row id (in-place update), got %d then %d", first.ID, rows[0].ID)
	}
	if rows[0].FilenameValue != "New Filename Title" || rows[0].Confidence != 0.62 {
		t.Fatalf("row not updated in place: %+v", rows[0])
	}
}

func TestExtractionReview_ResolveAndDismiss(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "Film", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, videoID, "title", "A", "B", 0.5, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(pending) != 1 {
		t.Fatalf("want 1 pending, got %d err=%v", len(pending), err)
	}
	id := pending[0].ID

	if err := r.ResolveExtractionReview(ctx, id); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if pending, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending); err != nil || len(pending) != 0 {
		t.Fatalf("expected no pending rows after resolve, got %d err=%v", len(pending), err)
	}
	resolved, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewResolved)
	if err != nil || len(resolved) != 1 || resolved[0].ResolvedAt == "" {
		t.Fatalf("expected 1 resolved row with resolved_at set, got %+v err=%v", resolved, err)
	}

	// F48.4d: dismissing is durable, but re-triggering extraction (a fresh
	// upsert) opens a new pending row rather than resurrecting the dismissed one.
	if err := r.UpsertExtractionReview(ctx, videoID, "studio", "X", "Y", 0.3, 0); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	pending2, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(pending2) != 1 {
		t.Fatalf("want 1 pending, got %d err=%v", len(pending2), err)
	}
	if err := r.DismissExtractionReview(ctx, pending2[0].ID); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	dismissed, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewDismissed)
	if err != nil || len(dismissed) != 1 {
		t.Fatalf("want 1 dismissed row, got %d err=%v", len(dismissed), err)
	}

	if err := r.UpsertExtractionReview(ctx, videoID, "studio", "X2", "Y2", 0.3, 0); err != nil {
		t.Fatalf("re-trigger after dismiss: %v", err)
	}
	rePending, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(rePending) != 1 {
		t.Fatalf("expected a fresh pending row after re-trigger, got %d err=%v", len(rePending), err)
	}
	if rePending[0].ID == dismissed[0].ID {
		t.Fatal("re-triggered row should be a new row, not the dismissed one resurrected")
	}
}

func TestExtractionReview_SuggestedEntityID(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/c.mkv", "Film", []string{"Alice Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	linkPeople(t, r, videoID, "Alice Smith")
	people, err := r.ListPeople(ctx, false)
	if err != nil || len(people) != 1 {
		t.Fatalf("seed person: %v %v", people, err)
	}
	suggestedID := people[0].ID

	if err := r.UpsertExtractionReview(ctx, videoID, "people", "Alise Smith", "", 0.55, suggestedID); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(rows) != 1 {
		t.Fatalf("want 1 pending row, got %d err=%v", len(rows), err)
	}
	if rows[0].SuggestedEntityID != suggestedID {
		t.Fatalf("SuggestedEntityID = %d, want %d", rows[0].SuggestedEntityID, suggestedID)
	}
}

func TestExtractionReview_Get(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/get.mkv", "Film", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, videoID, "title", "A", "B", 0.5, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pending, err := r.ListExtractionReviews(ctx, repo.ExtractionReviewPending)
	if err != nil || len(pending) != 1 {
		t.Fatalf("want 1 pending, got %d err=%v", len(pending), err)
	}

	got, err := r.GetExtractionReview(ctx, pending[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.VideoID != videoID || got.FieldKey != "title" || got.FilenameValue != "A" || got.TagValue != "B" {
		t.Fatalf("got = %+v, want video/field/values matching the seeded row", got)
	}

	if _, err := r.GetExtractionReview(ctx, 999999); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrNotFound", err)
	}
}

// TestExtractionQueue_JoinsVideoAndSuggestedEntityName proves the video-join
// (video_title/file_path land on every row) and the suggested-entity-name
// resolution (an entity-field row's fuzzy suggestion carries a human-readable
// name, not just an id) that the Extraction tab (F48.6) needs and the flat
// ListExtractionReviews read doesn't provide.
func TestExtractionQueue_JoinsVideoAndSuggestedEntityName(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/queue.mkv", "Queued Film", []string{"Alice Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	linkPeople(t, r, videoID, "Alice Smith")
	people, err := r.ListPeople(ctx, false)
	if err != nil || len(people) != 1 {
		t.Fatalf("seed person: %v %v", people, err)
	}
	suggestedID := people[0].ID

	if err := r.UpsertExtractionReview(ctx, videoID, "people", "Alise Smith", "", 0.55, suggestedID); err != nil {
		t.Fatalf("upsert people: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, videoID, "title", "New Title", "Old Title", 0.4, 0); err != nil {
		t.Fatalf("upsert title: %v", err)
	}

	rows, err := r.ExtractionQueue(ctx, 0)
	if err != nil {
		t.Fatalf("extraction queue: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(rows), rows)
	}
	byField := map[string]repo.ExtractionQueueRow{}
	for _, row := range rows {
		if row.VideoID != videoID || row.VideoTitle != "Queued Film" || row.FilePath != "/m/queue.mkv" {
			t.Fatalf("row missing video join: %+v", row)
		}
		byField[row.FieldKey] = row
	}
	if byField["people"].SuggestedEntityName != "Alice Smith" {
		t.Fatalf("SuggestedEntityName = %q, want %q", byField["people"].SuggestedEntityName, "Alice Smith")
	}
	if byField["title"].SuggestedEntityID != 0 || byField["title"].SuggestedEntityName != "" {
		t.Fatalf("title row should carry no suggestion, got %+v", byField["title"])
	}

	// Resolving the people row drops it from the pending queue.
	if err := r.ResolveExtractionReview(ctx, byField["people"].ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	rows, err = r.ExtractionQueue(ctx, 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("want 1 row after resolve, got %d err=%v", len(rows), err)
	}
}

// TestExtractionQueue_PerPersonCandidates proves the per-value entity
// candidates the Extraction tab's chip UI needs (HOLODEX-196 #1): a multi-value
// People field splits into one candidate per person, each marked as an existing
// entity (with its canonical name) or a new one; a non-entity field carries no
// candidates.
func TestExtractionQueue_PerPersonCandidates(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/cast.mkv", "Cast Film", []string{"Alice Smith"}, nil), nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	linkPeople(t, r, videoID, "Alice Smith")
	people, err := r.ListPeople(ctx, false)
	if err != nil || len(people) != 1 {
		t.Fatalf("seed person: %v %v", people, err)
	}
	aliceID := people[0].ID

	// Alice exists (lower-cased in the filename to prove case-insensitive spine
	// matching); Bob does not.
	if err := r.UpsertExtractionReview(ctx, videoID, "people", "alice smith, Bob Jones", "", 0.6, 0); err != nil {
		t.Fatalf("upsert people: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, videoID, "title", "A, B", "", 0.4, 0); err != nil {
		t.Fatalf("upsert title: %v", err)
	}

	rows, err := r.ExtractionQueue(ctx, 0)
	if err != nil {
		t.Fatalf("extraction queue: %v", err)
	}
	byField := map[string]repo.ExtractionQueueRow{}
	for _, row := range rows {
		byField[row.FieldKey] = row
	}

	cands := byField["people"].Candidates
	if len(cands) != 2 {
		t.Fatalf("people candidates = %+v, want 2", cands)
	}
	if cands[0].Name != "alice smith" || cands[0].EntityID != aliceID || cands[0].EntityName != "Alice Smith" {
		t.Fatalf("candidate[0] = %+v, want existing Alice Smith (id %d, canonical name)", cands[0], aliceID)
	}
	if cands[1].Name != "Bob Jones" || cands[1].EntityID != 0 {
		t.Fatalf("candidate[1] = %+v, want new (no entity id)", cands[1])
	}

	// A non-entity field is not split into candidates even though it contains ", ".
	if byField["title"].Candidates != nil {
		t.Fatalf("title candidates = %+v, want none (scalar field)", byField["title"].Candidates)
	}
}

// TestExtractionQueue_ScopedByVideo covers F48.6k: a non-zero videoID returns only
// that video's pending rows, so the media detail page's inline panel (ADR-090 D2's
// entity-scoped view of the same queue) does not pull the whole library down to
// show its own handful. videoID 0 keeps the owner tab's whole-library behaviour.
func TestExtractionQueue_ScopedByVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	a, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "Film A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	b, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "Film B", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, a, "title", "A New", "A Old", 0.4, 0); err != nil {
		t.Fatalf("upsert a/title: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, a, "studio", "Neon", "", 0.5, 0); err != nil {
		t.Fatalf("upsert a/studio: %v", err)
	}
	if err := r.UpsertExtractionReview(ctx, b, "title", "B New", "B Old", 0.4, 0); err != nil {
		t.Fatalf("upsert b/title: %v", err)
	}

	all, err := r.ExtractionQueue(ctx, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("unfiltered = %d rows err=%v, want 3", len(all), err)
	}

	scoped, err := r.ExtractionQueue(ctx, a)
	if err != nil {
		t.Fatalf("scoped: %v", err)
	}
	if len(scoped) != 2 {
		t.Fatalf("scoped to A = %d rows, want 2: %+v", len(scoped), scoped)
	}
	for _, row := range scoped {
		if row.VideoID != a {
			t.Fatalf("scoped query leaked video %d into A's rows: %+v", row.VideoID, row)
		}
	}

	// The join still populates, so the scoped rows are usable without a second read.
	if scoped[0].VideoTitle != "Film A" || scoped[0].FilePath != "/m/a.mkv" {
		t.Fatalf("scoped row lost its video join: %+v", scoped[0])
	}

	// A video with no pending rows is empty, not an error — the panel's "nothing
	// needs review" state (F48.6l) depends on this, not on a 404.
	none, err := r.ExtractionQueue(ctx, b+9999)
	if err != nil {
		t.Fatalf("unknown video id should not error: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("unknown video id = %d rows, want 0", len(none))
	}
}
