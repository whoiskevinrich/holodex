package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
)

// mountCuration registers the owner-gated value-level curation endpoints (F30,
// ADR-048). Mounted inside the requireOwner group in Mount.
func (h *Handlers) mountCuration(r chi.Router) {
	r.Post("/media/{id}/curation", h.setCuration)
	r.Post("/media/{id}/curation/clear", h.clearCuration)
}

// curationBody is the shared request shape for set/clear. Override bypasses the
// People composite-key collision check (HOLODEX-272) — set only on a resubmit after
// the owner has already seen and dismissed a collision verdict for this exact edit;
// unused (and harmless) for any field other than a person-typed one.
type curationBody struct {
	Field    string `json:"field"`
	Value    string `json:"value"`
	Action   string `json:"action"` // add | suppress | nowrite
	Override bool   `json:"override"`
}

func validCurationAction(a string) bool {
	switch a {
	case repo.CurationAdd, repo.CurationSuppress, repo.CurationNoWrite:
		return true
	}
	return false
}

// validateCurationBody returns "" when the request has a field and a valid action,
// else the owner-facing error message. Shared by set/clear so the contract is stated once.
func validateCurationBody(b curationBody) string {
	if b.Field == "" || !validCurationAction(b.Action) {
		return "field and a valid action (add|suppress|nowrite) are required"
	}
	return ""
}

// setCuration records one value-level decision for a field of a video (F30.2/F30.5a).
// The manual value is sanitized as untrusted input (security condition C4/F30.6b).
func (h *Handlers) setCuration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body curationBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := validateCurationBody(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	value := enrich.SanitizeValue(body.Value)
	if value == "" {
		writeError(w, http.StatusBadRequest, "value required")
		return
	}
	// Confirm the video exists so curation can't accumulate against unknown ids.
	if _, _, err := h.repo.GetVideo(r.Context(), id); err != nil {
		h.videoLookupError(w, err)
		return
	}

	// People composite-key collision gate (HOLODEX-272, reusing HOLODEX-270/271's
	// mechanism): a person-typed field (actors/director) add or suppress changes
	// video_people's composite-key dimension exactly as a Title rename or Studio pick
	// changes theirs, so every non-override add/suppress on a person-typed field is
	// checked. Field-generic via the registry marker (ADR-072 §3), not a hardcoded
	// field-name list. Other curation fields (genres, etc.) aren't part of the
	// composite key and skip this gate entirely (check stays nil, same as override).
	var check func() (*repo.VideoCollision, error)
	if !body.Override && registry.Lookup(body.Field).EntityKind == registry.EntityKindPerson &&
		(body.Action == repo.CurationAdd || body.Action == repo.CurationSuppress) {
		// Recomputed inside check() itself, which SetCurationChecked calls only once
		// it holds writeMu — the current-people read and the collision check it feeds
		// must observe the same locked snapshot, or two concurrent edits to the same
		// video can each pass a stale check before either commits.
		check = func() (*repo.VideoCollision, error) {
			names, err := h.proposedPeopleNames(r.Context(), id, value, body.Field, body.Action)
			if err != nil {
				return nil, err
			}
			return h.repo.FindPeopleCollision(r.Context(), id, names)
		}
	}
	collision, err := h.repo.SetCurationChecked(r.Context(), model.EnrichEntityVideo, id, body.Field, value, body.Action, check)
	if err != nil {
		h.fail(w, "set curation", err)
		return
	}
	if collision != nil {
		writeCollisionConflict(w, collision)
		return
	}
	// Curating an entity-typed field (studio, actors, director) moves its resolved
	// value → re-derive links (F38/F40). Also how the owner-view link picker
	// attaches a person/studio to a video — a link IS a curation add (ADR-072 RD1).
	h.relinkIfEntity(r.Context(), id, body.Field)
	w.WriteHeader(http.StatusNoContent)
}

// proposedPeopleNames computes videoID's resulting linked-people name set after a
// pending person-typed-field curation add or suppress, without persisting anything —
// the People-flavored sibling of resolveProposedStudioNames (decisions.go). Used only
// to feed FindPeopleCollision; the actual write still goes through SetCurationChecked.
// A suppress only drops the current link matching both the field's role (actors →
// 'actor', director → 'director') and the target name — a person linked under both
// roles has two video_people rows sharing a name, and suppressing one role must leave
// the other's link (and its contribution to the collision key) in place.
func (h *Handlers) proposedPeopleNames(ctx context.Context, videoID int64, value, field, action string) ([]string, error) {
	people, err := h.repo.PeopleForVideos(ctx, []int64{videoID})
	if err != nil {
		return nil, err
	}
	current := people[videoID]
	fieldRole := registry.Lookup(field).Role
	names := make([]string, 0, len(current)+1)
	for _, p := range current {
		if action == repo.CurationSuppress && p.Role == fieldRole &&
			strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(value)) {
			continue
		}
		names = append(names, p.Name)
	}
	if action == repo.CurationAdd {
		names = append(names, value)
	}
	return names, nil
}

// clearCuration removes one decision so the underlying source value is restored
// (F30.2e). A clear of a non-existent decision is a no-op success (idempotent).
func (h *Handlers) clearCuration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body curationBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := validateCurationBody(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if _, err := h.repo.ClearCuration(r.Context(), model.EnrichEntityVideo, id, body.Field, body.Value, body.Action); err != nil {
		h.fail(w, "clear curation", err)
		return
	}
	h.relinkIfEntity(r.Context(), id, body.Field)
	w.WriteHeader(http.StatusNoContent)
}
