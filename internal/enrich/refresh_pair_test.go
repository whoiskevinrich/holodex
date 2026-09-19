package enrich

import (
	"context"
	"errors"
	"testing"
	"time"

	"holodex/internal/model"
)

// RefreshPair is the one per-pair step both the click and the sweep run (F66 RD1,
// ADR-103 D7). These cover its routing against the in-process fake + real repo.

func TestRefreshPairUnlinkedAutoAppliesWithBatchID(t *testing.T) {
	fake := NewFake("fake")
	fake.Searched = []string{"miyazaki"}
	svc, r := newSvc(t, fake)
	ctx := context.Background()

	out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{Query: "miyazaki"}, nil,
		RefreshOpts{BatchID: "sweep-1"})
	if out.Status != PairAutoApplied || out.Err != nil || len(out.Enriched) == 0 {
		t.Fatalf("outcome = %+v; want auto_applied with fields", out)
	}
	if id, ok, _ := svc.ExistingMatch(ctx, model.EnrichEntityPerson, 1, "fake"); !ok || id != "tmdb:608" {
		t.Fatalf("auto-apply should have stored the link, got %q ok=%v", id, ok)
	}
	runs, err := r.ListJobRuns(ctx, 1)
	if err != nil || len(runs) != 2 {
		t.Fatalf("want the enrich run + the searched run, got %d (%v)", len(runs), err)
	}
	for _, run := range runs {
		if run.Kind != model.JobKindEnrich || run.BatchID != "sweep-1" || run.EntityID != 1 {
			t.Errorf("run %+v should carry kind=enrich batch_id=sweep-1 entity 1", run)
		}
	}
}

func TestRefreshPairLinkedStalenessAndForce(t *testing.T) {
	fake := NewFake("fake")
	svc, _ := newSvc(t, fake)
	ctx := context.Background()
	fresh := &Match{Provider: "fake", ExternalID: "tmdb:608", FetchedAt: time.Now().Add(-3 * time.Hour)}
	old := &Match{Provider: "fake", ExternalID: "tmdb:608", FetchedAt: time.Now().Add(-25 * time.Hour)}

	if out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{}, fresh, RefreshOpts{}); out.Status != PairStaleSkipped {
		t.Fatalf("fresh link without force = %s; want stale_skipped", out.Status)
	}
	if fake.Calls != 0 {
		t.Fatalf("a stale-skipped pair must not dial the provider (calls=%d)", fake.Calls)
	}
	if out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{}, fresh, RefreshOpts{Force: true}); out.Status != PairRefreshed || len(out.Enriched) == 0 {
		t.Fatalf("fresh link with force = %+v; want refreshed", out)
	}
	if out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{}, old, RefreshOpts{}); out.Status != PairRefreshed {
		t.Fatalf("link older than the window = %s; want refreshed", out.Status)
	}
}

func TestRefreshPairDismissedNeverDials(t *testing.T) {
	fake := NewFake("fake")
	svc, r := newSvc(t, fake)
	ctx := context.Background()
	if err := r.DismissEnrichment(ctx, model.EnrichEntityPerson, 1, "fake"); err != nil {
		t.Fatal(err)
	}
	out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{Query: "miyazaki"}, nil, RefreshOpts{Force: true})
	if out.Status != PairDismissed || fake.Calls != 0 {
		t.Fatalf("dismissed pair = %s calls=%d; want dismissed, 0 calls", out.Status, fake.Calls)
	}
}

func TestRefreshPairReviewAndNoCandidates(t *testing.T) {
	fake := NewFake("fake")
	fake.People["tmdb:609"] = FakePerson{Label: "Hayao Miyazaki (the other one)"}
	svc, _ := newSvc(t, fake)
	ctx := context.Background()

	if out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{Query: "miyazaki"}, nil, RefreshOpts{}); out.Status != PairNeedsReview {
		t.Fatalf("two strong candidates = %s; want needs_review", out.Status)
	}
	if id, ok, _ := svc.ExistingMatch(ctx, model.EnrichEntityPerson, 1, "fake"); ok {
		t.Fatalf("needs_review must not apply anything, but stored %q", id)
	}
	if out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{Query: "nobody"}, nil, RefreshOpts{}); out.Status != PairNoCandidates {
		t.Fatalf("no match = %s; want no_candidates", out.Status)
	}
}

func TestRefreshPairFailureKeepsTypedError(t *testing.T) {
	fake := NewFake("fake")
	fake.Searched = []string{"miyazaki"}
	fake.EnrichErr = &errRateLimited{RetryAfter: 20 * time.Second}
	svc, r := newSvc(t, fake)
	ctx := context.Background()

	out := svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{Query: "miyazaki"}, nil, RefreshOpts{BatchID: "b"})
	var paused *ErrProviderPaused
	if out.Status != PairFailed || !errors.As(out.Err, &paused) || paused.RetryAfter != 20*time.Second {
		t.Fatalf("429 on the apply = %+v; want failed with *ErrProviderPaused{20s}", out)
	}
	// The resolve was still recorded (without "applied:"), under the batch.
	runs, _ := r.ListJobRuns(ctx, 1)
	sawSearched := false
	for _, run := range runs {
		if run.BatchID != "b" {
			t.Errorf("run %+v should carry batch_id b", run)
		}
		if run.Status == model.JobStatusOK && run.Detail != "" {
			sawSearched = true
		}
	}
	if !sawSearched {
		t.Fatalf("a failed apply must still record the resolve, runs=%+v", runs)
	}
	// The bucket is now paused: a linked refresh fails fast. The fake would succeed
	// if it were reached (EnrichErr cleared), so PairFailed proves it never was.
	fake.EnrichErr = nil
	out = svc.RefreshPair(ctx, model.EnrichEntityPerson, 1, "fake", Hint{}, &Match{Provider: "fake", ExternalID: "tmdb:608"}, RefreshOpts{Force: true})
	if out.Status != PairFailed || !errors.As(out.Err, &paused) {
		t.Fatalf("paused bucket = %+v; want failed/paused without reaching the provider", out)
	}
}
