package extract

import "testing"

func TestRoute_ManualOverrideAlwaysWins(t *testing.T) {
	// F48.3e: even a perfect, exact-match candidate must queue when a manual:
	// decision already exists for the field.
	d := Route("people", true, MatchExact, 1.0, true)
	if d.AutoApply {
		t.Fatal("manual override must never auto-apply")
	}
	if d.Reason != ReasonManualOverride {
		t.Fatalf("Reason = %q, want %q", d.Reason, ReasonManualOverride)
	}
}

func TestRoute_ExactMatchGate_AutoApplies(t *testing.T) {
	score := ScoreEntity(AgreementExact, SpecificityFull, MatchExact) // 1.0
	d := Route("people", true, MatchExact, score, false)
	if !d.AutoApply {
		t.Fatalf("expected auto-apply, got Decision %+v (score %v)", d, score)
	}
}

// TestRoute_FuzzyMatchNeverAutoApplies is F48.3d's core invariant: a fuzzy
// entity match that scores at or above the tier threshold still routes to
// review — the exact-match gate is a hard rule, not just a score.
func TestRoute_FuzzyMatchNeverAutoApplies(t *testing.T) {
	score := ScoreEntity(AgreementExact, SpecificityFull, MatchFuzzy) // 0.30+0.20+0.20 = 0.70, but High tier threshold is 0.80
	// Bump the aggregate above the 0.80 High-tier threshold via source
	// agreement + specificity alone, while entity resolution stays fuzzy —
	// exercising the case where aggregate score clears threshold but the
	// entity-resolution component didn't come from an exact match.
	score = 0.30 + 0.20 + 0.20*1.5 // synthetic: > 0.80 aggregate, entity component still "fuzzy-derived"
	if score < 0.80 {
		t.Fatalf("test setup: score %v must exceed the High-tier threshold", score)
	}
	d := Route("studio", true, MatchFuzzy, score, false)
	if d.AutoApply {
		t.Fatalf("a fuzzy entity match must never auto-apply regardless of score (score=%v)", score)
	}
	if d.Reason != ReasonFuzzyGate {
		t.Fatalf("Reason = %q, want %q", d.Reason, ReasonFuzzyGate)
	}
}

func TestRoute_BelowThreshold_RoutesToReview(t *testing.T) {
	score := ScoreNonEntity(AgreementConflict, SpecificityGarbled) // 0
	d := Route("title", false, MatchNone, score, false)
	if d.AutoApply {
		t.Fatal("a low-confidence non-entity candidate must not auto-apply")
	}
	if d.Reason != ReasonBelowThreshold {
		t.Fatalf("Reason = %q, want %q", d.Reason, ReasonBelowThreshold)
	}
}

func TestRoute_NonEntityField_AutoAppliesWithoutEntityMatch(t *testing.T) {
	score := ScoreNonEntity(AgreementExact, SpecificityFull) // 1.0
	// isEntityField=false, match is irrelevant (MatchNone is the zero value a
	// non-entity caller would pass).
	d := Route("title", false, MatchNone, score, false)
	if !d.AutoApply {
		t.Fatalf("expected auto-apply for a high-confidence non-entity field, got %+v", d)
	}
}

func TestRoute_UnknownField_NeverAutoApplies(t *testing.T) {
	d := Route("not_a_real_field", false, MatchNone, 1.0, false)
	if d.AutoApply {
		t.Fatal("an unmapped field must never auto-apply")
	}
	if d.Reason != ReasonUnknownField {
		t.Fatalf("Reason = %q, want %q", d.Reason, ReasonUnknownField)
	}
}

func TestRoute_TierThresholds(t *testing.T) {
	tests := []struct {
		field     string
		score     float64
		wantApply bool
	}{
		{"comment", 0.40, true},  // Low tier, at threshold
		{"comment", 0.39, false}, // Low tier, just under
		{"title", 0.70, true},    // Medium tier, at threshold
		{"title", 0.69, false},
	}
	for _, tt := range tests {
		d := Route(tt.field, false, MatchNone, tt.score, false)
		if d.AutoApply != tt.wantApply {
			t.Errorf("Route(%q, score=%v) AutoApply = %v, want %v", tt.field, tt.score, d.AutoApply, tt.wantApply)
		}
	}
}
