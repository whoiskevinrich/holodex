// Command holodex is the single-binary entrypoint: it loads configuration,
// opens the database and applies migrations, then serves the HTTP API and runs
// the background scanner, shutting down gracefully on SIGINT/SIGTERM.
package main

//go:generate rsrc -manifest holodex.manifest -arch amd64 -o holodex_windows_amd64.syso

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/config"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/extract"
	"holodex/internal/imagesink"
	"holodex/internal/mapping"
	"holodex/internal/mcp"
	"holodex/internal/metadata"
	"holodex/internal/metrics"
	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/personorphan"
	"holodex/internal/purge"
	"holodex/internal/refresh"
	"holodex/internal/repo"
	"holodex/internal/scanner"
	"holodex/internal/thumbnail"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

func main() {
	var (
		configPath  = flag.String("config", "", "path to holodex.yaml (optional)")
		migrateOnly = flag.Bool("migrate-only", false, "apply migrations and exit (ADR-016)")
		healthcheck = flag.Bool("healthcheck", false, "probe /healthz and exit 0/1 (Docker HEALTHCHECK)")
		// CLI overrides — highest config precedence (ADR-014, F9.5).
		hostFlag      = flag.String("host", "", "override HOST (bind address; empty = all interfaces, e.g. 127.0.0.1 for loopback-only)")
		portFlag      = flag.Int("port", 0, "override PORT")
		mediaPathFlag = flag.String("media-path", "", "override MEDIA_PATH")
		dataPathFlag  = flag.String("data-path", "", "override DATA_PATH")
		logLevelFlag  = flag.String("log-level", "", "override LOG_LEVEL")
		mcpTransport  = flag.String("mcp-transport", "", "run as an MCP server over this transport instead of the web server; only \"stdio\" is valid (ADR-005)")
	)
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	overrides := config.Overrides{
		Host:      *hostFlag,
		Port:      *portFlag,
		MediaPath: *mediaPathFlag,
		DataPath:  *dataPathFlag,
		LogLevel:  *logLevelFlag,
	}

	// stdio MCP entrypoint (`docker exec -i holodex holodex -mcp-transport stdio`):
	// run only the MCP server over the pipe, never the web server.
	if *mcpTransport == "stdio" {
		if err := runMCPStdio(*configPath, overrides); err != nil {
			fmt.Fprintln(os.Stderr, "fatal:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*configPath, *migrateOnly, overrides); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// runMCPStdio serves the MCP tools over stdin/stdout, sharing the same database
// as the main process (ADR-005). Logs go to stderr because stdout is the
// JSON-RPC pipe — anything written there corrupts the protocol stream.
func runMCPStdio(configPath string, overrides config.Overrides) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	cfg.ApplyOverrides(overrides)

	log := newLogger(cfg.LogLevel, os.Stderr)
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer database.Close()

	mappings, err := mapping.NewStore(cfg.MetadataMappingsPath)
	if err != nil {
		return err
	}

	log.Info("mcp stdio server starting", "database", cfg.DatabasePath)
	auth := api.NewAuth(cfg.AdminToken)
	return mcp.New(repo.New(database), log, mappings, auth, cfg.FilmsEnabled).ServeStdio()
}

// version is the build identifier surfaced in the activity read-model (F21.1).
// Overridable via -ldflags "-X main.version=...".
var version = "dev"

// defaultSkin is the app's default theme (ADR-021), used as the placeholder skin
// label when a person-image request omits ?skin=. The placeholder is token-driven
// so it re-themes via the page's [data-theme] regardless; this is just the label.
const defaultSkin = "cinematheque"

func run(configPath string, migrateOnly bool, overrides config.Overrides) error {
	startedAt := time.Now()
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	cfg.ApplyOverrides(overrides) // CLI > env > yaml > defaults (ADR-014)

	log := newLogger(cfg.LogLevel, os.Stdout)
	log.Info("starting holodex", "port", cfg.Port, "data_path", cfg.DataPath)

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	log.Info("database ready", "path", cfg.DatabasePath)

	if migrateOnly {
		log.Info("migrate-only: done")
		return nil
	}

	repository := repo.New(database)
	repository.SetGalleryCap(cfg.PersonGalleryMax) // per-person gallery cap (F25, PERSON_GALLERY_MAX)

	extractor := metadata.NewExtractor()
	if err := extractor.Available(); err != nil {
		log.Warn("metadata binaries unavailable; scans will error per-file until installed", "err", err)
	}

	// Thumbnail pipeline (ADR-009): shared by the scanner (Tier 1 + new-file
	// enqueue) and the API (Tier 3 priority bump + serving).
	thumbs := thumbnail.New(thumbnail.Config{
		Enabled:      cfg.ThumbnailEnabled,
		Backfill:     cfg.ThumbnailBackfill,
		Workers:      cfg.ThumbnailWorkers,
		Nice:         cfg.ThumbnailNice,
		SeekPercent:  cfg.ThumbnailSeekPercent,
		Width:        cfg.ThumbnailWidth,
		PosterWidth:  cfg.PosterWidth,
		Dir:          cfg.ThumbnailPath,
		FfmpegPath:   "ffmpeg",
		ExiftoolPath: "exiftool",
	}, log, repository)
	if err := thumbs.Available(); err != nil {
		log.Warn("ffmpeg unavailable; thumbnail generation will error until installed", "err", err)
	}

	// Metrics registry (ADR-019/ADR-026, F13): the queue-depth gauge is pulled
	// live from the thumbnail pipeline at scrape time; scan and search durations
	// are pushed by the scanner and handlers below.
	reg := metrics.New()
	reg.SetQueueDepthSource(thumbs.QueueDepth)

	// Configurable metadata field mapping (ADR-013) + facet cache (ADR-008/022).
	cacheBackend := cache.New(cfg.CacheBackend, cfg.CacheMaxMemoryMB)
	mappings, err := mapping.NewStore(cfg.MetadataMappingsPath)
	if err != nil {
		return fmt.Errorf("load metadata mappings: %w", err)
	}
	log.Info("metadata field mappings loaded", "path", cfg.MetadataMappingsPath, "fields", len(mappings.Current().Fields()))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background scanner (ADR-018), constructed before the router so the admin
	// rescan endpoint (F13.3) can drive it. SetBaseContext ties manual rescans to
	// the server lifetime so they stop on shutdown rather than when the request
	// returns.
	sc := scanner.New(scanner.Config{
		MediaPath:      cfg.MediaPath,
		FollowSymlinks: cfg.FollowSymlinks,
		MaxDepth:       cfg.ScanMaxDepth,
		MinAge:         time.Duration(cfg.ScanMinAgeSeconds) * time.Second,
		Workers:        cfg.ScanWorkers,
	}, log, repository, extractor)
	sc.SetThumbnailer(thumbs)
	sc.SetBaseContext(ctx)
	sc.SetMetrics(reg)
	sc.SetJobRecorder(repository) // persist scan history (F21.3, ADR-028)

	// Trim any history past the retention window once at startup (F21.3); a
	// long-idle instance shouldn't wait for its next scan to prune.
	if n, err := repository.PruneJobRuns(ctx); err != nil {
		log.Warn("prune job history failed", "err", err)
	} else if n > 0 {
		log.Info("pruned old job history", "removed", n)
	}

	// Metadata source plugins (F22, ADR-033): a registry of sidecar providers; the
	// service is the only thing that dials them, and only on an owner action.
	sources, err := enrich.NewStore(cfg.MetadataSourcesPath, log)
	if err != nil {
		return fmt.Errorf("load metadata sources: %w", err)
	}
	enrichSvc := enrich.NewService(sources, repository, log)
	log.Info("metadata source providers loaded", "path", cfg.MetadataSourcesPath, "enabled", len(sources.Current().Enabled()))

	// Warn on provider_trust_order names that match no enabled provider (F36 P1-2):
	// an unknown name is inert (it just never ranks any real source), so this is a
	// fail-loud hint for a typo, not an error. Config can't do this check — provider
	// identity is only known here, once the registry is loaded.
	for _, name := range cfg.ProviderTrustOrder {
		if _, ok := sources.Current().ByName(name); !ok {
			log.Warn("provider_trust_order names an unknown or disabled provider; it will be ignored",
				"provider", name)
		}
	}

	// Person images (F25, ADR-038): on-disk store under DATA_PATH/person-images.
	if err := os.MkdirAll(cfg.PersonImagePath, 0o755); err != nil {
		log.Warn("person image dir create failed", "dir", cfg.PersonImagePath, "err", err)
	}
	// Studio images (F51, ADR-079; generalizes HOLODEX-130/ADR-057's single logo cache
	// to icon/logo/poster): on-disk store under DATA_PATH/studio-images.
	if err := os.MkdirAll(cfg.StudioImagePath, 0o755); err != nil {
		log.Warn("studio image dir create failed", "dir", cfg.StudioImagePath, "err", err)
	}
	// Film images (F56/HOLODEX-280, ADR-086; poster/thumb): on-disk store under
	// DATA_PATH/film-images.
	if err := os.MkdirAll(cfg.FilmImagePath, 0o755); err != nil {
		log.Warn("film image dir create failed", "dir", cfg.FilmImagePath, "err", err)
	}
	// One entity-generic sink (internal/imagesink) backs all three: the enrichment
	// asset path and the upload handlers share one normalize+store per entity kind, so
	// a provider photo gets the same metadata strip as an upload, for person, studio,
	// and film alike (F51/ADR-079, widened to Film by F56/ADR-086 — the third real use
	// of this pipeline).
	enrichSvc.SetImageSink(imagesink.New(
		repository, cfg.PersonImagePath, cfg.PersonImageMaxDimension,
		repository, cfg.StudioImagePath, cfg.StudioImageMaxDimension,
		repository, cfg.FilmImagePath, cfg.FilmImageMaxDimension,
	))

	// Self-hosted provider brand icon (HOLODEX-134, ADR-059): on-disk store under
	// DATA_PATH/provider-icons. One normalized icon per provider, a cache of the URL the
	// provider advertises in its /describe brand_icon; refreshed at boot + config-reload.
	if err := os.MkdirAll(cfg.ProviderIconPath, 0o755); err != nil {
		log.Warn("provider icon dir create failed", "dir", cfg.ProviderIconPath, "err", err)
	}

	// One-time content-hash backfill (F34, ADR-050): hash any pre-F34 person images
	// from their on-disk bytes and collapse galleries that already hold duplicates.
	// Idempotent — a no-op once every row is hashed and deduped, so it runs every boot.
	if hashed, removed, err := personimage.Backfill(ctx, repository, cfg.PersonImagePath, log); err != nil {
		log.Warn("person image content-hash backfill failed", "err", err)
	} else if hashed > 0 || removed > 0 {
		log.Info("person image content-hash backfill", "hashed", hashed, "removed_duplicates", removed)
	}

	// One-time identity near-miss backfill (F43/RD10, ADR-061): migration 0022 already
	// folded the pure-case hard pairs before the nameKey unique indexes; this seeds the
	// review queue with the residual fuzzy near-misses (punctuation / internal-whitespace
	// variants) for the owner to confirm — never merging any. Needs only the repo, so it
	// runs here with the other startup backfills, after migrations have landed the spine.
	seedIdentityReviewQueue(ctx, repository, log)

	health := api.NewHealth()
	handlers := api.NewHandlers(repository, log, thumbs, cfg.ThumbnailPath, sc, reg)
	handlers.SetMetadataFields(mappings, cacheBackend)
	handlers.SetEnrichment(enrichSvc)
	// Per-item forced re-extract + re-enrich (F31, ADR-047). The scanner is the
	// forced-extract seam (no change-detection); the repo resolves the target and
	// persists the file layer; the enrich service re-pulls linked providers.
	refreshSvc := refresh.NewService(sc, repository, enrichSvc, log)
	handlers.SetRefresh(refreshSvc)

	// Entity link derivation (F38 studio + F40 person, ADR-053/ADR-072):
	// video_studios/video_people follow their resolved fields. RelinkVideoEntity is
	// the single resolution entry point (studio + person together); wire it into
	// every path that changes a video's resolved value. The enrich/decision/
	// curation triggers live in the handlers themselves.
	sc.SetRelinker(handlers.RelinkVideoEntity)
	refreshSvc.SetRelinker(handlers.RelinkVideoEntity)
	// One-time backfills so promotion doesn't require a manual rescan (ADR-053 §5 /
	// ADR-072 P0-4). Each is gated on its own empty link table, genuinely one-time;
	// idempotent. Person backfill is loss-guarded: it fails loudly (logs, doesn't
	// panic) rather than silently accepting fewer links than raw extraction held
	// pre-cutover (ADR-072 RD9).
	backfillStudioLinks(ctx, repository, handlers.RelinkVideoStudios, log)
	backfillPersonLinks(ctx, repository, handlers.RelinkVideoPeople, log)
	// SSRF-guarded cover-art download (HOLODEX-212, ADR-039): downloadImageToTemp
	// refuses every image-field download until this is wired, so it must be set
	// before either writeback path below can write an image field.
	writeback.SetImageFetcher(enrichSvc.FetchAllowedImage)
	handlers.SetWriteback(writeback.WriteBatch)
	// Durable batch-writeback queue (F30, ADR-048): owner "write to file" actions are
	// enqueued and drained by a bounded worker pool (WRITEBACK_CONCURRENCY, default 1)
	// so bulk curation can't thrash the filesystem. Survives restart; on boot it
	// recovers crash-interrupted jobs and sweeps orphan temp files.
	writeQ := writequeue.New(repository, writeback.WriteBatch, log, cfg.WritebackConcurrency, cfg.MediaPath)
	writeQ.SetPostWrite(func(ctx context.Context, id int64, path string) {
		if thumbs != nil && thumbs.Enabled() {
			if _, err := thumbs.ExtractEmbedded(ctx, id, path); err != nil {
				log.Warn("post-writeback thumbnail re-extract", "id", id, "err", err)
			}
		}
		// Read the file back, unconditionally: it materializes a newly-written
		// Person/Studio (HOLODEX-196 #4) and refreshes the stored file tags that
		// `in_sync` is compared against (HOLODEX-214). See ADR-073, which
		// supersedes ADR-068 D1's entity-only gate.
		if err := refreshSvc.ReExtract(ctx, id); err != nil {
			log.Warn("post-writeback re-extract", "id", id, "err", err)
		}
	})
	handlers.SetWriteQueue(writeQ)

	// Filename extraction (F48, ADR-067): the orchestrator ties pattern
	// matching (F48.1), the filename shadow-store write (F48.2), and
	// scoring+routing (F48.3/F48.4) into the one pipeline every trigger
	// shares (F48.5d). AutoApplyEnabled stays off by default (ADR-067 Action
	// Item 2) until the ADR is Accepted.
	patterns, err := extract.NewPatternStore(cfg.FilenamePatternsPath)
	if err != nil {
		return fmt.Errorf("load filename patterns: %w", err)
	}
	log.Info("filename extraction patterns loaded", "path", cfg.FilenamePatternsPath, "auto_apply", cfg.ExtractionAutoApplyEnabled)
	extraction := &extract.Orchestrator{
		Videos:   repository,
		Mappings: mappings,
		Patterns: patterns,
		Store:    repository,
		Deps: extract.Deps{
			Resolver:         repository,
			ManualSource:     repository,
			Reviews:          repository,
			Queue:            writeQ,
			AutoApplyEnabled: cfg.ExtractionAutoApplyEnabled,
			Log:              log,
		},
	}
	extractBatch := &extract.BatchRunner{Orchestrator: extraction, Videos: repository, Recorder: repository, Log: log}
	extractBatch.SetBaseContext(ctx)
	handlers.SetExtraction(extraction, extractBatch)
	// Import-time trigger (F48.5c): run extraction right after every scan
	// upsert, mirroring SetRelinker's best-effort post-upsert hook shape.
	sc.SetExtractionRunner(func(ctx context.Context, id int64) error {
		_, err := extraction.ExtractVideo(ctx, id)
		return err
	})

	handlers.SetPersonImages(cfg.PersonImagePath, cfg.PersonImageMaxBytes, cfg.PersonImageMaxDimension, defaultSkin)
	// Studio images (F51, ADR-079). No backfill needed here: migration 0036 already
	// carried forward every pre-existing studio_logos row into studio_images(role=
	// 'logo') as part of the schema change itself, unlike ADR-057's original derived-
	// cache backfill (which had to reconstruct state the old table never had).
	handlers.SetStudioImages(cfg.StudioImagePath, cfg.StudioImageMaxBytes, cfg.StudioImageMaxDimension)
	handlers.SetFilmImages(cfg.FilmImagePath, cfg.FilmImageMaxBytes, cfg.FilmImageMaxDimension)
	handlers.SetProviderIcons(cfg.ProviderIconPath, cfg.ProviderIconMaxDimension)
	// Provider brand-icon refresh (ADR-059): fetch/normalize each enabled provider's
	// advertised brand_icon and prune orphans. Off the main path in a bounded goroutine
	// so boot never blocks on a slow/unreachable provider describe; best-effort per
	// provider. Icons also refresh on config-reload; a brand mark is otherwise static.
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		handlers.RefreshProviderIcons(bg)
	}()
	handlers.SetActivity(sc, health, version, startedAt, cfg.MediaPath != "")

	// Soft-delete purge job (F24, ADR-037): a dedicated ticker that hard-deletes
	// items whose grace has expired — independent of the scanner's clock and runs
	// even when scanning is disabled. Records each pass in the activity history.
	purgeCfg := purge.Config{
		Grace:       time.Duration(cfg.DeleteGracePeriodSeconds) * time.Second,
		Interval:    time.Duration(cfg.PurgeIntervalSeconds) * time.Second,
		RemoveFiles: cfg.DeleteRemoveFiles,
	}
	purger := purge.New(repository, purgeCfg, log)
	handlers.SetDelete(purger, purgeCfg.Grace)

	// Person orphan sweep (F40, ADR-072 §4/P0-9): a daily ticker that deletes people
	// orphaned (zero video links) for more than the grace period, skipping anyone
	// carrying authored identity (alias/curated image/manual decision or curation).
	orphanSweeper := personorphan.New(repository, personorphan.Config{}, log)

	// Owner gate (F21.7, ADR-030): empty ADMIN_TOKEN keeps the single-user
	// zero-config default; on a non-loopback bind that means the admin surface is
	// reachable without a token — warn loudly (fail-loud condition 1).
	auth := api.NewAuth(cfg.AdminToken)
	auth.SetSessionSecret(cfg.SessionSecret) // optional independent session key (ADR-046)
	exposedBind := cfg.ExposedBind()
	if !auth.Required() && exposedBind {
		log.Warn("admin controls are reachable WITHOUT a token on a non-loopback bind; set ADMIN_TOKEN to require authentication",
			"host", cfg.Host)
	}
	handlers.SetAuth(auth, exposedBind)
	handlers.SetCardLayout(cfg.CardLayout)
	handlers.SetFilmsEnabled(cfg.FilmsEnabled)
	handlers.SetDefaultSource(cfg.DefaultSource)
	handlers.SetProviderTrustOrder(cfg.ProviderTrustOrder)
	apiHandler := api.Router(log, health, handlers, reg.Handler())

	// In production the SvelteKit SPA is embedded; in dev Vite proxies /api here.
	var handler http.Handler = apiHandler
	if fe := frontendFS(); fe != nil {
		spa := spaHandler{fs: fe}
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
				apiHandler.ServeHTTP(w, r)
				return
			}
			spa.ServeHTTP(w, r)
		})
	}

	// Bind address: cfg.Host empty → all interfaces (Docker default); set to
	// 127.0.0.1 to listen on loopback only (avoids the Windows Firewall prompt in
	// local dev, since loopback listeners aren't filtered).
	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go sc.Run(ctx, time.Duration(cfg.ScanIntervalSeconds)*time.Second)
	go thumbs.Run(ctx)
	go purger.Run(ctx)
	go orphanSweeper.Run(ctx)
	go writeQ.Start(ctx) // boot recovery + orphan sweep + worker pool (F30, ADR-048)

	// MCP server (ADR-005): shares the repository with the web/scanner; HTTP/SSE
	// transport on MCPPort. stdio is a separate entrypoint (-mcp-transport stdio).
	if cfg.MCPEnabled && (cfg.MCPTransport == "http" || cfg.MCPTransport == "both") {
		mcpAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.MCPPort))
		go func() {
			if err := mcp.New(repository, log, mappings, auth, cfg.FilmsEnabled).StartHTTP(ctx, mcpAddr); err != nil {
				log.Error("mcp http server failed", "err", err)
			}
		}()
	} else if cfg.MCPEnabled {
		log.Info("mcp enabled but transport is stdio-only; HTTP server not started", "transport", cfg.MCPTransport)
	}

	// Serve.
	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "url", "http://localhost:"+strconv.Itoa(cfg.Port))
		// Ready = migrations applied + server listening; scanner bootstrap is deferred to a later phase (ADR-019).
		health.SetReady(true)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Graceful shutdown (ADR-019).
	health.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("stopped cleanly")
	return nil
}

// backfillStudioLinks derives video_studios for every existing video once, after
// migration 0017, so studio promotion doesn't require a manual rescan (F38, ADR-053
// §5). It is genuinely one-time via two gates: the common case skips once any link
// exists (`StudioLinkCount > 0`); the edge case — a library where *nothing* resolves
// to a studio, so no link is ever created — skips on a prior successful backfill job
// run, so we don't re-scan the whole library (and re-record a job run) on every boot.
// Best-effort — failures are logged, never fatal; the pass is idempotent. Runs to
// completion before the server serves, matching the person-image startup backfill.
func backfillStudioLinks(ctx context.Context, r *repo.Repo, relink func(context.Context, int64) error, log *slog.Logger) {
	n, err := r.StudioLinkCount(ctx)
	if err != nil {
		log.Warn("studio link backfill: count failed", "err", err)
		return
	}
	if n > 0 {
		return // links exist — the one-time pass already ran (common case)
	}
	// No links yet: either a fresh migration, or a library with no studios at all.
	// The job-run marker distinguishes them so we don't re-pass every boot in the
	// latter case. A marker lookup failure falls through and runs (safe — idempotent).
	if ran, err := r.HasSuccessfulJobRun(ctx, model.JobKindStudioBackfill); err != nil {
		log.Warn("studio link backfill: marker check failed; running anyway", "err", err)
	} else if ran {
		return
	}
	ids, err := r.AllActiveVideoIDs(ctx)
	if err != nil {
		log.Warn("studio link backfill: list videos failed", "err", err)
		return
	}
	if len(ids) == 0 {
		return // nothing to backfill; no marker needed (a later boot with videos runs)
	}
	started := time.Now()
	var errs int
	for _, id := range ids {
		if err := relink(ctx, id); err != nil {
			errs++
			log.Warn("studio link backfill: relink failed", "id", id, "err", err)
		}
	}
	finished := time.Now()
	status := model.JobStatusOK
	if errs > 0 {
		status = model.JobStatusErr
	}
	// The job run doubles as the one-time marker (see the edge-case gate above), so
	// it is recorded whether or not any link resulted. Detail states what it did —
	// processed N videos — not how many links resulted (which may legitimately be 0).
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind:       model.JobKindStudioBackfill,
		Trigger:    model.TriggerInitial,
		Status:     status,
		StartedAt:  started,
		FinishedAt: finished,
		DurationMs: finished.Sub(started).Milliseconds(),
		Errors:     errs,
		Detail:     fmt.Sprintf("studio-link backfill: processed %d videos", len(ids)),
	}); err != nil {
		log.Warn("studio link backfill: record job run failed", "err", err)
	}
	log.Info("studio link backfill complete", "videos", len(ids), "errors", errs)
}

// backfillPersonLinks runs the one-time video→person link derivation cutover
// (F40, ADR-072 P0-4). Unlike studio's video_studios (greenfield table),
// migration 0037 CARRIES FORWARD the pre-existing raw-extraction video_people
// rows (role=''), so a non-empty table does not mean the backfill already ran —
// this gates purely on the job-run marker, not PersonLinkCount. Loss-guarded
// (ADR-072 RD9): logs loudly (never panics) if the post-backfill active link
// count shrinks vs. pre-backfill, which would mean the derivation's source set
// misses something raw extraction covered. Best-effort; idempotent.
func backfillPersonLinks(ctx context.Context, r *repo.Repo, relink func(context.Context, int64) error, log *slog.Logger) {
	if ran, err := r.HasSuccessfulJobRun(ctx, model.JobKindPersonBackfill); err != nil {
		log.Warn("person link backfill: marker check failed; running anyway", "err", err)
	} else if ran {
		return
	}
	preCount, err := r.PersonLinkCount(ctx)
	if err != nil {
		log.Warn("person link backfill: pre-count failed", "err", err)
		return
	}
	ids, err := r.AllActiveVideoIDs(ctx)
	if err != nil {
		log.Warn("person link backfill: list videos failed", "err", err)
		return
	}
	if len(ids) == 0 {
		return // nothing to backfill; no marker needed (a later boot with videos runs)
	}
	started := time.Now()
	var errs int
	for _, id := range ids {
		if err := relink(ctx, id); err != nil {
			errs++
			log.Warn("person link backfill: relink failed", "id", id, "err", err)
		}
	}
	postCount, err := r.PersonLinkCount(ctx)
	if err != nil {
		log.Warn("person link backfill: post-count failed", "err", err)
		postCount = preCount // avoid a false-positive loss-guard trip on a read failure
	}
	if postCount < preCount {
		log.Error("person link backfill: active link count SHRANK — derivation source set may miss an extraction tag",
			"pre", preCount, "post", postCount)
	}
	finished := time.Now()
	status := model.JobStatusOK
	if errs > 0 {
		status = model.JobStatusErr
	}
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind:       model.JobKindPersonBackfill,
		Trigger:    model.TriggerInitial,
		Status:     status,
		StartedAt:  started,
		FinishedAt: finished,
		DurationMs: finished.Sub(started).Milliseconds(),
		Errors:     errs,
		Detail:     fmt.Sprintf("person-link backfill: processed %d videos (pre=%d post=%d links)", len(ids), preCount, postCount),
	}); err != nil {
		log.Warn("person link backfill: record job run failed", "err", err)
	}
	log.Info("person link backfill complete", "videos", len(ids), "errors", errs, "pre_links", preCount, "post_links", postCount)
}

// seedIdentityReviewQueue runs the one-time near-miss seed of identity_review_queue
// (F43/RD10, HOLODEX-149): the fuzzy name near-misses the migration-0022 fold left
// behind (it only auto-folds the provably-safe pure-case pairs) become review pairs
// the owner works from the Duplicates tab (S5). Recorded as one observable job run
// (ADR-028) whose detail is a bare count — no path or secret. Gated on a prior
// successful run so a queue the owner has worked down is never re-seeded; correctness
// does not depend on the gate, since the pass is INSERT OR IGNORE and skips
// keep-separate pairs, so even a post-retention re-run only adds still-pending
// near-misses. Best-effort: a failure is logged (status=error, so the gate lets the
// next boot retry) and never blocks startup.
func seedIdentityReviewQueue(ctx context.Context, r *repo.Repo, log *slog.Logger) {
	if ran, err := r.HasSuccessfulJobRun(ctx, model.JobKindIdentityBackfill); err != nil {
		log.Warn("identity backfill: marker check failed; running anyway", "err", err)
	} else if ran {
		return
	}
	started := time.Now()
	queued, err := r.SeedIdentityReviewQueue(ctx)
	finished := time.Now()
	status := model.JobStatusOK
	var errs int
	detail := fmt.Sprintf("identity review-queue seed: queued %d near-miss pairs", queued)
	if err != nil {
		status, errs, detail = model.JobStatusErr, 1, "identity review-queue seed: failed"
		log.Warn("identity backfill: seed failed", "err", err)
	}
	if err := r.RecordJobRun(ctx, model.JobRun{
		Kind:       model.JobKindIdentityBackfill,
		Trigger:    model.TriggerInitial,
		Status:     status,
		StartedAt:  started,
		FinishedAt: finished,
		DurationMs: finished.Sub(started).Milliseconds(),
		Added:      int(queued),
		Errors:     errs,
		Detail:     detail,
	}); err != nil {
		log.Warn("identity backfill: record job run failed", "err", err)
	}
	if err == nil {
		log.Info("identity review-queue seed complete", "queued", queued)
	}
}

func newLogger(level string, w io.Writer) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl}))
}

// runHealthcheck probes the local /healthz endpoint for the Docker HEALTHCHECK.
func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "7800"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	return 1
}
