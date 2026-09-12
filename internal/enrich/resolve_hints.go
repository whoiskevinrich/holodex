// Structured /resolve hints (ADR-095): the manifest opt-in gate that decides which
// of Hint's structured keys reach a provider, and the ingest sanitizer for the
// provider's `searched[]` reply. Callers (internal/api) build the full Hint; this file
// is the one place that decides what leaves the box.
package enrich

import "strings"

// /describe.resolve_hints vocabulary (ADR-095 D1, contract §4.10).
const (
	ResolveHintFields   = "fields"
	ResolveHintFilename = "filename"
)

// SearchFieldKeys is the whole vocabulary hint.fields may carry (ADR-095 D2): the
// five canonical fields §4.9's pattern tokens are rendered from, and nothing else.
// Bounded on purpose — `overview`/`tagline` can carry the owner's own free text and
// `homepage`/`external_provider_id`/`poster_url` would tell provider A which other
// providers the owner uses (security review, 2026-09-11). Widening this list is a
// contract change with its own review, not a convenience.
var SearchFieldKeys = []string{"title", "studio", "actors", "director", "release_date"}

// maxSearched caps a provider's searched[] reply (contract §5) — Holodex keeps the
// first N, in issue order.
const maxSearched = 10

// gateHint applies the ADR-095 D1 opt-in and operator deny to a caller-built hint,
// returning what may go on the wire to this provider: Fields only when the manifest
// lists "fields" (and then only SearchFieldKeys ∩ the provider's advertised fields),
// Filename only when it lists "filename" AND the operator has not set
// send_filename: false, QuerySource with either opt-in. A manifest with no
// resolve_hints yields a hint whose new keys are all zero — and, through omitempty,
// a request body byte-identical to pre-ADR-095. Unknown list entries are ignored.
func gateHint(hint Hint, src Source, m Manifest) Hint {
	wantFields, wantFilename := false, false
	for _, h := range m.ResolveHints {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case ResolveHintFields:
			wantFields = true
		case ResolveHintFilename:
			wantFilename = true
		}
	}
	if wantFields {
		hint.Fields = intersectSearchFields(hint.Fields, m.Fields)
	} else {
		hint.Fields = nil
	}
	if !wantFilename || !src.FilenameAllowed() {
		hint.Filename = ""
	}
	if !wantFields && !wantFilename {
		hint.QuerySource = ""
	}
	return hint
}

// intersectSearchFields keeps only the SearchFieldKeys the provider advertises in
// /describe.fields (case-insensitive on the advertised side) that have at least one
// value. Nil when nothing survives, so omitempty drops the key rather than sending {}.
func intersectSearchFields(fields map[string][]string, advertised []string) map[string][]string {
	adv := make(map[string]bool, len(advertised))
	for _, a := range advertised {
		adv[strings.ToLower(strings.TrimSpace(a))] = true
	}
	var out map[string][]string
	for _, key := range SearchFieldKeys {
		vals := fields[key]
		if !adv[key] || len(vals) == 0 {
			continue
		}
		if out == nil {
			out = map[string][]string{}
		}
		out[key] = vals
	}
	return out
}

// sanitizeSearched bounds an untrusted searched[] reply (contract §5): first
// maxSearched entries, each through SanitizeValue (control characters stripped,
// newlines collapsed, capped at maxFieldLen — the candidates[].label treatment),
// empty entries dropped. Nil in ⇒ nil out, so an omitted key stays distinguishable
// from an empty list.
func sanitizeSearched(in []string) []string {
	if len(in) > maxSearched {
		in = in[:maxSearched]
	}
	out := SanitizeValues(in)
	if len(out) == 0 {
		return nil
	}
	return out
}
