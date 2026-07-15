package extract

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"holodex/internal/model"
	"holodex/internal/writequeue"
)

// Resolver is the narrow repo slice Process needs for entity resolution
// (F48.3c/d): ExactEntityMatch reuses F43's identity-spine detector;
// EntityNames is the candidate pool Jaro-Winkler ranks when no exact match
// exists. *repo.Repo satisfies this directly.
type Resolver interface {
	ExactEntityMatch(ctx context.Context, entityType, name string) (id int64, ok bool, err error)
	EntityNames(ctx context.Context, entityType string) (map[int64]string, error)
}

// ManualSourceChecker reports whether a field already carries a manual:
// decision (F36) — F48.3e's one-time-import rule. *repo.Repo satisfies this
// directly.
type ManualSourceChecker interface {
	HasManualSource(ctx context.Context, entityType string, entityID int64, fieldKey string) (bool, error)
}

// ReviewStore persists the extraction review queue (F48.4b/c/d). *repo.Repo
// satisfies this directly.
type ReviewStore interface {
	UpsertExtractionReview(ctx context.Context, videoID int64, fieldKey, filenameValue, tagValue string, confidence float64, suggestedEntityID int64) error
}

// Enqueuer is the F30 write-queue slice Process needs to apply an
// auto-approved candidate (F48.4a) — no new write mechanism, same call path
// manual curation uses. *writequeue.Queue satisfies this directly.
type Enqueuer interface {
	Enqueue(ctx context.Context, videoID int64, fields []writequeue.JobField) (int64, error)
}

// entityTypeForField maps an entity-field's canonical field key to the
// entity_type F43's identity spine uses (people values are Person entities;
// studio values are Studio entities).
var entityTypeForField = map[string]string{
	"people": model.EnrichEntityPerson,
	"studio": model.EnrichEntityStudio,
}

// Outcome is what Process actually did with one field's extraction candidate.
type Outcome string

const (
	OutcomeNoop        Outcome = "noop"         // nothing extracted from the filename for this field
	OutcomeAutoApplied Outcome = "auto_applied" // cleared the gate and a write was enqueued
	OutcomeLoggedOnly  Outcome = "logged_only"  // would have auto-applied, but the flag is off (ADR-067 Action Item 2)
	OutcomeQueued      Outcome = "queued"       // routed to the review queue
)

// FieldExtraction is one field's raw extracted values, already read from the
// filename: and file: sources — Process never touches the resolver directly
// (F48.2b's "no resolver code change" invariant holds).
type FieldExtraction struct {
	VideoID        int64
	Field          string // canonical field key, e.g. "people", "title"
	FilenameValues []string
	TagValues      []string
}

// Deps bundles Process's collaborators. AutoApplyEnabled gates whether an
// AutoApply-routed candidate actually enqueues a write or only logs what it
// would have done (ADR-067 Action Item 2). Log may be nil (no logging).
type Deps struct {
	Resolver         Resolver
	ManualSource     ManualSourceChecker
	Reviews          ReviewStore
	Queue            Enqueuer
	AutoApplyEnabled bool
	Log              *slog.Logger
}

// Process scores and routes one field's extraction candidate end to end
// (F48.3+F48.4): classify agreement/specificity/entity-match from the raw
// values, score, route, then either enqueue a write (auto-apply, gated by
// Deps.AutoApplyEnabled), persist a review row, or do nothing (no filename
// data for this field).
func Process(ctx context.Context, d Deps, fe FieldExtraction) (Outcome, error) {
	if len(fe.FilenameValues) == 0 {
		return OutcomeNoop, nil
	}

	isEntity := IsEntityField(fe.Field)
	filenameJoined := joinSorted(fe.FilenameValues)
	tagJoined := joinSorted(fe.TagValues)
	agreement := classifyAgreement(filenameJoined, tagJoined)

	var (
		score             float64
		match             EntityMatch
		suggestedEntityID int64
	)
	if isEntity {
		entityType, known := entityTypeForField[fe.Field]
		if !known {
			return "", fmt.Errorf("extract: entity field %q has no entity type mapping", fe.Field)
		}
		if d.Resolver == nil {
			return "", fmt.Errorf("extract: field %q requires a Resolver", fe.Field)
		}
		var err error
		match, suggestedEntityID, err = resolveEntityMatch(ctx, d.Resolver, entityType, fe.FilenameValues)
		if err != nil {
			return "", fmt.Errorf("extract: resolve entity match: %w", err)
		}
		score = ScoreEntity(agreement, minSpecificity(fe.FilenameValues, true), match)
	} else {
		score = ScoreNonEntity(agreement, minSpecificity(fe.FilenameValues, false))
	}

	hasManual := false
	if d.ManualSource != nil {
		var err error
		hasManual, err = d.ManualSource.HasManualSource(ctx, model.EnrichEntityVideo, fe.VideoID, fe.Field)
		if err != nil {
			return "", fmt.Errorf("extract: check manual source: %w", err)
		}
	}

	decision := Route(fe.Field, isEntity, match, score, hasManual)

	if decision.AutoApply {
		if !d.AutoApplyEnabled {
			d.logDecision(ctx, "extraction candidate would auto-apply (log-only, ADR-067 pending Accepted)", fe, score, decision)
			return OutcomeLoggedOnly, nil
		}
		if d.Queue == nil {
			return "", fmt.Errorf("extract: auto-apply enabled but no write queue configured")
		}
		if _, err := d.Queue.Enqueue(ctx, fe.VideoID, []writequeue.JobField{{
			Field:  fe.Field,
			Values: fe.FilenameValues,
			Source: Provider,
		}}); err != nil {
			return "", fmt.Errorf("extract: enqueue auto-apply write: %w", err)
		}
		return OutcomeAutoApplied, nil
	}

	if d.Reviews == nil {
		return "", fmt.Errorf("extract: routed to review but no review store configured")
	}
	if err := d.Reviews.UpsertExtractionReview(ctx, fe.VideoID, fe.Field, filenameJoined, tagJoined, score, suggestedEntityID); err != nil {
		return "", fmt.Errorf("extract: upsert review row: %w", err)
	}
	return OutcomeQueued, nil
}

func (d Deps) logDecision(ctx context.Context, msg string, fe FieldExtraction, score float64, decision Decision) {
	if d.Log == nil {
		return
	}
	d.Log.InfoContext(ctx, msg,
		"video_id", fe.VideoID,
		"field", fe.Field,
		"confidence", score,
		"reason", decision.Reason,
	)
}

// resolveEntityMatch classifies every extracted value against existing
// entities and reduces to the weakest-link overall match (F48.3d's hard
// gate applies to the whole field, not just one value): every value must
// resolve exactly for the field to count as MatchExact. suggestedID is the
// best-scoring fuzzy match among the values that didn't resolve exactly —
// zero when every value did (there is nothing to suggest).
//
// Fetches EntityNames once per call. Fine for on-demand/single-field use;
// when Phase 4's batch trigger (F48.5b) calls Process per field across a
// whole library, that becomes one full-table read per call for an
// unchanging table within the run — worth hoisting into a per-run cache the
// batch driver passes through Resolver at that point, not before (no caller
// exists yet to hoist for).
func resolveEntityMatch(ctx context.Context, r Resolver, entityType string, values []string) (EntityMatch, int64, error) {
	names, err := r.EntityNames(ctx, entityType)
	if err != nil {
		return MatchNone, 0, fmt.Errorf("entity names: %w", err)
	}

	overall := MatchExact
	var suggestedID int64
	var bestScore float64
	for _, v := range values {
		id, exact, err := r.ExactEntityMatch(ctx, entityType, v)
		if err != nil {
			return MatchNone, 0, fmt.Errorf("exact entity match: %w", err)
		}

		m := MatchNone
		var candidateID int64
		var score float64
		switch {
		case exact:
			m, candidateID, score = MatchExact, id, 1
		default:
			if fid, fscore, ok := BestFuzzyMatch(v, names); ok {
				m, candidateID, score = MatchFuzzy, fid, fscore
			}
		}

		if m < overall {
			overall = m
		}
		if m != MatchExact && score > bestScore {
			bestScore, suggestedID = score, candidateID
		}
	}
	if overall == MatchExact {
		suggestedID = 0
	}
	return overall, suggestedID, nil
}

// minSpecificity reduces a multi-value field (e.g. {people}) to its weakest
// value's specificity — one garbled/single-word value is enough to hold the
// whole field back, matching the exact-match gate's conservative posture.
func minSpecificity(values []string, entity bool) Specificity {
	worst := SpecificityFull
	for _, v := range values {
		if s := classifySpecificity(v, entity); s < worst {
			worst = s
		}
	}
	return worst
}

// joinSorted renders a multi-value field as one comparable string, order
// independent, for agreement classification (Alice, Bob and Bob, Alice
// should agree).
func joinSorted(values []string) string {
	if len(values) == 0 {
		return ""
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}
