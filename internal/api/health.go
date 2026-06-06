package api

import (
	"net/http"
	"sync/atomic"
)

// Health tracks liveness vs readiness (ADR-019). Liveness is true once the
// process serves HTTP; readiness flips true after migrations + bootstrap.
type Health struct {
	ready atomic.Bool
}

func NewHealth() *Health { return &Health{} }

// SetReady marks the service ready to serve traffic.
func (h *Health) SetReady(ready bool) { h.ready.Store(ready) }

// Liveness always returns 200 if the process is running.
func (h *Health) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeStatus(w, http.StatusOK, "ok")
}

// Readiness returns 503 until bootstrap completes, then 200.
func (h *Health) Readiness(w http.ResponseWriter, _ *http.Request) {
	if h.ready.Load() {
		writeStatus(w, http.StatusOK, "ready")
		return
	}
	writeStatus(w, http.StatusServiceUnavailable, "not ready")
}

func writeStatus(w http.ResponseWriter, code int, body string) {
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}
