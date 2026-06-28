package api

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// Owner session endpoints (ADR-045). The SPA exchanges the ADMIN_TOKEN once for
// an HttpOnly session cookie so the owner stays signed in across reloads without
// the token ever living in a JS-readable store. The header path (auth.go) is
// untouched for API/script clients.

// postSession exchanges a valid ADMIN_TOKEN (presented as the X-Admin-Token
// header — keeps the secret out of URLs/bodies) for a session cookie. With the
// gate open (no token configured) there is nothing to authenticate, so it is a
// no-op success and no cookie is set. ?remember=1 requests the longer
// "trust this device" lifetime (OS6).
func (h *Handlers) postSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Required() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !h.auth.matchesToken(r.Header.Get(AdminTokenHeader)) {
		writeError(w, http.StatusUnauthorized, "invalid admin token")
		return
	}
	class := sessionClassShort
	if r.URL.Query().Get("remember") == "1" {
		class = sessionClassLong
	}
	now := time.Now()
	exp := now.Add(ttlForClass(class))
	http.SetCookie(w, h.sessionCookie(r, h.auth.mintSession(now.Unix(), exp.Unix(), class), exp))
	w.WriteHeader(http.StatusNoContent)
}

// deleteSession signs the owner out by expiring the cookie. Idempotent: it
// succeeds whether or not a session was present.
func (h *Handlers) deleteSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.expireSessionCookie(r))
	w.WriteHeader(http.StatusNoContent)
}

// maybeRenewSession slides an active cookie session's expiry when it is past
// half its lifetime (the half-life throttle avoids a Set-Cookie on every
// request), preserving its class and bounded by the absolute cap (ADR-045 §5,
// OS7). It never resurrects an expired cookie or upgrades a short session.
func (h *Handlers) maybeRenewSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Required() {
		return
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	claims, ok := h.auth.validateSession(c.Value)
	if !ok {
		return
	}
	ttl := int64(ttlForClass(claims.class).Seconds())
	now := time.Now().Unix()
	if now < claims.exp-ttl/2 {
		return // still in the first half of its life — nothing to do
	}
	newExp := now + ttl
	if absCap := claims.iat + int64(sessionMaxAge.Seconds()); newExp > absCap {
		newExp = absCap
	}
	if newExp <= claims.exp {
		return // at the absolute cap; let it lapse and force re-auth
	}
	http.SetCookie(w, h.sessionCookie(r, h.auth.mintSession(claims.iat, newExp, claims.class), time.Unix(newExp, 0)))
}

// baseSessionCookie carries the shared security attributes (ADR-045 §2/§6):
// HttpOnly (never JS-readable), SameSite=Strict (the CSRF mitigation), Path=/,
// and Secure (except plain-HTTP loopback dev — see secureCookie). Callers fill in
// the value and expiry.
func baseSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteStrictMode,
	}
}

// sessionCookie builds a live session cookie with a bounded Max-Age.
func (h *Handlers) sessionCookie(r *http.Request, value string, exp time.Time) *http.Cookie {
	c := baseSessionCookie(r)
	c.Value = value
	c.Expires = exp
	c.MaxAge = int(time.Until(exp).Seconds())
	return c
}

// expireSessionCookie builds a cookie that clears holodex_session immediately,
// matching the attributes so the browser overwrites the live one.
func (h *Handlers) expireSessionCookie(r *http.Request) *http.Cookie {
	c := baseSessionCookie(r)
	c.Expires = time.Unix(0, 0)
	c.MaxAge = -1
	return c
}

// secureCookie decides the Secure attribute (ADR-045 §6): set it unless the
// request is plain HTTP to a loopback host (http://localhost dev). Keying on the
// connection's TLS state plus a loopback host keeps `make web-dev` working while
// every exposed/proxied deployment gets Secure. Host-header spoofing only lets a
// client downgrade its own cookie (not the victim's), and loopback binds are not
// remotely reachable, so this is safe for the threat model.
func secureCookie(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return false
	}
	return true
}
