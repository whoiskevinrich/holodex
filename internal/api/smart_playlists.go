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
	"holodex/internal/query"
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
//
// An F87 `expr` is stored in its canonical form (ids, not names), so a rename never
// changes what it matches (P0-10). The owner writes playlists, so it is parsed as the owner.
func (h *Handlers) canonicalPlaylistQuery(ctx context.Context, q url.Values) (string, error) {
	mapped := h.filterableKeys()
	out := url.Values{}
	for key, vals := range q {
		switch {
		case fetchKeys[key]:
			continue
		case key == "expr":
			v := url.Values{key: vals}.Get(key)
			if strings.TrimSpace(v) == "" {
				continue
			}
			parsed, err := ParseExpr(ctx, h.repo, v, true)
			if err != nil {
				return "", fmt.Errorf("expr: %w", err)
			}
			out.Set(key, parsed.String())
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
	staleOwnerOnly = "owner_only"  // an owner-only input read by a visitor, or on a public playlist
	staleInvalid   = "invalid"     // the stored string doesn't parse (never written by canonicalPlaylistQuery)
)

type staleRef struct {
	Kind  string `json:"kind"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	// Via is the reference in the playlist being read that a problem came through: the
	// one chip the owner can remove. Name and ViaName are the current names, owner only.
	Via     int64  `json:"via,omitempty"`
	Name    string `json:"name,omitempty"`
	ViaName string `json:"via_name,omitempty"`
}

// smartPlaylistFilter turns a smart playlist into the filter its read runs (ADR-121 D3):
// the stored query through browseFilter — the same parse and posture as /media — with
// the playlist's sort. The stale check comes first (D5); then, for a visitor or on a
// public playlist, any owner-only input is reported as a stale ref rather than evaluated
// (D4). The caller has already applied visibility, so a visitor never reaches this for a
// private playlist.
//
// `playlist:` conditions are expanded here too (playlist_refs.go), and the playlist's cap
// is set on the filter, so every read of it — page, count, ids, freeze, a reference —
// sees the capped set (F89 P0-2, P0-3).
func (h *Handlers) smartPlaylistFilter(ctx context.Context, p *model.Playlist, isOwner bool) (repo.VideoFilter, []staleRef, error) {
	return h.smartFilterAt(ctx, p, isOwner, newRefExpansion(p.ID))
}

func (h *Handlers) smartFilterAt(ctx context.Context, p *model.Playlist, isOwner bool, x *refExpansion) (repo.VideoFilter, []staleRef, error) {
	if p.Query == nil {
		return repo.VideoFilter{}, []staleRef{{Kind: staleInvalid}}, nil
	}
	q, err := url.ParseQuery(*p.Query)
	if err != nil {
		return repo.VideoFilter{}, []staleRef{{Kind: staleInvalid}}, nil
	}
	stale := []staleRef{}
	mapped := h.filterableKeys()
	refs := map[string][]int64{}
	var expr *query.Query
	for key, vals := range q {
		switch {
		case repo.PlaylistIDFacets[key] != "":
			refs[key] = parseIDs(vals)
		case scalarFilterKeys[key], key == "missing_facet", mapped[key]:
		case key == "expr":
			// Stored canonical, so it is parsed, never re-canonicalised: an id that no
			// longer exists must be a stale ref, not re-read as a name of those digits.
			if expr, err = query.Parse(q.Get(key)); err != nil {
				return repo.VideoFilter{}, []staleRef{{Kind: staleInvalid, Key: key}}, nil
			}
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
	if p.Cap != nil {
		f.Cap = *p.Cap
	}
	ownerOnly := wantsCompleteness(f.Sort, f.MissingFacets)
	restricted := !isOwner || p.Visibility == model.PlaylistPublic
	if expr != nil {
		if s := x.countConds(expr.Root); s != nil {
			return repo.VideoFilter{}, []staleRef{*s}, nil
		}
		exprRefs := map[string][]int64{}
		for field, ids := range query.EntityIDs(expr.Root) {
			if field == query.FieldPlaylist {
				continue // expanded below, by expandRefs
			}
			exprRefs[repo.ExprFacetKey[field]] = ids
		}
		missing, err := h.repo.MissingFacetIDs(ctx, exprRefs)
		if err != nil {
			return repo.VideoFilter{}, nil, err
		}
		for _, ids := range missing {
			for _, id := range ids {
				stale = append(stale, staleRef{Kind: staleMissing, Key: "expr", Value: strconv.FormatInt(id, 10)})
			}
		}
		refStale, err := h.expandRefs(ctx, x, expr.Root, restricted)
		if err != nil {
			return repo.VideoFilter{}, nil, err
		}
		stale = append(stale, refStale...)
		f.Expr = expr.Root
		f.Refs = x.refs
		ownerOnly = ownerOnly || query.OwnerOnly(expr.Root)
	}
	// A public playlist reports owner_only to every reader, owner included (testing-strategy
	// §23.8 Q5), so the owner sees the notice and fixes it; only a repo-level write gets here.
	if len(stale) == 0 && restricted && ownerOnly {
		stale = append(stale, staleRef{Kind: staleOwnerOnly})
	}
	return f, stale, nil
}

// usesOwnerOnlyInputs reports whether a playlist with this sort and stored query would
// need owner-only inputs to evaluate (ADR-121 D4), so it can't be public.
func usesOwnerOnlyInputs(sort string, stored *string) bool {
	var facets []string
	if stored != nil {
		if q, err := url.ParseQuery(*stored); err == nil {
			facets = q["missing_facet"]
			if src := q.Get("expr"); src != "" {
				if expr, err := query.Parse(src); err == nil && query.OwnerOnly(expr.Root) {
					return true
				}
			}
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
