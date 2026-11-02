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
type QueryResolver struct{ Repo *repo.Repo }

func (r QueryResolver) Lookup(ctx context.Context, kind, name string) ([]query.Candidate, error) {
	return r.Repo.QueryLookup(ctx, kind, name)
}

func (r QueryResolver) NameOf(ctx context.Context, kind string, id int64) (string, bool, error) {
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
	pool, err := r.Repo.QueryNames(ctx, kind)
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
	if err := q.Canonicalize(ctx, QueryResolver{r}); err != nil {
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
	f.Expr = parsed.Root
	return nil
}

// parseQuery handles GET /query/parse?expr= (F87): the canonical string and a display
// tree with current names for the chips, or the error with its offsets. Ungated, and
// it runs the same owner check as /media, so a visitor's `matched:` is refused here too.
func (h *Handlers) parseQuery(w http.ResponseWriter, r *http.Request) {
	q, err := ParseExpr(r.Context(), h.repo, r.URL.Query().Get("expr"), h.auth.authorized(r))
	if err != nil {
		h.writeQueryError(w, "parse query", err)
		return
	}
	tree, err := q.Display(r.Context(), QueryResolver{h.repo})
	if err != nil {
		h.fail(w, "parse query", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"canonical": q.String(), "tree": tree})
}
