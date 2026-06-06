package metadata

import "testing"

// Pins the ADR-012 width-based buckets and their 10%-tolerance boundaries.
// These are critical-invariant tests: a regression here silently mislabels the
// user's entire library.
func TestClassifyResolution(t *testing.T) {
	cases := []struct {
		name  string
		width int
		want  ResolutionBucket
	}{
		{"zero", 0, ResolutionSD},
		{"sd 854", 854, ResolutionSD},
		{"sd upper boundary 1151", 1151, ResolutionSD},
		{"hd lower boundary 1152", 1152, ResolutionHD},
		{"hd 1280", 1280, ResolutionHD},
		{"hd upper boundary 1727", 1727, ResolutionHD},
		{"fhd lower boundary 1728", 1728, ResolutionFHD},
		{"fhd 1920", 1920, ResolutionFHD},
		{"fhd near-miss 1888", 1888, ResolutionFHD},
		{"fhd upper boundary 3455", 3455, ResolutionFHD},
		{"4k lower boundary 3456", 3456, Resolution4K},
		{"4k uhd 3840", 3840, Resolution4K},
		{"4k scope 3840x1606 (width 3840)", 3840, Resolution4K},
		{"4k near-miss 3792", 3792, Resolution4K},
		{"8k 7680 rolls into 4K+", 7680, Resolution4K},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyResolution(c.width); got != c.want {
				t.Errorf("ClassifyResolution(%d) = %q, want %q", c.width, got, c.want)
			}
		})
	}
}

func TestResolutionWidthRange(t *testing.T) {
	cases := []struct {
		b        ResolutionBucket
		min, max int
	}{
		{ResolutionSD, 0, 1152},
		{ResolutionHD, 1152, 1728},
		{ResolutionFHD, 1728, 3456},
		{Resolution4K, 3456, 0},
	}
	for _, c := range cases {
		min, max := ResolutionWidthRange(c.b)
		if min != c.min || max != c.max {
			t.Errorf("ResolutionWidthRange(%q) = (%d,%d), want (%d,%d)", c.b, min, max, c.min, c.max)
		}
	}
}
