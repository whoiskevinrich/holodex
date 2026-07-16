package extract

import "testing"

func approxEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

// TestJaroWinkler checks against known reference values for the classic
// algorithm (e.g. Winkler 1990's own worked examples).
func TestJaroWinkler(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"", "", 1},
		{"MARTHA", "MARTHA", 1},
		{"MARTHA", "MARHTA", 0.9611},
		{"DIXON", "DICKSONX", 0.8133},
		{"JELLYFISH", "SMELLYFISH", 0.8962},
		{"totally", "different", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			got := JaroWinkler(tt.a, tt.b)
			if !approxEqual(got, tt.want, 0.01) {
				t.Fatalf("JaroWinkler(%q, %q) = %v, want ~%v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestJaroWinkler_CaseInsensitive(t *testing.T) {
	if got := JaroWinkler("Alice Smith", "alice smith"); got != 1 {
		t.Fatalf("case/whitespace-folded match should be 1.0, got %v", got)
	}
}

func TestClassifyAgreement(t *testing.T) {
	tests := []struct {
		name          string
		filename, tag string
		want          Agreement
	}{
		{"exact", "Alice Smith", "Alice Smith", AgreementExact},
		{"exact case-insensitive", "alice smith", "Alice Smith", AgreementExact},
		{"single source (no tag data)", "Alice Smith", "", AgreementSingleSource},
		{"fuzzy (typo)", "Alice Smith", "Alise Smith", AgreementFuzzy},
		{"conflict", "Alice Smith", "Bob Jones", AgreementConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAgreement(tt.filename, tt.tag)
			if got != tt.want {
				t.Fatalf("classifyAgreement(%q, %q) = %v, want %v", tt.filename, tt.tag, got, tt.want)
			}
		})
	}
}

func TestClassifySpecificity(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		entity bool
		want   Specificity
	}{
		{"entity multi-word", "Alice Smith", true, SpecificityFull},
		{"entity single word", "Alice", true, SpecificityPartial},
		{"entity garbled", "   ", true, SpecificityGarbled},
		{"non-entity structured", "The Great Movie", false, SpecificityFull},
		{"non-entity partial (too short)", "Hi", false, SpecificityPartial},
		{"non-entity garbled (empty)", "", false, SpecificityGarbled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySpecificity(tt.value, tt.entity)
			if got != tt.want {
				t.Fatalf("classifySpecificity(%q, %v) = %v, want %v", tt.value, tt.entity, got, tt.want)
			}
		})
	}
}

func TestBestFuzzyMatch(t *testing.T) {
	candidates := map[int64]string{
		1: "Alice Smith",
		2: "Bob Jones",
	}
	id, score, ok := BestFuzzyMatch("Alise Smith", candidates)
	if !ok {
		t.Fatalf("expected a fuzzy match above threshold, got score %v", score)
	}
	if id != 1 {
		t.Fatalf("expected candidate 1 (Alice Smith), got %d", id)
	}

	if _, _, ok := BestFuzzyMatch("Zed Totally Different", candidates); ok {
		t.Fatal("expected no match above threshold")
	}

	if _, _, ok := BestFuzzyMatch("Alice Smith", map[int64]string{}); ok {
		t.Fatal("expected no match against an empty candidate set")
	}
}
