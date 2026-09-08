package writeback

import (
	"log/slog"
	"sort"
	"strings"

	"holodex/internal/mapping"
)

// This file connects the two tables that HOLODEX-335 proved can silently disagree:
// formatMap (the tag writeback WRITES for a canonical field) and a mapping's `file:`
// sources (the tag the resolver READS back to decide whether the file already carries
// the decided value). Nothing else relates them, so a field can be writable and
// unreadable indefinitely — which is how `title` and `release_date` reported "out of
// sync" forever after a successful write. ADR-093 D1 makes that state honest (in_sync
// reports unknown rather than false); this reports it to the operator, who is the only
// one who can fix it, because metadata-mappings.yaml is per-deployment and gitignored.

// UnreadableWriteTargets are the canonical fields whose write target cannot be read
// back on every container that can write it, keyed to the reason. They are exempt from
// the gap check: reporting them would be noise, since no `file:` source can close the
// gap and the operator has nothing to act on.
//
// Two known limits, both deliberate. The key is a canonical, but every reason below is
// really per-*container* — all three round-trip fine on Matroska and break only on MP4,
// so an exemption forfeits a knowable answer on the common container to avoid a wrong
// one on the rare one. And nothing fires when an underlying reason is *fixed*: when
// HOLODEX-336 lands the MP4 write targets, these entries must be removed by hand.
var UnreadableWriteTargets = map[string]string{
	"original_title": "written only on Matroska (OriginalMediaType); MP4 has no write target at all, " +
		"so a file source would report a permanent mismatch on every MP4",
	"tagline": "MP4 writes QuickTime:Keywords, which exiftool reads back as `Keywords` — a key " +
		"internal/metadata classifies as a tag and consumes into Extracted.Tags, so it never " +
		"reaches extra_metadata for a file: source to address",
	"original_language": "MP4 writes QuickTime:MediaLanguage, which is not a defined exiftool tag — " +
		"the write is dropped with a warning, so there is nothing to read back (HOLODEX-336)",
}

// ReadbackGap is one replace field that writeback can write but whose mapping declares
// no `file:` source matching the written tag. WantKeys are the source keys that would
// close it, sorted.
type ReadbackGap struct {
	Canonical string
	WantKeys  []string
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

// ReadbackGaps returns the replace fields in the mapping that writeback can write but
// cannot verify, in canonical order.
//
// It compares keys rather than checking that some `file:` source is merely present:
// `overview: [Description, tmdb:overview]` would satisfy a presence check while reading
// a different tag than the one written, reproducing HOLODEX-335 exactly.
//
// Scoped to replace fields, because merge fields carry no decision and no in_sync by
// contract (ADR-051 RD1) — there is nothing for a read-back to verify. That also
// sidesteps `genres`/`actors`, whose write targets (Genre/Artist) are consumed by
// internal/metadata into Extracted.Tags/People rather than extra_metadata, so no `file:`
// source could ever satisfy them.
func ReadbackGaps(fields []mapping.Field) []ReadbackGap {
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

	var gaps []ReadbackGap
	for _, f := range fields {
		want := wantKeys[f.Canonical]
		if f.Multi || f.Merge || want == nil {
			continue
		}
		if _, exempt := UnreadableWriteTargets[f.Canonical]; exempt {
			continue
		}
		reads := false
		for _, src := range f.ParsedSources {
			if src.Namespace == "file" && want[readKey(src.Key)] {
				reads = true
				break
			}
		}
		if !reads {
			gaps = append(gaps, ReadbackGap{Canonical: f.Canonical, WantKeys: sortedKeys(want)})
		}
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].Canonical < gaps[j].Canonical })
	return gaps
}

// LogReadbackGaps warns once per gap. Called wherever a mapping becomes live — process
// start and POST /admin/reload-config — so an operator editing metadata-mappings.yaml
// finds out immediately rather than via a bug report about a pill that never clears.
// A gap is a config problem, never a reason to refuse to start.
func LogReadbackGaps(log *slog.Logger, fields []mapping.Field) {
	for _, g := range ReadbackGaps(fields) {
		log.Warn("writeback read-back gap: this field declares no file source matching the tag writeback writes, so its sync state can never be verified",
			"field", g.Canonical,
			"add_one_of", strings.Join(g.WantKeys, ", "),
			"doc", "docs/reference/canonical-fields.md#writeback-round-trip")
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
