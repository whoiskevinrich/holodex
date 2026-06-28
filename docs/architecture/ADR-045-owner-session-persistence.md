# ADR-045: Owner session persistence via HttpOnly token-exchange cookie

**Status**: Proposed (pending `/security-review` sign-off — auth/access change)
**Date**: 2026-06-27
**Deciders**: Project owner
**Relates to**: [ADR-030](ADR-030-access-control-gating-seam.md) (access-control gating seam — this ADR
**amends** its "in-memory only / re-entered after a reload" consequence), [ADR-014](ADR-014-configuration-and-data-layout.md)
(configuration strategy & precedence), [ADR-027](ADR-027-dotenv-local-config.md) (`.env` dev config),
[ADR-006](ADR-006-api-design.md) (API design), [ADR-007](ADR-007-docker-structure.md) (same-origin SPA+API).
Spec: [owner-session-persistence](../specs/owner-session-persistence.md).

---

## Context

ADR-030 established the owner-access gate: an optional `ADMIN_TOKEN`, a single `requireOwner` middleware,
and a **header-only** scheme (`X-Admin-Token`) chosen so the gate is **CSRF-immune** — a cross-site form
cannot set a custom header. To keep the token safe from XSS exfiltration, ADR-030 deliberately had the SPA
hold it **in memory only, never `localStorage`** ([api.ts:39–48](../../web/src/lib/api.ts)). The accepted
trade-off was stated outright: *"re-entered after a reload."*

In daily use that trade-off bites. The owner uses the gated surface (rescan, reload-config, enrich, delete,
writeback, person-image management), and **every page reload silently signs them out** — controls vanish,
authed requests `401`, the token must be re-typed. The [owner-session-persistence spec](../specs/owner-session-persistence.md)
asks to persist the authenticated session across reloads **without** reopening the exfiltration hole ADR-030
closed, and to keep ADR-030's other guarantees (single choke point, constant-time compare, CSRF posture,
zero-config single-user default, unchanged header path for API/script clients).

The forces in tension:

- **Persistence vs. exfiltration.** Anything the SPA can read to "remember" the token (`localStorage`,
  `sessionStorage`, a JS-readable cookie) is also readable by an injected script — exactly what ADR-030
  refused.
- **Cookies vs. CSRF.** A cookie persists without being JS-readable (`HttpOnly`), but cookies are sent
  automatically by the browser, reintroducing the CSRF exposure the header-only scheme was immune to.
- **Zero-dependency posture.** ADR-030's rationale rejects a session store / auth framework for a
  single-user tool; whatever we add should stay config-via-env and lean on `go.mod`.

ADR-030 itself anticipated this exact path — it noted the gate could accept *"a cookie established by a
minimal token exchange for the SPA."* This ADR makes that concrete.

## Decision

Add a **minimal token-exchange + HttpOnly session cookie** in front of the existing gate. The `ADMIN_TOKEN`
remains the root credential; the cookie is a derived, JS-unreadable session carrier.

### 1. Exchange endpoint — `POST /api/v1/session` (ungated)
Validates the presented `ADMIN_TOKEN` with the **same `subtle.ConstantTimeCompare`** used by the header gate
([auth.go:35–41](../../internal/api/auth.go)). The token is presented via the existing `X-Admin-Token`
header on the POST (reuse — keeps the secret out of URLs/logs and out of a body that might be logged). On
success, set the session cookie (below) and respond `204`. Wrong/missing token → `401`, no cookie.

### 2. Session cookie — HttpOnly, signed, opaque
Cookie `holodex_session` with attributes **`HttpOnly`**, **`Secure`** (see §6), **`SameSite=Strict`**,
**`Path=/`**, and a bounded **`Max-Age`** (§5). The value is **not the raw token** — it is a **signed,
self-contained** value: `base64(payload).base64(HMAC-SHA256(payload, secret))`, where `payload` carries an
issued-at, an expiry, and a lifetime-class flag (short vs. trusted). The server validates by recomputing the
HMAC (constant-time) and checking expiry — **no server-side session store** (zero-dependency, per ADR-030).

### 3. Signing secret — derived, no new required config
The HMAC secret is **derived from `ADMIN_TOKEN`** via a domain-separated KDF
(`HMAC-SHA256(ADMIN_TOKEN, "holodex/session/v1")`), so there is **no new required env var** and the dominant
single-user path stays zero-extra-config. **Consequence by construction:** rotating `ADMIN_TOKEN`
invalidates all existing session cookies (they fail HMAC) — a desirable property (changing the root
credential logs everyone out). An optional explicit `SESSION_SECRET` override (ADR-014 precedence) is
permitted for operators who want to rotate sessions independently of the token; if unset, the derived secret
is used.

### 4. Gate accepts cookie **or** header
`requireOwner` / `authorized()` returns true for a valid `X-Admin-Token` header **(unchanged — API/script
clients keep working)** **or** a valid, unexpired session cookie. `GET /api/v1/capabilities` reports
`owner: true` for either, so on reload the SPA learns it is still the owner with no re-prompt. An
expired/tampered cookie is treated as no credential (`401`) and the response **expires** the cookie so the
browser stops resending a dead value. `DELETE /api/v1/session` (sign-out) expires the cookie and is
idempotent (`204` even when already signed out).

### 5. Lifetimes — short default, opt-in long, sliding renewal
Two server-defined lifetime classes (the client only signals which; it cannot request an arbitrary
`Max-Age`):
- **Short (default): 7 days.** Issued unless "trust this device" is requested.
- **Trusted (opt-in): 30 days.** Requested via a parameter on the exchange (OS6), for a private machine.

**Sliding renewal (OS7):** when a request carries a valid cookie that is **past half its lifetime**, the
gate re-issues it with a fresh `Max-Age` of the **same class** (the half-life throttle avoids a `Set-Cookie`
on every request). An **absolute cap of 90 days** of continuous renewal bounds total session age; past it,
re-auth is required. Renewal never resurrects an already-expired cookie and never upgrades a short session to
a long one.

### 6. `Secure` flag and local dev
`Secure` is set **except** when the request arrives over plain HTTP to a loopback host
(`localhost`/`127.0.0.1`/`[::1]`), so `make web-dev` (HTTP on localhost) works while every
exposed/reverse-proxied deployment (the case ADR-030 cares about) always gets `Secure`. The decision keys on
the request being loopback-and-not-already-HTTPS, not on a build flag.

### 7. CSRF posture
`SameSite=Strict` is the primary mitigation: the cookie is simply **not sent** on any cross-site navigation
or sub-request, so a cross-origin page cannot drive a state-changing gated route with it. This is sufficient
for these same-origin-only owner actions (the SPA and API share an origin, ADR-007). The header path remains
available and CSRF-immune for non-browser clients. The exchange endpoint is itself state-changing but
requires the secret token, so it cannot be driven by a third party. (`SameSite=Lax` is explicitly **not**
used — it would allow the cookie on top-level cross-site GET navigations, and we want Strict's stronger
guarantee since there is no cross-site flow that needs the cookie.)

### 8. Zero-config passthrough unchanged
With `ADMIN_TOKEN` unset the gate stays open (ADR-030), the SPA never calls `POST /session`, and no cookie is
ever set — the zero-config single-user local experience is byte-for-byte unchanged.

## Options Considered

### Option A: HttpOnly signed-cookie token exchange (chosen)
| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — one exchange + sign-out endpoint, HMAC sign/verify, gate accepts a second credential |
| Cost | No new infra; no session store; secret derived from existing `ADMIN_TOKEN` |
| Security | Preserves ADR-030's no-JS-read property (`HttpOnly`); CSRF handled by `SameSite=Strict` |
| Team familiarity | Standard signed-cookie pattern; stdlib `crypto/hmac` only — consistent with lean go.mod |

**Pros:** Token never enters a JS-readable channel; survives reloads; no server-side store; header path and
zero-config default untouched; rotating the token invalidates sessions for free.
**Cons:** Reintroduces CSRF surface (mitigated by SameSite=Strict); self-contained cookies can't be revoked
before expiry without adding state (accepted — absolute cap + token-rotation kill-switch bound the risk);
the `Secure`-on-localhost rule is a small special case to get right.

### Option B: `sessionStorage` (or `localStorage`) for the token
| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — a few frontend lines, no backend change |
| Cost | None |
| Security | **Reopens the exact hole ADR-030 closed** — JS-readable, XSS-exfiltratable |
| Team familiarity | Trivial |

**Pros:** Smallest possible change; `sessionStorage` survives F5 in the same tab.
**Cons:** Defeats the spec's core goal (token must stay unreadable by JS). `localStorage` is durable +
JS-readable (worst case); `sessionStorage` narrows the window to one tab but is still JS-readable. Rejected
on security grounds — this is precisely what ADR-030 forbade.

### Option C: Server-side session store (DB/in-memory table) keyed by an opaque cookie
| Dimension | Assessment |
|-----------|------------|
| Complexity | High — session table/store, lifecycle, cleanup; new migration or in-mem structure |
| Cost | New persistent (or volatile) state to operate |
| Security | Strong — supports instant server-side revocation |
| Team familiarity | Standard but heavier |

**Pros:** Real revocation list; no secret-in-cookie at all.
**Cons:** Contradicts ADR-030's explicit zero-dependency, no-session-store rationale for a single-user tool;
volatile store loses sessions on restart, persistent store adds a migration for one user. Deferred to a P2
future-consideration (introduce only if self-contained cookies prove insufficient).

## Trade-off Analysis

The decisive axis is **persistence without JS-readability**. Option B is cheapest but fails the one
requirement that motivated the spec. Option C is the most capable but over-builds for a single-owner tool and
directly contradicts ADR-030's standing rationale. Option A threads the needle: `HttpOnly` keeps the
credential unreadable by script (ADR-030's property preserved), a **signed self-contained** cookie avoids any
new store (ADR-030's zero-dependency posture preserved), and deriving the secret from `ADMIN_TOKEN` keeps the
zero-config default intact while giving free session-invalidation on token rotation. The price is the CSRF
surface a cookie reintroduces; `SameSite=Strict` on a same-origin-only app neutralizes it, and the header
path remains for anyone who wants the CSRF-immune scheme. Revocability — Option C's edge — is bounded here by
the absolute renewal cap and the token-rotation kill-switch, which is proportionate for one owner.

## Consequences

- **Easier:** the owner stays signed in across reloads; sign-out is explicit and effective; rotating
  `ADMIN_TOKEN` cleanly invalidates all sessions; no new infrastructure to run.
- **Harder / new surface:** a cookie path adds CSRF considerations that the header-only gate didn't have
  (mitigated, but now part of the threat model the security review must keep blessing); two credential paths
  (`header` and `cookie`) into `authorized()` instead of one; a small `Secure`-on-localhost special case.
- **Amends ADR-030:** the "in-memory only / re-entered after a reload" consequence is superseded by this ADR;
  ADR-030's gate, choke-point, constant-time compare, header CSRF-immunity, and zero-config default all
  stand. ADR-030 stays **Accepted**; this ADR records the amendment (ADRs are immutable — supersede, don't
  rewrite).
- **To revisit:** when real multi-user auth lands, the cookie becomes the carrier for a real identity and
  `ADMIN_TOKEN` becomes a bootstrap/legacy path — the exchange + cookie plumbing is designed so that is an
  identity-source swap behind `requireOwner`, not a call-site rework. Server-side revocation (Option C) is the
  fallback if self-contained cookies become insufficient.
- **Security review is a merge gate** (touches auth/access): must sign off cookie attributes, the
  secret-derivation/rotation scheme, the CSRF posture under a cookie, the `Secure`/localhost rule, and the
  preserved no-exfiltration property before this moves to **Accepted**.

## Action Items
1. [ ] `/security-review` of this ADR + the spec; resolve the blocking Open Questions (cookie value scheme,
       CSRF posture, `Secure`/dev rule). Move ADR to **Accepted** on sign-off.
2. [ ] `POST /api/v1/session` (exchange, constant-time validate, set signed cookie) + `DELETE /api/v1/session`
       (sign-out) in `internal/api`; HMAC sign/verify helper (stdlib `crypto/hmac`).
3. [ ] Teach `authorized()` / `capabilities` to accept the cookie; expire dead cookies; keep header path
       intact. Extend [auth_test.go](../../internal/api/auth_test.go) per spec OS5.
4. [ ] Lifetime classes (7d / 30d), sliding half-life renewal, 90d absolute cap; derived secret with optional
       `SESSION_SECRET` override (document in `holodex.yaml.example` / `.env.example`).
5. [ ] Frontend: exchange on sign-in, "trust this device" checkbox, rely on `capabilities.owner` on reload,
       sign-out control, graceful `401` fallback; remove token-in-memory reliance; `credentials` mode on
       authed fetches; 3-skin QA.
6. [ ] Update [docs/testing-strategy.md](../testing-strategy.md) and `docs/reference/configuration.md`
       (session lifetimes, optional `SESSION_SECRET`).
