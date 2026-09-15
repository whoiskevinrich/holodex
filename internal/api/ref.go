package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/model"
)

// Reference handles (F60 RD1, ADR-096 D1): every place that takes an entity id
// — a `{id}` route segment, an MCP tool argument — also takes the `kind:id`
// handle the server emits as `ref`. This file is the one parser; nothing else
// splits on the colon.

var errInvalidID = errors.New("invalid id")

// RefKindError is the kind-mismatch rejection: the ref parsed, but names a
// different entity kind than the route or tool expects. Its message names the
// expected kind so the caller can correct the handle.
type RefKindError struct {
	Expected model.Kind
	Ref      string
}

func (e *RefKindError) Error() string {
	return fmt.Sprintf("expected a %s ref, got %s", e.Expected, e.Ref)
}

// ParseRef parses s as either a bare positive integer or a `kind:id` reference.
// A ref whose kind differs from kind is rejected with *RefKindError — the row
// may well exist, the request is malformed, so callers answer 400, not 404.
// With kind "" (the caller has no entity kind, e.g. a category or a writeback
// job) only the bare form is accepted.
func ParseRef(kind model.Kind, s string) (int64, error) {
	s = strings.TrimSpace(s)
	if got, rest, ok := strings.Cut(s, ":"); ok {
		if kind == "" {
			return 0, errInvalidID
		}
		if model.Kind(got) != kind {
			return 0, &RefKindError{Expected: kind, Ref: s}
		}
		s = rest
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidID
	}
	return id, nil
}

// collectionKinds maps the API collection segment to the entity kind its `{id}`
// names, so a route's kind is read off its pattern rather than repeated at
// every pathID call site.
var collectionKinds = map[string]model.Kind{
	"media":   model.KindVideo,
	"people":  model.KindPerson,
	"studios": model.KindStudio,
	"tags":    model.KindTag,
	"films":   model.KindFilm,
}

// paramKinds covers the routes that nest a second entity id under a different
// param name (film_videos.go, film_people_roles.go, video_tags.go).
var paramKinds = map[string]model.Kind{
	"filmId":   model.KindFilm,
	"personId": model.KindPerson,
	"videoId":  model.KindVideo,
	"tagID":    model.KindTag,
}

// routeKind returns the entity kind a path param names on this request: by
// param name for the nested forms, otherwise the first entity collection in the
// matched route pattern (`/api/v1/people/{id}/aliases` → person). "" when the
// route is not an entity route.
func routeKind(r *http.Request, param string) model.Kind {
	if k, ok := paramKinds[param]; ok {
		return k
	}
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		for _, seg := range strings.Split(rctx.RoutePattern(), "/") {
			if k, ok := collectionKinds[seg]; ok {
				return k
			}
		}
	}
	return ""
}
