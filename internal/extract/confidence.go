// Confidence scoring for extraction candidates (F48.3, ADR-067 §1). Pure, no
// I/O — callers classify raw filename/tag values into the enums below (see
// jarowinkler.go for the classifier helpers), then Score combines them into
// the weighted rubric from the spec's Concepts & Model. This is a second,
// independent implementation from internal/enrich's provider threshold
// (ADR-066) — ADR-067 explicitly rejected a shared abstraction for v1.
package extract

// Agreement describes how a filename-derived value compares to the file
// tag's existing value for the same field (Concepts & Model, "Source
// agreement"). AgreementSingleSource means only the filename has data — the
// only case Process ever scores, since a field extraction with nothing new
// to contribute never reaches scoring at all.
type Agreement int

const (
	AgreementConflict Agreement = iota
	AgreementFuzzy
	AgreementSingleSource
	AgreementExact
)

// Specificity buckets how structured/complete an extracted value looks
// (Concepts & Model, "Value specificity"). The same three buckets apply to
// both entity and non-entity fields; only the point values differ.
type Specificity int

const (
	SpecificityGarbled Specificity = iota
	SpecificityPartial
	SpecificityFull
)

// EntityMatch describes how an extracted entity value (People/Studio) lines
// up against existing entities via F43's identity spine (ADR-061). MatchExact
// must come from the same nameKey normalization F43 already uses (F48.3c) —
// callers resolve this through repo.ExactEntityMatch, never by reimplementing
// name comparison here.
type EntityMatch int

const (
	MatchNone EntityMatch = iota
	MatchFuzzy
	MatchExact
)

// Tier is a field's stakes bucket (Field tiers and thresholds), which selects
// its AutoApplyThreshold.
type Tier int

const (
	TierHigh Tier = iota
	TierMedium
	TierLow
)

// tierThresholds is the hardcoded-for-v1 AutoApplyThreshold per tier (ADR-067
// Resolved Decision #4). Revisit only if empirical misclassification data
// (ADR-067 Action Item 4) shows these are wrong in practice.
var tierThresholds = map[Tier]float64{
	TierHigh:   0.80,
	TierMedium: 0.70,
	TierLow:    0.40,
}

// fieldTiers is the exhaustive, hardcoded field->tier map (Field tiers and
// thresholds / Resolved Decision #10). A field absent from this map has no
// known tier and is never auto-apply eligible (Route treats it as
// unconditionally review-routed) — this map is the allowlist, not a
// best-effort classifier.
var fieldTiers = map[string]Tier{
	"people": TierHigh,
	"studio": TierHigh,
	"movie":  TierHigh, // non-entity rubric (no Movie entity yet, HOLODEX-191) but high-stakes threshold

	"title":        TierMedium,
	"release_date": TierMedium,

	"comment":      TierLow,
	"genre":        TierLow,
	"tags":         TierLow,
	"scene_number": TierLow,
}

// entityFields are the fields scored on the 3-component entity rubric
// (People, Studio). Movie stays on the non-entity rubric despite its
// high-stakes tier (Non-Goals: "Movie ... as first-class entities" is
// deferred to HOLODEX-191) — everything else in fieldTiers not listed here
// uses the 2-component non-entity rubric.
var entityFields = map[string]bool{
	"people": true,
	"studio": true,
}

// IsEntityField reports whether field uses the 3-component entity-resolution
// rubric (true) or the 2-component non-entity rubric (false).
func IsEntityField(field string) bool { return entityFields[field] }

// AutoApplyThreshold returns field's tier threshold and whether field is
// known at all (F48's tier table is the exhaustive allowlist for auto-apply
// eligibility — see Route in routing.go).
func AutoApplyThreshold(field string) (float64, bool) {
	tier, ok := fieldTiers[field]
	if !ok {
		return 0, false
	}
	return tierThresholds[tier], true
}

// ScoreEntity implements the 3-component weighted rubric for People/Studio
// (F48.3a): source agreement (0-0.30) + value specificity (0-0.20) + entity
// resolution (0-0.50), entity resolution weighted heaviest since an exact
// match is the strongest signal the value is *known*, not just plausible.
func ScoreEntity(agreement Agreement, specificity Specificity, match EntityMatch) float64 {
	return scoreAgreement(agreement, 0.30, 0.20, 0.10) +
		scoreSpecificity(specificity, 0.20, 0.07) +
		scoreEntityMatch(match)
}

// ScoreNonEntity implements the 2-component rubric for Title, Release Date,
// Comment, Genre/Tags, Movie, Scene Number (F48.3b): source agreement
// (0-0.50) + value specificity (0-0.50), evenly split since there's no
// entity to resolve against.
func ScoreNonEntity(agreement Agreement, specificity Specificity) float64 {
	return scoreAgreement(agreement, 0.50, 0.30, 0.20) +
		scoreSpecificity(specificity, 0.50, 0.25)
}

func scoreAgreement(a Agreement, exact, single, fuzzy float64) float64 {
	switch a {
	case AgreementExact:
		return exact
	case AgreementSingleSource:
		return single
	case AgreementFuzzy:
		return fuzzy
	default: // AgreementConflict
		return 0
	}
}

func scoreSpecificity(s Specificity, full, partial float64) float64 {
	switch s {
	case SpecificityFull:
		return full
	case SpecificityPartial:
		return partial
	default: // SpecificityGarbled
		return 0
	}
}

func scoreEntityMatch(m EntityMatch) float64 {
	switch m {
	case MatchExact:
		return 0.50
	case MatchFuzzy:
		return 0.20
	default: // MatchNone
		return 0.05
	}
}
