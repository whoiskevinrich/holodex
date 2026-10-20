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
