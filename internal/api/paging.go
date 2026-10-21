package api

import (
	"net/url"

	"holodex/internal/repo"
)

// defaultPageLimit is the page size a list read uses when the caller sends no limit,
// the same default GET /media applies.
const defaultPageLimit = 50

// page is a parsed limit/offset pair (HOLODEX-587). pageFromQuery clamps it the way
// ListVideos clamps GET /media's: a missing or non-positive limit is the default, a
// larger one is capped at repo.MaxListLimit, and a negative offset is 0. Responses echo
// the clamped values, so a client sees the page it actually got.
type page struct {
	Limit  int
	Offset int
}

func pageFromQuery(q url.Values) page {
	limit := atoiDefault(q.Get("limit"), defaultPageLimit)
	if limit <= 0 {
		limit = defaultPageLimit
	}
	return page{
		Limit:  min(limit, repo.MaxListLimit),
		Offset: max(atoiDefault(q.Get("offset"), 0), 0),
	}
}

// namedListFilterFromQuery reads the People / Studios / Tags list params (F80): sort,
// the owner's missing_facet, q (search), seed (random), and a page.
func namedListFilterFromQuery(q url.Values) (repo.NamedListFilter, page) {
	p := pageFromQuery(q)
	f := repo.NamedListFilter{
		Sort: q.Get("sort"), MissingFacets: q["missing_facet"], Query: q.Get("q"),
		Limit: p.Limit, Offset: p.Offset,
	}
	if f.Sort == "random" {
		f.Seed = parseSeedOrRandom(q.Get("seed"))
	}
	return f, p
}

// letterExtra adds the A–Z restart offsets as `letters` when the request asks with
// letters=1 (F80 EL6): the client asks once per list, not on every Load more.
func letterExtra(q url.Values, offsets func() (map[string]int, error)) (map[string]any, error) {
	if q.Get("letters") != "1" {
		return nil, nil
	}
	m, err := offsets()
	if err != nil {
		return nil, err
	}
	return map[string]any{"letters": m}, nil
}

// pageOf returns p's window of all: never nil, so an empty page marshals as `[]`
// (HOLODEX-275), and empty rather than a panic when the offset runs past the end.
func pageOf[T any](all []T, p page) []T {
	if p.Offset >= len(all) {
		return []T{}
	}
	return all[p.Offset:min(p.Offset+p.Limit, len(all))]
}

// envelope is GET /media's list shape: the page's items, the total across every page,
// and the clamped limit and offset. extra adds a queue's own fields (per-group counts).
func (p page) envelope(items any, total int, extra map[string]any) map[string]any {
	out := map[string]any{"items": items, "total": total, "limit": p.Limit, "offset": p.Offset}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
