package writeback

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"holodex/internal/mapping"
)

// unreadableWriteTargets are the canonical fields deliberately left WITHOUT a `file:`
// baseline source in the shipped example mapping, because the tag the writeback writes
// cannot be read back on every container that can write it. Each entry is a decision,
// not an oversight: these fields report `in_sync: nil` (unknown) rather than a
// permanent false, which is the ADR-093 contract for an unreadable baseline.
//
// Adding a `file:` source for one of these would be worse than leaving it off — it
// makes the sync state *look* knowable and then reports every write as out of sync on
// the container where the round-trip is broken.
//
// Two known limits, both deliberate. The key is a canonical, but every reason below is
// really per-*container* — all three round-trip fine on Matroska and break only on MP4,
// so an exemption forfeits a knowable answer on the common container to avoid a wrong
// one on the rare one. And the test below only fires when an exemption is contradicted,
// never when its underlying reason is fixed: when HOLODEX-336 lands the MP4 write
// targets, these entries must be removed by hand.
var unreadableWriteTargets = map[string]string{
	"original_title": "written only on Matroska (OriginalMediaType); MP4 has no write target at all, " +
		"so a file source would report a permanent mismatch on every MP4",
	"tagline": "MP4 writes QuickTime:Keywords, which exiftool reads back as `Keywords` — a key " +
		"internal/metadata classifies as a tag and consumes into Extracted.Tags, so it never " +
		"reaches extra_metadata for a file: source to address",
	"original_language": "MP4 writes QuickTime:MediaLanguage, which is not a defined exiftool tag — " +
		"the write is dropped with a warning, so there is nothing to read back (HOLODEX-336)",
}

// readKey normalizes a formatMap tag to the `file:` source key that reads it back: drop
// any exiftool group prefix ("QuickTime:Title" → "Title") and fold case, matching how
// internal/metadata canonicalizes a tag into extra_metadata and how the resolver looks a
// file: source up. "Title" folds to "title", which is also the `file:title` alias for
// videos.title (mapping.Source.IsFileTitle) — the same key whichever way it is read.
func readKey(tag string) string {
	if i := strings.LastIndex(tag, ":"); i >= 0 {
		tag = tag[i+1:]
	}
	return strings.ToLower(strings.TrimSpace(tag))
}

// TestExampleMappingCoversWriteTargets locks the pairing between the two tables that
// disagreed in HOLODEX-335: formatMap (what writeback WRITES) and the shipped example
// mapping's `file:` sources (what the resolver READS back to compute in_sync). Nothing
// else connects them, so a canonical could be writable and unreadable indefinitely —
// which is exactly how `title` and `release_date` stayed "out of sync" forever after a
// successful write.
//
// It asserts agreement, not merely presence: declaring SOME file tag is not enough,
// because `overview: [Description, tmdb:overview]` would satisfy a presence check while
// reading a different tag than the one written and reproducing the bug exactly.
//
// Scoped to REPLACE fields: merge fields carry no decision and no in_sync by contract
// (ADR-051 RD1), so the round-trip does not gate anything for them. That also sidesteps
// `genres`/`actors`, whose write targets (Genre/Artist) are consumed by internal/metadata
// into Extracted.Tags/People rather than extra_metadata.
//
// Blind spot worth stating: a canonical the example ships commented out (today, `title`)
// is absent from Fields() and therefore uncovered here.
func TestExampleMappingCoversWriteTargets(t *testing.T) {
	m, err := mapping.Load(filepath.Join("..", "..", "metadata-mappings.yaml.example"))
	if err != nil {
		t.Fatalf("load example mapping: %v", err)
	}

	// canonical → the read keys that would pick its written value back up, across every
	// container that can write it.
	wantKeys := map[string]map[string]bool{}
	for _, tags := range formatMap {
		for canonical, tag := range tags {
			if wantKeys[canonical] == nil {
				wantKeys[canonical] = map[string]bool{}
			}
			wantKeys[canonical][readKey(tag)] = true
		}
	}

	for _, f := range m.Fields() {
		want := wantKeys[f.Canonical]
		if f.Multi || f.Merge || want == nil {
			continue
		}
		reads := false
		for _, src := range f.ParsedSources {
			if src.Namespace == "file" && want[readKey(src.Key)] {
				reads = true
				break
			}
		}
		reason, exempt := unreadableWriteTargets[f.Canonical]
		switch {
		case reads && exempt:
			t.Errorf("%s reads back its write target but is listed as unreadable (%s) — "+
				"drop the exemption if the round-trip now works, or drop the source", f.Canonical, reason)
		case !reads && !exempt:
			t.Errorf("%s is writeback-capable but declares no file: source matching the tag "+
				"writeback writes, so its written value can never be read back and in_sync stays "+
				"unknown forever. Add one of %v to its sources, or record why it can't round-trip "+
				"in unreadableWriteTargets.", f.Canonical, sortedKeys(want))
		}
	}
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
