# ADR-030: Access-control / "Pro mode" gating seam for owner-only surfaces

**Status**: Accepted (security-review sign-off 2026-06-14: gate complete, constant-time compare, header-only CSRF posture, fail-loud default-open — no HIGH/MEDIUM findings)
**Date**: 2026-06-14
**Deciders**: Project owner
**Relates to**: ADR-014 (Configuration strategy & precedence), ADR-006 (API design),
spec [F21 (System Activity)](../specs/system-activity.md) F21.6 / F21.7

---

## Context

Holodex is single-user today and its admin endpoints — `POST /api/v1/admin/rescan`
(F13.3), `POST /api/v1/admin/reload-config` (F20.10), and the thumbnail-regenerate
route — are **unauthenticated**. That was acceptable while they were API-only and
undocumented in the UI. Spec F21.6 changes the exposure: it surfaces these
infrastructure-affecting controls as **one-click buttons** on the activity page,
and adds new owner-oriented read surfaces (`/admin/activity`, `/admin/activity/history`,
and the future SSE stream). On a server that may be reverse-proxied to the internet,
a one-click "rescan the whole library" / "reload config" reachable by anyone who
loads the app is not acceptable to ship.

The full answer (multi-user accounts, sessions, roles) is explicitly **out of scope**
for this feature and premature — there is no multi-user decision yet. What F21 needs
is a **single seam** that (a) can lock down the owner-only surface now and (b) becomes
the swap point for real auth / a "Pro mode" later, without scattering checks across
handlers. F21.7 promotes this seam to **P0**: it lands with the activity feature.

## Decision

### One choke point: a `requireOwner` middleware
All owner-only routes — the activity read-model, history, the (future) SSE stream,
and the three admin control endpoints — are grouped behind a **single**
`requireOwner` middleware in `internal/api`. No owner check lives in an individual
handler; adding a new owner-only route means mounting it in this group, nothing else.

### v1 owner identity: an optional `ADMIN_TOKEN`
Owner identity in v1 is established by an optional **`ADMIN_TOKEN`** env var
(following ADR-014 config precedence; loadable via the ADR-027 `.env` in dev):

- **`ADMIN_TOKEN` set** → the gated routes require it (presented as a request header,
  or a cookie established by a minimal token exchange for the SPA). Missing/wrong →
  `401`. This is the posture for any exposed/reverse-proxied deployment.
- **`ADMIN_TOKEN` unset** → the seam is **open** (pass-through), preserving today's
  zero-config single-user local experience. The read-model still omits all secrets
  (ADR-028), so an open *read* surface leaks no paths/credentials; the risk being
  mitigated is unauthenticated *control* + write actions.

The `/metrics`, `/healthz`, `/readyz` endpoints are **not** behind this gate — they
keep their ADR-019 semantics (scrapers/orchestrators need them, and they are
read-only and secret-free).

### Frontend capability flag
The SPA reads a capability flag (e.g. from a small `GET /api/v1/capabilities` or a
field on the activity payload) indicating whether the current client is an owner.
Controls and raw-internal sections render **only** when the flag is set; a non-owner
(or token-less) client sees the read-only/friendly view. This is the seam that the
eventual general-user / "Pro mode" audience (F21.12) will reuse — verified by a test
that toggles the flag even though no real non-owner exists yet.

## Rationale

- **A single guard is auditable.** Security review (required by F21.6) can reason
  about one middleware and one route group, not N handlers.
- **Env-gated token is zero-dependency** and consistent with the project's
  config-via-env posture (ADR-014/027) and lean go.mod — no session store, no auth
  framework for a single-user tool.
- **Zero-config dev is preserved.** Unset `ADMIN_TOKEN` keeps `go run` and local
  preview frictionless; locking down is a one-variable change for a real deployment.
- **Future-proof by construction.** When multi-user/SSO is actually decided, the
  identity source behind `requireOwner` is swapped and the capability flag gains
  roles — the call sites and the frontend gating do not move.

## Consequences

- `internal/api` gains the `requireOwner` middleware, the owner-only route group,
  and a capability signal for the SPA; the SPA gains conditional rendering of
  controls/internals.
- Tests must exercise **both** states: open (no token) and gated (token set → 401
  without it, 200 with it), plus the frontend flag toggle.
- `/security-review` signed off (2026-06-14) that this seam adequately mitigates the
  unauthenticated-controls risk introduced by F21.6 — gate applied to every owner
  route, constant-time token compare, header-only (CSRF-immune) scheme, and the
  fail-loud `controls_unauthenticated` signal; no HIGH/MEDIUM findings. This ADR is
  therefore **Accepted**.
- This ADR is **superseded**, not edited, when real multi-user authentication is
  introduced; `ADMIN_TOKEN` becomes a legacy/bootstrap path at that point.
- Default-open when `ADMIN_TOKEN` is unset is a deliberate trade-off (usability for
  the dominant single-user, local case); it is documented in the README and
  `holodex.yaml.example` so an operator exposing the app knows to set the token.
