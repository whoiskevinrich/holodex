package api

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// Studio pair evidence (F70 P0-10, HOLODEX-598): one side of a studio pair's compare
// panel, loaded when the panel opens. It says where the studio's name came from — which
// sources carry it on the studio's videos now — and never which spelling is right.

// studioPosterCount is how many sample posters a side shows (F70 OQ7).
const studioPosterCount = 3

// studioNameSource is one source that carries the studio's name on some of its videos
// now: "file" (the file tags) or a provider namespace. One video can count toward two
// sources, so the counts may sum past the studio's video count.
type studioNameSource struct {
	Source     string `json:"source"`
	VideoCount int    `json:"video_count"`
}

// studioPoster is one sample video. Source is the namespace that WON that video's studio
// field — "file", a provider, or "manual" for a curated value — the one place the panel
// shows a winner; "" when the field resolves to nothing.
type studioPoster struct {
	VideoID int64  `json:"video_id"`
	Source  string `json:"source"`
}

// studioEvidence is GET /owner/duplicates/studios/{id}. Aliases carry their source: a
// provider namespace, or none (omitted) for the owner's — typed, or kept by a merge or
// rename, which store no source and so cannot be told apart (F70 OQ6).
type studioEvidence struct {
	ID            int64               `json:"id"`
	Name          string              `json:"name"`
	VideoCount    int                 `json:"video_count"`
	Sources       []studioNameSource  `json:"sources"`
	Posters       []studioPoster      `json:"posters"`
	Aliases       []model.EntityAlias `json:"aliases"`
	ExternalLinks []ExternalLink      `json:"external_links"`
}

// getStudioEvidence serves one side of a studio pair's panel. Owner-gated (mounted with
// the review queue). The external links are best-effort, as on the studio page.
func (h *Handlers) getStudioEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := h.repo.GetStudio(ctx, id)
	if err != nil {
		h.studioLookupError(w, err)
		return
	}
	ids, err := h.repo.VideoIDsForStudio(ctx, id)
	if err != nil {
		h.fail(w, "studio evidence", err)
		return
	}
	ev := studioEvidence{
		ID: id, Name: s.Name, VideoCount: len(ids),
		Sources: []studioNameSource{}, Posters: []studioPoster{}, Aliases: s.Aliases, ExternalLinks: []ExternalLink{},
	}
	if ev.Aliases == nil {
		ev.Aliases = []model.EntityAlias{}
	}
	if links, err := h.externalLinksForEntity(ctx, model.EnrichEntityStudio, id, nil); err != nil {
		h.log.Warn("external links for studio evidence", "id", id, "err", err)
	} else if links != nil {
		ev.ExternalLinks = links
	}
	var field mapping.Field
	mapped := false
	if h.mappings != nil {
		field, mapped = h.mappings.Current().ByCanonical("studio")
	}
	if !mapped || len(ids) == 0 {
		// No studio mapping: no source can be read, so the posters are just the newest.
		for i := len(ids) - 1; i >= 0 && len(ev.Posters) < studioPosterCount; i-- {
			ev.Posters = append(ev.Posters, studioPoster{VideoID: ids[i]})
		}
		writeJSON(w, http.StatusOK, ev)
		return
	}
	layers, err := h.studioFieldLayers(ctx, field, ids)
	if err != nil {
		h.fail(w, "studio evidence", err)
		return
	}
	carriers := h.studioNameCarriers(field, layers, ids, s.Name)
	for _, src := range carriers {
		ev.Sources = append(ev.Sources, studioNameSource{Source: src.ns, VideoCount: len(src.videos)})
	}
	posters := studioPosterPicks(carriers, ids)
	winners, err := h.studioWinners(ctx, field, layers, posters, s.Name)
	if err != nil {
		h.fail(w, "studio evidence", err)
		return
	}
	for _, vid := range posters {
		ev.Posters = append(ev.Posters, studioPoster{VideoID: vid, Source: winners[vid]})
	}
	writeJSON(w, http.StatusOK, ev)
}

// studioLayers is the file and provider data the studio field reads, for a set of videos;
// enrichment is converted once per video, not once per resolve.
type studioLayers struct {
	extra  map[int64][]model.ExtraMetadata
	enrich map[int64]resolver.Enrichment
}

// studioFieldLayers loads only the studio field's own file keys and provider field keys
// for ids, chunked under SQLite's bound-parameter ceiling (as partsFor is), so a studio
// with thousands of videos pays two narrow reads per chunk.
func (h *Handlers) studioFieldLayers(ctx context.Context, field mapping.Field, ids []int64) (studioLayers, error) {
	var fileKeys, providerKeys []string
	for _, s := range field.ParsedSources {
		switch {
		case s.Namespace == "file":
			fileKeys = append(fileKeys, s.Key)
		case !slices.Contains(providerKeys, s.Key):
			providerKeys = append(providerKeys, s.Key)
		}
	}
	out := studioLayers{extra: map[int64][]model.ExtraMetadata{}, enrich: map[int64]resolver.Enrichment{}}
	enrichRows := map[int64][]repo.EnrichmentRow{}
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		part := ids[start:min(start+chunk, len(ids))]
		extra, err := h.repo.ExtraMetadataForVideosByKey(ctx, part, fileKeys)
		if err != nil {
			return out, err
		}
		for id, rows := range extra {
			out.extra[id] = rows
		}
		for _, key := range providerKeys {
			enr, err := h.repo.EnrichmentForVideosField(ctx, part, key)
			if err != nil {
				return out, err
			}
			for id, rows := range enr {
				enrichRows[id] = append(enrichRows[id], rows...)
			}
		}
	}
	for id, rows := range enrichRows {
		out.enrich[id] = enrichmentFromRows(rows)
	}
	return out, nil
}

// studioCarrier is one namespace and the videos (ascending id) on which it carries the
// studio's name.
type studioCarrier struct {
	ns     string
	videos []int64
}

// studioNameCarriers resolves the studio field one namespace at a time for every video,
// through the same pure resolver as the relink (so splitting and casing match), and keeps
// the namespaces whose own values name this studio (its nameKey, lower(trim)). No
// decisions or curation: this is what each source says, not which one wins. Sorted by
// most videos first, then the mapping's source order.
func (h *Handlers) studioNameCarriers(field mapping.Field, layers studioLayers, ids []int64, name string) []studioCarrier {
	var namespaces []string
	for _, s := range field.ParsedSources {
		if !slices.Contains(namespaces, s.Namespace) {
			namespaces = append(namespaces, s.Namespace)
		}
	}
	key := resolver.NormKey(name)
	opts := h.resolveOptions(nil)
	var out []studioCarrier
	for _, ns := range namespaces {
		one := field
		one.ParsedSources = slices.DeleteFunc(slices.Clone(field.ParsedSources), func(s mapping.Source) bool { return s.Namespace != ns })
		c := studioCarrier{ns: ns}
		for _, id := range ids {
			v := model.Video{ID: id}
			resolved := resolver.Resolve(&v, layers.extra[id], layers.enrich[id], nil, []mapping.Field{one}, opts)
			if len(resolved) > 0 && slices.ContainsFunc(resolved[0].Values, func(val string) bool { return resolver.NormKey(val) == key }) {
				c.videos = append(c.videos, id)
			}
		}
		if len(c.videos) > 0 {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b studioCarrier) int { return cmp.Compare(len(b.videos), len(a.videos)) })
	return out
}

// studioPosterPicks chooses the sample videos (F70 OQ7): the newest video of each name
// source first, in the order the sources are listed, then the newest of the rest —
// "newest" being the highest id, so the same side always shows the same posters.
func studioPosterPicks(carriers []studioCarrier, ids []int64) []int64 {
	var picks []int64
	add := func(vid int64) {
		if len(picks) < studioPosterCount && !slices.Contains(picks, vid) {
			picks = append(picks, vid)
		}
	}
	for _, c := range carriers {
		for i := len(c.videos) - 1; i >= 0; i-- {
			if !slices.Contains(picks, c.videos[i]) {
				add(c.videos[i])
				break
			}
		}
	}
	for i := len(ids) - 1; i >= 0 && len(picks) < studioPosterCount; i-- {
		add(ids[i])
	}
	return picks
}

// studioWinners resolves the studio field in full — decisions and curation included — for
// the sampled videos only, and returns each one's winning namespace. A merge-mode studio
// field has no single winner; there it is the first namespace contributing this studio's
// own value ("manual" when the owner added it).
func (h *Handlers) studioWinners(ctx context.Context, field mapping.Field, layers studioLayers, ids []int64, name string) (map[int64]string, error) {
	curation, err := h.repo.CurationForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	decisions, err := h.repo.DecisionsForVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	key := resolver.NormKey(name)
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		v := model.Video{ID: id}
		resolved := resolver.Resolve(&v, layers.extra[id], layers.enrich[id],
			curationFromRows(curation[id]), []mapping.Field{field}, h.resolveOptions(decisionsFromRows(decisions[id])))
		if len(resolved) == 0 {
			continue
		}
		if rf := resolved[0]; rf.WinningSource != "" {
			out[id], _, _ = strings.Cut(rf.WinningSource, ":")
			continue
		}
		for _, item := range resolved[0].Items {
			if resolver.NormKey(item.Value) != key {
				continue
			}
			if item.Manual {
				out[id] = "manual"
			} else if len(item.Sources) > 0 {
				out[id] = item.Sources[0]
			}
			break
		}
	}
	return out, nil
}
