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
	// composite key and skip this gate entirely (isPeopleField stays false, same as
	// a non-person field). Also requires the field be mapped in
	// metadata-mappings.yaml (personFieldMapped): an unconfigured field has no
	// resolved value to link, so treating it as people-typed anyway would route
	// through relinkPeopleWithContext and write video_people where the guarded slow
	// path (relinkVideoPeople) would no-op — the exact conflation HOLODEX-256 fixed.
	isPeopleField := registry.Lookup(body.Field).EntityKind == registry.EntityKindPerson &&
		(body.Action == repo.CurationAdd || body.Action == repo.CurationSuppress) &&
		h.personFieldMapped(body.Field)
	var links []repo.PersonRoleName
	var check func() (*repo.VideoCollision, error)
	var commit func()
	if isPeopleField {
		// Recomputed inside check() itself, which SetCurationChecked calls only once
		// it holds writeMu — the current-people read and the collision check it feeds
		// must observe the same locked snapshot, or two concurrent edits to the same
		// video can each pass a stale check before either commits. check() always runs
		// (even under Override, which only skips the collision query below) so links
		// is populated from that same locked snapshot for commit (below) to relink
		// against — reusing this read instead of paying for a second
		// loadRelinkContext + resolver.Resolve pass immediately afterward (HOLODEX-274).
		check = func() (*repo.VideoCollision, error) {
			var err error
			links, err = h.proposedPeopleLinks(r.Context(), id, value, body.Field, body.Action)
			if err != nil {
				return nil, err
			}
			if body.Override {
				return nil, nil
			}
			names := make([]string, len(links))
			for i, l := range links {
				names[i] = l.Name
			}
			return h.repo.FindPeopleCollision(r.Context(), id, names)
		}
		// Runs under the same writeMu lock as check() and the curation write, right
		// after the write succeeds (ADR-084) — closing the HOLODEX-277 race where two
		// concurrent edits to different person-typed fields could each relink from a
		// snapshot captured before the other's write landed. Best-effort like
		// relinkPeopleWithContext always was: a relink failure must never fail the
		// owner's curation write, so it's logged here and swallowed, not returned.
		commit = func() {
			h.relinkPeopleWithContext(r.Context(), id, links)
		}
	}
	collision, err := h.repo.SetCurationChecked(r.Context(), model.EnrichEntityVideo, id, body.Field, value, body.Action, check, commit)
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
	// People already relinked inside commit (above), under the lock; only the
	// non-people entity path (studio) still relinks here, unlocked (ADR-084 non-goal).
	if !isPeopleField {
		h.relinkIfEntity(r.Context(), id, body.Field)
	}
	w.WriteHeader(http.StatusNoContent)
}

// proposedPeopleLinks computes videoID's resulting role-tagged people links (both
// actors and director, per registry.PersonTypedFields — not just the field being
// edited) after a pending person-typed-field curation add or suppress, without
// persisting anything — the People-flavored sibling of resolveProposedStudioNames
// (decisions.go). Returns the full ReconcileVideoPeople-ready link set, so the caller
// can both derive the flat name list FindPeopleCollision checks and commit the
// pending write's resulting link state directly (relinkPeopleWithContext) instead of
// a fresh resolve pass (HOLODEX-274). A suppress only drops the current link matching
// both the field's role (actors → 'actor', director → 'director') and the target
// name — a person linked under both roles has two video_people rows sharing a name,
// and suppressing one role must leave the other's link (and its contribution to the
// collision key) in place.
func (h *Handlers) proposedPeopleLinks(ctx context.Context, videoID int64, value, field, action string) ([]repo.PersonRoleName, error) {
	people, err := h.repo.PeopleForVideos(ctx, []int64{videoID})
	if err != nil {
		return nil, err
	}
	current := people[videoID]
	fieldRole := registry.Lookup(field).Role
	links := make([]repo.PersonRoleName, 0, len(current)+1)
	for _, p := range current {
		if action == repo.CurationSuppress && p.Role == fieldRole &&
			strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(value)) {
			continue
		}
		links = append(links, repo.PersonRoleName{Name: p.Name, Role: p.Role})
	}
	if action == repo.CurationAdd {
		links = append(links, repo.PersonRoleName{Name: value, Role: fieldRole})
	}
	return links, nil
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
