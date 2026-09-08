package writeback

import (
	"path/filepath"
	"testing"

	"holodex/internal/mapping"
)

// TestExampleMappingCoversWriteTargets holds the shipped example to the invariant
// ReadbackGaps enforces at runtime: every replace field writeback can write declares a
// `file:` source matching the tag it writes. Without it the example is free to drift
// back into the HOLODEX-335 shape, and a fresh install starts out unable to verify its
// own writes.
//
// Blind spot worth stating: a canonical the example ships commented out (today, `title`
// — one of the two fields the bug was reported against) is absent from Fields() and so
// is uncovered here.
func TestExampleMappingCoversWriteTargets(t *testing.T) {
	m, err := mapping.Load(filepath.Join("..", "..", "metadata-mappings.yaml.example"))
	if err != nil {
		t.Fatalf("load example mapping: %v", err)
	}
	for _, g := range ReadbackGaps(m.Fields()) {
		t.Errorf("%s is writeback-capable but declares no file: source matching the tag "+
			"writeback writes, so its written value can never be read back and in_sync stays "+
			"unknown forever. Add one of %v to its sources, or record why it can't round-trip "+
			"in UnreadableWriteTargets.", g.Canonical, g.WantKeys)
	}
}

// TestUnreadableExemptionsStayJustified is the other direction: an exemption claims a
// field CANNOT round-trip, so the example must not declare a source for it. A field that
// has both is a contradiction — either the round-trip was fixed (drop the exemption, per
// HOLODEX-336) or the source is dead config that will report a permanent mismatch on the
// container the exemption is about.
func TestUnreadableExemptionsStayJustified(t *testing.T) {
	m, err := mapping.Load(filepath.Join("..", "..", "metadata-mappings.yaml.example"))
	if err != nil {
		t.Fatalf("load example mapping: %v", err)
	}
	for _, f := range m.Fields() {
		reason, exempt := UnreadableWriteTargets[f.Canonical]
		if !exempt {
			continue
		}
		for _, src := range f.ParsedSources {
			if src.Namespace == "file" {
				t.Errorf("%s declares file source %q but is listed as unreadable (%s) — drop the "+
					"exemption if the round-trip now works, or drop the source", f.Canonical, src.Key, reason)
			}
		}
	}
}
