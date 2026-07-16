package extract

// Decision is the routing outcome for one scored extraction candidate
// (F48.4a/b). AutoApply true means the candidate cleared its field tier's
// AutoApplyThreshold and, for entity fields, the exact-match gate — Process
// (process.go) is what actually decides whether that turns into a write,
// since ADR-067 Action Item 2 requires auto-apply to stay log-only behind a
// flag until the ADR is Accepted. Reason is a short, stable string for
// logging/tests, not user-facing copy.
type Decision struct {
	AutoApply bool
	Reason    string
}

const (
	ReasonAutoApply      = "auto_apply"
	ReasonBelowThreshold = "below_threshold"
	ReasonFuzzyGate      = "fuzzy_gate"      // scored above threshold, but only via a fuzzy entity match
	ReasonManualOverride = "manual_override" // F48.3e: a manual: source always wins
	ReasonUnknownField   = "unknown_field"   // no configured tier — never auto-apply eligible
)

// Route decides auto-apply vs. review for one candidate (F48.3d/e, F48.4a/b).
// Two hard rules win regardless of score, checked before the threshold:
//
//  1. hasManualSource (F48.3e, manual-edit precedence): a field already
//     carrying a manual: decision always routes to review on re-extraction,
//     never auto-applies over it.
//  2. The exact-match gate (F48.3d): for entity fields, auto-apply requires
//     match == MatchExact. A fuzzy match that pushes the aggregate score
//     above threshold still routes to review — Jaro-Winkler only ranks a
//     suggestion, it never itself authorizes a write.
//
// field must be a known tier (see AutoApplyThreshold) or Route never
// auto-applies — F48's tier table is the exhaustive allowlist.
func Route(field string, isEntityField bool, match EntityMatch, score float64, hasManualSource bool) Decision {
	if hasManualSource {
		return Decision{AutoApply: false, Reason: ReasonManualOverride}
	}

	threshold, known := AutoApplyThreshold(field)
	if !known {
		return Decision{AutoApply: false, Reason: ReasonUnknownField}
	}

	if isEntityField && match != MatchExact {
		if score >= threshold {
			return Decision{AutoApply: false, Reason: ReasonFuzzyGate}
		}
		return Decision{AutoApply: false, Reason: ReasonBelowThreshold}
	}

	if score >= threshold {
		return Decision{AutoApply: true, Reason: ReasonAutoApply}
	}
	return Decision{AutoApply: false, Reason: ReasonBelowThreshold}
}
