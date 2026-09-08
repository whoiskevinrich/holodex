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
	"regexp"
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

	// Fail fast on a swapped credential. The emptiness check above cannot catch this: both
	// values are non-empty, so the sidecar would start healthy and every enrichment would
	// then fail with an opaque 401 far from the cause. Only an unambiguous swap exits; an
	// unrecognized shape warns, so a future TMDB credential format degrades to a hint
	// rather than an outage. Only the classification is logged, never the value.
	if token != "" {
		switch classifyCredential(token) {
		case credAPIKey:
			log.Error("TMDB_API_TOKEN looks like a v3 API key (32 hex characters), not a Read Access Token; " +
				"it would be sent as a bearer token and rejected — set TMDB_API_KEY instead")
			os.Exit(1)
		case credUnknown:
			log.Warn("TMDB_API_TOKEN is not shaped like a Read Access Token (expected a three-part JWT); " +
				"TMDB may reject requests with 401")
		}
	}
	// apiKey is consulted only when token is empty (see newTMDBClient), so validate it only then.
	if token == "" && apiKey != "" {
		switch classifyCredential(apiKey) {
		case credReadAccessToken:
			log.Error("TMDB_API_KEY looks like a Read Access Token (JWT), not a v3 API key; " +
				"it would be sent as an api_key query parameter and rejected — set TMDB_API_TOKEN instead")
			os.Exit(1)
		case credUnknown:
			log.Warn("TMDB_API_KEY is not shaped like a v3 API key (expected 32 hex characters); " +
				"TMDB may reject requests")
		}
	}

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
	// Self-served brand mark advertised by /describe.brand_icon (HOLODEX-161, ADR-059).
	mux.HandleFunc("GET /brand-icon.png", h.brandIcon)

	addr := net.JoinHostPort(host, port)
	log.Info("holodex-provider-tmdb starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// credentialKind classifies a TMDB credential by shape alone. TMDB's dashboard shows the v3
// API key and the Read Access Token side by side and regenerates them as a pair, which makes
// putting one in the other's variable an easy mistake — and an expensive one to diagnose,
// since the two are sent completely differently (bearer header vs api_key query parameter).
type credentialKind int

const (
	credUnknown credentialKind = iota
	credReadAccessToken // three-part JWT, sent as `Authorization: Bearer <token>`
	credAPIKey          // 32 hex characters, sent as the `api_key` query parameter
)

var (
	// A JWT is three base64url segments. Matching the structure rather than a literal "eyJ"
	// prefix keeps this from breaking if TMDB changes the header it encodes.
	jwtShape    = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
	apiKeyShape = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
)

// classifyCredential reports which TMDB credential a value looks like. A hex key contains no
// dots and so can never match the JWT shape, making the two cases mutually exclusive.
func classifyCredential(v string) credentialKind {
	switch {
	case jwtShape.MatchString(v):
		return credReadAccessToken
	case apiKeyShape.MatchString(v):
		return credAPIKey
	default:
		return credUnknown
	}
}
