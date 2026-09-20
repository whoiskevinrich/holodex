package enrich

import (
	"context"
	"time"

	"holodex/internal/model"
)

// StaleWindow is how recently a linked pair must have been fetched for a
// non-forced refresh to skip it (F66 RD2, ADR-103 D7). An interactive click always
// forces; the sweep's default pass honours it.
const StaleWindow = 24 * time.Hour

// PairStatus is the outcome of one (entity, provider) refresh step.
type PairStatus string

const (
	PairRefreshed    PairStatus = "refreshed"     // linked: re-fetched and stored
	PairAutoApplied  PairStatus = "auto_applied"  // unlinked: exactly one strong match, applied (ADR-066 D1)
	PairNeedsReview  PairStatus = "needs_review"  // unlinked: candidates, none auto-applicable
	PairNoCandidates PairStatus = "no_candidates" // unlinked: the provider found nothing
	PairDismissed    PairStatus = "dismissed"     // unlinked, owner-dismissed: never called (F47 RD4)
	PairStaleSkipped PairStatus = "stale_skipped" // linked, fetched inside StaleWindow, not forced
	PairFailed       PairStatus = "failed"        // a call errored; Err says how
)

// PairOutcome is what RefreshPair reports. Err is set only for PairFailed and keeps
// its type — a *ErrProviderPaused passes through so the caller can branch on it
// (ADR-103 D4): the sweep waits and retries, an interactive handler answers 503.
type PairOutcome struct {
	Provider string
	Status   PairStatus
	Enriched []model.EnrichedField
	Err      error
}

// RefreshOpts carries the per-call policy RefreshPair does not decide itself.
type RefreshOpts struct {
	// Force ignores StaleWindow — a click means "now".
	Force bool
	// BatchID stamps every job run this step records (the sweep's audit key,
	// ADR-103 D9); empty for a click.
	BatchID string
	// BypassGalleryCap is Enrich's owner-privilege input (HOLODEX-174).
	BypassGalleryCap bool
}

// RefreshPair is the one per-(entity, provider) refresh step both the interactive
// refresh-all handler and the sweep run (F66 RD1, ADR-103 D7): a linked pair (link
// non-nil, from the caller's batched ProviderMatches lookup) re-fetches; an unlinked
// pair resolves and auto-applies exactly one strong match (ADR-066 D1, called not
// copied) or is left for the owner. A dismissed pair is never dialed. Post-apply side
// effects (studio/logo relink) are the caller's — they run once per entity, after
// every provider has settled.
func (s *Service) RefreshPair(ctx context.Context, entityType string, id int64, provider string, hint Hint, link *Match, opts RefreshOpts) PairOutcome {
	out := PairOutcome{Provider: provider}
	fail := func(err error) PairOutcome {
		out.Status, out.Err = PairFailed, err
		return out
	}

	if link != nil {
		if !opts.Force && !link.FetchedAt.IsZero() && time.Since(link.FetchedAt) < StaleWindow {
			out.Status = PairStaleSkipped
			return out
		}
		fields, err := s.enrichRecorded(ctx, entityType, id, provider, link.ExternalID, opts.BypassGalleryCap, opts.BatchID)
		if err != nil {
			return fail(err)
		}
		out.Status, out.Enriched = PairRefreshed, fields
		return out
	}

	dismissed, err := s.repo.EnrichmentDismissed(ctx, entityType, id, provider)
	if err != nil {
		return fail(err)
	}
	if dismissed {
		out.Status = PairDismissed
		return out
	}

	started := time.Now()
	res, err := s.Resolve(ctx, provider, entityType, hint)
	if err != nil {
		return fail(err)
	}
	// The only trace an unattended resolve leaves of what was actually tried
	// (ADR-095 D6) and of which record it bound (F61 FR5). `applied` is passed only
	// once the apply has actually succeeded, so the audit line never claims a
	// binding that didn't happen.
	if strong, ok := SingleStrongMatch(res.Candidates); ok {
		fields, err := s.enrichRecorded(ctx, entityType, id, provider, strong.ExternalID, opts.BypassGalleryCap, opts.BatchID)
		if err != nil {
			s.RecordSearched(started, provider, entityType, id, res, nil, opts.BatchID)
			return fail(err)
		}
		s.RecordSearched(started, provider, entityType, id, res, &strong, opts.BatchID)
		out.Status, out.Enriched = PairAutoApplied, fields
		return out
	}
	s.RecordSearched(started, provider, entityType, id, res, nil, opts.BatchID)
	if len(res.Candidates) == 0 {
		out.Status = PairNoCandidates
	} else {
		out.Status = PairNeedsReview
	}
	return out
}
