package registry

import "testing"

// TestCriticality proves F55.1's acceptance criteria: a P0-scored facet
// carries its criticality tag, and an excluded field carries none.
func TestCriticality(t *testing.T) {
	if got := Lookup("title").Criticality; got != CriticalityCritical {
		t.Errorf("title criticality = %q, want %q", got, CriticalityCritical)
	}
	if got := Lookup("commentary").Criticality; got != "" {
		t.Errorf("commentary criticality = %q, want unset (excluded)", got)
	}
	// A Computed field is always excluded from scoring, regardless of any
	// criticality tag — the scorer treats Computed as automatic exclusion.
	if age := Lookup("age"); !age.Computed || age.Criticality != "" {
		t.Errorf("age = %+v, want Computed=true, Criticality unset", age)
	}
}
