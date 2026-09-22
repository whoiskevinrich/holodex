package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"holodex/internal/cache"
	"holodex/internal/enrich"
	"holodex/internal/extract"
	"holodex/internal/mapping"
	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/refresh"
	"holodex/internal/registry"
	"holodex/internal/repo"
	"holodex/internal/resolver"
	"holodex/internal/thumbnail"
	"holodex/internal/writeback"
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

	// Filename extraction triggers (F48.5, ADR-067). extract/extractBatch nil
	// disable the on-demand/batch extraction endpoints (503). patterns is a
	// direct peer of mappings/enrich (both reloaded alongside it below) rather
	// than reached through extract.Orchestrator's composition.
	extract      *extract.Orchestrator
	extractBatch *extract.BatchRunner
	// Entity refresh sweep (F67, ADR-103 D8). nil disables the trigger (503) and
	// leaves the activity `sweep` block idle.
	sweep    *enrich.SweepRunner
	patterns *extract.PatternStore

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

	// Studio images (F51, ADR-079; generalizes HOLODEX-130/ADR-057's single logo cache
	// to icon/logo/poster). studioImageDir is the on-disk root; the bounds guard
	// untrusted uploads like personImage's. Zero studioImageDir leaves the image
	// endpoints returning 404/503 (the SPA renders its own fallback).
	studioImageDir      string
	studioImageMaxBytes int64
	studioImageMaxDim   int

	// Film images (F56/HOLODEX-280, ADR-086; poster/thumb). filmImageDir is the
	// on-disk root; the bounds guard untrusted uploads like studioImage's. Zero
	// filmImageDir leaves the image endpoints returning 404/503 (the SPA renders its
	// own fallback).
	filmImageDir      string
	filmImageMaxBytes int64
	filmImageMaxDim   int

	// Self-hosted provider brand icon (HOLODEX-134, ADR-059). providerIconDir is the
	// on-disk root; providerIconMaxDim bounds the downscale. Zero providerIconDir leaves
	// the icon serve route returning 404 (the SPA renders the monogram) and disables the
	// cache. One icon per provider, keyed by the provider name.
	providerIconDir    string
	providerIconMaxDim int

	// Soft-delete + purge (F24, ADR-037). purger executes purge-now; deleteGrace
	// drives the Trash view's purge_at. Both optional — nil purger disables only
	// purge-now (soft-delete/restore/Trash still work).
	purger      purger
	deleteGrace time.Duration

	// cardLayout is the operator's preferred card aspect ratio ("wide" or "poster"),
	// surfaced via /capabilities so all visitors see a consistent grid presentation.
	cardLayout string
	// customTheme is the owner's configured palette (F67 S3); nil until config wires it.
	// Read by themePayload for /capabilities and PUT /admin/theme.
	customTheme *ThemeCustom

	// filmsEnabled gates the Films entity (F56, ADR-085); default false. Surfaced
	// via /capabilities so the SPA knows whether to render films routes/nav at all.
	filmsEnabled bool

	// defaultSource is the F36 undecided source-of-truth mode ("file" | "mapping",
	// ADR-051/RD4). It feeds resolver.Options so an undecided field resolves
	// file-first by default; empty means file-first.
	defaultSource string

	// providerTrustOrder ranks providers for the undecided winner among them on a
	// replace field (F36 P1-2, ADR-051 §8). Fed into resolver.Options alongside
	// defaultSource; empty means mapping order among providers.
	providerTrustOrder []string

	// now is the read-path clock for the F45 derived-field pass (ADR-063 §D5).
	// Injected here so the resolver's Derive post-pass stays pure — the resolver
	// package never reads the clock. Defaults to time.Now in NewHandlers; tests
	// override it (via clock) for deterministic Age values. Use h.clock(), which is
	// nil-safe for handlers built by a struct literal.
	now func() time.Time
}

// NewHandlers wires the REST handlers. thumbs, sc, and m are optional (nil-safe):
// they disable thumbnail bumping, admin rescan, and search instrumentation
// respectively in tests or health-only mode.
func NewHandlers(r *repo.Repo, log *slog.Logger, thumbs thumbnailer, thumbDir string, sc rescanner, m searchMetrics) *Handlers {
	return &Handlers{repo: r, log: log, thumbs: thumbs, thumbDir: thumbDir, scanner: sc, metrics: m, now: time.Now}
}

// clock returns the read-path time for the F45 derived-field pass (ADR-063 §D5),
// falling back to time.Now when now is unset (a struct-literal handler in a test).
func (h *Handlers) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

// SetNow overrides the read-path clock (F45, ADR-063 §D5). Used by tests to make the
// time-varying derived fields (Age) deterministic; production leaves the NewHandlers
// default (time.Now).
func (h *Handlers) SetNow(now func() time.Time) { h.now = now }

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

// SetExtraction wires the filename-extraction triggers (F48.5, ADR-067): the
// per-video orchestrator (on-demand, F48.5a), the library-wide batch runner
// (F48.5b), and the pattern-list config store (F48.1a) as a direct field
// alongside mappings/enrich. orch/batch nil-safe — either left nil disables
// its endpoint (503). Called once at startup before serving.
func (h *Handlers) SetExtraction(orch *extract.Orchestrator, batch *extract.BatchRunner) {
	h.extract = orch
	h.extractBatch = batch
	if orch != nil {
		h.patterns = orch.Patterns
	}
}

// SetSweep wires the entity refresh sweep runner (F67, ADR-103 D8). Called once
// at startup before serving; nil-safe.
func (h *Handlers) SetSweep(s *enrich.SweepRunner) { h.sweep = s }

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

// SetStudioImages wires studio image storage (F51, ADR-079): the on-disk root, the
// upload bounds, and the downscale bound — mirrors SetPersonImages. An empty dir
// leaves the serve routes returning 404 (the SPA renders its own fallback) and
// uploads failing closed. Called once at startup.
func (h *Handlers) SetStudioImages(dir string, maxBytes int64, maxDim int) {
	h.studioImageDir = dir
	h.studioImageMaxBytes = maxBytes
	h.studioImageMaxDim = maxDim
}

// SetFilmImages wires film image storage (F56/HOLODEX-280, ADR-086): the on-disk
// root, the upload bounds, and the downscale bound — mirrors SetStudioImages. An
// empty dir leaves the serve routes returning 404 (the SPA renders its own fallback)
// and uploads failing closed. Called once at startup.
func (h *Handlers) SetFilmImages(dir string, maxBytes int64, maxDim int) {
	h.filmImageDir = dir
	h.filmImageMaxBytes = maxBytes
	h.filmImageMaxDim = maxDim
}

// SetProviderIcons wires the self-hosted provider brand-icon store (HOLODEX-134,
// ADR-059): the on-disk root and the downscale bound. An empty dir leaves the icon
// serve route returning 404 (the SPA renders the monogram) and disables the icon cache.
// Called once at startup.
func (h *Handlers) SetProviderIcons(dir string, maxDim int) {
	h.providerIconDir = dir
	h.providerIconMaxDim = maxDim
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

// SetFilmsEnabled wires the Films entity flag (F56, ADR-085). Called once at
// startup before serving; route registration, search/MCP exclusion, and the film
// resolver source injection point all read this via capabilities()/direct field
// access rather than re-deriving it from config.
func (h *Handlers) SetFilmsEnabled(enabled bool) {
	h.filmsEnabled = enabled
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
		ImageURLAllowed:    h.imageURLAllowed,
	}
}

// imageURLAllowed is the nil-safe ResolveFields gate (HOLODEX-212): with no
// enrichment service wired, no provider-sourced image_url is trusted.
func (h *Handlers) imageURLAllowed(provider, rawURL string) bool {
	return h.enrich != nil && h.enrich.ImageURLAllowed(provider, rawURL)
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
	// Detail-page poster tier (F53, HOLODEX-253) — public read, mirrors
	// /thumbnail's posture exactly; falls back to thumbnail bytes (servePoster).
	r.Get("/media/{id}/poster", h.servePoster)
	r.Get("/studios", h.listStudios)
	r.Get("/studios/{id}", h.getStudio)
	// Films (F56, ADR-085) — unregistered entirely when films_enabled is off,
	// per spec: not merely hidden, the routes don't exist. Mutations gated below.
	if h.filmsEnabled {
		r.Get("/films", h.listFilms)
		r.Get("/films/{id}", h.getFilm)
		// Film images (F56/HOLODEX-280, ADR-086): the on-disk normalized JPEG for a
		// filled role, or 404 (the SPA renders its own fallback). Public read, gated
		// off with the rest of the films surface when films_enabled is off; mutations
		// are gated below.
		r.Get("/films/{id}/images/{role}", h.serveFilmImage)
	}
	// Studio images (F51, ADR-079): the on-disk normalized JPEG for a filled role, or
	// 404 (the SPA renders its own fallback). Public read; mutations are gated below.
	r.Get("/studios/{id}/images/{role}", h.serveStudioImage)
	// Provider directory + brand icons (HOLODEX-134, ADR-059) — PUBLIC: provenance
	// badges render for everyone but can't reach the owner-gated /enrich/sources, so the
	// visitor path resolves a provider name to its icon here. Exposes only names /
	// entity types / icon URLs (provider names are already visitor-visible via
	// provenance). The icon is the on-disk normalized JPEG, or 404 → monogram.
	r.Get("/providers", h.listProviders)
	r.Get("/providers/{name}/icon", h.serveProviderIcon)
	r.Get("/people", h.listPeople)
	r.Get("/people/{id}", h.getPerson)
	// Person images (F25, ADR-038) — public reads: a filled role serves the on-disk
	// JPEG, an empty role the themed placeholder SVG. Mutations are gated below.
	r.Get("/people/{id}/image/{role}", h.servePersonImageByRole)
	r.Get("/people/{id}/images", h.getPersonImages)
	r.Get("/people/{id}/images/{imageId}", h.servePersonImageByID)
	r.Get("/tags", h.listTags)
	r.Get("/tags/{id}", h.getTag)
	// Tag Categories (HOLODEX-240, ADR-078) — public reads; mutations gated below.
	h.mountCategories(r)
	h.mountPlaylists(r) // visibility-filtered reads (F69, ADR-104 D5)
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
		r.Get("/admin/activity/digest", h.adminActivityDigest)
		r.Get("/admin/activity/history", h.adminActivityHistory)
		// Dismiss handled failures from the digest (HOLODEX-416, ADR-100).
		r.Post("/admin/activity/runs/{id}/dismiss", h.adminDismissJobRun)
		r.Post("/admin/activity/failures/dismiss", h.adminDismissJobFailures)
		r.Post("/admin/rescan", h.adminRescan)
		r.Post("/admin/reload-config", h.adminReloadConfig)
		r.Put("/admin/theme", h.adminSetTheme)
		// Filename extraction — library-wide batch trigger (F48.5b, ADR-067).
		r.Post("/admin/extract-all", h.adminExtractAll)
		// Entity refresh sweep — one background pass per kind (F67, ADR-103 D8).
		r.Post("/admin/enrich/sweep/{kind}", h.adminEnrichSweep)
		// Metadata source plugins — People enrichment (F22, ADR-033).
		h.mountEnrich(r)
		// Person aliases — owner-curated alternate names (F23, ADR-036).
		h.mountAliases(r)
		// Person images — owner-gated upload/delete/promote/reorder (F25, ADR-038).
		h.mountPersonImages(r)
		// Studio images — owner-gated upload/delete for icon/logo/poster (F51, ADR-079).
		h.mountStudioImages(r)
		// Films — create + attach/detach/bulk-attach (F56, ADR-085); unregistered
		// entirely when films_enabled is off, mirroring the public routes above.
		if h.filmsEnabled {
			h.mountFilms(r)
			// Film images — owner-gated upload/delete for poster/banner (F56/HOLODEX-280,
			// ADR-086; banner replaced thumb in F59/ADR-089 D4).
			h.mountFilmImages(r)
		}
		// Video poster — owner-gated upload/remove, a new tier on the existing
		// thumbnail pipeline (F52, HOLODEX-252).
		h.mountVideoPoster(r)
		// Media soft-delete / purge-now / restore / Trash (F24, ADR-037).
		h.mountDelete(r)
		// Metadata writeback — embed enriched values into media files (F28, ADR-041).
		h.mountWriteback(r)
		// Value-level metadata curation — manual add/suppress/nowrite (F30, ADR-048).
		h.mountCuration(r)
		// Per-field source-of-truth decisions — pin file/provider/manual (F36, ADR-051).
		h.mountDecisions(r)
		// Not-applicable facet exclusions for the completeness score (F55, ADR-081).
		h.mountFacetNotApplicable(r)
		// Missing-facet filter chip options + live counts, for browse sort/filter
		// (F55.6, ADR-081 D4).
		r.Get("/completeness/facets", h.completenessFacets)
		// Facet-first remediation queue — grouped by missing facet, split
		// candidate-ready/needs-research (F55.7, ADR-081 D4).
		r.Get("/owner/completeness-queue", h.completenessQueue)
		// In-app field promotion — promote an auto-registered field to curatable (F44, ADR-062).
		h.mountFieldPromotions(r)
		h.mountFieldClaims(r)
		// People on the unified model — person decisions/curation + rename (F37).
		h.mountPersonDecisions(r)
		// Studio on the unified model — studio decisions/curation (F38, ADR-053).
		h.mountStudioDecisions(r)
		// Studio + tag name-identity — alias/merge/rename over the shared spine (F43, ADR-061).
		h.mountStudioTagIdentity(r)
		// Near-miss review queue — the owner's Duplicates tab (F43 S5, ADR-061).
		h.mountDuplicates(r)
		// Tag deny-list — the owner's Deny-list tab (F50, ADR-075 D2).
		h.mountTagDenylist(r)
		// Tag hierarchy — the owner's /tags pill-menu "set parent" action (F50, ADR-075 D1).
		h.mountTagHierarchy(r)
		// Tag writeback exclusion — per-tag Genre writeback flag + manual sync (HOLODEX-239, ADR-077).
		h.mountTagWritebackSync(r)
		// Video↔tag attach/detach — the owner's media-page add/remove tag chips (F50, ADR-075 P0-7).
		h.mountVideoTags(r)
		// Tag Categories — CRUD + member-tag assign/unassign (HOLODEX-240, ADR-078).
		h.mountCategoryMutations(r)
		h.mountPlaylistMutations(r)
		// Per-item forced re-extract + re-enrich (F31, ADR-047).
		r.Post("/media/{id}/refresh", h.refreshMedia)
		// Filename extraction — on-demand single-video trigger (F48.5a, ADR-067).
		r.Post("/media/{id}/extract", h.extractMedia)
		// Filename extraction review queue — list/resolve/dismiss (F48.6, ADR-067).
		h.mountExtractionReview(r)
	})
}

// listMedia handles GET /media with filters (F4). Query params:
//
//	q, person (repeatable), tag (repeatable), duration_min/max (minutes),
//	resolution (SD|HD|FHD|4K), year_min/max, sort, limit, offset.
//
// sort (F12.1) is one of title_asc|title_desc|added_asc|added_desc|
// duration_asc|duration_desc|resolution_asc|resolution_desc|completeness_asc|
// completeness_desc; default added_desc. completeness_asc/desc, and the
// repeatable missing_facet param, are owner-only (F55.5/F55.6) — score is an
// owner curation signal, never public library metadata (spec "Access control &
// security"), so a non-owner request using either is rejected rather than
// silently ignored. Both are ordinary SQL over the materialized store (ADR-099
// D2/D3), so they page like every other sort. For the owner, every item also
// carries `completeness` (F65.5) — drained first so the badge is current.
func (h *Handlers) listMedia(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := h.videoFilterFromQuery(q)
	f.HideFullFilmVideos = h.filmsEnabled
	f.MissingFacets = q["missing_facet"]
	if wantsCompleteness(f.Sort, f.MissingFacets) && !h.requireOwnerInline(w, r) {
		return
	}
	isOwner := h.auth.authorized(r)
	if isOwner {
		h.drainCompleteness(r.Context())
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
	h.applyPartsTo(r.Context(), items)
	if isOwner {
		if err := h.attachVideoCompleteness(r.Context(), items); err != nil {
			h.fail(w, "list media", err)
			return
		}
	}
	redactFileMetadataForVisitors(items, isOwner)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": f.Limit, "offset": f.Offset,
	})
}

// videoFilterFromQuery builds a VideoFilter from GET /media's query params.
// Factored out of listMedia so /completeness/facets (F55.6) can compute its
// missing-facet counts against the exact same filtered subset a completeness
// sort/filter request on /media would score — the two must never disagree.
// Also reused by the film→video-candidates picker (film_videos.go), which
// must still surface full-film videos (e.g. to flag "already attached"
// conflicts) — so it deliberately leaves HideFullFilmVideos unset; callers
// that want RD6 hiding (browse, completeness facets) set it themselves.
func (h *Handlers) videoFilterFromQuery(q url.Values) repo.VideoFilter {
	f := repo.VideoFilter{
		Query:          q.Get("q"),
		PersonIDs:      parseIDs(q["person"]),
		TagIDs:         parseIDs(q["tag"]),
		StudioIDs:      parseIDs(q["studio_id"]),
		CategoryIDs:    parseIDs(q["category_id"]),
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
	return f
}

// completenessFacets handles GET /completeness/facets?entity_type=video|
// person|studio (F55.6): the "Missing facet" filter chip's option list, with
// a missing-count per facet from entity_completeness_missing (F65.7) — the
// same rows the corresponding list's missing_facet filter selects on, so the
// chip's counts can never disagree with what selecting a facet actually
// filters to. video additionally accepts /media's other filter params (q, tag,
// person, ...) so the counts reflect the caller's current browse filters, not
// the whole library. Owner-only: mounted in the requireOwner group (Mount).
func (h *Handlers) completenessFacets(w http.ResponseWriter, r *http.Request) {
	entityType := r.URL.Query().Get("entity_type")
	switch entityType {
	case model.EnrichEntityVideo, model.EnrichEntityPerson, model.EnrichEntityStudio:
	default:
		writeError(w, http.StatusBadRequest, "entity_type must be video, person, or studio")
		return
	}
	h.drainCompleteness(r.Context())
	var counts []repo.MissingFacetCount
	var err error
	if entityType == model.EnrichEntityVideo {
		vf := h.videoFilterFromQuery(r.URL.Query())
		vf.HideFullFilmVideos = h.filmsEnabled
		counts, err = h.repo.MissingFacetCountsForVideos(r.Context(), vf)
	} else {
		counts, err = h.repo.MissingFacetCounts(r.Context(), entityType)
	}
	if err != nil {
		h.fail(w, "completeness facets", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"facets": facetSummaries(counts)})
}

// setThumbnailURL fills ThumbnailURL when an image exists on disk (ADR-009). The
// ?v= token is the file mtime (Unix seconds): it changes whenever the source file
// is rewritten (e.g. a metadata writeback that embeds new cover art), so the grid
// and detail page fetch a never-before-seen URL instead of a stale browser-cached
// copy. Paired with the endpoint's no-cache header for revalidation.
// redactFileMetadataForVisitor blanks the technical file fields (codec, container,
// bitrate, on-disk path) for a non-owner caller (F52, HOLODEX-249's owner-mode
// editing bundle): "hide file metadata unless in owner mode" must hold at the API
// response, not just the SPA render — a visitor reading the JSON directly (or the
// list/detail response before the SPA gates its own display) must not see it
// either. video_codec/audio_codec/bitrate_kbps/container all carry `omitempty`, so
// zeroing them drops the key entirely rather than serializing a misleading zero.
func redactFileMetadataForVisitor(v *model.Video, isOwner bool) {
	if isOwner {
		return
	}
	v.VideoCodec = ""
	v.AudioCodec = ""
	v.BitrateKbps = 0
	v.Container = ""
	v.FilePath = ""
}

// redactFileMetadataForVisitors applies redactFileMetadataForVisitor to every video
// in a slice — every handler that serializes a []model.Video (not just
// listMedia/getMedia) must call this so a non-owner can't reach file metadata
// through /related, /people/{id}, /tags/{id}, /studios/{id}, or /search instead.
func redactFileMetadataForVisitors(items []model.Video, isOwner bool) {
	if isOwner {
		return
	}
	for i := range items {
		redactFileMetadataForVisitor(&items[i], false)
	}
}

// redactWritebackStatusForVisitor blanks a video's writeback failure message for a
// non-owner (ADR-091/HOLODEX-323 spec R2.1a) — the same "hide it at the API response,
// not just the SPA render" posture as redactFileMetadataForVisitor just above, for the
// same reason: every failure path in internal/writeback/writeback.go embeds absolute
// filesystem paths in this message. Pending/Failed stay untouched; the booleans
// disclose nothing. GetMedia is the one call site that computes a
// repo.VideoWritebackStatus today, so this is a named, greppable guard rather than an
// inline conditional a future second call site could add without noticing.
func redactWritebackStatusForVisitor(status *repo.VideoWritebackStatus, isOwner bool) {
	if isOwner {
		return
	}
	status.Error = ""
}

func (h *Handlers) setThumbnailURL(v *model.Video) {
	v.PosterUploaded = v.ThumbnailState == model.ThumbnailUploaded
	if !model.HasThumbnailImage(v.ThumbnailState) {
		return
	}
	// The ?v= token is the served image file's own mtime (nanoseconds), so the URL
	// changes whenever the bytes do — a poster upload, a regenerate, or extracted
	// cover art all overwrite {id}.jpg / {id}-poster.jpg in place. Versioning off
	// the video's mtime (as before HOLODEX-415) left the URL identical across every
	// one of those, so the grid depended entirely on no-cache revalidation, which
	// http.ServeContent resolves at one-second granularity: an overwrite in the same
	// second as the cached Last-Modified answered 304 with the old bytes. Fall back
	// to the video's mtime when the file can't be stat'd (the state says an image
	// exists, so this is the rare in-flight or out-of-band case), and the poster
	// falls back to the thumbnail exactly as servePoster does.
	var fallback int64
	if !v.FileMtime.IsZero() {
		fallback = v.FileMtime.Unix()
	}
	thumbVer := imageVersion(thumbnail.ThumbPath(h.thumbDir, v.ID), fallback)
	posterVer := imageVersion(thumbnail.PosterPath(h.thumbDir, v.ID), thumbVer)
	v.ThumbnailURL = fmt.Sprintf("/api/v1/media/%d/thumbnail?v=%d", v.ID, thumbVer)
	v.PosterURL = fmt.Sprintf("/api/v1/media/%d/poster?v=%d", v.ID, posterVer)
}

// imageVersion is the file's mtime in nanoseconds, or fallback when it can't be stat'd.
func imageVersion(path string, fallback int64) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return fallback
	}
	return info.ModTime().UnixNano()
}

// prepareThumbnails sets the serving URL on each video and enqueues never-attempted
// covers at high priority (Tier 3, ADR-009). Previously-failed items are left to the
// startup sweep so a broken file isn't re-attempted on every browse.
func (h *Handlers) prepareThumbnails(videos []model.Video) {
	var pending []int64
	for i := range videos {
		h.setThumbnailURL(&videos[i])
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
	h.setThumbnailURL(v)
	authorized := h.auth.authorized(r)
	redactFileMetadataForVisitor(v, authorized)
	// Films section (F56, design handoff §3a): fetched once, ahead of both consumers —
	// the resolver-source injection below (mappings path only) and the response's
	// "films" field (always) — avoiding two identical DB round-trips for the same
	// video (same principle as the enrichment-rows fetch a few lines down). Non-nil so
	// a video with no film link marshals "films": [], never null (HOLODEX-275
	// precedent, same as studios below). Gated on filmsEnabled: reads are suppressed,
	// never destructive, when the flag is off.
	films := []repo.FilmAttachment{}
	if h.filmsEnabled {
		if fa, ferr := h.repo.FilmsForVideo(r.Context(), id); ferr != nil {
			h.log.Warn("films for media detail", "id", id, "err", ferr)
		} else {
			// Posters are loaded here rather than inside FilmsForVideo(s): this is the only
			// caller that renders one. A missing poster degrades to the SPA's monogram
			// plate, so a failure here warns and serves the attachments anyway.
			if perr := h.repo.AttachFilmPosters(r.Context(), fa); perr != nil {
				h.log.Warn("film posters for media detail", "id", id, "err", perr)
			}
			setFilmAttachmentPosterURLs(fa)
			// append, not assign: FilmsForVideo returns a NIL slice for a video with no
			// attachments (its map simply has no entry), and assigning that would undo the
			// non-nil initializer above and emit "films": null -- exactly what the comment
			// there promises it won't. film_videos.go's candidate picker already normalizes
			// the same nil for the same reason.
			films = append(films, fa...)
		}
	}
	var fields []mapping.Resolved
	var resolved []resolver.ResolvedField
	var enriched []model.EnrichedField
	var mfields []mapping.Field
	var links []ExternalLink
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
			// Film resolver source (F56, ADR-085 §4/§5): injecting synthetic
			// "film:<id>" candidates is the whole of films_enabled's read-suppression
			// -- when off, `films` above was never fetched and this is a no-op, and
			// any standing per-field decision on a film source falls through to the
			// existing decided-but-currently-unmatched-provider path (empty, not
			// re-derived).
			if h.filmsEnabled {
				enr = injectFilmSources(enr, films)
			}
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
			var promoted map[string]bool
			mfields, promoted = h.mergePromotions(r.Context(), model.EnrichEntityVideo, m.Fields(), enrichRows)
			mfields = h.mergeClaims(r.Context(), model.EnrichEntityVideo, mfields)
			opts := h.resolveOptions(dec)
			// ADR-101: the write ledger is the sync witness for image fields (nothing reads
			// cover art back). Loaded only here, on the detail the writeback dialog reads —
			// a failure degrades to "unknown", the pre-ADR-101 posture, never to "differs".
			if written, wErr := h.repo.LastWrittenValues(r.Context(), id); wErr != nil {
				h.log.Warn("last written values for detail", "id", id, "err", wErr)
			} else {
				opts.LastWritten = written
			}
			resolved = resolver.Resolve(v, extra, enr, cur, mfields, opts)
			h.markPromoted(resolved, promoted)
			resolved = h.appendAutoRegistered(r.Context(), enrichRows, mfields, resolved)
			// P0-10 (F50, ADR-075 RD9): show the actual genre-writeback union (tags +
			// deny-filtered raw genres), not the plain provider/file merge above — see
			// applyGenreWriteback for why a video tagged only manually otherwise never
			// gets a "genres" row to write back at all. Reuses the "genres" entry the
			// resolve pass above already produced (genreWritebackItemsFrom) rather than
			// re-resolving it from scratch, which would repeat the same enrichment/
			// curation/decision queries and a second full resolver.Resolve pass.
			if field, ok := m.ByCanonical("genres"); ok {
				rawGenres, rawOK := resolvedByCanonical(resolved, "genres")
				if items, gerr := h.genreWritebackItemsFrom(r.Context(), v.ID, rawGenres, rawOK); gerr != nil {
					h.log.Warn("genre writeback items for detail", "id", id, "err", gerr)
				} else {
					resolved = applyGenreWriteback(resolved, field, items)
				}
			}
			// HOLODEX-216: last mutation of `resolved` before the response is built, so
			// every row present in the final slice — including a "genres" row appended by
			// applyGenreWriteback above — gets a WriteTarget stamp. Stamping before either
			// append (as an earlier draft did) leaves append-only rows permanently
			// unwritable in the dialog regardless of whether they actually have a mapping.
			h.markWriteTargets(resolved, v.Container)
			// The summary's `part` (HOLODEX-389) rides the same model.Video the lists
			// stamp, so the detail's `video` object carries it too rather than only
			// the resolved[] row — one field, present on every payload of the type.
			if rf, ok := resolvedByCanonical(resolved, "part"); ok && len(rf.Values) > 0 {
				v.Part = rf.Values[0]
			}
			if h.enrich != nil {
				enriched = h.enrich.FieldsFromRows(enrichRows)
			}
			// HOLODEX-394 (F63 P0-7, ADR-098 D4): the header pill from the winning
			// external_provider_id, over the enrichment rows fetched above.
			links = h.externalLinksForVideo(r.Context(), resolved, enrichRows)
		}
	} else if h.enrich != nil {
		enriched = h.videoEnrichment(r, id)
	}
	// Studio entities linked to this video (F38, ADR-053): the resolved studio
	// value links to its /studios/{id} page, and the link target always matches the
	// displayed value because video_studios is derived from that same resolution.
	// Non-nil so a video with no studio link marshals "studios": [], never null
	// (HOLODEX-275) — StudiosForVideos omits any such video from its map entirely.
	studios := []model.Studio{}
	if byVideo, serr := h.repo.StudiosForVideos(r.Context(), []int64{id}); serr != nil {
		h.log.Warn("studios for media detail", "id", id, "err", serr)
	} else if s := byVideo[id]; s != nil {
		studios = s
	}
	for i := range studios {
		setStudioImageURLs(&studios[i])
	}
	// enrich_queries only ever feeds the owner-only Enrich picker (EnrichPicker.svelte
	// is gated behind isOwner client-side) — skip rendering it for a visitor request,
	// who would only ever discard it.
	var enrichQueries map[string]string
	var completeness *resolver.Completeness
	if authorized {
		enrichQueries = h.buildVideoQueries(v, resolved)
		if mfields != nil {
			na, naErr := h.repo.FacetsNotApplicableForEntity(r.Context(), model.EnrichEntityVideo, id)
			if naErr != nil {
				h.log.Warn("facets not applicable for detail", "id", id, "err", naErr)
				na = map[string]bool{}
			}
			c := resolver.Complete(mfields, resolved, na)
			completeness = &c
			h.selfHealCompleteness(r.Context(), model.EnrichEntityVideo, id, c)
		}
	}
	// Writeback status (ADR-091, HOLODEX-323, spec R2.1): per-video pending/failed
	// state read from the queue rather than a client-held job id, so it survives
	// reload, another tab, and a restart. redactWritebackStatusForVisitor (R2.1a)
	// strips the raw error for a non-owner — every writeback.WriteBatch failure
	// path embeds absolute filesystem paths, the same class of exposure
	// redactFileMetadataForVisitor guards against just above.
	wbStatus, wbErr := h.repo.GetVideoWritebackStatus(r.Context(), id)
	if wbErr != nil {
		h.log.Warn("writeback status for detail", "id", id, "err", wbErr)
	}
	redactWritebackStatusForVisitor(&wbStatus, authorized)

	// Playlists this video belongs to (F69 spec P0-7, the rail's PLAYLISTS row):
	// visibility-filtered like every playlist read (ADR-104 D5) — a visitor sees the
	// public ones only. Non-nil so it marshals `[]`, never null (HOLODEX-275).
	playlists, plErr := h.repo.PlaylistsForVideo(r.Context(), id, !authorized)
	if plErr != nil {
		h.log.Warn("playlists for media detail", "id", id, "err", plErr)
		playlists = []model.Playlist{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"video":            v,
		"playlists":        playlists,
		"metadata":         extra,
		"fields":           fields,
		"resolved":         resolved,
		"enriched":         enriched,
		"studios":          studios,
		"films":            films,
		"enrich_queries":   enrichQueries,
		"completeness":     completeness,
		"writeback_status": wbStatus,
		"external_links":   links,
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
	related, err := h.repo.Related(r.Context(), id, 5, h.filmsEnabled)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	if err != nil {
		h.fail(w, "related media", err)
		return
	}
	// Same cover-art treatment as the grid, per shelf.
	isOwner := h.auth.authorized(r)
	for _, shelf := range []*repo.RelatedShelf{related.Person, related.Tag} {
		if shelf != nil {
			h.prepareThumbnails(shelf.Items)
			h.applyPartsTo(r.Context(), shelf.Items)
			redactFileMetadataForVisitors(shelf.Items, isOwner)
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
	h.serveImageFile(w, r, id, "thumbnail", thumbnail.ThumbPath(h.thumbDir, id))
}

// serveImageFile serves the first candidate path that exists on disk, after
// confirming the video is visible (not soft-deleted, F24/ADR-037 §4 — the only
// way to reach a hidden one is a guessed id, but the bytes stay consistent
// with the 404 its detail/stream now return). Shared by serveThumbnail (one
// candidate) and servePoster (poster-tier then thumbnail-tier fallback,
// P0-6/F53/HOLODEX-253) so both id-keyed public static-file reads agree on the
// visibility check, the no-cache posture, and the not-ready contract the
// frontend's retry loop relies on — they differ only in which path(s) they
// try and the label in their error/log messages.
func (h *Handlers) serveImageFile(w http.ResponseWriter, r *http.Request, id int64, label string, candidates ...string) {
	if visible, err := h.repo.VideoVisible(r.Context(), id); err != nil {
		h.fail(w, label+" visibility", err)
		return
	} else if !visible {
		writeError(w, http.StatusNotFound, label+" not ready")
		return
	}
	var f *os.File
	var err error
	for _, path := range candidates {
		if f, err = os.Open(path); err == nil {
			break
		}
	}
	if err != nil {
		writeError(w, http.StatusNotFound, label+" not ready")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, label+" not ready")
		return
	}
	// no-cache so the browser always revalidates. http.ServeContent sets Last-Modified
	// and handles If-Modified-Since, so unchanged images return 304 (no bytes
	// transferred). The ?v= the model emits (setThumbnailURL) is the file's own mtime
	// and changes on every overwrite, so a long max-age would be safe in principle —
	// no-cache stays as the belt to that brace, for any write path that reaches the
	// file without going through a handler that re-reads the model.
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
	// The field list is a completeness denominator (ADR-099 D4): every stored
	// score is suspect once the mappings change, so flag them all for recompute.
	if err := h.repo.MarkAllCompletenessDirty(r.Context()); err != nil {
		h.fail(w, "reload config", err)
		return
	}
	// Reload the filename-pattern list alongside the mappings (F48.1a, ADR-067)
	// so an edited metadata-patterns.yaml takes effect without a restart.
	if h.patterns != nil {
		if err := h.patterns.Reload(); err != nil {
			h.fail(w, "reload config", err)
			return
		}
	}
	// Reload the provider registry alongside the mappings (F22.2d) so both config
	// files take effect without a restart.
	if h.enrich != nil {
		if err := h.enrich.Store().Reload(); err != nil {
			h.fail(w, "reload sources", err)
			return
		}
		// Re-sync provider brand icons to the reloaded registry (ADR-059): a newly added
		// provider gets its icon, a removed one is pruned. Off the request path — a
		// provider describe/fetch must not block the reload response — and best-effort.
		go h.RefreshProviderIcons(context.Background())
	}
	// Re-check the write/read-back pairing against the mapping that just went live
	// (ADR-093 D5) — this is the edit where an operator closes or opens such a gap, so
	// it is the moment the warning is most actionable. Logged only once every reload
	// above has succeeded, so the advice never accompanies a reload the operator was
	// told had failed.
	writeback.LogReadbackGaps(h.log, h.mappings.Current().Fields())
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
			// v != nil guards against a stale pre-HOLODEX-275 (or rolled-back) cache
			// entry that was marshaled from a nil slice as literal JSON `null` —
			// json.Unmarshal("null", &v) succeeds but leaves v nil, which would
			// otherwise bypass FacetValues's non-nil guarantee for the entry's TTL.
			if json.Unmarshal(b, &v) == nil && v != nil {
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

// listPeople handles GET /people (F19): name-sorted (or count-sorted, or
// completeness-sorted/filtered) people with active-video counts. sort=
// completeness_asc|completeness_desc and the repeatable missing_facet param
// are owner-only (F55.5/F55.6), same posture as listMedia; both read the
// materialized store in SQL (ADR-099 D2/D3), and the owner's items carry
// `completeness` (F65.5).
func (h *Handlers) listPeople(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := repo.NamedListFilter{Sort: q.Get("sort"), MissingFacets: q["missing_facet"]}
	if wantsCompleteness(f.Sort, f.MissingFacets) && !h.requireOwnerInline(w, r) {
		return
	}
	isOwner := h.auth.authorized(r)
	if isOwner {
		h.drainCompleteness(r.Context())
	}
	people, err := h.repo.ListPeopleFiltered(r.Context(), f)
	if err != nil {
		h.fail(w, "list people", err)
		return
	}
	if isOwner {
		if err := h.attachPersonCompleteness(r.Context(), people); err != nil {
			h.fail(w, "list people", err)
			return
		}
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
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{PersonIDs: []int64{id}, Limit: 500, HideFullFilmVideos: h.filmsEnabled})
	if err != nil {
		h.fail(w, "person videos", err)
		return
	}
	authorized := h.auth.authorized(r)
	h.applyPartsTo(r.Context(), items)
	redactFileMetadataForVisitors(items, authorized)
	resolved, fields := h.personResolve(r, id, p)
	images := h.personImageSet(r, id) // F25: per-role presence + version + gallery
	var completeness *resolver.Completeness
	if authorized {
		na, naErr := h.repo.FacetsNotApplicableForEntity(r.Context(), model.EnrichEntityPerson, id)
		if naErr != nil {
			h.log.Warn("facets not applicable for person detail", "id", id, "err", naErr)
			na = map[string]bool{}
		}
		// photo is delivered as an asset (person_images), never a field value —
		// score it off the headshot role's presence, same signal
		// completenessForPeople uses (F55.13).
		cFields, cResolved := injectSyntheticFacet(fields, resolved, "photo", registry.Lookup("photo").Label,
			images.Roles[model.PersonImageHeadshot].Present)
		// alternate_names lives in the identity spine, not the field model (F58/ADR-088
		// D7). GetPerson already loaded them for the Aliases panel, so this reuses that
		// read rather than adding one. Blind to `source`: an owner-typed name and a
		// provider-supplied one count the same.
		cFields, cResolved = injectSyntheticFacet(cFields, cResolved, "alternate_names",
			registry.Lookup("alternate_names").Label, len(p.Aliases) > 0)
		c := resolver.Complete(cFields, cResolved, na)
		completeness = &c
		h.selfHealCompleteness(r.Context(), model.EnrichEntityPerson, id, c)
	}
	// HOLODEX-266 (ADR-083): the provider-link badge projection — best-effort, a
	// lookup failure logs and serves the page with no badges rather than failing it.
	links, linksErr := h.externalLinksForEntity(r.Context(), model.EnrichEntityPerson, id, nil)
	if linksErr != nil {
		h.log.Warn("external links for person detail", "id", id, "err", linksErr)
	}
	body := map[string]any{
		"person": p, "items": items, "total": total,
		// F37 (P0-2): the unified resolver payload — record vocabulary, no
		// in_sync. It supersedes the raw F22 enriched[] block, retired here.
		"resolved":       resolved,
		"images":         images,
		"completeness":   completeness,
		"external_links": links,
	}
	if skipped := h.skippedAliases(r, model.EnrichEntityPerson, id, authorized); len(skipped) > 0 {
		body["skipped_aliases"] = skipped
	}
	writeJSON(w, http.StatusOK, body)
}

// skippedAliases fetches the provider names a collision kept off this entity (ADR-088 D5)
// for the Aliases panel's review line. Owner-only, like every other control on that panel,
// and best-effort: the panel losing one advisory line is not worth failing the detail page
// that otherwise rendered fine.
func (h *Handlers) skippedAliases(r *http.Request, entityType string, id int64, authorized bool) []repo.SkippedAlias {
	if !authorized {
		return nil
	}
	skipped, err := h.repo.SkippedAliasesForEntity(r.Context(), entityType, id)
	if err != nil {
		h.log.Warn("skipped aliases for detail", "entity_type", entityType, "id", id, "err", err)
		return nil
	}
	return skipped
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
	items, total, err := h.repo.ListVideos(r.Context(), repo.VideoFilter{TagIDs: []int64{id}, Limit: 500, HideFullFilmVideos: h.filmsEnabled})
	if err != nil {
		h.fail(w, "tag videos", err)
		return
	}
	h.applyPartsTo(r.Context(), items)
	redactFileMetadataForVisitors(items, h.auth.authorized(r))
	writeJSON(w, http.StatusOK, map[string]any{"tag": t, "items": items, "total": total})
}

func (h *Handlers) search(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	res, err := h.repo.Search(r.Context(), r.URL.Query().Get("q"), atoiDefault(r.URL.Query().Get("limit"), 10), h.filmsEnabled)
	if err != nil {
		h.fail(w, "search", err)
		return
	}
	if h.metrics != nil {
		h.metrics.ObserveSearch(time.Since(start))
	}
	h.applyPartsTo(r.Context(), res.Videos)
	redactFileMetadataForVisitors(res.Videos, h.auth.authorized(r))
	writeJSON(w, http.StatusOK, res)
}

// ---- helpers ----

func (h *Handlers) fail(w http.ResponseWriter, op string, err error) {
	h.log.Error("api error", "op", op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

// pathID parses the conventional single-{id} path param.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return urlParamID(w, r, "id")
}

// urlParamID parses a named chi path param as a positive int64 or a `kind:id`
// ref of the route's entity kind (F60 RD1, ref.go), writing 400 and returning
// false otherwise; a kind mismatch names the expected kind. Routes that nest two
// ids (e.g. film_videos.go's {filmId}/{videoId}) name them explicitly; pathID is
// the single-{id} shorthand.
func urlParamID(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	id, err := ParseRef(routeKind(r, param), chi.URLParam(r, param))
	if err != nil {
		msg := "invalid " + param
		var kindErr *RefKindError
		if errors.As(err, &kindErr) {
			msg = kindErr.Error()
		}
		writeError(w, http.StatusBadRequest, msg)
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
//
// A minted seed stays below 2^53: GET /playlists/{id} echoes it as a JSON number
// (F69) and the SPA carries it back in every next-up href, and a JSON consumer
// parses numbers as float64 — a full UnixNano would round on the way back and
// holo_shuffle would walk a different order on the first hop.
func parseSeedOrRandom(s string) int64 {
	if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return n
	}
	return time.Now().UnixNano() & maxJSONSafeInt
}

// maxJSONSafeInt is 2^53-1, the largest integer a float64 (hence any JSON number
// consumer) represents exactly.
const maxJSONSafeInt = 1<<53 - 1

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

// injectFilmSources adds synthetic "film:<id>" resolver-source candidates for a video's
// film attachments (F56, ADR-085 §4): the film name as a "collection" (Album) candidate
// for every attachment, plus a "title" candidate when the file represents the entire
// film. Mirrors enrichmentFromRows' shape so resolveDecided/gather's "film:"-prefixed
// branches (internal/resolver/resolver.go) read it the same way they read a provider's
// enrichment row.
func injectFilmSources(enr resolver.Enrichment, films []repo.FilmAttachment) resolver.Enrichment {
	if len(films) == 0 {
		return enr
	}
	if enr == nil {
		enr = make(resolver.Enrichment, len(films))
	}
	for _, f := range films {
		fields := map[string][]string{"collection": {f.FilmName}}
		if f.IsFullFilm {
			fields["title"] = []string{f.FilmName}
		}
		enr["film:"+strconv.FormatInt(f.FilmID, 10)] = fields
	}
	return enr
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
