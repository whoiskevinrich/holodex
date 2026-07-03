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
	"holodex/internal/enrich"
	"holodex/internal/mapping"
	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/refresh"
	"holodex/internal/repo"
	"holodex/internal/resolver"
	"holodex/internal/thumbnail"
	"holodex/internal/writequeue"
)

// thumbnailer is the subset of the thumbnail pipeline the API needs: enqueue
// visible/regenerated items (Tier 3) and report queue depth. Nil when thumbnail
// generation is not wired (tests, or a build without it).
type thumbnailer interface {
	ExtractEmbedded(ctx context.Context, id int64, path string) (bool, error)
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
	repo       *repo.Repo
	log        *slog.Logger
	thumbs     thumbnailer
	thumbDir   string
	scanner    rescanner
	metrics    searchMetrics
	mappings   *mapping.Store    // configurable metadata fields (F20); nil disables them
	cache      cache.Cache       // facet-value cache (F20.8); nil disables caching
	enrich     *enrich.Service   // metadata source plugins (F22, ADR-033); nil disables them
	writeback  WriteBatchFunc    // file tag write (F28, ADR-041); nil disables the endpoint
	writeQueue *writequeue.Queue // durable batch-write queue (F30, ADR-048); nil → synchronous write
	refresh    *refresh.Service  // per-item forced re-extract + re-enrich (F31, ADR-047); nil disables it

	// Activity surface (F21.1, ADR-028). All optional/nil-safe. Thumbnail stats
	// come from the existing thumbs seam; scan status from scanStatus.
	scanStatus       scanStatusSource
	health           *Health
	version          string
	startedAt        time.Time
	mediaPathPresent bool

	// Owner gating (F21.7, ADR-030). auth nil = open. exposedBind is true when the
	// server binds beyond loopback; with no token that combination is the
	// fail-loud "controls reachable without a token" condition.
	auth        *Auth
	exposedBind bool

	// Person images (F25, ADR-038). personImageDir is the on-disk root; the bounds
	// guard untrusted uploads. Zero personImageDir leaves the image endpoints serving
	// placeholders only (no on-disk store wired) — but uploads then fail closed.
	personImageDir      string
	personImageMaxBytes int64
	personImageMaxDim   int
	defaultSkin         string

	// Soft-delete + purge (F24, ADR-037). purger executes purge-now; deleteGrace
	// drives the Trash view's purge_at. Both optional — nil purger disables only
	// purge-now (soft-delete/restore/Trash still work).
	purger      purger
	deleteGrace time.Duration

	// cardLayout is the operator's preferred card aspect ratio ("wide" or "poster"),
	// surfaced via /capabilities so all visitors see a consistent grid presentation.
	cardLayout string

	// defaultSource is the F36 undecided source-of-truth mode ("file" | "mapping",
	// ADR-051/RD4). It feeds resolver.Options so an undecided field resolves
	// file-first by default; empty means file-first.
	defaultSource string

	// providerTrustOrder ranks providers for the undecided winner among them on a
	// replace field (F36 P1-2, ADR-051 §8). Fed into resolver.Options alongside
	// defaultSource; empty means mapping order among providers.
	providerTrustOrder []string
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

// SetEnrichment wires the metadata source plugin service (F22, ADR-033). A nil
// service disables the enrichment endpoints and the person-page enriched fields.
// Called once at startup before serving.
func (h *Handlers) SetEnrichment(svc *enrich.Service) { h.enrich = svc }

// SetRefresh wires the per-item refresh service (F31, ADR-047). A nil service
// disables POST /media/{id}/refresh (503). Called once at startup before serving.
func (h *Handlers) SetRefresh(svc *refresh.Service) { h.refresh = svc }

// SetPersonImages wires per-person image storage (F25, ADR-038): the on-disk root,
// the upload bounds, and the default skin used when a placeholder is served without
// a ?skin= query. An empty dir leaves the public serving endpoint working (it falls
// back to placeholders) but uploads fail closed. Called once at startup.
func (h *Handlers) SetPersonImages(dir string, maxBytes int64, maxDim int, defaultSkin string) {
	h.personImageDir = dir
	h.personImageMaxBytes = maxBytes
	h.personImageMaxDim = maxDim
	h.defaultSkin = defaultSkin
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

// SetAuth wires the owner gate (F21.7, ADR-030). auth nil leaves the admin
// surface open (the single-user default). exposedBind marks a non-loopback bind,
// which together with an absent token drives the fail-loud
// controls_unauthenticated signal. Called once at startup before serving.
func (h *Handlers) SetAuth(auth *Auth, exposedBind bool) {
	h.auth = auth
	h.exposedBind = exposedBind
}

// SetCardLayout wires the operator's preferred card aspect ratio. Config validates
// the value to "wide" or "poster" before this is called; this is a simple assignment.
func (h *Handlers) SetCardLayout(layout string) {
	h.cardLayout = layout
}

// SetDefaultSource wires the F36 undecided source-of-truth mode ("file" | "mapping",
// ADR-051/RD4). Config validates the value before this is called. Empty means
// file-first (the default). Called once at startup before serving.
func (h *Handlers) SetDefaultSource(mode string) {
	h.defaultSource = mode
}

// SetProviderTrustOrder wires the F36 inter-provider trust order (P1-2, ADR-051 §8):
// the ranking that decides the undecided winner among providers on a replace field.
// Config normalizes the list before this is called; empty leaves mapping order among
// providers. Called once at startup before serving.
func (h *Handlers) SetProviderTrustOrder(order []string) {
	h.providerTrustOrder = order
}

// resolveOptions builds the resolver options for one video from its pre-loaded
// standing decisions, the global default-source mode, and the inter-provider trust
// order (F36).
func (h *Handlers) resolveOptions(decisions resolver.Decisions) resolver.Options {
	return resolver.Options{
		Decisions:          decisions,
		DefaultSource:      h.defaultSource,
		ProviderTrustOrder: h.providerTrustOrder,
	}
}

// controlsUnauthenticated is true when the admin surface is reachable beyond
// loopback with no token configured (F21.7 condition 1). Required() is
// nil-receiver safe, so no separate h.auth nil check is needed.
func (h *Handlers) controlsUnauthenticated() bool {
	return h.exposedBind && !h.auth.Required()
}

// Mount registers the REST routes under the given router.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/media", h.listMedia)
	r.Get("/media/{id}", h.getMedia)
	r.Get("/media/{id}/related", h.getRelated)
	r.Get("/media/{id}/stream", h.streamMedia)
	r.Get("/media/{id}/thumbnail", h.serveThumbnail)
	r.Post("/media/{id}/thumbnail", h.regenerateThumbnail)
	r.Get("/studios", h.listStudios)
	r.Get("/studios/{id}", h.getStudio)
	r.Get("/people", h.listPeople)
	r.Get("/people/{id}", h.getPerson)
	// Person images (F25, ADR-038) — public reads: a filled role serves the on-disk
	// JPEG, an empty role the themed placeholder SVG. Mutations are gated below.
	r.Get("/people/{id}/image/{role}", h.servePersonImageByRole)
	r.Get("/people/{id}/images", h.getPersonImages)
	r.Get("/people/{id}/images/{imageId}", h.servePersonImageByID)
	r.Get("/tags", h.listTags)
	r.Get("/tags/{id}", h.getTag)
	r.Get("/search", h.search)
	r.Get("/facets", h.facets)
	// Ungated: lets the SPA discover whether it is an owner / needs a token (F21.7).
	r.Get("/capabilities", h.capabilities)
	// Owner session exchange (ADR-046): POST validates the token and sets an
	// HttpOnly cookie; DELETE signs out. Ungated — POST authenticates itself, and
	// DELETE only clears a cookie. The cookie then authorizes the group below.
	r.Post("/session", h.postSession)
	r.Delete("/session", h.deleteSession)

	// Owner-only surface (F21.7, ADR-030): the single choke point for the activity
	// read-model, history, and the admin controls. Open when no ADMIN_TOKEN is set.
	r.Group(func(r chi.Router) {
		r.Use(h.requireOwner)
		// Raw metadata-key discovery powers the owner-only /owner/keys tab (F35). It
		// enumerates container metadata keys + sample values across the library, so
		// gate it like the rest of the owner tooling — closing the F20-era public
		// exposure the F35 nav split surfaced (spec P0-4).
		r.Get("/metadata-keys", h.metadataKeys)
		r.Get("/admin/status", h.adminStatus)
		r.Get("/admin/activity", h.adminActivity)
		r.Get("/admin/activity/history", h.adminActivityHistory)
		r.Post("/admin/rescan", h.adminRescan)
		r.Post("/admin/reload-config", h.adminReloadConfig)
		// Metadata source plugins — People enrichment (F22, ADR-033).
		h.mountEnrich(r)
		// Person aliases — owner-curated alternate names (F23, ADR-036).
		h.mountAliases(r)
		// Person images — owner-gated upload/delete/promote/reorder (F25, ADR-038).
		h.mountPersonImages(r)
		// Media soft-delete / purge-now / restore / Trash (F24, ADR-037).
		h.mountDelete(r)
		// Metadata writeback — embed enriched values into media files (F28, ADR-041).
		h.mountWriteback(r)
		// Value-level metadata curation — manual add/suppress/nowrite (F30, ADR-048).
		h.mountCuration(r)
		// Per-field source-of-truth decisions — pin file/provider/manual (F36, ADR-051).
		h.mountDecisions(r)
		// People on the unified model — person decisions/curation + rename (F37).
		h.mountPersonDecisions(r)
		// Studio on the unified model — studio decisions/curation (F38, ADR-053).
		h.mountStudioDecisions(r)
		// Per-item forced re-extract + re-enrich (F31, ADR-047).
		r.Post("/media/{id}/refresh", h.refreshMedia)
	})
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
		StudioIDs:      parseIDs(q["studio_id"]),
		DurationMinSec: atoiDefault(q.Get("duration_min"), 0) * 60,
		DurationMaxSec: atoiDefault(q.Get("duration_max"), 0) * 60,
		YearMin:        atoiDefault(q.Get("year_min"), 0),
		YearMax:        atoiDefault(q.Get("year_max"), 0),
		Sort:           q.Get("sort"),
		Limit:          atoiDefault(q.Get("limit"), 50),
		Offset:         atoiDefault(q.Get("offset"), 0),
	}
	if f.Sort == "random" {
		f.Seed = parseSeedOrRandom(q.Get("seed"))
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
	h.prepareThumbnails(items)
	// Browse-title resolution (F27): any field with browse:true overwrites video.Title
	// with the highest-precedence source (e.g. tmdb:title before file:title).
	if h.mappings != nil {
		h.applyBrowseTitles(r.Context(), items, h.mappings.Current().Fields())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": f.Limit, "offset": f.Offset,
	})
}

// setThumbnailURL fills ThumbnailURL when an image exists on disk (ADR-009). The
// ?v= token is the file mtime (Unix seconds): it changes whenever the source file
// is rewritten (e.g. a metadata writeback that embeds new cover art), so the grid
// and detail page fetch a never-before-seen URL instead of a stale browser-cached
// copy. Paired with the endpoint's no-cache header for revalidation.
func setThumbnailURL(v *model.Video) {
	if !model.HasThumbnailImage(v.ThumbnailState) {
		return
	}
	// Guard against a zero-value mtime, which would render a large negative token.
	// Indexed videos always carry an mtime; fall back to 0 only for safety.
	var ver int64
	if !v.FileMtime.IsZero() {
		ver = v.FileMtime.Unix()
	}
	v.ThumbnailURL = fmt.Sprintf("/api/v1/media/%d/thumbnail?v=%d", v.ID, ver)
}

// prepareThumbnails sets the serving URL on each video and enqueues never-attempted
// covers at high priority (Tier 3, ADR-009). Previously-failed items are left to the
// startup sweep so a broken file isn't re-attempted on every browse.
func (h *Handlers) prepareThumbnails(videos []model.Video) {
	var pending []int64
	for i := range videos {
		setThumbnailURL(&videos[i])
		if videos[i].ThumbnailState == model.ThumbnailNone {
			pending = append(pending, videos[i].ID)
		}
	}
	if h.thumbs != nil && len(pending) > 0 {
		h.thumbs.EnqueueHigh(pending)
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
	var resolved []resolver.ResolvedField
	var enriched []model.EnrichedField
	if h.mappings != nil {
		m := h.mappings.Current()
		fields = m.Resolve(extra)
		// Fetch enrichment rows once — the resolver (F27 merged view) and the raw
		// enriched-display (F26 per-provider table) both read from the same rows,
		// avoiding two identical DB round-trips for the same entity.
		enrichRows, err2 := h.repo.EnrichmentForEntity(r.Context(), model.EnrichEntityVideo, id)
		if err2 != nil {
			h.log.Warn("enrichment for detail", "id", id, "err", err2)
		} else {
			enr := enrichmentFromRows(enrichRows)
			// Value-level curation (F30): manual adds, suppressions, no-write flags.
			var cur resolver.Curation
			if curRows, curErr := h.repo.CurationForEntity(r.Context(), model.EnrichEntityVideo, id); curErr != nil {
				h.log.Warn("curation for detail", "id", id, "err", curErr)
			} else {
				cur = curationFromRows(curRows)
			}
			// Standing per-field source decisions (F36): pre-loaded so the resolver
			// short-circuits mapping order without a per-field query (pure resolution).
			var dec resolver.Decisions
			if decRows, decErr := h.repo.DecisionsForEntity(r.Context(), model.EnrichEntityVideo, id); decErr != nil {
				h.log.Warn("decisions for detail", "id", id, "err", decErr)
			} else {
				dec = decisionsFromRows(decRows)
			}
			resolved = resolver.Resolve(v, extra, enr, cur, m.Fields(), h.resolveOptions(dec))
			if h.enrich != nil {
				enriched = h.enrich.FieldsFromRows(enrichRows)
			}
		}
	} else if h.enrich != nil {
		enriched = h.videoEnrichment(r, id)
	}
	// Studio entities linked to this video (F38, ADR-053): the resolved studio
	// value links to its /studios/{id} page, and the link target always matches the
	// displayed value because video_studios is derived from that same resolution.
	var studios []model.Studio
	if byVideo, serr := h.repo.StudiosForVideos(r.Context(), []int64{id}); serr != nil {
		h.log.Warn("studios for media detail", "id", id, "err", serr)
	} else {
		studios = byVideo[id]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"video":    v,
		"metadata": extra,
		"fields":   fields,
		"resolved": resolved,
		"enriched": enriched,
		"studios":  studios,
	})
}

// getRelated handles GET /media/{id}/related — the "More with …" shelves (ADR-031):
// a person-keyed and a tag-keyed set of up to 5 random sibling videos. 404 if the
// item is missing/inactive (consistent with getMedia).
func (h *Handlers) getRelated(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	related, err := h.repo.Related(r.Context(), id, 5)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	if err != nil {
		h.fail(w, "related media", err)
		return
	}
	// Same cover-art treatment as the grid, per shelf.
	for _, shelf := range []*repo.RelatedShelf{related.Person, related.Tag} {
		if shelf != nil {
			h.prepareThumbnails(shelf.Items)
		}
	}
	writeJSON(w, http.StatusOK, related)
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
	// Hide a soft-deleted item's cover during the grace window (F24/ADR-037 §4) —
	// the only way to reach it is a guessed id, but keep the bytes consistent with
	// the 404 its detail/stream now return. One indexed PK lookup; negligible.
	if visible, err := h.repo.VideoVisible(r.Context(), id); err != nil {
		h.fail(w, "thumbnail visibility", err)
		return
	} else if !visible {
		writeError(w, http.StatusNotFound, "thumbnail not ready")
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
	// no-cache so the browser always revalidates. http.ServeContent sets Last-Modified
	// and handles If-Modified-Since, so unchanged thumbnails return 304 (no bytes
	// transferred). max-age=86400 would pin a stale frame-grab for a day after a
	// writeback or regenerate — the grid has no URL version parameter to bust with.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// regenerateThumbnail forces re-extraction for one video (F11.6): tries embedded
// cover art first (Tier 1); falls back to queued frame generation (Tier 2/3) when
// no art is found. Returns 200 when art was extracted synchronously, 202 when
// generation was queued.
func (h *Handlers) regenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.thumbs == nil || !h.thumbs.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "thumbnail generation disabled")
		return
	}
	path, err := h.repo.PathByID(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
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
	if extracted, err := h.thumbs.ExtractEmbedded(r.Context(), id, path); err != nil {
		h.fail(w, "extract embedded cover art", err)
		return
	} else if extracted {
		w.WriteHeader(http.StatusOK)
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
	// Reload the provider registry alongside the mappings (F22.2d) so both config
	// files take effect without a restart.
	if h.enrich != nil {
		if err := h.enrich.Store().Reload(); err != nil {
			h.fail(w, "reload sources", err)
			return
		}
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
	if err != nil {
		h.personLookupError(w, err)
		return
	}
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{PersonIDs: []int64{id}, Limit: 500})
	if err != nil {
		h.fail(w, "person videos", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"person": p, "items": items, "total": total,
		// F37 (P0-2): the unified resolver payload — record vocabulary, no
		// in_sync. It supersedes the raw F22 enriched[] block, retired here.
		"resolved": h.personResolved(r, id, p),
		"images":   h.personImageSet(r, id), // F25: per-role presence + version + gallery
	})
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

// parseSeedOrRandom parses the client-supplied shuffle seed for the "random" sort
// (ADR-045). A valid integer is used as-is so successive "Load more" pages share
// one shuffle; a missing/invalid seed falls back to a per-request seed (the page
// is still internally consistent, but the client always sends one so pages tile).
// The value is only ever passed to holo_shuffle() as a bound parameter — never
// interpolated into SQL.
func parseSeedOrRandom(s string) int64 {
	if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return n
	}
	return time.Now().UnixNano()
}

// enrichmentFromRows converts repo enrichment rows to the resolver.Enrichment map
// (provider → field → values). Returns nil when rows is empty so callers on hot
// paths (browse list) avoid allocating a map for every non-enriched video.
func enrichmentFromRows(rows []repo.EnrichmentRow) resolver.Enrichment {
	if len(rows) == 0 {
		return nil
	}
	out := make(resolver.Enrichment, 2)
	for _, r := range rows {
		if out[r.Provider] == nil {
			out[r.Provider] = make(map[string][]string)
		}
		out[r.Provider][r.FieldKey] = r.Values
	}
	return out
}

// curationFromRows converts repo curation rows to the resolver.Curation map
// (field → adds/suppress/nowrite). Returns nil when rows is empty so hot paths
// avoid allocating a map for every non-curated video.
func curationFromRows(rows []repo.CurationRow) resolver.Curation {
	if len(rows) == 0 {
		return nil
	}
	out := make(resolver.Curation, 2)
	for _, r := range rows {
		fc := out[r.FieldKey]
		switch r.Action {
		case repo.CurationAdd:
			fc.Add = append(fc.Add, r.Value)
		case repo.CurationSuppress:
			if fc.Suppress == nil {
				fc.Suppress = make(map[string]bool)
			}
			fc.Suppress[r.NormValue] = true
		case repo.CurationNoWrite:
			if fc.NoWrite == nil {
				fc.NoWrite = make(map[string]bool)
			}
			fc.NoWrite[r.NormValue] = true
		}
		out[r.FieldKey] = fc
	}
	return out
}

// decisionsFromRows converts repo decision rows to the resolver.Decisions map
// (canonical field → standing decision). Returns nil when rows is empty so hot paths
// avoid allocating a map for every undecided video.
func decisionsFromRows(rows []repo.DecisionRow) resolver.Decisions {
	if len(rows) == 0 {
		return nil
	}
	out := make(resolver.Decisions, len(rows))
	for _, r := range rows {
		out[strings.ToLower(strings.TrimSpace(r.FieldKey))] = resolver.Decision{
			Source:      r.Source,
			ManualValue: r.ManualValue,
		}
	}
	return out
}

// applyBrowseTitles resolves the highest-precedence title for each video (F27) and
// overwrites video.Title when a provider source wins. Extra-metadata is not loaded
// for list pages, so only file:title (already in Video.Title) and provider sources
// participate; file:<Key> sources are skipped unless the video already has that
// data in memory (it doesn't in the list path).
func (h *Handlers) applyBrowseTitles(ctx context.Context, items []model.Video, fields []mapping.Field) {
	var browseFields []mapping.Field
	for _, f := range fields {
		if f.Browse {
			browseFields = append(browseFields, f)
		}
	}
	if len(browseFields) == 0 {
		return
	}
	ids := make([]int64, len(items))
	for i, v := range items {
		ids[i] = v.ID
	}
	batchEnrich, err := h.repo.EnrichmentForVideos(ctx, ids)
	if err != nil {
		h.log.Warn("batch enrichment for browse titles", "err", err)
		return
	}
	// Curation can override/suppress the browse title too (F30); batch-load it
	// alongside enrichment to keep the list path free of N+1 queries.
	batchCuration, err := h.repo.CurationForVideos(ctx, ids)
	if err != nil {
		h.log.Warn("batch curation for browse titles", "err", err)
	}
	// A standing per-field decision (F36) drives the browse title just as it drives
	// the detail view, so an adopted-provider or custom title shows on the card.
	batchDecisions, err := h.repo.DecisionsForVideos(ctx, ids)
	if err != nil {
		h.log.Warn("batch decisions for browse titles", "err", err)
	}
	for i := range items {
		enr := enrichmentFromRows(batchEnrich[items[i].ID])
		cur := curationFromRows(batchCuration[items[i].ID])
		opts := h.resolveOptions(decisionsFromRows(batchDecisions[items[i].ID]))
		if t, _ := resolver.BrowseTitle(&items[i], nil, enr, cur, browseFields, opts); t != "" {
			items[i].Title = t
		}
	}
}
