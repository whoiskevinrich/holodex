package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/cache"
	"holodex/internal/mapping"
	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

// thumbnailer is the subset of the thumbnail pipeline the API needs: enqueue
// visible/regenerated items (Tier 3) and report queue depth. Nil when thumbnail
// generation is not wired (tests, or a build without it).
type thumbnailer interface {
	EnqueueHigh(ids []int64)
	QueueDepth() int
	QueueStats() thumbnail.QueueStats // pipeline snapshot for the activity surface (F21.1)
	Enabled() bool
}

// rescanner triggers an out-of-band full library re-index (F13.3). Nil in tests
// or when the scanner is not wired (health-only mode).
type rescanner interface {
	TriggerRescan() bool
}

// searchMetrics records search latency (F13.2). Optional — nil disables it.
type searchMetrics interface {
	ObserveSearch(d time.Duration)
}

// scanStatusSource exposes the scanner's live state for the activity read-model
// (F21.1/F21.2). Nil disables the scan section (tests / health-only mode).
type scanStatusSource interface {
	Status() model.ScanStatus
}

// Handlers serves the REST API (ADR-006) over the repository.
type Handlers struct {
	repo     *repo.Repo
	log      *slog.Logger
	thumbs   thumbnailer
	thumbDir string
	scanner  rescanner
	metrics  searchMetrics
	mappings *mapping.Store // configurable metadata fields (F20); nil disables them
	cache    cache.Cache    // facet-value cache (F20.8); nil disables caching

	// Activity surface (F21.1, ADR-028). All optional/nil-safe. Thumbnail stats
	// come from the existing thumbs seam; scan status from scanStatus.
	scanStatus       scanStatusSource
	health           *Health
	version          string
	startedAt        time.Time
	mediaPathPresent bool
}

// NewHandlers wires the REST handlers. thumbs, sc, and m are optional (nil-safe):
// they disable thumbnail bumping, admin rescan, and search instrumentation
// respectively in tests or health-only mode.
func NewHandlers(r *repo.Repo, log *slog.Logger, thumbs thumbnailer, thumbDir string, sc rescanner, m searchMetrics) *Handlers {
	return &Handlers{repo: r, log: log, thumbs: thumbs, thumbDir: thumbDir, scanner: sc, metrics: m}
}

// SetMetadataFields wires the configurable metadata field mapping (F20) and the
// facet-value cache. Called once at startup before serving; a nil store disables
// mapped-field display, facets, and the reload endpoint.
func (h *Handlers) SetMetadataFields(store *mapping.Store, c cache.Cache) {
	h.mappings = store
	h.cache = c
}

// SetActivity wires the read-only activity surface (F21.1, ADR-028): the scanner
// status source, the health state, the build version, the process start time
// (for uptime), and whether MEDIA_PATH is configured. Thumbnail stats are read
// from the thumbnailer seam wired in NewHandlers. Called once at startup before
// serving; all parts are nil-safe.
func (h *Handlers) SetActivity(scan scanStatusSource, health *Health, version string, startedAt time.Time, mediaPathPresent bool) {
	h.scanStatus = scan
	h.health = health
	h.version = version
	h.startedAt = startedAt
	h.mediaPathPresent = mediaPathPresent
}

// Mount registers the REST routes under the given router.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/media", h.listMedia)
	r.Get("/media/{id}", h.getMedia)
	r.Get("/media/{id}/stream", h.streamMedia)
	r.Get("/media/{id}/thumbnail", h.serveThumbnail)
	r.Post("/media/{id}/thumbnail", h.regenerateThumbnail)
	r.Get("/people", h.listPeople)
	r.Get("/people/{id}", h.getPerson)
	r.Get("/tags", h.listTags)
	r.Get("/tags/{id}", h.getTag)
	r.Get("/search", h.search)
	r.Get("/facets", h.facets)
	r.Get("/metadata-keys", h.metadataKeys)
	r.Get("/admin/status", h.adminStatus)
	r.Get("/admin/activity", h.adminActivity)
	r.Get("/admin/activity/history", h.adminActivityHistory)
	r.Post("/admin/rescan", h.adminRescan)
	r.Post("/admin/reload-config", h.adminReloadConfig)
}

// listMedia handles GET /media with filters (F4). Query params:
//
//	q, person (repeatable), tag (repeatable), duration_min/max (minutes),
//	resolution (SD|HD|FHD|4K), year_min/max, sort, limit, offset.
//
// sort (F12.1) is one of title_asc|title_desc|added_asc|added_desc|
// duration_asc|duration_desc|resolution_asc|resolution_desc; default added_desc.
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
		Sort:           q.Get("sort"),
		Limit:          atoiDefault(q.Get("limit"), 50),
		Offset:         atoiDefault(q.Get("offset"), 0),
	}
	if b, ok := metadata.ParseResolutionBucket(q.Get("resolution")); ok {
		f.WidthMin, f.WidthMax = metadata.ResolutionWidthRange(b)
	}
	// Filterable mapped fields become query params keyed by canonical name (F20.5),
	// e.g. ?studio=Acme.
	if h.mappings != nil {
		for _, fld := range h.mappings.Current().Filterable() {
			if val := q.Get(fld.Canonical); val != "" {
				f.MappedFilters = append(f.MappedFilters, repo.MappedFilter{SourceKeys: fld.Sources, Value: val})
			}
		}
	}

	items, total, err := h.repo.ListVideos(r.Context(), f)
	if err != nil {
		h.fail(w, "list media", err)
		return
	}
	// One pass: set the serving URL on ready videos, and collect never-attempted
	// ones to enqueue at high priority (Tier 3). Previously-failed items are left
	// to the startup sweep so a broken file isn't re-attempted on every browse.
	var pending []int64
	for i := range items {
		setThumbnailURL(&items[i])
		if items[i].ThumbnailState == model.ThumbnailNone {
			pending = append(pending, items[i].ID)
		}
	}
	if h.thumbs != nil && len(pending) > 0 {
		h.thumbs.EnqueueHigh(pending)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": f.Limit, "offset": f.Offset,
	})
}

// setThumbnailURL fills ThumbnailURL when an image exists on disk (ADR-009).
func setThumbnailURL(v *model.Video) {
	if model.HasThumbnailImage(v.ThumbnailState) {
		v.ThumbnailURL = fmt.Sprintf("/api/v1/media/%d/thumbnail", v.ID)
	}
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
	setThumbnailURL(v)
	var fields []mapping.Resolved
	if h.mappings != nil {
		fields = h.mappings.Current().Resolve(extra)
	}
	writeJSON(w, http.StatusOK, map[string]any{"video": v, "metadata": extra, "fields": fields})
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

// serveThumbnail serves the on-disk thumbnail for a video (ADR-009). A missing
// file returns 404 — the contract the frontend's retry loop relies on while a
// background thumbnail is still generating. The id comes from the route (an
// integer), never a client-supplied path, so traversal is impossible.
func (h *Handlers) serveThumbnail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	f, err := os.Open(thumbnail.ThumbPath(h.thumbDir, id))
	if err != nil {
		writeError(w, http.StatusNotFound, "thumbnail not ready")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "thumbnail not ready")
		return
	}
	// Cached for a day; regeneration rewrites the file (new mtime) and the client
	// cache-busts, so a stale image is never pinned.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// regenerateThumbnail forces re-extraction for one video (F11.6): it clears the
// stored state and enqueues the id at high priority, returning 202 Accepted.
func (h *Handlers) regenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.thumbs == nil || !h.thumbs.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "thumbnail generation disabled")
		return
	}
	if _, err := h.repo.PathByID(r.Context(), id); errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	} else if err != nil {
		h.fail(w, "regenerate thumbnail", err)
		return
	}
	if err := h.repo.ResetThumbnailState(r.Context(), id); err != nil {
		h.fail(w, "reset thumbnail state", err)
		return
	}
	h.thumbs.EnqueueHigh([]int64{id})
	w.WriteHeader(http.StatusAccepted)
}

// adminStatus surfaces operational counters. thumbnail_queue_depth is the F11.8
// metric (full Prometheus /metrics is deferred to a later Phase 2 task).
func (h *Handlers) adminStatus(w http.ResponseWriter, _ *http.Request) {
	depth := 0
	if h.thumbs != nil {
		depth = h.thumbs.QueueDepth()
	}
	writeJSON(w, http.StatusOK, map[string]any{"thumbnail_queue_depth": depth})
}

// adminRescan triggers a full library re-index (F13.3) and returns 202 Accepted
// immediately; the scan runs in the background. "started":false means a scan was
// already in progress, which already satisfies the request.
func (h *Handlers) adminRescan(w http.ResponseWriter, _ *http.Request) {
	if h.scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "rescan unavailable")
		return
	}
	started := h.scanner.TriggerRescan()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "started": started})
}

// adminReloadConfig re-reads metadata-mappings.yaml without a restart (F20.10) and
// invalidates cached facet values so new mappings take effect immediately.
func (h *Handlers) adminReloadConfig(w http.ResponseWriter, r *http.Request) {
	if h.mappings == nil {
		writeError(w, http.StatusServiceUnavailable, "config reload unavailable")
		return
	}
	if err := h.mappings.Reload(); err != nil {
		h.fail(w, "reload config", err)
		return
	}
	if h.cache != nil {
		_ = h.cache.InvalidatePrefix(r.Context(), facetCachePrefix)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reloaded", "fields": len(h.mappings.Current().Fields())})
}

const facetCachePrefix = "facet:"

// facets returns the filterable mapped fields with their distinct values (F20.4).
func (h *Handlers) facets(w http.ResponseWriter, r *http.Request) {
	type facet struct {
		Canonical string            `json:"canonical"`
		Label     string            `json:"label"`
		Multi     bool              `json:"multi"`
		Values    []repo.FacetValue `json:"values"`
	}
	out := []facet{}
	if h.mappings != nil {
		for _, fld := range h.mappings.Current().Filterable() {
			vals, err := h.facetValues(r.Context(), fld)
			if err != nil {
				h.fail(w, "facets", err)
				return
			}
			out = append(out, facet{Canonical: fld.Canonical, Label: fld.Label, Multi: fld.Multi, Values: vals})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"facets": out})
}

// facetValues returns a field's distinct values, served from cache when available
// (F20.8). With the Noop cache (ADR-022) this recomputes; the seam + TTL +
// reload-invalidation are in place for when a real backend is enabled.
func (h *Handlers) facetValues(ctx context.Context, fld mapping.Field) ([]repo.FacetValue, error) {
	key := facetCachePrefix + strings.ToLower(fld.Canonical)
	if h.cache != nil {
		if b, ok := h.cache.Get(ctx, key); ok {
			var v []repo.FacetValue
			if json.Unmarshal(b, &v) == nil {
				return v, nil
			}
		}
	}
	v, err := h.repo.FacetValues(ctx, fld.Sources)
	if err != nil {
		return nil, err
	}
	if h.cache != nil {
		if b, err := json.Marshal(v); err == nil {
			_ = h.cache.Set(ctx, key, b, 5*time.Minute)
		}
	}
	return v, nil
}

// metadataKeys is the library-wide mapping-authoring aid (F20.9): every distinct
// raw source key with counts, sample values, and whether a mapping covers it.
func (h *Handlers) metadataKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.repo.MetadataKeys(r.Context(), 3)
	if err != nil {
		h.fail(w, "metadata keys", err)
		return
	}
	mapped := map[string]bool{}
	if h.mappings != nil {
		for _, fld := range h.mappings.Current().Fields() {
			for _, s := range fld.Sources {
				mapped[strings.ToLower(s)] = true
			}
		}
	}
	type keyOut struct {
		repo.MetadataKey
		Mapped bool `json:"mapped"`
	}
	out := make([]keyOut, len(keys))
	for i, k := range keys {
		out[i] = keyOut{MetadataKey: k, Mapped: mapped[strings.ToLower(k.SourceKey)]}
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out})
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
	start := time.Now()
	res, err := h.repo.Search(r.Context(), r.URL.Query().Get("q"), atoiDefault(r.URL.Query().Get("limit"), 10))
	if err != nil {
		h.fail(w, "search", err)
		return
	}
	if h.metrics != nil {
		h.metrics.ObserveSearch(time.Since(start))
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
