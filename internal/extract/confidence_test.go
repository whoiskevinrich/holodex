package extract

import "testing"

// TestScoreEntity reproduces each named scenario from the spec's Concepts &
// Model entity rubric (F48.3a).
func TestScoreEntity(t *testing.T) {
	tests := []struct {
		name        string
		agreement   Agreement
		specificity Specificity
		match       EntityMatch
		want        float64
	}{
		{"exact agreement + multi-word + exact entity", AgreementExact, SpecificityFull, MatchExact, 0.30 + 0.20 + 0.50},
		{"single source + multi-word + no entity (would create new)", AgreementSingleSource, SpecificityFull, MatchNone, 0.20 + 0.20 + 0.05},
		{"fuzzy agreement + single word + fuzzy entity", AgreementFuzzy, SpecificityPartial, MatchFuzzy, 0.10 + 0.07 + 0.20},
		{"conflict + garbled + no entity (floor)", AgreementConflict, SpecificityGarbled, MatchNone, 0 + 0 + 0.05},
		{"single source + multi-word + exact entity (pre-existing person)", AgreementSingleSource, SpecificityFull, MatchExact, 0.20 + 0.20 + 0.50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreEntity(tt.agreement, tt.specificity, tt.match)
			if diff := got - tt.want; diff > 1e-9 || diff < -1e-9 {
				t.Fatalf("ScoreEntity() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestScoreNonEntity reproduces each named scenario for Title/Release
// Date/Comment/Genre/Movie/Scene Number (F48.3b).
func TestScoreNonEntity(t *testing.T) {
	tests := []struct {
		name        string
		agreement   Agreement
		specificity Specificity
		want        float64
	}{
		{"exact agreement + structured", AgreementExact, SpecificityFull, 0.50 + 0.50},
		{"single source + structured", AgreementSingleSource, SpecificityFull, 0.30 + 0.50},
		{"fuzzy agreement + partial", AgreementFuzzy, SpecificityPartial, 0.20 + 0.25},
		{"conflict + garbled (floor)", AgreementConflict, SpecificityGarbled, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreNonEntity(tt.agreement, tt.specificity)
			if diff := got - tt.want; diff > 1e-9 || diff < -1e-9 {
				t.Fatalf("ScoreNonEntity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoApplyThreshold(t *testing.T) {
	tests := []struct {
		field string
		want  float64
	}{
		{"people", 0.80},
		{"studio", 0.80},
		{"movie", 0.80},
		{"title", 0.70},
		{"release_date", 0.70},
		{"comment", 0.40},
		{"genre", 0.40},
		{"tags", 0.40},
		{"scene_number", 0.40},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, ok := AutoApplyThreshold(tt.field)
			if !ok {
				t.Fatalf("AutoApplyThreshold(%q): expected known field", tt.field)
			}
			if got != tt.want {
				t.Fatalf("AutoApplyThreshold(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestAutoApplyThreshold_UnknownField(t *testing.T) {
	if _, ok := AutoApplyThreshold("not_a_real_field"); ok {
		t.Fatal("expected unknown field to report ok=false")
	}
}

func TestIsEntityField(t *testing.T) {
	if !IsEntityField("people") || !IsEntityField("studio") {
		t.Fatal("people and studio must be entity fields")
	}
	if IsEntityField("movie") {
		t.Fatal("movie stays on the non-entity rubric (no Movie entity yet, HOLODEX-191)")
	}
	if IsEntityField("title") {
		t.Fatal("title is not an entity field")
	}
}
