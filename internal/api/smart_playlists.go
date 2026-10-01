package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// Smart playlists (F75, ADR-121): a playlist row with a stored canonical /media query,
// re-evaluated on every read under the reader's own posture. Membership is never stored.

// scalarFilterKeys are the single-valued browse filter keys videoFilterFromQuery reads
// (it takes the first value), and intFilterKeys the subset that must parse as integers.
var (
	scalarFilterKeys = map[string]bool{
		"q": true, "duration_min": true, "duration_max": true,
		"year_min": true, "year_max": true, "resolution": true,
	}
	intFilterKeys = map[string]bool{"duration_min": true, "duration_max": true, "year_min": true, "year_max": true}
	// fetchKeys are fetch mechanics, plus sort, which lives in playlists.sort (D2).
	fetchKeys = map[string]bool{"limit": true, "offset": true, "seed": true, "sort": true}
)

// filterableKeys returns the current filterable mapped canonical keys (F20.5).
func (h *Handlers) filterableKeys() map[string]bool {
	keys := map[string]bool{}
	if h.mappings != nil {
		for _, fld := range h.mappings.Current().Filterable() {
			keys[fld.Canonical] = true
		}
	}
	return keys
}

// canonicalPlaylistQuery is the only writer of playlists.query (ADR-121 D2). It keeps
// the browse filter keys, strips sort and fetch mechanics, and refuses anything else —
// an unknown key, a non-numeric id, an unparsable number or resolution — because the
// parser would silently ignore it and the stored query would mean less than it looks.
// Scalar keys keep their first value, as the parser reads them; the rest is
// normalised by repo.EncodePlaylistQuery so equal sets store equal strings.
func (h *Handlers) canonicalPlaylistQuery(q url.Values) (string, error) {
	mapped := h.filterableKeys()
	out := url.Values{}
	for key, vals := range q {
		switch {
		case fetchKeys[key]:
			continue
		case repo.PlaylistIDFacets[key] != "":
			for _, v := range vals {
				for _, part := range strings.Split(v, ",") {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					n, err := strconv.ParseInt(part, 10, 64)
					if err != nil || n <= 0 {
						return "", fmt.Errorf("%s: %q is not an id", key, part)
					}
					out.Add(key, strconv.FormatInt(n, 10))
				}
			}
		case key == "missing_facet":
			out[key] = vals
		case scalarFilterKeys[key] || mapped[key]:
			v := strings.TrimSpace(url.Values{key: vals}.Get(key)) // the first value, as browse reads it
			if v == "" {
				continue
			}
			if intFilterKeys[key] {
				if _, err := strconv.Atoi(v); err != nil {
					return "", fmt.Errorf("%s: %q is not a number", key, v)
				}
			}
			if key == "resolution" {
				if _, ok := metadata.ParseResolutionBucket(v); !ok {
					return "", fmt.Errorf("resolution: unknown bucket %q", v)
				}
			}
			out.Set(key, v)
		default:
			return "", fmt.Errorf("unknown filter key %q", key)
		}
	}
	return repo.EncodePlaylistQuery(out), nil
}

// parseQueryString parses a filter string as the browse URL carries it (leading ? allowed).
func parseQueryString(s string) (url.Values, error) {
	return url.ParseQuery(strings.TrimPrefix(s, "?"))
}

// Stale reference kinds (ADR-121 D4/D5). A smart playlist with any stale ref is not
// evaluated: dropping the clause would broaden the set, keeping it would empty it for
// a reason the owner can't see.
const (
	staleMissing   = "missing"     // an id facet names an entity that no longer exists
	staleUnknown   = "unknown_key" // a mapped key no longer filterable (mapping reload)
	staleOwnerOnly = "owner_only"  // an owner-only input on a playlist a visitor is reading
	staleInvalid   = "invalid"     // the stored string doesn't parse (never written by canonicalPlaylistQuery)
)

type staleRef struct {
	Kind  string `json:"kind"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// smartPlaylistFilter turns a smart playlist into the filter its read runs (ADR-121 D3):
// the stored query through browseFilter — the same parse and posture as /media — with
// the playlist's sort. The stale check comes first (D5); then, for a visitor, any
// owner-only input is reported as a stale ref rather than evaluated (D4). The caller has
// already applied visibility, so a visitor never reaches this for a private playlist.
func (h *Handlers) smartPlaylistFilter(ctx context.Context, p *model.Playlist, isOwner bool) (repo.VideoFilter, []staleRef, error) {
	q, err := url.ParseQuery(*p.Query)
	if err != nil {
		return repo.VideoFilter{}, []staleRef{{Kind: staleInvalid}}, nil
	}
	stale := []staleRef{}
	mapped := h.filterableKeys()
	refs := map[string][]int64{}
	for key, vals := range q {
		switch {
		case repo.PlaylistIDFacets[key] != "":
			refs[key] = parseIDs(vals)
		case scalarFilterKeys[key], key == "missing_facet", mapped[key]:
		default:
			stale = append(stale, staleRef{Kind: staleUnknown, Key: key, Value: q.Get(key)})
		}
	}
	missing, err := h.repo.MissingFacetIDs(ctx, refs)
	if err != nil {
		return repo.VideoFilter{}, nil, err
	}
	for key, ids := range missing {
		for _, id := range ids {
			stale = append(stale, staleRef{Kind: staleMissing, Key: key, Value: strconv.FormatInt(id, 10)})
		}
	}
	f := h.browseFilter(q)
	f.Sort = p.Sort
	if len(stale) == 0 && !isOwner && wantsCompleteness(f.Sort, f.MissingFacets) {
		stale = append(stale, staleRef{Kind: staleOwnerOnly})
	}
	return f, stale, nil
}

// usesOwnerOnlyInputs reports whether a playlist with this sort and stored query would
// need owner-only inputs to evaluate (ADR-121 D4), so it can't be public.
func usesOwnerOnlyInputs(sort string, query *string) bool {
	var facets []string
	if query != nil {
		if q, err := url.ParseQuery(*query); err == nil {
			facets = q["missing_facet"]
		}
	}
	return wantsCompleteness(sort, facets)
}

// freezePlaylist: POST /playlists/{id}/freeze (spec P0-7). Evaluates the stored query
// once in the playlist's order and makes the result its membership, turning it into an
// ordinary snapshot playlist. A stale query is refused (409, with the refs) — freezing
// it would snapshot a set the owner can't see the reason for.
func (h *Handlers) freezePlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := h.repo.GetPlaylist(r.Context(), id, false)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		h.fail(w, "freeze playlist", err)
		return
	}
	if !p.Smart() {
		writeError(w, http.StatusBadRequest, "not a smart playlist")
		return
	}
	f, stale, err := h.smartPlaylistFilter(r.Context(), p, true)
	if err != nil {
		h.fail(w, "freeze playlist", err)
		return
	}
	if len(stale) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "the stored query has stale references", "stale_refs": stale})
		return
	}
	if f.Sort == "random" {
		f.Seed = parseSeedOrRandom("")
	}
	if wantsCompleteness(f.Sort, f.MissingFacets) {
		h.drainCompleteness(r.Context()) // score before ranking, as listMedia does
	}
	ids, err := h.repo.ListVideoIDs(r.Context(), f)
	if err != nil {
		h.fail(w, "freeze playlist", err)
		return
	}
	err = h.repo.FreezePlaylist(r.Context(), id, ids)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	case errors.Is(err, repo.ErrNotSmart):
		writeError(w, http.StatusBadRequest, "not a smart playlist")
		return
	case err != nil:
		h.fail(w, "freeze playlist", err)
		return
	}
	h.writePlaylist(w, r, id)
}
