package api

import (
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/resolver"
)

// The "record" baseline vocabulary shared by every entity whose baseline is the DB
// record rather than a file (person F37, studio F38). The resolver and the decision
// store keep the internal "file" token so the resolver core needs zero changes
// (ADR-052); these helpers translate at the API edge. "file" is factually wrong for
// a record-backed entity, so the payload grammar is record | provider:<x> | manual.

const recordSource = "record"

// recordDecisionSource maps a record-payload decision source to the internal
// fieldsource grammar: "record" → "file"; provider/manual pass through. The literal
// "file" is rejected — a record entity's vocabulary is record | provider:<x> |
// manual only, keeping the payload grammar per-entity and unambiguous.
func recordDecisionSource(s string) (string, bool) {
	switch {
	case s == recordSource:
		return fieldsource.File, true
	case s == fieldsource.Manual, fieldsource.Provider(s) != "":
		return s, true
	}
	return "", false
}

// recordize maps the internal "file" token — bare or as a "file:<key>"
// winning-source prefix — to the record vocabulary "record"; anything else passes
// through.
func recordize(s string) string {
	if s == fieldsource.File {
		return recordSource
	}
	if rest, ok := strings.CutPrefix(s, fieldsource.File+":"); ok {
		return recordSource + ":" + rest
	}
	return s
}

// recordizeResolved converts resolver output to the record payload vocabulary and
// strips the writeback concept a record entity doesn't have: every "file" token
// becomes "record" (decision source, candidate sources, winning-source prefix,
// per-value provenance), and in_sync is omitted — a record entity has no file to be
// out of sync with. Mutates in place and returns fields.
func recordizeResolved(fields []resolver.ResolvedField) []resolver.ResolvedField {
	for i := range fields {
		f := &fields[i]
		f.InSync = nil
		if f.Decision != nil {
			f.Decision.Source = recordize(f.Decision.Source)
		}
		for j := range f.Candidates {
			f.Candidates[j].Source = recordize(f.Candidates[j].Source)
		}
		f.WinningSource = recordize(f.WinningSource)
		for j := range f.Items {
			for k := range f.Items[j].Sources {
				f.Items[j].Sources[k] = recordize(f.Items[j].Sources[k])
			}
		}
	}
	return fields
}
