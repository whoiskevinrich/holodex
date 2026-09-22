package api

import (
	"context"
	"net/http"

	"holodex/internal/model"
	"holodex/internal/resolver"
)

// PersonCard is the hover-card payload (F68, HOLODEX-431): the recognition cues a
// PersonLinkChip needs to draw its floating card without the detail read's 500
// videos, image set and full field list. Absent facts are absent keys, never
// null strings — the card drops the segment (spec R4/R5).
type PersonCard struct {
	ID              int64  `json:"id"`
	Ref             string `json:"ref"`
	Name            string `json:"name"`
	DisplayName     string `json:"display_name,omitempty"`
	HeadshotVersion int64  `json:"headshot_version,omitempty"`
	VideoCount      int    `json:"video_count"`
	FilmCount       int    `json:"film_count"`
	// Age and AgeAtDeath are the resolver's derived rows (F45, ADR-063) — exactly
	// what the profile shows, mutually exclusive by construction (deriveAge).
	Age           *int           `json:"age,omitempty"`
	AgeAtDeath    *int           `json:"age_at_death,omitempty"`
	Nationality   []string       `json:"nationality,omitempty"`
	Aliases       []string       `json:"aliases,omitempty"`
	ExternalLinks []ExternalLink `json:"external_links,omitempty"`
	// Completeness is the ring badge's owner-only bands (F65.5/F65.8): present
	// only when the requester passes the owner gate, absent (not null) otherwise —
	// the one owner branch in this read (RD12).
	Completeness *model.CompletenessSummary `json:"completeness,omitempty"`
}

// getPersonCard handles GET /people/{id}/card. It reuses personResolve — the
// same pipeline the profile runs — so the card's age, nationality and display
// spelling can never drift from the page; the resolve is cheap (no video list,
// no image set), and the card is fetched once per person per session.
func (h *Handlers) getPersonCard(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	p, err := h.repo.GetPerson(ctx, id)
	if err != nil {
		h.personLookupError(w, err)
		return
	}
	card := PersonCard{ID: p.ID, Ref: model.Ref(model.KindPerson, p.ID), Name: p.Name, VideoCount: p.VideoCount}
	for _, a := range p.Aliases {
		card.Aliases = append(card.Aliases, a.Alias)
	}

	versions, err := h.repo.PersonImageVersions(ctx, []int64{id})
	if err != nil {
		h.log.Warn("image versions for person card", "id", id, "err", err)
	} else {
		card.HeadshotVersion = versions[id][model.PersonImageHeadshot]
	}
	if h.filmsEnabled {
		n, err := h.repo.CountFilmsForPerson(ctx, id)
		if err != nil {
			h.log.Warn("film count for person card", "id", id, "err", err)
		} else {
			card.FilmCount = n
		}
	}

	resolved, _ := h.personResolve(r, id, p)
	for _, f := range resolved {
		switch f.Canonical {
		case "name":
			// The resolved spelling is the display name when a standing decision
			// picks something other than the canonical column (F60 RD9).
			if v := firstValue(f); v != "" && v != p.Name {
				card.DisplayName = v
			}
		case "age":
			card.Age = intValue(f)
		case "age_at_death":
			card.AgeAtDeath = intValue(f)
		case "nationality":
			card.Nationality = f.Values
		}
	}

	links, err := h.externalLinksForEntity(ctx, model.EnrichEntityPerson, id, nil)
	if err != nil {
		h.log.Warn("external links for person card", "id", id, "err", err)
	} else {
		card.ExternalLinks = links
	}

	if h.auth.authorized(r) {
		card.Completeness = h.storedCompleteness(ctx, model.EnrichEntityPerson, id)
	}
	// Private: the owner's copy carries completeness, a visitor's must not be
	// served from a shared cache in its place.
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, http.StatusOK, card)
}

// storedCompleteness reads the ring-badge bands for one entity after draining,
// as a list read would (F65.6); nil when the store has no row.
func (h *Handlers) storedCompleteness(ctx context.Context, entityType string, id int64) *model.CompletenessSummary {
	h.drainCompleteness(ctx)
	stored, err := h.repo.CompletenessForEntities(ctx, entityType, []int64{id})
	if err != nil {
		h.log.Warn("completeness for card", "type", entityType, "id", id, "err", err)
		return nil
	}
	if c, ok := stored[id]; ok {
		return &c
	}
	return nil
}

func firstValue(f resolver.ResolvedField) string {
	if len(f.Values) == 0 {
		return ""
	}
	return f.Values[0]
}

// intValue parses a derived row's single value; a derived row that exists is
// always an integer string (deriveAge), so a parse failure means "no row".
func intValue(f resolver.ResolvedField) *int {
	v := firstValue(f)
	if v == "" {
		return nil
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return nil
		}
		n = n*10 + int(c-'0')
	}
	return &n
}
