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
func Router(log *slog.Logger, health *Health) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Health / readiness (ADR-019).
	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)

	// REST API (ADR-006). Endpoints land here as the service layer is built:
	//   GET /api/v1/media          list + filter + search  (F4)
	//   GET /api/v1/media/{id}     detail                  (F7)
	//   GET /api/v1/media/{id}/stream  range-served file   (ADR-015)
	//   GET /api/v1/people | /people/{id}                  (F5)
	//   GET /api/v1/tags   | /tags/{id}                    (F6)
	//   GET /api/v1/search         global mixed-entity     (ADR-017, F4.10)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		// TODO(phase1): mount media, people, tags, search handlers.
	})

	log.Debug("router constructed")
	return r
}
