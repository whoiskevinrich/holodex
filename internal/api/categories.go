package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Tag Categories API (HOLODEX-240, ADR-077): owner-gated CRUD + member-tag
// assign/unassign; public list/detail reads, mirroring tags/studios. Category
// never joins the entity-name-identity spine (ADR-077 D1/D4), so its
// mutations route through dedicated repo functions, not the generic
// alias/merge/rename handlers in entity_identity.go.

// mountCategories registers the public category reads.
func (h *Handlers) mountCategories(r chi.Router) {
	r.Get("/categories", h.listCategories)
	r.Get("/categories/{id}", h.getCategory)
}

// mountCategoryMutations registers the owner-gated category CRUD and
// member-tag assign/unassign. Mounted inside the requireOwner group in Mount.
func (h *Handlers) mountCategoryMutations(r chi.Router) {
	r.Post("/categories", h.createCategory)
	r.Post("/categories/{id}/rename", h.renameCategory)
	r.Delete("/categories/{id}", h.deleteCategory)
	r.Post("/categories/{id}/tags", h.assignCategoryTags)
	r.Delete("/categories/{id}/tags", h.unassignCategoryTags)
	// Standalone resolve-or-create-tag (no video attach) -- the first step of
	// the /categories/{id} "+ Add tag" control; the caller assigns the
	// returned id via POST /categories/{id}/tags above. Filed here (not
	// video_tags.go) since categories is its only caller today.
	r.Post("/tags", h.createOrResolveTag)
}

func (h *Handlers) listCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.repo.ListCategories(r.Context())
	if err != nil {
		h.fail(w, "list categories", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": categories})
}

func (h *Handlers) getCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := h.repo.GetCategory(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		h.fail(w, "get category", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

// parseCategoryName decodes and validates a {"name": "..."} body — shared by
// createCategory and renameCategory. ok is false once an error response has
// already been written.
func parseCategoryName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return "", false
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return "", false
	}
	if len([]rune(name)) > maxAliasLen {
		writeError(w, http.StatusBadRequest, "name is too long")
		return "", false
	}
	return name, true
}

// writeCategoryMutationError translates CreateCategory/RenameCategory's error
// set — not found, a collision with a tag or another category (ADR-077 D3),
// or a generic failure — into a response, and reports whether it wrote one.
func (h *Handlers) writeCategoryMutationError(w http.ResponseWriter, op string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "category not found")
	case errors.Is(err, repo.ErrCategoryNameCollidesWithTag):
		writeError(w, http.StatusConflict, "that name already belongs to a tag")
	case errors.Is(err, repo.ErrNameTaken):
		writeError(w, http.StatusConflict, "a category with that name already exists")
	default:
		h.fail(w, op, err)
	}
	return true
}

// createCategory creates a category. 409 when name collides (tag-style fold)
// with an existing tag or another category (ADR-077 D3).
func (h *Handlers) createCategory(w http.ResponseWriter, r *http.Request) {
	name, ok := parseCategoryName(w, r)
	if !ok {
		return
	}
	c, err := h.repo.CreateCategory(r.Context(), name)
	if h.writeCategoryMutationError(w, "create category", err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

// renameCategory sets a category's name. Same collision handling as create.
func (h *Handlers) renameCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	name, ok := parseCategoryName(w, r)
	if !ok {
		return
	}
	c, err := h.repo.RenameCategory(r.Context(), id, name)
	if h.writeCategoryMutationError(w, "rename category", err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

// deleteCategory deletes a category. Cascade-unassigns every member tag via
// ON DELETE CASCADE (ADR-077 D2) — no dependent-tag block.
func (h *Handlers) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	switch err := h.repo.DeleteCategory(r.Context(), id); {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "category not found")
		return
	case err != nil:
		h.fail(w, "delete category", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// categoryTagIDsBody is the shared request shape for bulk assign/unassign —
// one call per action, covering both the single-tag pill ⋯-menu "Add to
// category…" item and the Manage-mode bulk action with the same endpoint.
type categoryTagIDsBody struct {
	TagIDs []int64 `json:"tag_ids"`
}

// categoryTagsMutation is the shared assign/unassign handler shape: decode
// the tag-id body, call the repo mutator (which already returns the updated
// category), and translate ErrNotFound. assignCategoryTags/
// unassignCategoryTags below differ only in which repo method they pass.
func (h *Handlers) categoryTagsMutation(w http.ResponseWriter, r *http.Request, op string,
	mutate func(ctx context.Context, categoryID int64, tagIDs []int64) (*model.Category, error)) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body categoryTagIDsBody
	if !decodeJSON(w, r, &body) {
		return
	}
	c, err := mutate(r.Context(), id, body.TagIDs)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "category not found")
		return
	case err != nil:
		h.fail(w, op, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

func (h *Handlers) assignCategoryTags(w http.ResponseWriter, r *http.Request) {
	h.categoryTagsMutation(w, r, "assign category tags", h.repo.AssignTagsToCategory)
}

func (h *Handlers) unassignCategoryTags(w http.ResponseWriter, r *http.Request) {
	h.categoryTagsMutation(w, r, "unassign category tags", h.repo.UnassignTagsFromCategory)
}

// createOrResolveTag resolves-or-creates a tag by name with no video attach
// (HOLODEX-240) -- mirrors attachVideoTag's error handling minus the video
// link concern (422 denied, 400 too long, 409 collides with a category).
func (h *Handlers) createOrResolveTag(w http.ResponseWriter, r *http.Request) {
	name, ok := parseCategoryName(w, r)
	if !ok {
		return
	}
	tag, err := h.repo.ResolveOrCreateTag(r.Context(), name)
	switch {
	case errors.Is(err, repo.ErrTagDenied):
		writeError(w, http.StatusUnprocessableEntity, "term is on the deny-list")
		return
	case errors.Is(err, repo.ErrTagNameTooLong):
		writeError(w, http.StatusBadRequest, "name is too long")
		return
	case errors.Is(err, repo.ErrTagNameCollidesWithCategory):
		writeError(w, http.StatusConflict, "that name already belongs to a category")
		return
	case err != nil:
		h.fail(w, "create tag", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag})
}
