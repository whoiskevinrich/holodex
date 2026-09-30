package main

import (
	"reflect"
	"slices"
	"testing"

	"holodex/internal/mapping"
)

// The replace-studio variant (HOLODEX-497) must stay the fixture's mapping with one change:
// studio as a replace field reading Publisher then Label. Any other difference means the two
// files drifted, and a QA session on the variant would no longer be testing the fixture.
func TestReplaceStudioVariantDiffersOnlyInStudio(t *testing.T) {
	base, err := mapping.Load("mappings.yaml")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := mapping.Load("mappings-replace-studio.yaml")
	if err != nil {
		t.Fatal(err)
	}

	byName := func(m *mapping.Mappings) map[string]mapping.Field {
		out := map[string]mapping.Field{}
		for _, f := range m.Fields() {
			out[f.Canonical] = f
		}
		return out
	}
	b, v := byName(base), byName(variant)
	if len(b) != len(v) {
		t.Fatalf("field count: base %d, variant %d", len(b), len(v))
	}
	for name, bf := range b {
		vf, ok := v[name]
		if !ok {
			t.Errorf("variant is missing field %q", name)
			continue
		}
		if name == "studio" {
			continue
		}
		if !reflect.DeepEqual(bf, vf) {
			t.Errorf("field %q drifted from mappings.yaml:\nbase    %+v\nvariant %+v", name, bf, vf)
		}
	}

	studio := v["studio"]
	if studio.Multi || studio.Merge {
		t.Errorf("variant studio must be a replace field, got multi=%v merge=%v", studio.Multi, studio.Merge)
	}
	if got := studio.FileTagSources(); !slices.Equal(got, []string{"Publisher", "Label"}) {
		t.Errorf("variant studio file sources = %v, want [Publisher Label]", got)
	}
	if !b["studio"].Multi {
		t.Error("the canonical fixture's studio is expected to be multi — if that changed, this variant may be unnecessary")
	}
}
