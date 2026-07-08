package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// In-app field promotion (F44, ADR-062). An owner promotes a non-canonical, F39
// auto-registered shadow field into a first-class curatable field by writing a
// field_promotions row (presentation only: label / render / group / order). The
// resolver consults that store as a new tier-0 — above operator metadata-mappings.yaml
// (D3) — by materializing each promotion into a synthetic mapping.Field merged into the
// entity's []mapping.Field *before* ResolveFields runs, so the field gains the full
// F36 source-decision + F30 curation machinery via the existing code paths, on all
// three entity types, with zero YAML editing.

// promotableEntityTypes is the closed set a promotion may target (the enrich entity
// vocabulary). person/studio have no operator-YAML remap surface at all, which is the
// whole point of the DB-backed store (ADR-062 D1).
var promotableEntityTypes = []string{model.EnrichEntityVideo, model.EnrichEntityPerson, model.EnrichEntityStudio}

// mergePromotions materializes the entity type's promotions into synthetic
// []mapping.Field merged over the base fields, and reports the set of promoted keys so
// the caller can flag them after resolve. It is the shared tier-0 seam for
// video/person/studio: each already builds its own base []mapping.Field and has the
// entity's shadow rows in hand, so this stays pure (no new I/O in the merge core) — the
// one query is the small per-type promotion set.
//
//   - base:   the entity's canonical/synthesized/YAML fields.
//   - rows:   the entity's shadow rows, used to derive each promotion's candidate
//     sources per-entity from provider provenance (D-candidate).
//
// A promotion whose key no present provider supplies simply contributes an empty field
// that ResolveFields drops for that entity — the promotion is global, but only manifests
// where the key has a value.
func (h *Handlers) mergePromotions(ctx context.Context, entityType string, base []mapping.Field, rows []repo.EnrichmentRow) (merged []mapping.Field, promoted map[string]bool) {
	promos, err := h.repo.PromotionsForEntityType(ctx, entityType)
	if err != nil {
		h.log.Warn("promotions for entity type", "entity_type", entityType, "err", err)
		return base, nil
	}
	if len(promos) == 0 {
		return base, nil
	}

	// Tier-3 provider hints, for the empty-column inherit fold.
	var hints map[string]map[string]repo.ProviderFieldHint
	if h.enrich != nil {
		hints = h.enrich.FieldHints(ctx)
	}

	// synthField pairs the materialized field with the (folded) group/order used to
	// position it — group/order aren't carried on mapping.Field, so ordering is applied
	// here before the merge, mirroring F39's (group rank, order, key) banding.
	type synthField struct {
		field mapping.Field
		group string
		order int
	}

	promoted = make(map[string]bool, len(promos))
	synth := make([]synthField, 0, len(promos))
	for _, p := range promos {
		key := strings.ToLower(strings.TrimSpace(p.FieldKey))
		// Defense in depth: the API rejects a canonical / reserved key 422, but never
		// materialize one even if a stale row somehow exists — the schema contract and
		// reserved sidecar keys stay inviolate (ADR-062 Security).
		if key == "" || strings.HasPrefix(key, model.InternalFieldPrefix) || registry.IsKnown(key) {
			continue
		}

		// Candidate sources derived per-entity from shadow provenance (D-candidate): one
		// provider:<ns> per namespace that supplied a non-empty value for this key. The
		// resolver adds the baseline candidate itself; manual is always available.
		var sources []mapping.Source
		seen := map[string]bool{}
		for _, row := range rows {
			if row.Provider == "" || seen[row.Provider] || !strings.EqualFold(row.FieldKey, key) || !hasNonEmpty(row.Values) {
				continue
			}
			seen[row.Provider] = true
			sources = append(sources, mapping.Source{Namespace: row.Provider, Key: key})
		}

		// Tier-0 → tier-3 → tier-4 fold: the promotion's non-empty columns win; empty
		// columns fall through to the first present provider's hint, then to the registry
		// title-case floor (left "" here so ResolveFields applies it).
		label := p.Label
		render := registry.NormalizeDisplay(p.Render)
		group := ""
		if strings.TrimSpace(p.Group) != "" {
			group = registry.NormalizeGroup(p.Group)
		}
		if (label == "" || p.Render == "" || group == "") && len(sources) > 0 {
			if hint, ok := lookupHint(hints, sources[0].Namespace, key); ok {
				if label == "" {
					label = hint.Label
				}
				if p.Render == "" {
					render = registry.NormalizeDisplay(hint.Render)
				}
				if group == "" {
					group = registry.NormalizeGroup(hint.Group)
				}
			}
		}
		if group == "" {
			group = registry.GroupExtended
		}

		raw := make([]string, len(sources))
		for i, s := range sources {
			raw[i] = s.Namespace + ":" + s.Key
		}
		synth = append(synth, synthField{
			field: mapping.Field{
				Canonical:     key,
				Label:         label,
				Display:       render,
				Sources:       raw,
				ParsedSources: sources,
				Multi:         render == registry.DisplayChips, // chips ⇒ merge field (D-candidate)
				Filterable:    false,                           // D-filterable: never a browse facet in v1
			},
			group: group,
			order: p.Order,
		})
		promoted[key] = true
	}
	if len(synth) == 0 {
		return base, promoted
	}

	// Position the promoted fields among themselves by (group rank, order, key).
	slices.SortStableFunc(synth, func(a, b synthField) int {
		if d := registry.GroupRank(a.group) - registry.GroupRank(b.group); d != 0 {
			return d
		}
		if d := a.order - b.order; d != 0 {
			return d
		}
		return strings.Compare(a.field.Canonical, b.field.Canonical)
	})

	// Replace-or-append by canonical: a promoted key that also has a base (YAML) mapping
	// replaces it in place (promotion wins the collision — D3, rendered once); otherwise
	// it appends after the base fields.
	merged = slices.Clone(base)
	for _, sf := range synth {
		if i := slices.IndexFunc(merged, func(f mapping.Field) bool {
			return strings.EqualFold(f.Canonical, sf.field.Canonical)
		}); i >= 0 {
			merged[i] = sf.field
		} else {
			merged = append(merged, sf.field)
		}
	}
	return merged, promoted
}

// markPromoted stamps ResolvedField.Promoted on the keys materialized from a promotion
// so the SPA offers the owner-only Edit / Remove-promotion affordance on exactly those
// rows (F44), and re-applies the ADR-039/056 image gate to a promoted image_url — a
// promoted field is a mapped field and so skips the auto-register gate, but promotion
// must not bypass the image perimeter (FR6). A no-op when nothing was promoted.
func (h *Handlers) markPromoted(fields []resolver.ResolvedField, promoted map[string]bool) {
	if len(promoted) == 0 {
		return
	}
	for i := range fields {
		f := &fields[i]
		if !promoted[strings.ToLower(strings.TrimSpace(f.Canonical))] {
			continue
		}
		f.Promoted = true
		// A promoted image_url is a mapped field and so skips the auto-register gate;
		// re-apply the shared allowlist gate so promotion never bypasses the perimeter.
		f.Display = h.gateImageURL(f.Display, f.WinningSource, f.Values)
	}
}

// gateImageURL enforces the ADR-039/056 asset-host allowlist on an image_url render:
// a value whose host is not on the supplying provider's allowlist degrades to text (no
// broken <img>, no error). It is the single definition shared by the F39
// auto-registration (appendAutoRegistered) and F44 promotion (markPromoted) paths so the
// image perimeter never drifts between them. Non-image displays pass through unchanged.
func (h *Handlers) gateImageURL(display, winningSource string, values []string) string {
	if display != registry.DisplayImageURL {
		return display
	}
	provider, _, _ := strings.Cut(winningSource, ":")
	if len(values) == 0 || h.enrich == nil || !h.enrich.ImageURLAllowed(provider, values[0]) {
		return registry.DisplayText
	}
	return display
}

// lookupHint returns the provider hint for a (provider, key), ok=false when none — the
// tier-3 lookup for the promotion inherit fold.
func lookupHint(hints map[string]map[string]repo.ProviderFieldHint, provider, key string) (repo.ProviderFieldHint, bool) {
	if byKey, ok := hints[provider]; ok {
		if ph, ok := byKey[key]; ok {
			return ph, true
		}
	}
	return repo.ProviderFieldHint{}, false
}

// hasNonEmpty reports whether any value is non-blank after trimming.
func hasNonEmpty(vals []string) bool {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// mountFieldPromotions registers the owner-gated in-app field-promotion surface (F44,
// ADR-062). Mounted inside the requireOwner group. A dedicated /admin/field-promotions
// path (not /{entity}/{id}/fields/...) is used because a promotion is type-global, not
// tied to the entity the owner is viewing — the URL must not imply per-entity scope.
func (h *Handlers) mountFieldPromotions(r chi.Router) {
	r.Get("/admin/field-promotions/{entity_type}", h.listFieldPromotions)
	r.Put("/admin/field-promotions/{entity_type}/{field_key}", h.setFieldPromotion)
	r.Delete("/admin/field-promotions/{entity_type}/{field_key}", h.clearFieldPromotion)
}

// promotionBody is the create/update payload. All fields are optional; an omitted
// column decodes to its zero value, which stores empty and inherits from the lower tiers
// (provider hint → title-case). SetPromotion is a full upsert, so there is no
// omitted-vs-empty distinction to preserve.
type promotionBody struct {
	Label  string `json:"label"`
	Render string `json:"render"`
	Group  string `json:"group"`
	Order  int    `json:"order"`
}

// promotionView is the GET response shape (owner tooling / debug).
type promotionView struct {
	FieldKey string `json:"field_key"`
	Label    string `json:"label,omitempty"`
	Render   string `json:"render,omitempty"`
	Group    string `json:"group,omitempty"`
	Order    int    `json:"order,omitempty"`
}

// setFieldPromotion creates or updates a promotion (upsert). label is sanitized and
// capped, render/group coerced to the F39 vocabulary; the key must be non-canonical and
// non-reserved (ADR-062 Mechanism 5). DB-only — no enrichment value or file is touched.
func (h *Handlers) setFieldPromotion(w http.ResponseWriter, r *http.Request) {
	entityType, fieldKey, ok := h.promotionTarget(w, r)
	if !ok {
		return
	}
	var body promotionBody
	if !decodeJSON(w, r, &body) {
		return
	}
	label := enrich.SanitizeFieldLabel(body.Label)
	render := registry.NormalizeDisplay(body.Render)
	group := ""
	if strings.TrimSpace(body.Group) != "" {
		group = registry.NormalizeGroup(body.Group)
	}
	if err := h.repo.SetPromotion(r.Context(), entityType, fieldKey, label, render, group, body.Order); err != nil {
		h.fail(w, "set field promotion", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearFieldPromotion de-promotes a field (delete). Idempotent: a de-promote of a
// missing row is a no-op success (204). The shadow value and any prior decisions/
// curation are untouched and re-apply on re-promotion (D-reversible).
func (h *Handlers) clearFieldPromotion(w http.ResponseWriter, r *http.Request) {
	entityType, fieldKey, ok := h.promotionTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.repo.ClearPromotion(r.Context(), entityType, fieldKey); err != nil {
		h.fail(w, "clear field promotion", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listFieldPromotions returns all promotions for an entity type (owner tooling / debug).
func (h *Handlers) listFieldPromotions(w http.ResponseWriter, r *http.Request) {
	entityType := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "entity_type")))
	if !slices.Contains(promotableEntityTypes, entityType) {
		writeError(w, http.StatusBadRequest, "entity_type must be one of video, person, studio")
		return
	}
	rows, err := h.repo.PromotionsForEntityType(r.Context(), entityType)
	if err != nil {
		h.fail(w, "list field promotions", err)
		return
	}
	out := make([]promotionView, 0, len(rows))
	for _, p := range rows {
		out = append(out, promotionView{FieldKey: p.FieldKey, Label: p.Label, Render: p.Render, Group: p.Group, Order: p.Order})
	}
	writeJSON(w, http.StatusOK, out)
}

// promotionTarget validates the {entity_type}/{field_key} pair shared by the write
// handlers: entity_type must name a known type (400), field_key must be non-canonical
// (422 — the registry owns canonical keys, you cannot promote `bio`) and non-reserved
// (422 — `_`-prefixed sidecar keys never display).
func (h *Handlers) promotionTarget(w http.ResponseWriter, r *http.Request) (entityType, fieldKey string, ok bool) {
	entityType = strings.ToLower(strings.TrimSpace(chi.URLParam(r, "entity_type")))
	if !slices.Contains(promotableEntityTypes, entityType) {
		writeError(w, http.StatusBadRequest, "entity_type must be one of video, person, studio")
		return "", "", false
	}
	fieldKey = strings.ToLower(strings.TrimSpace(chi.URLParam(r, "field_key")))
	if fieldKey == "" {
		writeError(w, http.StatusBadRequest, "field_key is required")
		return "", "", false
	}
	if strings.HasPrefix(fieldKey, model.InternalFieldPrefix) {
		writeError(w, http.StatusUnprocessableEntity, "cannot promote a reserved field key")
		return "", "", false
	}
	if registry.IsKnown(fieldKey) {
		writeError(w, http.StatusUnprocessableEntity, "cannot promote a canonical field")
		return "", "", false
	}
	return entityType, fieldKey, true
}
