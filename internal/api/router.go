// Package api wires the HTTP surface: REST under /api/v1 (ADR-006) and health
// endpoints (ADR-019). Handlers are added as the service layer lands.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Router builds the chi router. The frontend (embedded SvelteKit dist) is
// mounted by the caller in production; in dev the Vite server proxies /api here.
// handlers may be nil before the data layer is wired (health-only mode).
func Router(log *slog.Logger, health *Health, handlers *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Health / readiness (ADR-019).
	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)

	// REST API (ADR-006): media (F4/F7), people (F5), tags (F6), search (F4.10).
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		if handlers != nil {
			handlers.Mount(r)
		}
	})

	log.Debug("router constructed")
	return r
}
