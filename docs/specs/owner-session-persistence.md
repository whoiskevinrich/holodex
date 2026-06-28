# Spec: Owner session persistence — stay signed in across reloads

**Status**: Draft
**Phase**: Post–Phase 2 backlog (auth/infra hardening; builds on the F21.7 / ADR-030 gating seam)
**Owner**: Project owner
**Date**: 2026-06-27

**Depends on**: The shipped owner-access gate — `requireOwner` middleware, the `X-Admin-Token`
header scheme, and `GET /api/v1/capabilities` ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md),
spec [F21 System Activity](system-activity.md) F21.7). No dependency on multi-user accounts (still out of scope).

**New ADRs required**:
- **ADR-045 (Proposed)** — *Owner session persistence via HttpOnly token-exchange cookie.* Establishes
  a `POST /api/v1/session` exchange that validates `ADMIN_TOKEN` once and sets an **HttpOnly, Secure,
  SameSite=Strict** session cookie; teaches `requireOwner` to accept **either** the existing
  `X-Admin-Token` header **or** the cookie; adds `DELETE /api/v1/session` (sign-out). This **amends
  ADR-030's "in-memory only / re-entered after a reload" condition** — it is recorded as superseding
  that specific consequence of ADR-030 while preserving the gate's choke-point and CSRF posture.

> **Security routing.** This change touches authentication and access control, so it **must** run through
> `/security-review` before merge (project working agreement #2). The review must confirm the cookie
> attributes, the CSRF posture under a cookie scheme, and that the no-XSS-exfiltration property ADR-030
> bought is preserved.

---

## Problem Statement

The owner authenticates to Holodex's gated surface (rescan, reload-config, enrich, delete, writeback,
person-image management) by entering the `ADMIN_TOKEN` once in the UI. Today that token is held in a
**module-level in-memory variable** and is **deliberately never persisted** ([api.ts:39–48](../../web/src/lib/api.ts)),
so **every page reload silently logs the owner out** — the token must be re-entered, gated controls
disappear mid-task, and authed requests start returning `401`. The friction is real for the one person
who actually uses these controls, but the in-memory design was a conscious ADR-030 trade-off to stop an
XSS payload from reading and exfiltrating the token out of `localStorage`. So the goal is not "store the
token somewhere" — it is **persist the owner's authenticated session across reloads without reopening the
exfiltration hole ADR-030 closed**.

## Goals

1. **Survive a reload.** After the owner authenticates once, an F5 / hard reload / reopened tab keeps them
   signed in — gated controls stay visible and authed requests keep succeeding — until they explicitly sign
   out or the session expires.
2. **Keep the token unreadable by JavaScript.** The persisted credential must **not** be readable by any
   script on the page, so an XSS payload cannot exfiltrate it — preserving the core property ADR-030 bought
   when it refused `localStorage`.
3. **Preserve the existing gate's guarantees.** One choke point (`requireOwner`), constant-time token
   compare, and CSRF-immunity all remain true under the new cookie path.
4. **Preserve zero-config single-user.** With `ADMIN_TOKEN` unset the gate stays open and nothing about the
   sign-in/persistence flow appears — no cookie, no exchange, no prompt.
5. **Clean sign-out.** The owner can explicitly end the session (and it ends server-side, not just in the
   browser).

## Non-Goals

- **Multi-user accounts, sessions, roles, or SSO.** Still explicitly out of scope (ADR-030's boundary is
  unchanged). This is one owner identity, persisted — not a user system. *(Why: no multi-user decision
  exists yet; that remains the event that supersedes ADR-030 wholesale.)*
- **"Remember me forever" across browser restarts by default.** The session is a bounded-lifetime cookie,
  not an indefinite credential. A long-lived "trust this device" option is a possible P1, not the headline.
  *(Why: a persisted-forever owner credential on a shared/exposed box is exactly the durability ADR-030 was
  wary of; bound the blast radius by default.)*
- **Storing the raw token in `localStorage` or `sessionStorage`.** Explicitly rejected — both are
  JS-readable and reintroduce the exfiltration risk this spec exists to avoid. *(Why: defeats Goal 2.)*
- **Server-side session store / database table.** The session is a self-contained signed/opaque cookie
  validated against the configured token; no new persistent store, consistent with the project's
  zero-dependency, config-via-env posture. *(Why: a single-owner tool doesn't warrant a session table; keep
  go.mod lean per ADR-030 rationale.)*
- **Changing how `ADMIN_TOKEN` is configured.** Same env/`holodex.yaml` precedence (ADR-014/027). This spec
  adds a *session* on top of the existing token; it does not touch token provisioning. *(Why: orthogonal.)*
- **Password/login UI, account recovery, rotation flows.** Out of scope — there is still exactly one
  pre-shared token. *(Why: no account model to hang these on.)*

---

## User Stories

- As the **library owner**, after I enter the admin token once, I want to stay signed in when I refresh or
  reopen the app so that I'm not re-typing the token and losing access to my controls all day.
- As the **library owner**, I want to explicitly sign out so that I can drop owner access on a shared or
  public machine when I'm done.
- As the **library owner**, I want my session to eventually expire on its own so that an unattended browser
  doesn't stay owner-authenticated indefinitely.
- As the **library owner**, when my session has expired or I've signed out, I want the app to cleanly fall
  back to the read-only/token-prompt view (not throw errors or wedge) so that re-authenticating is obvious
  and painless.
- As the **security-conscious owner**, I want confidence that even if a script were injected into the page,
  it could not read or steal my session credential so that persisting it doesn't make me less safe than the
  in-memory scheme did.
- As the **operator running the open single-user default** (`ADMIN_TOKEN` unset), I want none of this to
  appear so that the zero-config local experience is unchanged.

---

## Requirements

### Must-Have (P0)

#### OS1 — Token-exchange endpoint sets an HttpOnly session cookie

A new ungated `POST /api/v1/session` accepts the owner token, validates it against `ADMIN_TOKEN` using the
**same constant-time compare** as the header path, and on success sets a session cookie.

- **Request.** The token is supplied in a way that does not persist it in the URL or logs — sent as a
  request **header** (`X-Admin-Token`, reusing the existing scheme) or a JSON body field, decided in
  ADR-045. Never a query string.
- **Success → cookie.** On a valid token, respond `204` (or `200` with the capabilities payload) and set a
  cookie named e.g. `holodex_session` with attributes: **`HttpOnly`**, **`Secure`**, **`SameSite=Strict`**,
  **`Path=/`**, and a bounded **`Max-Age`** (default lifetime defined in ADR-045, e.g. 7 days; revisit at
  review). The cookie **value is not the raw token** — it is a signed/opaque session value the server can
  validate, so the literal `ADMIN_TOKEN` never travels in a JS-reachable channel and is not sitting in the
  cookie jar verbatim.
- **Failure.** Wrong/missing token → `401`, no cookie set. Constant-time compare (no early return that
  leaks length/equality by timing) — identical to `authorized()` today.
- **Gate-open passthrough.** If `ADMIN_TOKEN` is unset, `POST /session` is a no-op success (or `404`/`409`
  per ADR-045) — there is nothing to authenticate; the frontend never calls it in this mode.
- **`Secure` and local dev.** `Secure` cookies require HTTPS; dev runs over plain HTTP on localhost. ADR-045
  must specify the rule (e.g. omit `Secure` only when the request is to `localhost`/loopback, set it
  otherwise) so the cookie works in `make web-dev` without weakening exposed deployments. *(Tracked as an
  Open Question for the security review to bless.)*

**Acceptance criteria — OS1**
- [ ] Given `ADMIN_TOKEN` is set, when I POST `/session` with the correct token, then the response sets a
      cookie with `HttpOnly`, `SameSite=Strict`, `Path=/`, a bounded `Max-Age`, and (on HTTPS) `Secure`.
- [ ] Given `ADMIN_TOKEN` is set, when I POST `/session` with a wrong/empty token, then I get `401` and **no**
      `Set-Cookie` header.
- [ ] The cookie value is **not** the raw `ADMIN_TOKEN` (verified by inspecting `Set-Cookie`).
- [ ] The token comparison is constant-time (same code path / helper as the header gate — no plain `==`).
- [ ] Given `ADMIN_TOKEN` is unset, when the SPA loads, then it never calls `POST /session` and no session
      cookie is ever set.
- [ ] `document.cookie` in the browser console does **not** reveal the session cookie (HttpOnly verified).

#### OS2 — `requireOwner` accepts the session cookie *or* the header

The gate authorizes a request when **either** a valid `X-Admin-Token` header **or** a valid session cookie
is present.

- **Either credential.** `authorized()` (and the `capabilities` owner check) returns true for a valid header
  **or** a valid, unexpired session cookie. The header path is unchanged so API clients / scripts keep
  working exactly as before.
- **Capabilities reflects the cookie.** `GET /api/v1/capabilities` returns `owner: true` for a request
  carrying a valid session cookie — so on reload the SPA learns it is still the owner with no re-prompt.
- **Expiry / invalid cookie.** An expired or tampered/invalid cookie is treated as no credential → gated
  routes `401`, `capabilities.owner: false`. The browser is told to drop it (expired `Set-Cookie`) so it
  doesn't keep sending a dead cookie.
- **CSRF posture (load-bearing).** Adding a cookie reintroduces CSRF exposure that the header-only scheme
  was immune to (ADR-030 condition 3). `SameSite=Strict` is the primary mitigation; ADR-045 must state the
  full posture (e.g. `SameSite=Strict` is sufficient for these same-origin owner actions, and/or an
  additional check) and the security review must sign it off. State-changing routes must not be triggerable
  cross-site.

**Acceptance criteria — OS2**
- [ ] Given a valid session cookie, when I GET a gated route (e.g. `/admin/status`), then I get `200` with no
      `X-Admin-Token` header present.
- [ ] Given a valid session cookie, when I GET `/capabilities`, then `owner` is `true`.
- [ ] Given **no** credential, when I GET a gated route, then `401` (unchanged from today).
- [ ] Given a valid `X-Admin-Token` header and no cookie, when I GET a gated route, then `200` (header path
      unchanged — no regression for API/script clients).
- [ ] Given an expired/tampered session cookie, when I GET a gated route, then `401` and the response expires
      the cookie; `capabilities.owner` is `false`.
- [ ] A cross-site form/`fetch` cannot drive a state-changing gated route using the cookie (CSRF posture
      verified per ADR-045 / security review).

#### OS3 — Sign-out endpoint ends the session

A `DELETE /api/v1/session` (or `POST /api/v1/session/logout`, per ADR-045) clears the session.

- **Effect.** Responds `204` and sends a `Set-Cookie` that **expires** `holodex_session` immediately. After
  this, the cookie is gone and subsequent gated requests `401` until the owner re-authenticates.
- **Idempotent.** Calling sign-out with no/already-expired session still succeeds (`204`) and leaves the
  client signed out.

**Acceptance criteria — OS3**
- [ ] Given I am signed in (valid cookie), when I DELETE `/session`, then I get `204` and the response expires
      the cookie.
- [ ] After sign-out, a gated request `401`s and `capabilities.owner` is `false`.
- [ ] Calling sign-out when already signed out still returns `204` (idempotent).

#### OS4 — Frontend uses the session and stops relying on the in-memory token

The SPA authenticates via the exchange, then **relies on the cookie**, and reflects session state in the UI.

- **Sign-in flow.** The existing token form ([status/+page.svelte:108–120](../../web/src/routes/status/+page.svelte))
  calls `POST /session` instead of (or in addition to) setting the in-memory `adminToken`. On success it
  refreshes `capabilities`/activity; the cookie now rides on every subsequent request automatically.
- **Credentialed requests.** The authed fetch wrappers (`getAuthed`/`sendAuthed`/`uploadAuthed` in
  [api.ts](../../web/src/lib/api.ts)) send the cookie. Because the cookie is same-origin and the SPA is
  served from the same origin as the API (ADR-007), `fetch` must use `credentials: 'same-origin'` (or
  `'include'` for the dev proxy case — ADR-045 to specify) so the cookie is actually attached.
- **Reload behavior.** On cold load the SPA calls `capabilities`; if `owner` is true (cookie still valid) it
  renders the owner view **without** prompting for a token. The in-memory `adminToken` variable is no longer
  the source of truth for "am I owner" — `capabilities.owner` is.
- **Sign-out affordance.** A visible "Sign out" control (where the owner controls live, e.g. the status /
  activity page) calls `DELETE /session` and returns the UI to the token-prompt / read-only state.
- **Graceful expiry.** When a gated request `401`s (session expired mid-use), the SPA drops to the
  read-only/token-prompt view cleanly — no error spew, no wedged controls — inviting re-auth.
- **No token in JS storage.** The frontend must **not** write the token to `localStorage`/`sessionStorage`
  at any point. The in-memory variable may remain only as a transient for the single exchange request, or be
  removed entirely.
- **Theming.** Any new control (Sign out, session-expired hint) uses semantic tokens only and is QA'd in
  **Cinémathèque, Broadcast, and Brutalist** (project frontend-theming rule). Use `--warn` tokens for the
  "session expired" notice, not `--accent`.

**Acceptance criteria — OS4**
- [ ] Given `ADMIN_TOKEN` is set, when I enter the token in the UI and reload the page, then I am still in the
      owner view and gated controls work — **without** re-entering the token.
- [ ] Given I reload, when the SPA boots, then it determines owner state from `capabilities.owner` (cookie),
      not from a re-prompt or in-memory token.
- [ ] Given I click "Sign out", then the owner controls disappear, the cookie is cleared, and a later reload
      shows the token prompt again.
- [ ] Given my session expires while the app is open, when I next trigger a gated action, then the UI cleanly
      returns to the token-prompt/read-only state (no unhandled error).
- [ ] No code path writes the admin token to `localStorage` or `sessionStorage` (grep clean).
- [ ] Sign-out control and any session notices render correctly in all three skins (warn-token styling for
      the expiry notice).

#### OS5 — Tests cover both credential paths and the lifecycle

- **Backend.** Extend [auth_test.go](../../internal/api/auth_test.go): exchange success sets cookie / failure
  doesn't; gated route authorized by cookie; gated route authorized by header (no regression); expired /
  tampered cookie → `401`; sign-out expires cookie; `capabilities.owner` reflects cookie state; constant-time
  compare still on the exchange path; gate-open (`ADMIN_TOKEN` unset) passthrough unaffected; **short vs.
  "trust this device" `Max-Age`** issued correctly and not client-forgeable (OS6); **sliding re-issue** on
  active use renews the same lifetime class and never resurrects an expired cookie (OS7).
- **Frontend.** A check/test that reload preserves owner state given a valid cookie, that `401` drops to the
  prompt, and that the token never lands in web storage.
- **Testing strategy.** Update [docs/testing-strategy.md](../testing-strategy.md) to record the cookie-session
  cases (significant auth change per the change-routing table).

**Acceptance criteria — OS5**
- [ ] Backend tests cover: exchange ok/fail, cookie-authorized route, header-authorized route, expired/tampered
      cookie, sign-out, capabilities reflection, gate-open passthrough.
- [ ] A frontend test/check asserts reload-persistence and the no-web-storage property.
- [ ] `docs/testing-strategy.md` reflects the new session cases.

#### OS6 — "Trust this device" longer session

An opt-in choice at sign-in that issues a longer-lived cookie for a private machine, distinct from the
default bounded session.

- **Affordance.** A "Trust this device" checkbox (or equivalent) on the token-entry form. Unchecked is the
  default, bounded session (OS1's default `Max-Age`); checked issues a **longer `Max-Age`** cookie
  (concrete value in ADR-045, e.g. 30 days).
- **Wired through the exchange.** The choice is a parameter on `POST /session` (header/body field per
  ADR-045) that the server reads to set the cookie's `Max-Age`. The server — not the client — decides the
  two lifetime values; the client only signals which.
- **Same security properties.** Both lifetimes use the identical cookie attributes (`HttpOnly`, `Secure`,
  `SameSite=Strict`); "trust this device" changes **only** duration, never the JS-readability or CSRF
  posture. The longer lifetime is a deliberate, owner-opted blast-radius trade-off, called out for the
  security review.
- **Default stays short.** Omitting/unchecking the option must yield the short default — a public/shared
  machine is never accidentally granted the long session.

**Acceptance criteria — OS6**
- [ ] Given I sign in **without** "trust this device", then the cookie carries the short default `Max-Age`.
- [ ] Given I sign in **with** "trust this device", then the cookie carries the longer `Max-Age`; all other
      cookie attributes (`HttpOnly`, `Secure`, `SameSite=Strict`, `Path`) are identical.
- [ ] The lifetime is set **server-side** from the exchange parameter — a client cannot request an arbitrary
      `Max-Age` (only the two server-defined options).
- [ ] The checkbox renders correctly in all three skins (semantic tokens only).

#### OS7 — Sliding expiry on active use

An actively-used session does not expire out from under the owner; an idle one still lapses at its `Max-Age`.

- **Renewal on use.** When a request carrying a valid, not-yet-expired session cookie reaches the gate, the
  server **re-issues** the cookie with a refreshed `Max-Age` (a fresh `Set-Cookie`), so continued activity
  keeps the session alive. An idle session (no requests) still expires at its last-issued lifetime.
- **Bounded renewal.** Renewal slides the **same** lifetime class — a short (default) session renews to the
  short window, a "trust this device" (OS6) session renews to its long window. ADR-045 decides whether to
  throttle re-issue (e.g. only refresh when the cookie is past, say, half its life) to avoid a `Set-Cookie`
  on literally every request, and whether there is an absolute cap on total session age.
- **Idle-then-return.** A session left idle past its window is expired on return → `401` → graceful drop to
  the prompt (OS2/OS4 behavior). Sliding expiry never resurrects an already-expired cookie.

**Acceptance criteria — OS7**
- [ ] Given a valid session, when I make a gated request, then the response re-issues the cookie with a
      refreshed `Max-Age` of the same lifetime class (short renews short; trusted renews long).
- [ ] Given a session idle past its window, when I next make a gated request, then it is expired (`401`) and
      not renewed.
- [ ] Re-issue does not change the lifetime class or other cookie attributes; a short session never silently
      becomes a long one through renewal.
- [ ] (If ADR-045 sets an absolute cap) a session cannot be renewed indefinitely past the absolute maximum
      age.

### Nice-to-Have (P1)

- **Session-expired toast.** A small, themed notice ("Your owner session expired — re-enter your token") on
  the `401`-drop, rather than a silent fall-back, so the transition is legible.

### Future Considerations (P2)

- **Real multi-user auth supersedes this.** When accounts/roles are decided, the session cookie becomes the
  carrier for a real identity and `ADMIN_TOKEN` becomes a bootstrap/legacy path — the exchange endpoint and
  cookie plumbing are designed so that swap is an identity-source change behind `requireOwner`, not a rework
  of the call sites (consistent with ADR-030's "future-proof by construction").
- **Server-side session revocation list.** If self-contained cookies prove insufficient (need to force-revoke
  a leaked session before its `Max-Age`), introduce a minimal server-side session record — deliberately
  deferred to keep zero-dependency now.
- **Rotation of the cookie-signing secret** as a documented operational step.

---

## Success Metrics

Single-user personal server → qualitative / self-observed:
- **Persistence works:** the owner authenticates once and a reload no longer logs them out; re-entry of the
  token in normal daily use effectively stops.
- **Security preserved:** `/security-review` signs off that the cookie scheme keeps the no-JS-readable /
  no-exfiltration property and an acceptable CSRF posture — i.e. persistence was added without regressing the
  protections ADR-030 established.
- **No regression to the open default:** `ADMIN_TOKEN` unset still yields the zero-config, no-prompt,
  no-cookie experience; header-based API/script clients still authenticate exactly as before.
- **Clean lifecycle:** sign-out and expiry both return the UI to the prompt state without errors, in all
  three skins.

## Open Questions

- **(security/eng — blocking) Cookie value scheme.** Signed token (HMAC of an opaque session id / expiry with
  a server secret) vs. an encrypted opaque value. Where does the signing secret come from (derive from
  `ADMIN_TOKEN`? separate `SESSION_SECRET`?) and how is it rotated? Resolve in ADR-045 / security review.
- **(security — blocking) CSRF posture under a cookie.** Is `SameSite=Strict` alone sufficient for the
  same-origin owner actions, or is an additional anti-CSRF measure (e.g. continue to require the
  `X-Admin-Token` header *or* a double-submit token on state-changing routes) warranted? Security review must
  bless the final posture.
- **(eng — blocking) `Secure` flag in local dev.** Exact rule for omitting `Secure` on localhost/loopback
  while always setting it for exposed deployments, so `make web-dev` works without weakening production.
- **(eng) Session lifetimes.** Concrete `Max-Age` for the short default (proposal: 7 days) **and** the
  "trust this device" long session (OS6, proposal: 30 days); the sliding-expiry renewal throttle and any
  absolute age cap (OS7).
- **(eng) Request shape for the exchange.** Token via `X-Admin-Token` header on `POST /session` (reuse) vs. a
  JSON body field — pick the one that keeps the token out of logs and is simplest for the SPA.
- **(eng) Dev-proxy credentials mode.** Confirm whether the Vite `/api` proxy needs `credentials: 'include'`
  vs `'same-origin'` for the cookie to attach in `make web-dev`.

## Timeline / Phasing

No hard deadline. Suggested order:
1. **ADR-045** — decide cookie scheme, CSRF posture, `Secure`/dev rule, lifetime (resolves the blocking Open
   Questions). Pair with an early `/security-review` read so the design is blessed before code.
2. **OS1–OS3 + OS6–OS7 (backend)** — exchange endpoint (with the trust-this-device lifetime parameter),
   gate accepts cookie-or-header, sliding re-issue on use, sign-out; backend tests (OS5).
3. **OS4 + OS6 (frontend)** — wire the sign-in form to the exchange, the "trust this device" checkbox, rely
   on `capabilities.owner` on reload, sign-out control, graceful `401` fallback; remove token-in-memory
   reliance; 3-skin QA.
4. **`/security-review` sign-off + testing-strategy update (OS5)** — required before merge; record the ADR-030
   amendment.
