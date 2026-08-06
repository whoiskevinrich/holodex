package repo

import "strings"

// This file, not identity.go, deliberately hosts the extIDByName lookup fallback
// below: identity.go's nameKeyExpr is the SQL-only identity spine (person, studio,
// tag alike — normalization computed in SQL, never in Go, per that file's package
// comment). foldNameKey/extIDFor are a narrow, Go-side exception scoped to one
// caller need (see extIDFor), kept out of that spine on purpose.

// foldNameKey trims and lowercases a name for extIDFor's fallback lookup below.
// Deliberately mirrors resolver.NormKey's fold rule (trim + lower, ADR-061) rather
// than importing it — repo sits below resolver in this codebase's layering.
func foldNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// foldedExtIDIndex builds a folded-name → external-id index from an extIDByName
// sidecar map (internal/api's externalIDsFromRows) once per Reconcile call, so
// ReconcileVideoPeople/ReconcileVideoStudios fall back to a case-insensitive lookup
// in O(1) per name instead of rescanning and re-folding the whole map on every miss.
func foldedExtIDIndex(byName map[string]string) map[string]string {
	if len(byName) == 0 {
		return nil
	}
	out := make(map[string]string, len(byName))
	for n, id := range byName {
		out[foldNameKey(n)] = id
	}
	return out
}

// extIDFor looks up name's external id: an exact match against byName first (the
// map's documented key convention, built from a provider's raw name — every caller,
// including tests, constructs and relies on this directly), then folded as a
// fallback for when name has since passed through the resolver's per-field casing
// transform (metadata-mappings.yaml), which an exact match can't see through. A miss
// falls back to name-only resolve-or-create (the pre-existing behavior) — never a
// wrong merge — so this fold doesn't need SQL-collation-grade correctness.
func extIDFor(byName, folded map[string]string, name string) string {
	if id, ok := byName[name]; ok {
		return id
	}
	return folded[foldNameKey(name)]
}
