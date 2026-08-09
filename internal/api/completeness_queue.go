package api

import (
	"context"
	"net/http"
	"sort"
	"sync"

	"holodex/internal/model"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// QueueRow is one (entity, missing facet) pair in the remediation queue
// (F55.7, design handoff docs/design/entity-completeness-handoff.md §1 DD2) —
// carries enough entity data for the frontend to build a thumbnail/link
// without a second fetch. Exactly one of ThumbnailURL/HeadshotVersion/IconURL
// is populated, matching EntityType.
type QueueRow struct {
	EntityType      string `json:"entity_type"` // model.EnrichEntityVideo | Person | Studio
	EntityID        int64  `json:"entity_id"`
	Name            string `json:"name"`
	ThumbnailURL    string `json:"thumbnail_url,omitempty"`    // video
	HeadshotVersion int64  `json:"headshot_version,omitempty"` // person
	IconURL         string `json:"icon_url,omitempty"`         // studio
	// Provider is the candidate's namespace (e.g. "tmdb") — set only on
	// candidate-ready rows, mirroring FacetScore.Provider.
	Provider string `json:"provider,omitempty"`
}

// FacetGroup is one missing-facet group in the remediation queue, pre-split
// into candidate-ready and needs-research rows (DD1/DD3) so the frontend
// renders the queue with zero client-side grouping logic.
type FacetGroup struct {
	Canonical      string     `json:"canonical"`
	Label          string     `json:"label"`
	Criticality    string     `json:"criticality"`
	CandidateReady []QueueRow `json:"candidate_ready"`
	NeedsResearch  []QueueRow `json:"needs_research"`
}

func (g FacetGroup) count() int { return len(g.CandidateReady) + len(g.NeedsResearch) }

// remediationQueue builds the F55.7 remediation queue: every video/person/
// studio's missing (non-not-applicable) scored facets, grouped by facet and
// split candidate-ready vs needs-research. Reads the same completenessFor*
// pass the browse completeness sort/filter and the Missing-facet chip use
// (F55.6's "one backend predicate" requirement) — this just reshapes
// Completeness.Facets by facet instead of by entity, across the whole
// library rather than the caller's current browse filters, since the queue
// is a standalone remediation surface, not a filtered view of one list.
func (h *Handlers) remediationQueue(ctx context.Context) ([]FacetGroup, error) {
	// The three scans are independent, read-only queries over disjoint tables
	// (SQLite WAL reads don't serialize under writeMu) and this endpoint scans
	// the whole library, not a filtered view — run them concurrently rather
	// than paying the sum of three full scans.
	var (
		videos                           []VideoCompleteness
		people                           []PersonCompleteness
		studios                          []StudioCompleteness
		errVideos, errPeople, errStudios error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		videos, errVideos = h.completenessForVideos(ctx, repo.VideoFilter{})
	}()
	go func() {
		defer wg.Done()
		people, errPeople = h.completenessForPeople(ctx)
	}()
	go func() {
		defer wg.Done()
		studios, errStudios = h.completenessForStudios(ctx)
	}()
	wg.Wait()
	if errVideos != nil {
		return nil, errVideos
	}
	if errPeople != nil {
		return nil, errPeople
	}
	if errStudios != nil {
		return nil, errStudios
	}

	groups := make(map[string]*FacetGroup)
	var order []string
	// addRow skips resolved/not-applicable facets and files everything else
	// into its facet group, split candidate-ready vs needs-research.
	addRow := func(f resolver.FacetScore, row QueueRow) {
		if f.Tier != resolver.TierMissing || f.NotApplicable {
			return
		}
		g, ok := groups[f.Canonical]
		if !ok {
			g = &FacetGroup{
				Canonical:      f.Canonical,
				Label:          f.Label,
				Criticality:    f.Criticality,
				CandidateReady: []QueueRow{},
				NeedsResearch:  []QueueRow{},
			}
			groups[f.Canonical] = g
			order = append(order, f.Canonical)
		}
		if f.Actionable {
			row.Provider = f.Provider
			g.CandidateReady = append(g.CandidateReady, row)
		} else {
			g.NeedsResearch = append(g.NeedsResearch, row)
		}
	}

	for _, vc := range videos {
		for _, f := range vc.Completeness.Facets {
			addRow(f, QueueRow{
				EntityType:   model.EnrichEntityVideo,
				EntityID:     vc.Video.ID,
				Name:         vc.Video.Title,
				ThumbnailURL: vc.Video.ThumbnailURL,
			})
		}
	}
	for _, pc := range people {
		for _, f := range pc.Completeness.Facets {
			addRow(f, QueueRow{
				EntityType:      model.EnrichEntityPerson,
				EntityID:        pc.Person.ID,
				Name:            pc.Person.Name,
				HeadshotVersion: pc.Person.HeadshotVersion,
			})
		}
	}
	for _, sc := range studios {
		for _, f := range sc.Completeness.Facets {
			addRow(f, QueueRow{
				EntityType: model.EnrichEntityStudio,
				EntityID:   sc.Studio.ID,
				Name:       sc.Studio.Name,
				IconURL:    sc.Studio.IconURL,
			})
		}
	}

	out := make([]FacetGroup, len(order))
	for i, c := range order {
		g := groups[c]
		sortRowsByName(g.CandidateReady)
		sortRowsByName(g.NeedsResearch)
		out[i] = *g
	}
	sortFacetGroups(out)
	return out, nil
}

func sortRowsByName(rows []QueueRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
}

// sortFacetGroups orders groups critical-first, then larger groups first
// within the same criticality (design handoff §1 DD1) — mirrors
// ExtractionQueueRow's "most pending fields sort first, clears the most
// backlog per click" idiom, applied to which kind of gap matters most rather
// than which entity has the most gaps. Stable, so groups tied on both keys
// keep first-seen order.
func sortFacetGroups(groups []FacetGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		ci, cj := groups[i].Criticality == registry.CriticalityCritical, groups[j].Criticality == registry.CriticalityCritical
		if ci != cj {
			return ci
		}
		return groups[i].count() > groups[j].count()
	})
}

// completenessQueue handles GET /owner/completeness-queue (F55.7): the
// facet-first remediation queue. Owner-gated: mounted in the requireOwner
// group (Mount), same as extraction-queue.
func (h *Handlers) completenessQueue(w http.ResponseWriter, r *http.Request) {
	groups, err := h.remediationQueue(r.Context())
	if err != nil {
		h.fail(w, "completeness queue", err)
		return
	}
	if groups == nil {
		groups = []FacetGroup{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}
