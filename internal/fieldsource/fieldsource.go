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
// "provider:<non-empty>", or "local:<kind>". Contextual checks (the provider is
// actually matched, the field is a replace field, a local source names poster_url
// and a held slot) live at the API layer where the data is.
func Valid(s string) bool {
	switch s {
	case File, Manual:
		return true
	}
	return Provider(s) != "" || Local(s) != ""
}

// The local poster kinds (F85, field-resolution.md "A poster Holodex holds"): an
// image Holodex stores itself, one source slot per kind per video. The set is
// closed — a kind is also a filename component, so nothing outside it may pass.
const (
	LocalFrame     = "frame"
	LocalUpload    = "upload"
	LocalAutomatic = "automatic"
	local          = "local:"
)

// LocalKinds lists the closed local kind set, in the chooser's tile order.
var LocalKinds = []string{LocalFrame, LocalUpload, LocalAutomatic}

// LocalField is the one field a local decision may name.
const LocalField = "poster_url"

// ValidLocalKind reports whether k is one of LocalKinds.
func ValidLocalKind(k string) bool {
	return k == LocalFrame || k == LocalUpload || k == LocalAutomatic
}

// Local returns the kind of a "local:<kind>" source, or "" when s is not one or
// names a kind outside the closed set.
func Local(s string) string {
	k, ok := strings.CutPrefix(s, local)
	if !ok || !ValidLocalKind(k) {
		return ""
	}
	return k
}

// ForLocal formats a local kind as a decision source ("local:<kind>").
func ForLocal(kind string) string { return local + kind }

// LocalToken is the resolved value of a local decision: the source plus the
// content hash of the slot's bytes, so the value moves whenever the image does.
func LocalToken(kind, hash string) string { return local + kind + "#" + hash }

// FileCoverToken is poster_url's file value when the file carries cover art: the
// file source plus the art's content hash (F85). It names the file's own image, so
// it is never a write — the file already holds it.
func FileCoverToken(hash string) string { return File + "#" + hash }

// ParseLocalToken splits a "local:<kind>#<sha256>" token. ok is false for any
// other string, an unknown kind, or a hash that is not 64 lower-case hex digits.
func ParseLocalToken(v string) (kind, hash string, ok bool) {
	src, hash, found := strings.Cut(v, "#")
	kind = Local(src)
	if !found || kind == "" || !isSHA256Hex(hash) {
		return "", "", false
	}
	return kind, hash, true
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
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
	switch {
	case ns == File, ns == Manual, Local(ns) != "":
		return ns
	default:
		return ForProvider(ns)
	}
}

// IsCleared reports whether a decision is an owner-cleared field (ADR-120 D1):
// a manual decision with no value. Only an explicit clear request can store
// one, since the API refuses an empty manual value otherwise.
func IsCleared(source, manualValue string) bool {
	return source == Manual && strings.TrimSpace(manualValue) == ""
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
