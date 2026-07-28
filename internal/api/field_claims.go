package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
)

// Claimed provider keys (F49, ADR-074). A canonical field may claim a differently-named
// provider key: the key contributes its value as a candidate source of that field and
// stops auto-registering as its own display-only row (the GH #178 fix). Operators express
// this in metadata-mappings.yaml as a field's `sources:` list; the owner expresses it
// in-app through the field_claims store — and both materialize into the same
// mapping.Source before anything reads them (D2), so suppression has one code path and
// the two halves cannot disagree.

// mergeClaims materializes the entity type's claims onto the base field set: each claim
// appends a mapping.Source on its target field. It runs immediately after mergePromotions
// at all three call sites, which is what lets a claim target a *promoted* field — key X
// promoted to its own field, later joined by another provider's spelling of it (D5).
//
// Claims append at the END of the target's candidate list, below every YAML source, and
// in (provider, field_key) order (the order the repo returns). That is deliberate: a
// claim states identity, not precedence, so adding one must never move the resolved
// winner — ADR-051 per-entity source decisions remain the instrument for that.
//
// A claim whose target canonical is absent from the effective set is skipped and logged
// (D4). It is inert rather than pruned: the key simply auto-registers again, exactly as
// pre-F49, because suppression reads this merged set and not the claims table — so
// "value suppressed with nowhere to go" is unrepresentable rather than guarded against.
// The warning repeats per resolve, which is the intent: it stops the moment the target
// reappears or the claim is removed from the owner's Attached keys list.
func (h *Handlers) mergeClaims(ctx context.Context, entityType string, base []mapping.Field) []mapping.Field {
	claims, err := h.repo.ClaimsForEntityType(ctx, entityType)
	if err != nil {
		h.log.Warn("claims for entity type", "entity_type", entityType, "err", err)
		return base
	}
	if len(claims) == 0 {
		return base
	}

	merged := slices.Clone(base)
	for _, c := range claims {
		provider := strings.TrimSpace(c.Provider)
		key := strings.ToLower(strings.TrimSpace(c.FieldKey))
		// Defense in depth: the API rejects these 422, but a stale row must never
		// materialize a source for a canonical or reserved sidecar key.
		if provider == "" || key == "" || strings.HasPrefix(key, model.InternalFieldPrefix) || registry.IsKnown(key) {
			continue
		}
		i := slices.IndexFunc(merged, func(f mapping.Field) bool {
			return strings.EqualFold(f.Canonical, c.Canonical)
		})
		if i < 0 {
			h.log.Warn("claim target field not present; claim is inert",
				"entity_type", entityType, "provider", provider, "field_key", key, "canonical", c.Canonical)
			continue
		}
		src := mapping.Source{Namespace: provider, Key: key}
		if slices.ContainsFunc(merged[i].ParsedSources, func(s mapping.Source) bool {
			return strings.EqualFold(s.Namespace, src.Namespace) && strings.EqualFold(s.Key, src.Key)
		}) {
			continue // already listed (operator YAML said the same thing) — appending would duplicate a candidate
		}
		// Clone before appending: base fields are shared across requests (the parsed
		// mappings store, the synthesized person/studio sets), and append would otherwise
		// write into a backing array another request is reading.
		f := merged[i]
		f.ParsedSources = append(slices.Clone(f.ParsedSources), src)
		f.Sources = rawSources(f.ParsedSources)
		merged[i] = f
	}
	return merged
}

// entityTypeFields returns the effective (post-promotion) field set for an entity *type*,
// with no entity in hand: the picker's target list (FR8/DD2) and the FR4 "target must be
// a field of this type" validation both need the type-level set, not one entity's.
//
// Shadow rows are deliberately not passed to mergePromotions — they only supply a
// promotion's per-entity candidate sources, and neither caller reads sources. What both
// need is which canonicals exist and whether each merges.
func (h *Handlers) entityTypeFields(ctx context.Context, entityType string) []mapping.Field {
	var base []mapping.Field
	switch entityType {
	case model.EnrichEntityPerson:
		base = personFields(nil)
	case model.EnrichEntityStudio:
		base = studioFields(nil)
	case model.EnrichEntityVideo:
		if h.mappings != nil {
			base = h.mappings.Current().Fields()
		}
	}
	fields, _ := h.mergePromotions(ctx, entityType, base, nil)
	return fields
}

// mountFieldClaims registers the owner-gated claimed-provider-key surface (F49, ADR-074).
// Mounted inside the requireOwner group. The type-global /admin path mirrors
// /admin/field-promotions for the same reason: a claim is not tied to the entity the
// owner happens to be viewing, and the URL must not imply per-entity scope.
func (h *Handlers) mountFieldClaims(r chi.Router) {
	r.Get("/admin/field-claims/{entity_type}", h.listFieldClaims)
	r.Put("/admin/field-claims/{entity_type}/{provider}/{field_key}", h.setFieldClaim)
	r.Delete("/admin/field-claims/{entity_type}/{provider}/{field_key}", h.clearFieldClaim)
	r.Get("/admin/field-targets/{entity_type}", h.listFieldTargets)
}

// claimBody is the PUT payload: the canonical field doing the claiming.
type claimBody struct {
	Canonical string `json:"canonical"`
}

// claimView is the GET response shape — the owner tooling's Attached keys list (FR8),
// which is the only durable surface a claim has: a successful claim's whole effect is to
// delete the row that was its own evidence.
type claimView struct {
	Provider  string `json:"provider"`
	FieldKey  string `json:"field_key"`
	Canonical string `json:"canonical"`
}

// targetView is one candidate claim target: what the picker shows (label), what the PUT
// sends (canonical), and whether the field merges — which decides the editor's outcome
// preview, because a claim appends at lowest precedence and an owner attaching to a
// *replace* field would otherwise watch their text disappear (DD2/DD4).
type targetView struct {
	Canonical string `json:"canonical"`
	Label     string `json:"label"`
	Merge     bool   `json:"merge"`
}

// listFieldClaims returns all claims for an entity type, in append order.
func (h *Handlers) listFieldClaims(w http.ResponseWriter, r *http.Request) {
	entityType, ok := parseEntityType(w, r)
	if !ok {
		return
	}
	rows, err := h.repo.ClaimsForEntityType(r.Context(), entityType)
	if err != nil {
		h.fail(w, "list field claims", err)
		return
	}
	out := make([]claimView, 0, len(rows))
	for _, c := range rows {
		out = append(out, claimView{Provider: c.Provider, FieldKey: c.FieldKey, Canonical: c.Canonical})
	}
	writeJSON(w, http.StatusOK, out)
}

// listFieldTargets returns the entity type's effective canonical fields — every field a
// claim may target. It exists because the SPA has no other way to know the type's field
// set, and cannot derive it from the page: ResolveFields drops undecided *empty* fields,
// so a screen-derived picker would omit exactly the target the owner needs (a person's
// empty `bio` is missing precisely when a provider's biography key is the only one on the
// page). Serving the effective set also lets a claim target a promoted field.
func (h *Handlers) listFieldTargets(w http.ResponseWriter, r *http.Request) {
	entityType, ok := parseEntityType(w, r)
	if !ok {
		return
	}
	fields := h.entityTypeFields(r.Context(), entityType)
	out := make([]targetView, 0, len(fields))
	for _, f := range fields {
		label := f.Label
		if label == "" {
			label = registry.Lookup(f.Canonical).Label // tier-4 title-case floor
		}
		out = append(out, targetView{Canonical: f.Canonical, Label: label, Merge: f.Multi || f.Merge})
	}
	writeJSON(w, http.StatusOK, out)
}

// setFieldClaim creates or updates a claim (upsert), clearing any F44 promotion of the
// same key in the same transaction (RD3/D5). DB-only — no enrichment value and no file is
// touched, and the claim manifests only where the provider actually supplied a value.
func (h *Handlers) setFieldClaim(w http.ResponseWriter, r *http.Request) {
	entityType, provider, fieldKey, ok := h.claimTarget(w, r)
	if !ok {
		return
	}
	var body claimBody
	if !decodeJSON(w, r, &body) {
		return
	}
	canonical := strings.ToLower(strings.TrimSpace(body.Canonical))
	if canonical == "" {
		writeError(w, http.StatusBadRequest, "canonical is required")
		return
	}
	// The target must be a field the entity type already declares (FR4). This is the
	// constraint the ADR-074 security deferral rests on: a claim adds a candidate to a
	// declared surface, it can never invent one — including a filterable one.
	if !slices.ContainsFunc(h.entityTypeFields(r.Context(), entityType), func(f mapping.Field) bool {
		return strings.EqualFold(f.Canonical, canonical)
	}) {
		writeError(w, http.StatusUnprocessableEntity, "canonical is not a field of this entity type")
		return
	}
	if err := h.repo.SetClaim(r.Context(), entityType, provider, fieldKey, canonical); err != nil {
		h.fail(w, "set field claim", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearFieldClaim removes a claim, returning the key to F39 auto-registration as its own
// row. Idempotent: clearing a missing claim is a no-op success (204). It does not restore
// a promotion that claiming cleared — that clear is a delete (D5).
func (h *Handlers) clearFieldClaim(w http.ResponseWriter, r *http.Request) {
	entityType, provider, fieldKey, ok := h.claimTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.ClearClaim(r.Context(), entityType, provider, fieldKey); err != nil {
		h.fail(w, "clear field claim", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// claimTarget validates the {entity_type}/{provider}/{field_key} triple shared by the
// write handlers: entity_type must name a known type (400), provider and field_key must
// be present (400), and field_key must be non-reserved (422) and non-canonical (422 — you
// cannot claim `bio`, it already *is* the field that would claim).
//
// field_key is lower-cased (shadow keys are stored and compared lower-cased); provider is
// preserved verbatim, because it is matched against the enrichment namespace exactly, and
// lower-casing it here would silently break a provider registered with capitals.
func (h *Handlers) claimTarget(w http.ResponseWriter, r *http.Request) (entityType, provider, fieldKey string, ok bool) {
	entityType, ok = parseEntityType(w, r)
	if !ok {
		return "", "", "", false
	}
	provider = strings.TrimSpace(chi.URLParam(r, "provider"))
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return "", "", "", false
	}
	fieldKey = strings.ToLower(strings.TrimSpace(chi.URLParam(r, "field_key")))
	if fieldKey == "" {
		writeError(w, http.StatusBadRequest, "field_key is required")
		return "", "", "", false
	}
	if strings.HasPrefix(fieldKey, model.InternalFieldPrefix) {
		writeError(w, http.StatusUnprocessableEntity, "cannot claim a reserved field key")
		return "", "", "", false
	}
	if registry.IsKnown(fieldKey) {
		writeError(w, http.StatusUnprocessableEntity, "cannot claim a canonical field")
		return "", "", "", false
	}
	return entityType, provider, fieldKey, true
}
