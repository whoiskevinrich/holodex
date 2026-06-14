# Spec: System Activity — "Under the Hood" (F21)

**Status**: Draft
**Phase**: Post–Phase 2 (standalone feature; builds on F13 Observability)
**Owner**: Project owner
**Date**: 2026-06-14

**Depends on**: Phase 2 complete (F11 thumbnails, F13 metrics/admin, F20 mapping reload).
**New ADRs required**:
- **[ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md) (Proposed)** — User-facing activity surface & job-history persistence (extends [ADR-019](../architecture/ADR-019-observability-conventions.md)). Covers the new `activity` read-model endpoint, the job-history table, and 30-day retention.
- **ADR-029 (reserved, P1)** — Live activity transport (Server-Sent Events) for real-time push (to be drafted when SSE is scheduled).
- **[ADR-030](../architecture/ADR-030-access-control-gating-seam.md) (Proposed, P0 — required before build)** — Access-control / "Pro mode" gating seam for owner-only surfaces (precondition for future multi-user). Pulled into P0 because F21.6 exposes infrastructure-affecting controls that are unauthenticated today.

---

## Problem Statement

Holodex does real work in the background — an initial library scan, a periodic reconciliation pass, a debounced filesystem watcher, and a background thumbnail-generation queue — but none of it is visible in the product. Today the only ways to answer *"what is the system doing right now?"* are to tail the container logs (`scan complete seen=… added=…`), scrape the operator-facing Prometheus `/metrics` endpoint, or hit the thin `GET /api/v1/admin/status` (which returns only `{thumbnail_queue_depth}`). For the owner of a personal media server, that's an opaque, log-diving experience: after dropping in 200 new files there is no in-product way to see indexing progress, confirm the library is current, or notice that 12 files failed extraction.

## Goals

1. **Answer "is it working / is it done?" without leaving the app.** The owner can see, on one screen, whether a scan or thumbnail backfill is in progress and what the last run did — no log tailing, no `curl /metrics`.
2. **Make background work legible.** Surface the jobs that exist (scan, thumbnail generation, config reload) with human-readable state, counts, and timing.
3. **Surface failures, not just successes.** Per-run error counts (and the last error) are visible in-product so silent extraction failures stop being silent.
4. **Put existing controls one click away.** The admin actions that already exist as API endpoints (rescan, reload mapping config, regenerate a thumbnail) become safe, confirmable buttons.
5. **Build a short history.** A 30-day timeline of job runs lets the owner spot patterns (a scan that's getting slower, recurring errors).

## Non-Goals

- **Multi-user accounts / authentication.** Holodex stays single-user for this feature. We design the surface as *owner-only* and leave a gating seam (ADR-030) for a future "Pro mode" or multi-user release, but we do **not** build auth here. *(Why: out of scope for a personal server today; premature without the multi-user decision.)*
- **A full metrics/Grafana replacement.** Prometheus `/metrics` remains the path for charts, alerting, and long-term retention. This surface is a *live operational view*, not a time-series analytics tool. *(Why: `/metrics` + Grafana already does this well; duplicating it is wasted effort.)*
- **New background jobs.** This feature *observes and controls existing* work (scanner, thumbnail queue). It does not add enrichment, preview-trailer, or writeback jobs — those are Phase 3 (F16–F18). *(Why: keep scope to making current work visible.)*
- **Per-file job drill-down / a job log viewer.** We show run-level summaries and counts, not a streaming per-file event log. *(Why: the structured logs already serve deep debugging; a full log UI is a separate, larger effort.)*
- **Configurable retention / export of history.** History is fixed at 30 days, pruned automatically. No UI to change the window or export CSV in v1. *(Why: 30 days covers the "spot a pattern" goal; configurability is P2 polish.)*

---

## User Stories

**Owner — observing**
- As the **library owner**, I want to see whether a scan is running right now so that I know whether to wait before expecting newly-added files to appear.
- As the **library owner**, I want to see the results of the last scan (added / updated / removed / skipped / **errors**) so that I can confirm my new files were indexed and notice anything that failed.
- As the **library owner**, I want to see how many thumbnails are still being generated so that I know why some cards still show placeholders.
- As the **library owner**, I want at-a-glance library totals (videos, people, tags) so that I have a sense of the catalog's size and growth.
- As the **library owner**, I want a small live indicator in the header on every page so that I can tell the system is busy without navigating to a dedicated screen.

**Owner — acting**
- As the **library owner**, I want to trigger a full rescan from the UI (with a confirm) so that I can force a re-index after a bulk change without using `curl`.
- As the **library owner**, I want to reload my metadata-mapping config from the UI so that edits to `metadata-mappings.yaml` take effect without restarting the container.

**Owner — history**
- As the **library owner**, I want a timeline of the last 30 days of scan runs so that I can see whether scans are slowing down or repeatedly erroring.

**Forward-looking (not built in v1, shapes design)**
- As a **future non-owner user / "Pro mode" toggle**, I should see a friendly read-only summary but **not** the destructive controls or raw internals.

---

## Requirements

### Must-Have (P0)

#### F21.1 — Activity read-model API
Extend the backend with a single status read-model that aggregates live state from the scanner, thumbnail pipeline, repository, and health subsystem.

- **Endpoint**: a **new** `GET /api/v1/admin/activity` carries the full read-model below. The existing `GET /api/v1/admin/status` is **preserved unchanged** as the legacy minimal shape (`{thumbnail_queue_depth}`) so nothing consuming it breaks; new clients use `activity`. *(Resolves Open Question 2.)*
- **Payload** (shape illustrative, finalized in ADR-028):

```jsonc
{
  "scan": {
    "state": "idle | running",
    "trigger": "initial | periodic | watch | manual",   // of the in-flight or last run
    "started_at": "2026-06-14T18:02:01Z",               // null when idle
    "last_run": {                                        // null until first run completes
      "trigger": "manual",
      "finished_at": "2026-06-14T18:01:40Z",
      "duration_ms": 8421,
      "seen": 204, "added": 12, "updated": 1,
      "removed": 0, "skipped": 191, "errors": 2
    },
    "next_scheduled_at": "2026-06-14T18:07:01Z"          // periodic tick estimate; null if disabled
  },
  "thumbnails": {
    "queue_depth": 7,            // pending (F11.8)
    "high": 2, "normal": 5,      // tier split
    "in_flight": 1,
    "workers": 4
  },
  "library": { "videos_active": 1840, "videos_inactive": 12, "people": 96, "tags": 211 },
  "system": {
    "ready": true, "uptime_seconds": 36120, "version": "1.4.0",
    "media_path_present": true,                  // boolean, never the path
    "controls_unauthenticated": false            // true when bound non-loopback with no ADMIN_TOKEN (F21.7 cond. 1)
  }
}
```

- **Acceptance criteria**:
  - Given a scan is in progress, when I GET `/api/v1/admin/activity`, then `scan.state == "running"` with a non-null `started_at` and the correct `trigger`.
  - Given at least one scan has completed, then `scan.last_run` reflects that pass's summary counts (matching the `scan complete` log line and `holodex_indexed_files_total` deltas).
  - The endpoint returns within the same latency envelope as today's `admin/status` (no full table scans on the hot path; library counts may be served from a short cache).
  - No secrets are exposed — `media_path_present` is a boolean, **not** the path; no env values, tokens, or absolute paths in the payload.

#### F21.2 — Scanner status accessor
The scanner exposes its current state without changing its single-pass concurrency guarantee.

- **Acceptance criteria**:
  - A `Status()` accessor reports `{state, trigger, started_at, last_run, next_scheduled_at}` derived from the existing `scanMu` / `stats` / ticker, with no new lock contention on the scan hot path.
  - `trigger` correctly distinguishes `initial`, `periodic`, `watch`, and `manual` (matches the four call sites in `internal/scanner/scanner.go`).
  - When `MEDIA_PATH` is unset, `state` is `idle` and `next_scheduled_at` is `null`.

#### F21.3 — Persisted job history (30-day)
Each completed scan pass is recorded durably so the UI can show a timeline across restarts.

- **Backend**: new migration (next sequential, e.g. `0004_job_runs`) adding a `job_runs` table:

```
job_runs
  id           PK
  kind         text     -- "scan" (extensible: "thumbnail_backfill", "reload_config")
  trigger      text     -- initial | periodic | watch | manual
  status       text     -- success | error
  started_at   timestamp
  finished_at  timestamp
  duration_ms  integer
  seen, added, updated, removed, skipped, errors  integer
  error_message text    -- nullable; populated when the pass errored
```

- **Retention**: runs older than **30 days** are pruned automatically (on insert or via a lightweight periodic sweep — decided in ADR-028). No configuration in v1.
- **Endpoint**: `GET /api/v1/admin/activity/history?days=30` (default and max 30) returns runs newest-first.
- **Acceptance criteria**:
  - After a scan completes, a `job_runs` row exists with counts matching the run summary.
  - A row whose `started_at` is older than 30 days is absent after the next prune.
  - History survives a process restart (it's in SQLite, not memory).
  - Recording a run never blocks or fails the scan itself (best-effort write, logged on failure — consistent with the "never abort the scan" NFR in ADR-019).

#### F21.4 — Dedicated activity page (polled)
A themed `/status` route (final path TBD in design-handoff; `/status` or `/admin`) renders the activity read-model.

- **Sections**: current scan card, thumbnail-queue card, library totals, 30-day history timeline/list, controls (F21.6).
- **Refresh**: client polls `GET /api/v1/admin/activity` on an interval (default ~3s) while the page is focused; backs off when the tab is hidden. Polling is the **baseline transport** and the permanent fallback for F21.8 (SSE).
- **States**: loading, empty (no runs yet), error (API unreachable), and the live grid must all be designed and themed.
- **Acceptance criteria**:
  - Given a scan starts, then within one poll interval the scan card switches to "running" with elapsed time.
  - Given the API is unreachable, then the page shows a themed error state, not a blank screen or console crash.
  - **Theming (ADR-021):** tokens only — no `zinc-*`/`sky-*`/hex/named-font/fixed-radius literals; `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` stays empty for new components.
  - **QA all three skins** — Cinémathèque, Broadcast, Brutalist all render the page (including the running-spinner and history timeline) without collision or unreadable accent-on-background.

#### F21.5 — Header activity indicator
A small live indicator in the shared header, present on every page, reflects whether background work is active.

- **Behavior**: shows an active/busy affticon (e.g. a subtle animated state on the existing header hook) when `scan.state == "running"` **or** `thumbnails.queue_depth > 0`; idle/hidden otherwise. Clicking it navigates to the activity page.
- **Acceptance criteria**:
  - The indicator reflects state within one refresh cycle and clears when work finishes.
  - It is driven by the same read-model as the page (one source of truth), and uses a shared themed hook class (per ADR-021) — no per-skin markup, no hardcoded styling.
  - QA'd in all three skins (the indicator must read against each skin's header background).

#### F21.6 — In-UI controls (wires existing admin actions)
Expose the **already-existing** admin endpoints as confirmable buttons on the activity page.

| Control | Endpoint (existing) | Behavior |
|---|---|---|
| Rescan library | `POST /api/v1/admin/rescan` (F13.3) | Confirm dialog → 202; button shows "scan starting…"; reflects `started:false` (already running) gracefully. |
| Reload mapping config | `POST /api/v1/admin/reload-config` (F20.10) | Confirm → toast with field count from the response. |
| Regenerate thumbnail | `POST /api/v1/media/{id}/thumbnail` (F11) | (Surfaced from this page only if a "stuck thumbnails" affordance is shown; otherwise lives on the media detail page.) |

- **Acceptance criteria**:
  - Destructive/expensive actions (rescan) require an explicit confirm before firing.
  - A `409`/`started:false` response is shown as informational ("a scan is already running"), not an error.
  - **Security gate (CLAUDE.md):** because this exposes infrastructure-affecting controls, `/security-review` must sign off before merge. The review must explicitly address that these endpoints are currently **unauthenticated** and reachable by anyone who can load the app, with the F21.7 gating seam (also P0) as the required mitigation.

#### F21.7 — Owner-only gating seam ("Pro mode" ready) — *promoted to P0*
Introduce a single gating seam so the activity surface and its controls can be restricted, without building full auth now. **Lands with v1** (resolves Open Question 7): exposing rescan/reload as one-click controls on a potentially reverse-proxied server makes an unguarded surface unacceptable to ship.

- **Acceptance criteria**:
  - A server-side seam (e.g. a middleware/guard + a `PRO_MODE`/owner flag) wraps the activity read-model, history, stream, and admin-control routes. In v1 the default may be open (single-user), but the seam exists, is the single choke point for all these routes, and is exercised by a test.
  - The frontend reads a capability flag and hides controls/raw internals for a non-owner view — verified by toggling the flag in a test, even though no real non-owner exists yet.
  - **ADR-030** records the access-control decision and the migration path to real multi-user auth, and is accepted before implementation begins.
  - `/security-review` confirms the seam adequately mitigates the unauthenticated-controls risk from F21.6.

- **Security conditions (sign-off from the security review of ADR-028/030 — required for ADR-030 to move Proposed → Accepted):**
  1. **Default-open must fail loud.** When the gate is pass-through because `ADMIN_TOKEN` is unset, the server must detect the dangerous case — bound to a **non-loopback** interface (`HOST`) *and* no token set — and emit a prominent `warn`/`error` at startup **and** surface it in the activity read-model's `system` section (so the page shows "controls are unauthenticated"). The single-user loopback case stays silent/zero-config. *Acceptance: starting with a non-loopback bind and empty `ADMIN_TOKEN` produces the warning log + a `system` flag; loopback or token-set does not.*
  2. **Token comparison and transport.** The token check uses a **constant-time** comparison (`crypto/subtle.ConstantTimeCompare`), never `==`. If a cookie carries owner identity to the SPA it must be `HttpOnly`, `Secure`, and `SameSite=Lax`/`Strict`; the raw token is **never** stored in `localStorage`. *Acceptance: unit test for the comparison path; cookie attributes asserted if the cookie scheme is used.*
  3. **CSRF protection on state-changing controls.** The admin `POST` routes (`rescan`, `reload-config`, regenerate-thumbnail) must not be triggerable cross-site. Preferred scheme: require the token in a **request header** (which cross-site forms cannot set), avoiding ambient-cookie CSRF entirely; if a cookie scheme is chosen instead, add `SameSite` **plus** a CSRF token. *Acceptance: a cross-site form POST to an admin route without the header/CSRF token is rejected.*

### Nice-to-Have (P1)

#### F21.8 — Live push via Server-Sent Events
Replace polling with a push stream for instant updates, keeping polling as the automatic fallback.

- **Endpoint**: `GET /api/v1/admin/activity/stream` (SSE) emits a status event on scan state transitions, queue-depth changes (debounced), and run completions.
- **ADR-029** decides the transport (SSE chosen over WebSocket for one-way, proxy-friendly simplicity).
- **Acceptance criteria**:
  - With SSE connected, the scan card and header indicator update with no perceptible poll delay.
  - If the SSE connection drops or is unsupported, the client transparently falls back to F21.4 polling.
  - The header indicator subscribes to the same stream (no second connection per page where avoidable).

### Future Considerations (P2)

- **F21.9** — Configurable history retention and CSV export.
- **F21.10** — Surface Phase 3 jobs (enrichment fetches F16, preview-trailer generation F18, writeback F17) in the same activity model once they exist — `job_runs.kind` is already extensible for this.
- **F21.11** — Trend sparklines on the history timeline (scan duration over time, error rate) — lightweight, client-rendered from the 30-day history.
- **F21.12** — A friendly, non-technical "everything's up to date / indexing N items" summary mode for the eventual general-user audience.

---

## Data Model Extensions

```
job_runs
  id            PK
  kind          text         -- "scan" (extensible)
  trigger       text         -- initial | periodic | watch | manual
  status        text         -- success | error
  started_at    timestamp
  finished_at   timestamp
  duration_ms   integer
  seen          integer
  added         integer
  updated       integer
  removed       integer
  skipped       integer
  errors        integer
  error_message text         -- nullable
```

Index on `started_at` for the history query and the retention prune. Migration follows ADR-016 (sequential numbered migrations).

---

## Success Metrics

This is a personal, self-hosted tool, so metrics are framed as owner-experience outcomes, measured by the owner, not a fleet analytics pipeline.

**Leading indicators**
- **Time to answer "is it done indexing?"** drops from "open a terminal, tail logs" (~minutes) to a glance at the page/header (<5s). *Measure: self-reported, before/after.*
- **Log-diving frequency**: the owner no longer needs `docker logs` to confirm a scan finished in normal operation. *Target: zero log checks for routine "did it index?" questions within the first week of use.*
- **Failure visibility**: extraction errors that previously went unnoticed are now seen — at least the *existence* of `errors > 0` is surfaced on every run.

**Lagging indicators**
- **Confidence**: the owner trusts the library is current without manual spot-checks.
- **Faster diagnosis**: when something's wrong (slow scans, recurring errors), the 30-day history makes the pattern visible without external tooling.

---

## Open Questions

1. ~~**Route & placement**~~ — **Resolved** in the [design handoff](../design/system-activity-handoff.md): page at **`/status`** (nav label "Status", peer of `/keys`); header indicator is a **compact pill** in the nav (left of the skin switcher) shown **only while work is active**, with a pulsing accent dot.
2. ~~**`admin/status` compatibility**~~ — **Resolved**: add a new `GET /api/v1/admin/activity` for the full read-model and leave `GET /api/v1/admin/status` as the minimal legacy shape (F21.1). *(Engineering still confirms nothing else consumes the old shape — tests/MCP/dashboards — but the direction is set.)*
3. **Library counts cost** *(engineering)* — Are `COUNT(*)` over videos/people/tags cheap enough to compute per poll at personal scale, or should they be cached with a short TTL (ristretto/Noop seam, ADR-008/ADR-022)?
4. **`next_scheduled_at` accuracy** *(engineering)* — The periodic ticker doesn't currently expose its next fire time; is a best-effort estimate (last tick + interval) acceptable, or do we track the deadline explicitly?
5. **Thumbnail in-flight count** *(engineering)* — The queue exposes `depth()` but not in-flight worker count; is adding an in-flight counter worth the small change to `internal/thumbnail/worker.go`, or is queue depth alone sufficient for v1?
6. **SSE vs poll default** *(engineering, ADR-029)* — Ship P0 polling first and add SSE as a clean upgrade, or design the client transport-agnostic from day one so F21.8 is a drop-in? (Recommended: transport-agnostic client store, polling impl first.)
7. ~~**Auth reality check**~~ — **Resolved**: the gating seam (F21.7) is **promoted to P0** and lands with v1. Exposing rescan/reload as one-click controls on a possibly reverse-proxied server makes an unguarded surface unacceptable to ship; ADR-030 must be accepted before build and `/security-review` confirms the mitigation.

---

## Cross-References

- Builds on: [phase-2-mcp-polish.md](phase-2-mcp-polish.md) F11 (thumbnails), F13 (observability/admin), F20 (mapping reload).
- ADRs: [ADR-019](../architecture/ADR-019-observability-conventions.md) (observability conventions, extended by ADR-028), [ADR-016](../architecture/ADR-016-database-migrations.md) (migrations), [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) (theming/skins), [ADR-008](../architecture/ADR-008-caching.md)/[ADR-022](../architecture/ADR-022-defer-in-process-cache.md) (caching seam).
- Design: [system-activity-handoff.md](../design/system-activity-handoff.md) — `/status` page + header indicator, component breakdown, states, three-skin QA.
- Forward link: Phase 3 jobs ([phase-3-enrichment.md](phase-3-enrichment.md) F16–F18) plug into `job_runs.kind` (F21.10).
