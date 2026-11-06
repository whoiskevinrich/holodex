package api

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"holodex/internal/extract"
	"holodex/internal/query"
	"holodex/internal/repo"
)

// F87 query expressions (query-language.md): the `expr` filter key, parsed and
// canonicalised once here and compiled in repo (browse-and-list-state.md, "Boolean
// queries"); every entry point runs the one owner check in internal/query
// (security-perimeter.md, "Query expressions").

// QueryResolver implements query.Resolver over the repository. The MCP server shares it.
// PublicOnly is set for a visitor: a private playlist then binds as an unknown name, never
// a refusal, so a reference can't confirm it exists (F89 SEC-1).
type QueryResolver struct {
	Repo       *repo.Repo
	PublicOnly bool
}

func (r QueryResolver) Lookup(ctx context.Context, kind, name string) ([]query.Candidate, error) {
	if kind == query.FieldPlaylist {
		return r.Repo.QueryLookupPlaylists(ctx, name, r.PublicOnly)
	}
	return r.Repo.QueryLookup(ctx, kind, name)
}

func (r QueryResolver) NameOf(ctx context.Context, kind string, id int64) (string, bool, error) {
	if kind == query.FieldPlaylist {
		return r.Repo.QueryPlaylistName(ctx, id, r.PublicOnly)
	}
	return r.Repo.QueryNameOf(ctx, kind, id)
}

func (r QueryResolver) Providers(ctx context.Context) ([]string, error) {
	return r.Repo.QueryProviders(ctx)
}

// suggestMinScore is the Jaro-Winkler floor for a "did you mean" name; a name that
// contains the typed text, or is contained in it, is offered whatever its score.
const suggestMinScore = 0.85

// Suggest ranks every name and alias of kind against an unknown name (P0-7).
func (r QueryResolver) Suggest(ctx context.Context, kind, name string, n int) ([]string, error) {
	var pool []string
	var err error
	if kind == query.FieldPlaylist {
		pool, err = r.Repo.QueryPlaylistNames(ctx, r.PublicOnly)
	} else {
		pool, err = r.Repo.QueryNames(ctx, kind)
	}
	if err != nil {
		return nil, err
	}
	typed := strings.ToLower(strings.Join(strings.Fields(name), " "))
	type scored struct {
		name  string
		score float64
	}
	var hits []scored
	for _, cand := range pool {
		lc := strings.ToLower(cand)
		score := extract.JaroWinkler(typed, lc)
		if typed != "" && (strings.Contains(lc, typed) || strings.Contains(typed, lc)) {
			score = max(score, suggestMinScore)
		}
		if score >= suggestMinScore {
			hits = append(hits, scored{cand, score})
		}
	}
	slices.SortFunc(hits, func(a, b scored) int {
		return cmp.Or(cmp.Compare(b.score, a.score), strings.Compare(a.name, b.name))
	})
	out := []string{}
	for _, h := range hits[:min(n, len(hits))] {
		out = append(out, h.name)
	}
	return out, nil
}

// ParseExpr is the whole path from expr text to a runnable query, shared by every entry
// point: parse (the caps refuse first), the owner check (before any lookup), then
// canonicalisation. A refused query is a *query.Error.
func ParseExpr(ctx context.Context, r *repo.Repo, src string, isOwner bool) (*query.Query, error) {
	q, err := query.Parse(src)
	if err != nil {
		return nil, err
	}
	if err := q.CheckOwner(isOwner); err != nil {
		return nil, err
	}
	if err := q.Canonicalize(ctx, QueryResolver{Repo: r, PublicOnly: !isOwner}); err != nil {
		return nil, err
	}
	return q, nil
}

// writeQueryError writes a refused query: 401 for an owner-only condition, as for
// today's owner-only browse inputs, 400 otherwise, each with the error's offsets for
// the SPA's error echo. A non-query error is a 500.
func (h *Handlers) writeQueryError(w http.ResponseWriter, op string, err error) {
	var qe *query.Error
	if !errors.As(err, &qe) {
		h.fail(w, op, err)
		return
	}
	status := http.StatusBadRequest
	if qe.Code == query.CodeOwnerOnly {
		status = http.StatusUnauthorized
	}
	writeJSON(w, status, map[string]any{"error": qe.Message, "query_error": qe})
}

// applyExpr parses q's `expr` (if any) for a caller of the given owner status into
// f.Expr. The returned error is a *query.Error for a refused query.
func (h *Handlers) applyExpr(ctx context.Context, f *repo.VideoFilter, q url.Values, isOwner bool) error {
	src := q.Get("expr")
	if strings.TrimSpace(src) == "" {
		return nil
	}
	parsed, err := ParseExpr(ctx, h.repo, src, isOwner)
	if err != nil {
		return err
	}
	// `playlist:` conditions read the named playlists as they are now (F89 P0-3), under
	// the same rules a smart playlist's read runs; a reference that can't be read refuses
	// the query rather than compiling to false and quietly widening a NOT.
	x := newRefExpansion(0)
	if s := x.countConds(parsed.Root); s != nil {
		msg, _ := refusalCopy(*s, false)
		return &query.Error{Code: query.CodeBadValue, Message: msg}
	}
	stale, err := h.expandRefs(ctx, x, parsed.Root, !isOwner)
	if err != nil {
		return err
	}
	if len(stale) > 0 {
		msg := "This refers to a playlist that can't be shown."
		if isOwner {
			h.nameStaleRefs(ctx, stale)
			msg, _ = refusalCopy(stale[0], false)
		}
		// A visitor gets no detail: the ref could name a private playlist a public one
		// reaches (SEC-1).
		return &query.Error{Code: query.CodeBadValue, Message: msg, Field: query.FieldPlaylist}
	}
	f.Expr, f.Refs = parsed.Root, x.refs
	return nil
}

// parseQuery handles GET /query/parse?expr= (F87): the canonical string and a display
// tree with current names for the chips, or the error with its offsets. Ungated, and
// it runs the same owner check as /media, so a visitor's `matched:` is refused here too.
func (h *Handlers) parseQuery(w http.ResponseWriter, r *http.Request) {
	isOwner := h.auth.authorized(r)
	var q *query.Query
	var err error
	if isOwner && r.URL.Query().Get("stored") != "" {
		// A stored definition is already canonical (ids): it is parsed, never re-bound, so
		// a deleted entity reads as itself, marked missing, not as a name of those digits
		// (F89 "in: #14"). The owner's only: a visitor never sees a definition.
		q, err = query.Parse(r.URL.Query().Get("expr"))
	} else {
		q, err = ParseExpr(r.Context(), h.repo, r.URL.Query().Get("expr"), isOwner)
	}
	if err != nil {
		h.writeQueryError(w, "parse query", err)
		return
	}
	tree, err := q.Display(r.Context(), QueryResolver{Repo: h.repo, PublicOnly: !isOwner})
	if err != nil {
		h.fail(w, "parse query", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"canonical": q.String(), "tree": tree})
}
