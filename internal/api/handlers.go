package api

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"holodex/internal/metadata"
	"holodex/internal/repo"
)

// Handlers serves the REST API (ADR-006) over the repository.
type Handlers struct {
	repo *repo.Repo
	log  *slog.Logger
}

func NewHandlers(r *repo.Repo, log *slog.Logger) *Handlers {
	return &Handlers{repo: r, log: log}
}

// Mount registers the REST routes under the given router.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/media", h.listMedia)
	r.Get("/media/{id}", h.getMedia)
	r.Get("/media/{id}/stream", h.streamMedia)
	r.Get("/people", h.listPeople)
	r.Get("/people/{id}", h.getPerson)
	r.Get("/tags", h.listTags)
	r.Get("/tags/{id}", h.getTag)
	r.Get("/search", h.search)
}

// listMedia handles GET /media with filters (F4). Query params:
//
//	q, person (repeatable), tag (repeatable), duration_min/max (minutes),
//	resolution (SD|HD|FHD|4K), year_min/max, limit, offset.
func (h *Handlers) listMedia(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := repo.VideoFilter{
		Query:          q.Get("q"),
		PersonIDs:      parseIDs(q["person"]),
		TagIDs:         parseIDs(q["tag"]),
		DurationMinSec: atoiDefault(q.Get("duration_min"), 0) * 60,
		DurationMaxSec: atoiDefault(q.Get("duration_max"), 0) * 60,
		YearMin:        atoiDefault(q.Get("year_min"), 0),
		YearMax:        atoiDefault(q.Get("year_max"), 0),
		Limit:          atoiDefault(q.Get("limit"), 50),
		Offset:         atoiDefault(q.Get("offset"), 0),
	}
	if b, ok := metadata.ParseResolutionBucket(q.Get("resolution")); ok {
		f.WidthMin, f.WidthMax = metadata.ResolutionWidthRange(b)
	}

	items, total, err := h.repo.ListVideos(r.Context(), f)
	if err != nil {
		h.fail(w, "list media", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": f.Limit, "offset": f.Offset,
	})
}

func (h *Handlers) getMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, extra, err := h.repo.GetVideo(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	if err != nil {
		h.fail(w, "get media", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"video": v, "metadata": extra})
}

// streamMedia serves the file by ID with HTTP Range support (ADR-015). The path
// is looked up server-side; clients never supply paths, so traversal is
// structurally impossible.
func (h *Handlers) streamMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	path, err := h.repo.PathByID(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	if err != nil {
		h.fail(w, "resolve media path", err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		h.log.Warn("open media for stream failed", "id", id, "err", err)
		writeError(w, http.StatusNotFound, "media file unavailable")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stat failed")
		return
	}
	// http.ServeContent handles Range, 206, and conditional requests; the name
	// is used only for content-type sniffing by extension.
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (h *Handlers) listPeople(w http.ResponseWriter, r *http.Request) {
	people, err := h.repo.ListPeople(r.Context(), r.URL.Query().Get("sort") == "count")
	if err != nil {
		h.fail(w, "list people", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": people})
}

func (h *Handlers) getPerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := h.repo.GetPerson(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "person not found")
		return
	}
	if err != nil {
		h.fail(w, "get person", err)
		return
	}
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{PersonIDs: []int64{id}, Limit: 500})
	if err != nil {
		h.fail(w, "person videos", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"person": p, "items": items, "total": total})
}

func (h *Handlers) listTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.repo.ListTags(r.Context(), r.URL.Query().Get("sort") == "count")
	if err != nil {
		h.fail(w, "list tags", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tags})
}

func (h *Handlers) getTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, err := h.repo.GetTag(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tag not found")
		return
	}
	if err != nil {
		h.fail(w, "get tag", err)
		return
	}
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{TagIDs: []int64{id}, Limit: 500})
	if err != nil {
		h.fail(w, "tag videos", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": t, "items": items, "total": total})
}

func (h *Handlers) search(w http.ResponseWriter, r *http.Request) {
	res, err := h.repo.Search(r.Context(), r.URL.Query().Get("q"), atoiDefault(r.URL.Query().Get("limit"), 10))
	if err != nil {
		h.fail(w, "search", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- helpers ----

func (h *Handlers) fail(w http.ResponseWriter, op string, err error) {
	h.log.Error("api error", "op", op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// parseIDs flattens repeated and comma-separated id params into a slice.
func parseIDs(values []string) []int64 {
	var out []int64
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && n > 0 {
				out = append(out, n)
			}
		}
	}
	return out
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}
