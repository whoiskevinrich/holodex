package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AdminTokenHeader carries the owner token on the gated header path and on the
// session exchange. The header path is CSRF-immune (a cross-site form cannot set
// a custom header) and unchanged for API/script clients (F21.7 condition 3,
// ADR-030). The browser SPA instead exchanges this token once for an HttpOnly
// session cookie so the owner stays signed in across reloads (ADR-045).
const AdminTokenHeader = "X-Admin-Token"

// Session cookie + signing details (ADR-045). The cookie value is a signed,
// self-contained claim ("iat|exp|class") — never the raw ADMIN_TOKEN — so the
// token never enters a JS-readable channel and there is no server-side store.
const (
	sessionCookieName = "holodex_session"
	sessionClassShort = "s" // default bounded session
	sessionClassLong  = "l" // opt-in "trust this device" session
)

// Session lifetimes (ADR-045 §5). Short is the default; long is the opt-in
// "trust this device" window. maxAge caps total renewal age so an actively-used
// session cannot slide forever.
const (
	sessionShortTTL = 7 * 24 * time.Hour
	sessionLongTTL  = 30 * 24 * time.Hour
	sessionMaxAge   = 90 * 24 * time.Hour
)

// Auth is the single owner-access gate for the admin surface (F21.7, ADR-030).
// An empty token means the gate is open — the single-user, zero-config default;
// a configured token must be presented (header or session cookie) on every gated
// route. It is the one choke point a future multi-user / "Pro mode" identity
// source swaps into.
type Auth struct {
	token  string
	secret []byte // HMAC key for session cookies (ADR-045); derived from token by default
}

// NewAuth builds the gate from the configured ADMIN_TOKEN (empty = open). The
// session-signing secret defaults to a domain-separated derivation of the token,
// so rotating ADMIN_TOKEN invalidates every issued session and no extra config
// is required (ADR-045 §3). Use SetSessionSecret to override with SESSION_SECRET.
func NewAuth(token string) *Auth {
	t := strings.TrimSpace(token)
	return &Auth{token: t, secret: deriveSessionSecret(t)}
}

// SetSessionSecret overrides the derived signing key with an explicit secret
// (the optional SESSION_SECRET, ADR-014 precedence), letting an operator rotate
// sessions independently of the token. Empty leaves the token-derived default.
func (a *Auth) SetSessionSecret(override string) {
	if a == nil {
		return
	}
	if override = strings.TrimSpace(override); override != "" {
		a.secret = deriveSessionSecret(override)
	}
}

// deriveSessionSecret turns a source secret into a fixed-length HMAC key with a
// domain separator, so the same source can't collide with other uses (ADR-045).
func deriveSessionSecret(src string) []byte {
	mac := hmac.New(sha256.New, []byte(src))
	mac.Write([]byte("holodex/session/v1"))
	return mac.Sum(nil)
}

// Required reports whether a token is configured (the gate is closed). Surfaced
// to the SPA via /capabilities so it knows to present a token.
func (a *Auth) Required() bool { return a != nil && a.token != "" }

// authorized reports whether a request may reach owner-only routes. It is
// nil-receiver safe: a nil *Auth means no gate is configured, so Required() is
// false and access is open — which is why callers never need their own nil check.
// A request authorizes via EITHER a constant-time match of the X-Admin-Token
// header (F21.7 condition 2 — never a plain ==, to avoid leaking the token by
// timing) OR a valid, unexpired session cookie (ADR-045). The header path is
// unchanged so API/script clients keep working.
func (a *Auth) authorized(r *http.Request) bool {
	if !a.Required() {
		return true
	}
	if got := r.Header.Get(AdminTokenHeader); got != "" &&
		subtle.ConstantTimeCompare([]byte(got), []byte(a.token)) == 1 {
		return true
	}
	if c, err := r.Cookie(sessionCookieName); err == nil {
		if _, ok := a.validateSession(c.Value); ok {
			return true
		}
	}
	return false
}

// matchesToken reports whether got equals the configured token in constant time.
// Used by the exchange endpoint (the only place that accepts the raw token to
// mint a cookie).
func (a *Auth) matchesToken(got string) bool {
	return a.Required() && subtle.ConstantTimeCompare([]byte(got), []byte(a.token)) == 1
}

// sessionClaims is the decoded, verified payload of a session cookie: the
// original issue time (for the absolute cap), the current expiry, and the
// lifetime class (short vs. trusted).
type sessionClaims struct {
	iat   int64
	exp   int64
	class string
}

// sign computes the HMAC-SHA256 of a payload with the session secret.
func (a *Auth) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

// mintSession builds a signed cookie value "base64(payload).base64(hmac)" from
// the claims. The payload is "iat|exp|class".
func (a *Auth) mintSession(iat, exp int64, class string) string {
	payload := []byte(fmt.Sprintf("%d|%d|%s", iat, exp, class))
	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(a.sign(payload))
}

// validateSession verifies the signature (constant time) and expiry of a cookie
// value, returning the claims when valid. A tampered, malformed, or expired
// value yields ok=false — treated as no credential by callers.
func (a *Auth) validateSession(value string) (sessionClaims, bool) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return sessionClaims{}, false
	}
	enc := base64.RawURLEncoding
	payload, err1 := enc.DecodeString(parts[0])
	sig, err2 := enc.DecodeString(parts[1])
	if err1 != nil || err2 != nil {
		return sessionClaims{}, false
	}
	if !hmac.Equal(sig, a.sign(payload)) {
		return sessionClaims{}, false
	}
	claims, ok := parseSessionClaims(string(payload))
	if !ok || time.Now().Unix() >= claims.exp {
		return sessionClaims{}, false
	}
	return claims, true
}

// parseSessionClaims decodes the "iat|exp|class" payload; an unknown class or a
// malformed field is rejected.
func parseSessionClaims(payload string) (sessionClaims, bool) {
	parts := strings.Split(payload, "|")
	if len(parts) != 3 {
		return sessionClaims{}, false
	}
	iat, err1 := strconv.ParseInt(parts[0], 10, 64)
	exp, err2 := strconv.ParseInt(parts[1], 10, 64)
	class := parts[2]
	if err1 != nil || err2 != nil || (class != sessionClassShort && class != sessionClassLong) {
		return sessionClaims{}, false
	}
	return sessionClaims{iat: iat, exp: exp, class: class}, true
}

// ttlForClass maps a lifetime class to its window.
func ttlForClass(class string) time.Duration {
	if class == sessionClassLong {
		return sessionLongTTL
	}
	return sessionShortTTL
}

// requireOwner wraps the owner-only route group. With no token configured (or no
// gate wired) it is a transparent pass-through (single-user); otherwise an
// unauthorized request gets 401 before reaching any handler.
func (h *Handlers) requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.auth.authorized(r) {
			h.maybeRenewSession(w, r) // slide an active cookie's lifetime (ADR-045)
			next.ServeHTTP(w, r)
			return
		}
		// An expired/tampered cookie is no credential: tell the browser to drop it
		// so it stops resending a dead value (ADR-045). Only when one was presented.
		if _, err := r.Cookie(sessionCookieName); err == nil {
			http.SetCookie(w, h.expireSessionCookie(r))
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
		// DeleteGracePeriodSeconds drives the delete-confirm copy ("…in N days") and
		// the Trash purge-window text (F24, ADR-037); 0 = auto-purge disabled.
		DeleteGracePeriodSeconds int `json:"delete_grace_period_seconds"`
		// CardLayout is the operator's preferred card aspect ratio for browse lists:
		// "wide" (16:9, default) for personal/AMV libraries, "poster" (2:3) for film libraries.
		CardLayout string `json:"card_layout"`
		// PersonGalleryMax is the per-person 'extra' gallery cap (F25), so the SPA can
		// warn at the limit and offer the owner an explicit over-cap "add anyway".
		PersonGalleryMax int `json:"person_gallery_max"`
	}{
		Owner:                    h.auth.authorized(r),
		AuthRequired:             h.auth.Required(),
		DeleteGracePeriodSeconds: int(h.deleteGrace.Seconds()),
		CardLayout:               h.cardLayout,
		PersonGalleryMax:         h.repo.GalleryCapValue(),
	})
}
