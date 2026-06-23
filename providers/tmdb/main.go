// Package main is the TMDB metadata provider sidecar (ADR-033/039/040).
// It speaks the Holodex provider HTTP contract (/healthz /describe /resolve /enrich)
// and translates calls into TMDB v3 API requests. The operator runs it as a sidecar
// container alongside Holodex; Holodex dials it over the internal compose network.
//
// Required env: TMDB_API_TOKEN (bearer, preferred) or TMDB_API_KEY (legacy).
// Optional env: PORT (default 9100), HOST (default all interfaces), LOG_LEVEL (default info),
// TMDB_LANGUAGE (default en-US).
package main

import (
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
)

func main() {
	// CLI flags take precedence over env vars (mirrors holodex backend pattern).
	// --host empty → all interfaces (Docker/compose default);
	// --host 127.0.0.1 → loopback-only (avoids Windows Firewall UAC prompt in dev).
	hostFlag := flag.String("host", "", "bind address; empty = all interfaces, 127.0.0.1 = loopback-only")
	portFlag := flag.String("port", "", "HTTP port (overrides PORT env var; default 9100)")
	flag.Parse()

	token := os.Getenv("TMDB_API_TOKEN")
	apiKey := os.Getenv("TMDB_API_KEY")
	if token == "" && apiKey == "" {
		slog.Error("TMDB_API_TOKEN or TMDB_API_KEY must be set")
		os.Exit(1)
	}

	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	language := os.Getenv("TMDB_LANGUAGE")
	if language == "" {
		language = "en-US"
	}

	host := *hostFlag
	if host == "" {
		host = os.Getenv("HOST")
	}
	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "9100"
	}

	client := newTMDBClient(token, apiKey, language)
	h := newHandler(client, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /describe", h.describe)
	mux.HandleFunc("POST /resolve", h.resolve)
	mux.HandleFunc("POST /enrich", h.enrich)

	addr := net.JoinHostPort(host, port)
	log.Info("holodex-provider-tmdb starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server exited", "err", err)
		os.Exit(1)
	}
}
