package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Studio & tag name-identity mutations (F43 S2, ADR-061): owner-gated alias
// add/delete, merge, and rename that mirror the F23 person endpoints exactly, sharing
// one set of generic handlers. Registered inside the requireOwner group; the alias
// lists themselves are served (ungated) as part of GET /studios/{id} and GET /tags.
//
// The person endpoints keep their own handlers (aliases.go / person_decisions.go) —
// their repo layer already delegates to the same generic identity ops, so all three
// entities share the merge/alias/rename implementation at the repo seam (ADR-061).

// identityRoutes captures the per-entity specifics the generic handlers need: the repo
// entity type, a human noun for error/conflict messages, the JSON key wrapping the
// entity in merge responses, the route base, and a hydrator that loads one entity for
// the merge/conflict bodies.
type identityRoutes struct {
	entityType string
	noun       string // "studio" | "tag" — used in error/conflict text
	respKey    string // JSON key for the merged entity ("studio" | "tag")
	base       string // route base ("studios" | "tags")
	get        func(ctx context.Context, id int64) (any, error)
	// writebackField is the canonical embedded-tag field a completed merge of
	// this entity type propagates to (F48.8, ADR-067) — "" means a merge of
	// this type never propagates (tags have no file field; F48.8's scope is
	// Person/Studio only). Routes mergeEntity's propagation decision through
	// this config, the same way every other per-entity specific is routed,
	// rather than a hardcoded entity-type check in the generic handler.
	writebackField string
}

// mountStudioTagIdentity registers the studio and tag alias/merge/rename routes. Person
// is intentionally not routed here — its endpoints predate the generalization and stay
// in place; convergence is at the repo layer (identity_ops.go).
func (h *Handlers) mountStudioTagIdentity(r chi.Router) {
	h.mountEntityIdentity(r, identityRoutes{
		entityType: model.EnrichEntityStudio, noun: "studio", respKey: "studio", base: "studios",
		get:            func(ctx context.Context, id int64) (any, error) { return h.repo.GetStudio(ctx, id) },
		writebackField: "studio",
	})
	h.mountEntityIdentity(r, identityRoutes{
		entityType: model.EntityTag, noun: "tag", respKey: "tag", base: "tags",
		get: func(ctx context.Context, id int64) (any, error) { return h.repo.GetTag(ctx, id) },
	})
}

func (h *Handlers) mountEntityIdentity(r chi.Router, cfg identityRoutes) {
	r.Post("/"+cfg.base+"/{id}/aliases", func(w http.ResponseWriter, r *http.Request) { h.addEntityAlias(w, r, cfg) })
	r.Delete("/"+cfg.base+"/{id}/aliases/{aliasId}", func(w http.ResponseWriter, r *http.Request) { h.deleteEntityAlias(w, r, cfg) })
	r.Post("/"+cfg.base+"/{id}/merge", func(w http.ResponseWriter, r *http.Request) { h.mergeEntity(w, r, cfg) })
	r.Post("/"+cfg.base+"/{id}/rename", func(w http.ResponseWriter, r *http.Request) { h.renameEntity(w, r, cfg) })
}

// addEntityAlias adds an alias and returns the entity's full alias list (mirrors
// addAlias). Idempotent; 409 with the colliding entity when the name already belongs to
// another entity of this type (the homonym rule — never a silent merge).
func (h *Handlers) addEntityAlias(w http.ResponseWriter, r *http.Request, cfg identityRoutes) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Alias string `json:"alias"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	alias := strings.TrimSpace(body.Alias)
	if alias == "" {
		writeError(w, http.StatusBadRequest, "alias is required")
		return
	}
	if len([]rune(alias)) > maxAliasLen {
		writeError(w, http.StatusBadRequest, "alias is too long")
		return
	}
	if err := h.repo.EntityExists(r.Context(), cfg.entityType, id); err != nil {
		h.entityLookupError(w, cfg, err)
		return
	}
	conflictID, found, err := h.repo.EntityConflict(r.Context(), cfg.entityType, id, alias)
	if err != nil {
		h.fail(w, "alias conflict check", err)
		return
	}
	if found {
		h.writeIdentityConflict(w, r, cfg, conflictID)
		return
	}
	if _, err := h.repo.AddEntityAlias(r.Context(), cfg.entityType, id, alias); err != nil {
		h.fail(w, "add alias", err)
		return
	}
	aliases, err := h.repo.AliasesForEntity(r.Context(), cfg.entityType, id)
	if err != nil {
		h.fail(w, "list aliases", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases})
}

// deleteEntityAlias removes one alias (mirrors deleteAlias). 404 when the alias does
// not belong to the entity.
func (h *Handlers) deleteEntityAlias(w http.ResponseWriter, r *http.Request, cfg identityRoutes) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	aliasID, err := strconv.ParseInt(chi.URLParam(r, "aliasId"), 10, 64)
	if err != nil || aliasID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid alias id")
		return
	}
	switch err := h.repo.DeleteEntityAlias(r.Context(), cfg.entityType, id, aliasID); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "alias not found")
		return
	case err != nil:
		h.fail(w, "delete alias", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mergeEntity folds body.from_id into the path entity (mirrors mergePerson): its
// associations move here, its name becomes an alias here, and it is deleted. Returns the
// updated survivor. For studios the registered alias makes the merge survive
// RelinkVideoStudios re-derivation (RD6). A studio merge also propagates to every
// affected video's embedded Studio tag (F48.8b, ADR-067), mirroring mergePerson; tag
// merges have no file field to propagate to (F48.8 scope is Person/Studio only).
func (h *Handlers) mergeEntity(w http.ResponseWriter, r *http.Request, cfg identityRoutes) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		FromID int64 `json:"from_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.FromID <= 0 {
		writeError(w, http.StatusBadRequest, "from_id required")
		return
	}
	if body.FromID == id {
		writeError(w, http.StatusBadRequest, "cannot merge a "+cfg.noun+" into itself")
		return
	}

	// videoIDs is the loser's affected-video list, captured atomically inside
	// MergeEntitiesWithAffectedVideos' own transaction (F48.8b) — not a
	// separate pre-read that could race a concurrent write to the loser's
	// associations between the read and the merge. Tag merges compute this
	// too (cfg.writebackField == "" for tags) but simply never use it below —
	// one shared merge call for every entity type this handler serves.
	videoIDs, err := h.repo.MergeEntitiesWithAffectedVideos(r.Context(), cfg.entityType, id, body.FromID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, cfg.noun+" not found")
		return
	case err != nil:
		h.fail(w, "merge "+cfg.noun, err)
		return
	}
	entity, err := cfg.get(r.Context(), id)
	if err != nil {
		h.fail(w, "get merged "+cfg.noun, err)
		return
	}

	if cfg.writebackField != "" && len(videoIDs) > 0 {
		if names, err := h.repo.StudiosForVideos(r.Context(), videoIDs); err != nil {
			h.log.Warn("merge writeback: load studios for videos", "err", err)
		} else {
			batchID := mergeBatchID(cfg.entityType, id, body.FromID)
			h.propagateMerge(r.Context(), cfg.writebackField, batchID, videoIDs, namesByVideo(names, func(s model.Studio) string { return s.Name }))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{cfg.respKey: entity})
}

// renameEntity sets the entity's name and keeps the old name as an alias (mirrors
// renamePerson). A collision with another entity's name is a 409 carrying that entity
// (the F23 conflict shape) with no mutation. Renaming to the current name is a no-op 204.
func (h *Handlers) renameEntity(w http.ResponseWriter, r *http.Request, cfg identityRoutes) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	// The old name becomes an alias, so it must satisfy the alias bound too.
	if len([]rune(name)) > maxAliasLen {
		writeError(w, http.StatusBadRequest, "name is too long")
		return
	}
	conflictID, err := h.repo.RenameEntity(r.Context(), cfg.entityType, id, name)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, cfg.noun+" not found")
		return
	case errors.Is(err, repo.ErrNameTaken):
		h.writeIdentityConflict(w, r, cfg, conflictID)
		return
	case err != nil:
		h.fail(w, "rename "+cfg.noun, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeIdentityConflict emits the F23-style 409 carrying the colliding entity so the
// owner can choose merge or keep-separate.
func (h *Handlers) writeIdentityConflict(w http.ResponseWriter, r *http.Request, cfg identityRoutes, conflictID int64) {
	entity, err := cfg.get(r.Context(), conflictID)
	if err != nil {
		h.fail(w, "load conflict", err)
		return
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":    "that name already belongs to another " + cfg.noun,
		"conflict": entity,
	})
}

func (h *Handlers) entityLookupError(w http.ResponseWriter, cfg identityRoutes, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, cfg.noun+" not found")
		return
	}
	h.fail(w, "lookup "+cfg.noun, err)
}
