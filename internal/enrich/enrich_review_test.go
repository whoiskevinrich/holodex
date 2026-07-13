package enrich

import "testing"

// SingleStrongMatch is the auto-apply cardinality check (ADR-065 D1) — exactly one
// AutoApply=true candidate routes to apply(); any other outcome (0, or 2+) leaves the
// ambiguity for the owner. AutoApply itself (the threshold computation) is
// sanitizeCandidates' job, covered by TestSanitizeCandidatesAutoApply — these cases
// set it directly to test SingleStrongMatch's cardinality logic in isolation.
func TestSingleStrongMatch(t *testing.T) {
	tests := []struct {
		name  string
		cands []Candidate
		want  bool
		id    string
	}{
		{"no candidates", nil, false, ""},
		{"one weak", []Candidate{{ExternalID: "a", AutoApply: false}}, false, ""},
		{"one strong", []Candidate{{ExternalID: "a", AutoApply: true}}, true, "a"},
		{"one strong among weak", []Candidate{
			{ExternalID: "weak", AutoApply: false},
			{ExternalID: "strong", AutoApply: true},
		}, true, "strong"},
		{"two strong", []Candidate{
			{ExternalID: "a", AutoApply: true},
			{ExternalID: "b", AutoApply: true},
		}, false, ""},
		{"two strong one weak", []Candidate{
			{ExternalID: "a", AutoApply: true},
			{ExternalID: "b", AutoApply: true},
			{ExternalID: "c", AutoApply: false},
		}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SingleStrongMatch(tt.cands)
			if ok != tt.want {
				t.Fatalf("ok = %v, want %v", ok, tt.want)
			}
			if ok && got.ExternalID != tt.id {
				t.Errorf("candidate = %v, want %v", got.ExternalID, tt.id)
			}
		})
	}
}
