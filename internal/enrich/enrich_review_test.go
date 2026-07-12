package enrich

import "testing"

// SingleStrongMatch is the auto-apply threshold check (ADR-065 D1) — exactly one
// candidate at/above StrongMatchThreshold routes to apply(); any other outcome
// (0, or 2+) leaves the ambiguity for the owner. Table-driven over the 0/1/2 boundary.
func TestSingleStrongMatch(t *testing.T) {
	tests := []struct {
		name  string
		cands []Candidate
		want  bool
		id    string
	}{
		{"no candidates", nil, false, ""},
		{"one weak", []Candidate{{ExternalID: "a", Confidence: 0.5}}, false, ""},
		{"one exactly at threshold", []Candidate{{ExternalID: "a", Confidence: StrongMatchThreshold}}, true, "a"},
		{"one strong among weak", []Candidate{
			{ExternalID: "weak", Confidence: 0.4},
			{ExternalID: "strong", Confidence: 0.95},
		}, true, "strong"},
		{"two strong", []Candidate{
			{ExternalID: "a", Confidence: 0.9},
			{ExternalID: "b", Confidence: 0.86},
		}, false, ""},
		{"two strong one weak", []Candidate{
			{ExternalID: "a", Confidence: 0.9},
			{ExternalID: "b", Confidence: 0.86},
			{ExternalID: "c", Confidence: 0.1},
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
