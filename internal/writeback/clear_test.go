package writeback

import (
	"slices"
	"testing"
)

var studioSources = []string{"Publisher", "Label", "Studio", "ProductionCompany"}

// TestValidClearTagName pins the delete-name allowlist the security review
// asked for (ADR-120 D4): the names are spliced into "-NAME=" / "NAME=", so
// "all", wildcards and argument syntax must never pass.
func TestValidClearTagName(t *testing.T) {
	cases := []struct {
		container, tag string
		want           bool
	}{
		{"MP4", "QuickTime:Publisher", true},
		{"MP4", "QuickTime:Label", true},
		{"Matroska", "Label", true},
		{"Matroska", "Publisher", true},
		{"mp3", "ProductionCompany", true},

		{"MP4", "all", false},
		{"MP4", "ALL", false},
		{"MP4", "QuickTime:all", false},
		{"MP4", "*", false},
		{"MP4", "QuickTime:*", false},
		{"MP4", "-Label", false},
		{"MP4", "Label=", false},
		{"MP4", "Label<x", false},
		{"MP4", "La bel", false},
		{"MP4", "Keys:Label:x", false},
		{"MP4", "Qu ick:Label", false},
		{"Matroska", "QuickTime:Label", false}, // grouped name on a bare-tag container
		{"MP4", "QuickTime:Genres", false},     // valid shape, not studio's tag
		{"MP4", "QuickTime:Title", false},
		{"Matroska", "Title", false},
		{"avi", "Publisher", false}, // container with no studio mapping
	}
	for _, c := range cases {
		if got := ValidClearTagName(c.container, "studio", c.tag, studioSources); got != c.want {
			t.Errorf("ValidClearTagName(%q, studio, %q) = %v, want %v", c.container, c.tag, got, c.want)
		}
	}
}

// A mapping that lists a hostile name can't smuggle it through: membership
// alone never overrides the shape and "all" checks.
func TestValidClearTagName_MappingCannotWiden(t *testing.T) {
	hostile := []string{"all", "*", "-all", "Label=x"}
	for _, n := range hostile {
		if ValidClearTagName("Matroska", "studio", n, hostile) {
			t.Errorf("hostile mapping source %q passed", n)
		}
	}
}

func TestClearTagNames(t *testing.T) {
	names, rejected := ClearTagNames("MP4", "studio", studioSources)
	want := []string{"QuickTime:Publisher", "QuickTime:Label", "QuickTime:Studio", "QuickTime:ProductionCompany"}
	if !slices.Equal(names, want) || len(rejected) != 0 {
		t.Fatalf("MP4 = %v rejected %v, want %v", names, rejected, want)
	}

	names, _ = ClearTagNames("Matroska", "studio", studioSources)
	if want := []string{"Publisher", "Label", "Studio", "ProductionCompany"}; !slices.Equal(names, want) {
		t.Fatalf("Matroska = %v, want %v", names, want)
	}

	names, rejected = ClearTagNames("Matroska", "studio", []string{"Label", "all", "Bad Name"})
	if !slices.Equal(names, []string{"Publisher", "Label"}) || !slices.Equal(rejected, []string{"all", "Bad Name"}) {
		t.Fatalf("hostile sources: names %v rejected %v", names, rejected)
	}

	if names, rejected := ClearTagNames("avi", "studio", studioSources); names != nil || rejected != nil {
		t.Fatalf("unmapped container should yield nothing, got %v / %v", names, rejected)
	}
}
