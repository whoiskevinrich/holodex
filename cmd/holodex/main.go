// Command holodex is the single-binary entrypoint: it loads configuration,
// opens the database and applies migrations, then serves the HTTP API and runs
// the background scanner, shutting down gracefully on SIGINT/SIGTERM.
package main

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
	"holodex/internal/mapping"
	"holodex/internal/mcp"
	"holodex/internal/metadata"
	"holodex/internal/metrics"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/scanner"
	"holodex/internal/thumbnail"
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
	return mcp.New(repo.New(database), log, mappings).ServeStdio()
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
	sources, err := enrich.NewStore(cfg.MetadataSourcesPath)
	if err != nil {
		return fmt.Errorf("load metadata sources: %w", err)
	}
	enrichSvc := enrich.NewService(sources, repository, log)
	log.Info("metadata source providers loaded", "path", cfg.MetadataSourcesPath, "enabled", len(sources.Current().Enabled()))

	// Person images (F24, ADR-037): on-disk store under DATA_PATH/person-images. The
	// enrichment asset path and the upload handler share one normalize+store sink so a
	// provider photo gets the same metadata strip as an upload.
	if err := os.MkdirAll(cfg.PersonImagePath, 0o755); err != nil {
		log.Warn("person image dir create failed", "dir", cfg.PersonImagePath, "err", err)
	}
	enrichSvc.SetImageSink(personimage.NewSink(repository, cfg.PersonImagePath, cfg.PersonImageMaxDimension))

	health := api.NewHealth()
	handlers := api.NewHandlers(repository, log, thumbs, cfg.ThumbnailPath, sc, reg)
	handlers.SetMetadataFields(mappings, cacheBackend)
	handlers.SetEnrichment(enrichSvc)
	handlers.SetPersonImages(cfg.PersonImagePath, cfg.PersonImageMaxBytes, cfg.PersonImageMaxDimension, defaultSkin)
	handlers.SetActivity(sc, health, version, startedAt, cfg.MediaPath != "")

	// Owner gate (F21.7, ADR-030): empty ADMIN_TOKEN keeps the single-user
	// zero-config default; on a non-loopback bind that means the admin surface is
	// reachable without a token — warn loudly (fail-loud condition 1).
	auth := api.NewAuth(cfg.AdminToken)
	exposedBind := cfg.ExposedBind()
	if !auth.Required() && exposedBind {
		log.Warn("admin controls are reachable WITHOUT a token on a non-loopback bind; set ADMIN_TOKEN to require authentication",
			"host", cfg.Host)
	}
	handlers.SetAuth(auth, exposedBind)
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

	// MCP server (ADR-005): shares the repository with the web/scanner; HTTP/SSE
	// transport on MCPPort. stdio is a separate entrypoint (-mcp-transport stdio).
	if cfg.MCPEnabled && (cfg.MCPTransport == "http" || cfg.MCPTransport == "both") {
		mcpAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.MCPPort))
		go func() {
			if err := mcp.New(repository, log, mappings).StartHTTP(ctx, mcpAddr); err != nil {
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
