// Package fieldsource is the single source of truth for the F36 per-field
// source-of-truth *decision grammar* (ADR-051): the string values that name which
// layer is true for a replace field — "file", "manual", or "provider:<name>".
//
// This grammar is a wire/DB contract shared across layers: the persisted
// field_source_decisions.source column (repo), the resolver decision short-circuit,
// the API validation, and the JSON FieldDecision.source the SPA reads. Defining it —
// and the provider:<name> parse/format — in one dependency-free leaf package keeps
// those layers in lockstep instead of re-encoding the same strings independently.
package fieldsource

import "strings"

// The three decision sources. File and Manual double as namespaces (a value's
// origin), so they coincide with the resolver's baseline/manual namespaces; a
// provider decision is the prefix plus the provider name.
const (
	File     = "file"
	Manual   = "manual"
	provider = "provider:"
)

// Valid reports whether s is a well-formed decision source: "file", "manual", or
// "provider:<non-empty>". Contextual checks (the provider is actually matched, the
// field is a replace field) live at the API layer where the data is.
func Valid(s string) bool {
	switch s {
	case File, Manual:
		return true
	}
	return Provider(s) != ""
}

// Provider returns the provider name of a "provider:<name>" source, or "" when s is
// not a provider decision.
func Provider(s string) string {
	if !strings.HasPrefix(s, provider) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(s, provider))
}

// ForProvider formats a provider name as a decision source ("provider:<name>").
func ForProvider(name string) string { return provider + name }

// ForNamespace maps a resolved value's namespace (e.g. "file", "manual", or a
// provider name like "tmdb") to its decision source — the inverse of resolution,
// used to report an undecided field's implicit selection.
func ForNamespace(ns string) string {
	switch ns {
	case File, Manual:
		return ns
	default:
		return ForProvider(ns)
	}
}

// Computed is the provenance namespace for a derived field (F45, ADR-063). It is
// display metadata only — deliberately NOT a decision source: a computed value has
// no underlying store to pin, so it can never be adopted. It is therefore kept out
// of Valid() and ForNamespace() by design; a "computed:<canonical>" winning source
// exists only to tag a derived row and drive its "calculated from …" badge.
const Computed = "computed"

// ForComputed formats a computed field's provenance token ("computed:<canonical>").
func ForComputed(canonical string) string { return Computed + ":" + canonical }

// IsComputed reports whether s is a computed provenance token ("computed:<...>").
func IsComputed(s string) bool { return strings.HasPrefix(s, Computed+":") }
