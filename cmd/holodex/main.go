// Command holodex is the single-binary entrypoint: it loads configuration,
// opens the database and applies migrations, then serves the HTTP API and runs
// the background scanner, shutting down gracefully on SIGINT/SIGTERM.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"holodex/internal/api"
	"holodex/internal/config"
	"holodex/internal/db"
	"holodex/internal/scanner"
)

func main() {
	var (
		configPath  = flag.String("config", "", "path to holodex.yaml (optional)")
		migrateOnly = flag.Bool("migrate-only", false, "apply migrations and exit (ADR-016)")
		healthcheck = flag.Bool("healthcheck", false, "probe /healthz and exit 0/1 (Docker HEALTHCHECK)")
	)
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	if err := run(*configPath, *migrateOnly); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run(configPath string, migrateOnly bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	log := newLogger(cfg.LogLevel)
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

	health := api.NewHealth()
	handler := api.Router(log, health)

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background scanner (ADR-018).
	sc := scanner.New(scanner.Config{
		MediaPath:      cfg.MediaPath,
		FollowSymlinks: cfg.FollowSymlinks,
		MaxDepth:       cfg.ScanMaxDepth,
		MinAge:         time.Duration(cfg.ScanMinAgeSeconds) * time.Second,
		Workers:        cfg.ScanWorkers,
	}, log)
	go sc.Run(ctx, time.Duration(cfg.ScanIntervalSeconds)*time.Second)

	// Serve.
	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
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

func newLogger(level string) *slog.Logger {
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
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
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
