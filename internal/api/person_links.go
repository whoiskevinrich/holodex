package api

import (
	"context"
	"errors"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Person entity links (F40, ADR-072). video_people is derived from the video's
// RESOLVED person-typed fields (registry.PersonTypedFields — actors, director),
// mirroring RelinkVideoStudios: RelinkVideoPeople is the single resolution entry
// point behind every relink trigger; the repo reconcile (ReconcileVideoPeople) is
// the sole writer of the table.

// relinkPeople re-derives a video's person links, best-effort: a relink failure is
// logged, never failing the user action that triggered it — mirrors relinkStudios.
func (h *Handlers) relinkPeople(ctx context.Context, videoID int64) {
	if err := h.RelinkVideoPeople(ctx, videoID); err != nil {
		h.log.Warn("relink people", "video", videoID, "err", err)
	}
}

// relinkIfEntity relinks only the entity kind the mutated field is marked with
// (registry EntityKind) — so the common case (a title/overview/… decision or
// curation) doesn't pay for a resolve (ADR-053 §4.3's rationale, generalized
// beyond the studio-only field-name check it replaces).
func (h *Handlers) relinkIfEntity(ctx context.Context, videoID int64, canonical string) {
	switch registry.Lookup(canonical).EntityKind {
	case registry.EntityKindStudio:
		h.relinkStudios(ctx, videoID)
	case registry.EntityKindPerson:
		h.relinkPeople(ctx, videoID)
	}
}

// relinkPeopleWithContext reconciles video_people directly from role-tagged links a
// caller already computed (proposedPeopleLinks, curation.go) — the People-flavored
// sibling of relinkStudiosWithContext (studios.go), skipping the fetch-and-resolve
// relinkPeople/RelinkVideoPeople would otherwise repeat immediately afterward for the
// same video and the same pending edit. Still pays for one EnrichmentForEntity fetch
// (extIDByName) that proposedPeopleLinks' PeopleForVideos read doesn't cover — far
// cheaper than a full loadRelinkContext + resolver.Resolve pass (HOLODEX-274). Its
// only caller (setCuration's commit closure) runs inside SetCurationChecked's
// writeMu lock (ADR-084), which is what lets it call ReconcileVideoPeopleLocked
// instead of ReconcileVideoPeople's own re-locking (which would deadlock;
// sync.Mutex isn't reentrant) — do not call this outside that commit callback.
// Best-effort like relinkPeople: a relink failure is logged, never failing the user
// action that triggered it. Self-guards against an unconfigured instance the same
// way relinkVideoPeople does — callers (setCuration) also gate on personFieldMapped
// before reaching here, but this must be safe to call on its own too, or a future
// caller could reintroduce the HOLODEX-256 regression class by skipping that gate.
func (h *Handlers) relinkPeopleWithContext(ctx context.Context, videoID int64, links []repo.PersonRoleName) {
	if !h.anyPersonFieldMapped() {
		return
	}
	enrRows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		h.log.Warn("relink people: enrichment fetch", "video", videoID, "err", err)
		return
	}
	if err := h.repo.ReconcileVideoPeopleLocked(ctx, videoID, links, personExternalIDsFromRows(enrRows)); err != nil {
		h.log.Warn("relink people: reconcile", "video", videoID, "err", err)
	}
}

// personFieldMapped reports whether canonical (assumed person-typed: actors or
// director) is actually mapped in metadata-mappings.yaml. setCuration gates the
// People collision check and the relinkPeopleWithContext fast path on this — an
// unconfigured field has no resolved value to link, so treating it as people-typed
// anyway would write to video_people where relinkVideoPeople's own guard would
// otherwise leave links untouched (HOLODEX-256).
func (h *Handlers) personFieldMapped(canonical string) bool {
	if h.mappings == nil {
		return false
	}
	_, ok := h.mappings.Current().ByCanonical(canonical)
	return ok
}

// anyPersonFieldMapped reports whether at least one person-typed field
// (registry.PersonTypedFields — actors, director) is mapped — the same
// "unconfigured has no opinion" check relinkVideoPeople performs before treating a
// resolve as authoritative (HOLODEX-256), reused here as relinkPeopleWithContext's
// own safety net.
func (h *Handlers) anyPersonFieldMapped() bool {
	for _, def := range registry.PersonTypedFields() {
		if h.personFieldMapped(def.Canonical) {
			return true
		}
	}
	return false
}

// RelinkVideoEntity re-derives every entity-linked table for a video — studio and
// person (ADR-072 RD6: the generalized reconcile every relink trigger wires to,
// via SetRelinker). Both run best-effort/independently: a studio relink failure
// never blocks the people relink or vice versa. Matches the func(context.Context,
// int64) error signature SetRelinker expects (RelinkVideoStudios' shape). Fetches
// the video's resolve inputs once and shares them across both re-derivations
// (loadRelinkContext) rather than each running its own 4-query fetch — this runs
// on every scanned/refreshed video, so halving the round-trips matters here even
// though RelinkVideoStudios/RelinkVideoPeople stay independently fetch-on-call for
// their single-entity dispatch callers (relinkIfEntity).
func (h *Handlers) RelinkVideoEntity(ctx context.Context, videoID int64) error {
	if h.mappings == nil {
		return nil
	}
	rc, err := h.loadRelinkContext(ctx, videoID)
	if err != nil {
		return err
	}
	studioErr := h.relinkVideoStudios(ctx, videoID, rc)
	peopleErr := h.relinkVideoPeople(ctx, videoID, rc)
	return errors.Join(studioErr, peopleErr)
}

// relinkContext bundles the per-video rows RelinkVideoStudios and RelinkVideoPeople
// each resolve against, fetched once by RelinkVideoEntity so a combined
// re-derivation costs 4 queries instead of 8. Nil means the video is
// missing/soft-deleted (loadRelinkContext's ErrNotFound case).
type relinkContext struct {
	video   *model.Video
	extra   []model.ExtraMetadata
	enrRows []repo.EnrichmentRow
	curRows []repo.CurationRow
	decRows []repo.DecisionRow
}

// loadRelinkContext fetches a video's resolve inputs: the video row + its extra
// file-tag metadata, enrichment shadow rows, curation rows, and per-field source
// decisions. Returns (nil, nil) for a missing/soft-deleted video — callers
// reconcile to an empty link set (with prune/orphan-stamp) in that case, same as
// every other not-found path in this file.
func (h *Handlers) loadRelinkContext(ctx context.Context, videoID int64) (*relinkContext, error) {
	v, extra, err := h.repo.GetVideo(ctx, videoID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	enrRows, err := h.repo.EnrichmentForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return nil, err
	}
	curRows, err := h.repo.CurationForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return nil, err
	}
	decRows, err := h.repo.DecisionsForEntity(ctx, model.EnrichEntityVideo, videoID)
	if err != nil {
		return nil, err
	}
	return &relinkContext{video: v, extra: extra, enrRows: enrRows, curRows: curRows, decRows: decRows}, nil
}

// RelinkVideoPeople re-derives a video's person links from its RESOLVED
// person-typed fields (actors, director — registry.PersonTypedFields) and
// reconciles video_people (ADR-072 RD2/RD3). It is the single resolution entry
// point behind every relink trigger; the repo reconcile is the sole writer of the
// table. A missing/soft-deleted video resolves to no names → all links removed +
// orphan-stamped. Safe to call redundantly; idempotent.
func (h *Handlers) RelinkVideoPeople(ctx context.Context, videoID int64) error {
	return h.relinkVideoPeople(ctx, videoID, nil)
}

// relinkVideoPeople is RelinkVideoPeople's implementation, taking an optional
// pre-fetched relinkContext (nil fetches it here) so RelinkVideoEntity can share
// one fetch across both entity kinds.
func (h *Handlers) relinkVideoPeople(ctx context.Context, videoID int64, rc *relinkContext) error {
	if h.mappings == nil {
		return nil
	}
	var fields []mapping.Field
	roleByCanonical := map[string]string{}
	for _, def := range registry.PersonTypedFields() {
		f, ok := h.mappings.Current().ByCanonical(def.Canonical)
		if !ok {
			continue
		}
		fields = append(fields, f)
		roleByCanonical[strings.ToLower(f.Canonical)] = def.Role
	}
	if len(fields) == 0 {
		// No person-typed field is configured. This is NOT the same as "resolved to
		// zero people" — it means metadata-mappings.yaml doesn't map actors/director
		// (yet), so this reconcile has no opinion at all. Leave any existing links
		// untouched rather than treating "unconfigured" as an affirmative empty
		// result and wiping them: that conflation is exactly what wiped video_people
		// for every video on an instance whose config predated F40 (HOLODEX-256).
		return nil
	}

	if rc == nil {
		var err error
		rc, err = h.loadRelinkContext(ctx, videoID)
		if err != nil {
			return err
		}
	}
	if rc == nil {
		return h.repo.ReconcileVideoPeople(ctx, videoID, nil, nil)
	}
	resolved := resolver.Resolve(rc.video, rc.extra, enrichmentFromRows(rc.enrRows), curationFromRows(rc.curRows),
		fields, h.resolveOptions(decisionsFromRows(rc.decRows)))

	var links []repo.PersonRoleName
	for _, rf := range resolved {
		role := roleByCanonical[strings.ToLower(rf.Canonical)]
		for _, name := range rf.Values {
			links = append(links, repo.PersonRoleName{Name: name, Role: role})
		}
	}
	return h.repo.ReconcileVideoPeople(ctx, videoID, links, personExternalIDsFromRows(rc.enrRows))
}

// personExternalIDsFromRows builds a resolved-name → provider external-id side-map
// from a video's internal _person_external_ids sidecar rows (F32, ADR-055) — the
// person analogue of studioExternalIDsFromRows (studios.go). See
// externalIDsFromRows for the shared parse.
func personExternalIDsFromRows(rows []repo.EnrichmentRow) map[string]string {
	return externalIDsFromRows(rows, model.PersonExternalIDsField)
}
