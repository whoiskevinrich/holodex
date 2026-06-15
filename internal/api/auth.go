package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminTokenHeader carries the owner token on gated requests. A header-only
// scheme (no cookie) is deliberate: cross-site forms cannot set a custom header,
// so the gate is immune to CSRF without a separate token (F21.7 condition 3,
// ADR-030).
const AdminTokenHeader = "X-Admin-Token"

// Auth is the single owner-access gate for the admin surface (F21.7, ADR-030).
// An empty token means the gate is open — the single-user, zero-config default;
// a configured token must be presented on every gated route. It is the one choke
// point a future multi-user / "Pro mode" identity source swaps into.
type Auth struct {
	token string
}

// NewAuth builds the gate from the configured ADMIN_TOKEN (empty = open).
func NewAuth(token string) *Auth { return &Auth{token: strings.TrimSpace(token)} }

// Required reports whether a token is configured (the gate is closed). Surfaced
// to the SPA via /capabilities so it knows to present a token.
func (a *Auth) Required() bool { return a != nil && a.token != "" }

// authorized reports whether a request may reach owner-only routes. It is
// nil-receiver safe: a nil *Auth means no gate is configured, so Required() is
// false and access is open — which is why callers never need their own nil check.
// When a token is set, the header must match in constant time (F21.7 condition 2
// — never a plain ==, to avoid leaking the token by timing).
func (a *Auth) authorized(r *http.Request) bool {
	if !a.Required() {
		return true
	}
	got := r.Header.Get(AdminTokenHeader)
	return subtle.ConstantTimeCompare([]byte(got), []byte(a.token)) == 1
}

// requireOwner wraps the owner-only route group. With no token configured (or no
// gate wired) it is a transparent pass-through (single-user); otherwise an
// unauthorized request gets 401 before reaching any handler.
func (h *Handlers) requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.auth.authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "owner authorization required")
	})
}

// capabilities (ungated) tells the SPA what it may do: whether the current
// request is an owner, and whether a token is required at all. The frontend
// shows owner controls only when owner is true (F21.7).
func (h *Handlers) capabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Owner        bool `json:"owner"`
		AuthRequired bool `json:"auth_required"`
	}{Owner: h.auth.authorized(r), AuthRequired: h.auth.Required()})
}
