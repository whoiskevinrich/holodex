# Spec: Entity refresh sweep — refresh all people / studios from System Activity (F66)

**Status**: Draft
**Phase**: 4 (Curation tooling)
**Issue**: [HOLODEX-421](https://whoiskevinrich.atlassian.net/browse/HOLODEX-421)
**Depends on**: the enrichment review workflow and its auto-apply routing ([enrichment-review-workflow.md](enrichment-review-workflow.md), F47; [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) D1 — `enrich.SingleStrongMatch`), the metadata provider contract ([metadata-provider-contract.md](metadata-provider-contract.md)), metadata source plugins ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md), F22), the System Activity surface and job-run history ([ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md), F21; [ADR-071](../architecture/ADR-071-job-run-attribution-and-paginated-history.md) — `batch_id`, `EntityType/EntityID`), the library-wide extraction pass as the background-job template ([ADR-067](../architecture/ADR-067-filename-extraction-confidence-and-rollback.md), F48.5b — `extract.BatchRunner.TriggerAll`), per-field source-of-truth decisions for revert ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md), F36), and the access-control gating seam ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)).
**Amends**: F47 — lifts its *Queue-wide bulk/background resolution* Non-Goal / P2-1 by supplying the provider rate-limit contract that Non-Goal was waiting on (§ Provider traffic). F47's per-row lazy model is unchanged; this spec adds the bulk path beside it.
**Architecture**: [ADR-103](../architecture/ADR-103-provider-traffic-contract-and-enrich-sweep.md) — pacing on `enrich.Service` keyed by provider (D1, `x/time/rate` D2, ADR-080 carriage D3), `429` as a typed pause the *caller* decides to wait on (D4), monotonic injected clock (D5), per-sweep breaker in the runner (D6), shared `Service.RefreshPair` (D7), `SweepRunner` + `sweep` activity block (D8), `batch_id` audit key (D9), TMDB `429` pass-through (D10).
**Design handoff**: [entity-refresh-sweep-handoff.md](../design/entity-refresh-sweep-handoff.md) — Option D, seven states, three-skin QA checklist.

---

## Objective

Let the owner bring **every** person or studio current against its metadata providers in **one
action**, unattended, with the same judgment the single-entity *Refresh all* already applies — a
linked provider refreshes; an unlinked one auto-links on **exactly one** strong match and is otherwise
left for review — and come back to a report that says what changed and where to look.

> **Why this is needed.** F47 made a single entity cheap to bring current (`Refresh all` on the
> detail page, `e` hotkey) and deliberately stopped there: bulk resolution was a Non-Goal "until the
> provider rate-limit contract exists." A library of a few hundred people and studios still has no
> path from "I added a provider" or "the provider improved its data" to "everything is current"
> except opening every page. This spec supplies the rate-limit contract and the sweep together,
> because neither is safe alone.

**The rule, stated once.** For each `(entity, provider)` pair where the provider's `entity_types`
includes the entity's kind:

| Pair state | Sweep does | Existing code |
|---|---|---|
| Linked (`entity_enrichment` row) | Refresh fields from the stored `external_id` | `refreshOneProvider` linked branch |
| Unlinked, dismissed (RD4 "None of these match") | **Skip** | `EnrichmentDismissed` check |
| Unlinked, resolve → exactly one candidate `confidence >= 0.85` | **Auto-apply** (link + adopt fields) | `enrich.SingleStrongMatch` → `Enrich` |
| Unlinked, resolve → 0 candidates | Nothing; counted `no_candidates` | — |
| Unlinked, resolve → ≥2 strong, or only possible/weak | Nothing; counted `needs_review` — the pair stays in the F47 queue | — |

**No new matching rule.** The sweep calls `refreshOneProvider` per pair — the code path one detail-page
click already runs — never a re-implementation of it. Anything that would change the table above is
a change to F47/ADR-066, not to this spec.

---

## Scope

### In scope

- **Two owner-triggered sweeps** — people, studios — started from `/owner/status` → Actions, each a
  single background job over every entity of that kind.
- **Staleness skip with a force option** — a linked pair refreshed in the last **24 h** is skipped by
  default; the confirm shows how many pairs are due and offers *Refresh everything* to override.
- **Live progress** on the activity read-model; **status line** on the matching list page; the job's
  **summary + per-entity audit trail** in System Activity under one `batch_id`.
- **Provider traffic contract** — core paces every outbound sidecar call, sidecars may declare their
  limit, operators may override it, back-pressure via `429 Retry-After`, per-provider circuit breaker.
- **Single flight** — one sweep at a time across both kinds.

### Out of scope (Non-Goals)

- **Films.** The film entity has its own provider enrichment (F56/F59) and would ride the same code
  path; the trigger is a third button. Deferred to P2 to keep this to the ask — the `{kind}` route
  parameter is left open so films is a follow-up, not a redesign.
- **Scheduled / automatic sweeps** (nightly, on provider reload, on startup). Every sweep is a click.
  F22's "strictly on-demand: nothing is fetched without an explicit click" posture stands; a sweep is
  one explicit click for many fetches, not zero clicks.
- **Resume after restart.** The job dies with the process (server-lifetime context, as Extract all).
  The 24 h skip makes re-running cheap; that is the resume story.
- **Whole-batch revert of auto-applied links.** Each link the sweep creates is revertable through the
  existing per-field Revert on the entity page (F36/ADR-051). A one-click "undo this sweep" is P2 —
  writeback batches have it (F30), enrichment batches do not, and building it needs its own snapshot
  design.
- **Sidecar-owned batching** (`/resolve/batch`) — [HOLODEX-422](https://whoiskevinrich.atlassian.net/browse/HOLODEX-422).
- **Cross-provider confidence calibration.** Still F47 P2-3; `>= 0.85` stays provider-native and advisory.
- **Re-extracting the file layer.** People and studios have no file; the media-page F31 Refresh is a
  different action and is not swept.
- **Video / media sweeps.** Video enrichment goes through the extraction and review queues (F47, F48);
  a bulk media path is its own initiative.

---

## User stories

- As the **owner**, I want to refresh every person against every provider in one action so that a
  provider I just added, or one that improved its data, reaches the whole library without a page-by-page tour.
- As the **owner**, I want a pair that already refreshed today to be skipped by default so that
  re-running a sweep — after a crash, or because I clicked twice — doesn't spend hundreds of provider
  calls redoing an hour-old result.
- As the **owner**, I want to force a full sweep when I know the provider's data changed so that the
  24 h skip never stands between me and current data.
- As the **owner**, I want an unlinked person to gain a provider link only when exactly one candidate is
  a strong match so that the sweep never guesses on my behalf; ambiguous ones wait for me in the queue.
- As the **owner**, I want to leave the status page and still see the sweep progressing on `/people` so
  that I don't have to babysit one screen.
- As the **owner**, I want the finished sweep to tell me how many entities gained a link, and to see
  *which* ones, so that an unattended link I disagree with is one click from its Revert.
- As the **owner**, I want a provider that goes down mid-sweep to stop being called so that the sweep
  finishes on the healthy providers instead of timing out 200 times.
- As a **sidecar author**, I want to declare my upstream's rate limit so that a sweep never gets me
  banned; and I want to answer `429 Retry-After` when my upstream pushes back and have core honour it.
- As the **operator**, I want to cap a provider's rate in `metadata-sources.yaml` regardless of what
  the sidecar declares.
- As a **visitor**, I never see any of this.

---

## Requirements decisions (RD)

- **RD1 — The sweep is `refreshOneProvider` × providers × entities, sequentially over entities.**
  Inside one entity the existing per-provider fan-out runs concurrently (as one click does today); the
  next entity starts when the previous one finishes. In-flight requests are therefore bounded by
  provider count, not catalog size. The handler and the sweep share one service-level function; the
  handler must not grow a second copy of the routing.
- **RD2 — Staleness skip is a linked-pair rule keyed on `entity_enrichment.fetched_at`.** Default
  window **24 h**. Unlinked pairs have no row (that is F47 RD2's queue membership signal) and no
  attempt marker (F47 P2-2 deferred), so in P0 they are **always re-resolved** — bounded by the rate
  limit, and no worse than the per-row click they replace. P1-1 adds the marker.
- **RD3 — *Refresh everything* is a per-sweep flag, not a setting.** A checkbox in the confirm row
  (`force: true` on the request); unchecked on every open. Nothing persists.
- **RD4 — Single flight across kinds.** One `TryLock` guards both sweeps (`extract.BatchRunner`
  pattern). A second `POST` while running answers `202 {started:false}` — informational, never an error.
- **RD5 — Progress is server truth on the existing activity poll.** `GET /admin/activity` gains a
  `sweep` block; the SPA's 3 s poll already runs on every owner page, so both the status page and the
  list pages derive their state from one source and survive navigation and reload. No SSE, no new poll.
- **RD6 — One `batch_id` ties the sweep together.** The summary `JobRun` (new kind `enrich-sweep`)
  and every per-entity run the sweep produces through the existing `RecordSearched` path carry the same
  `batch_id`. History gains `?batch=`. This is the audit trail behind the done line's counts, and the
  only reason an unattended auto-link is acceptable.
- **RD7 — Core paces every `/resolve` and `/enrich` call, not only the sweep's.** A per-provider token
  bucket in `internal/enrich/client.go`, default **`2 req/s`, burst `4`**, shared by both endpoints and
  every entity type. `/healthz` and `/describe` are **not** paced (they are Holodex's own probes, never
  upstream traffic — pacing them would slow readiness and `reload-config` for nothing). Precedence for the limit:
  `metadata-sources.yaml` `rate_limit:` → `/describe.rate_limit` → default — the precedence
  `search_pattern` already established. `/describe.rate_limit` is untrusted input and is clamped to
  `requests_per_second ∈ [0.1, 50]`, `burst ∈ [1, 100]`, like every other `/describe` field.
- **RD8 — `429` is back-pressure, not failure.** A `429` from a sidecar pauses **that provider's**
  bucket for `Retry-After` seconds (default **30 s** when the header is absent or unparseable; cap
  **300 s**). Other providers continue. This is how a sidecar that owns its own queue says "not yet"
  without holding a connection past the 8 s ceiling. **Sweep vs. click differ in what happens next:**
  - **Sweep calls wait.** The pair is retried once after the pause; if the retry also `429`s the pair
    counts as `failed` and the pause counts toward RD9's `429` breaker.
  - **Interactive calls never wait on a paused bucket.** A single owner click (Enrich / Refresh /
    Re-match / per-entity Refresh all) that finds its provider's bucket paused fails **fast** with
    `503` + `Retry-After: <remaining seconds>`, which the existing inline status line renders as
    `<provider> is rate-limiting — try again in 42 s`. The per-entity Refresh all is a fan-out, so
    it reports the paused provider as its own result row (`status:"rate_limited"`, `retry_after`)
    with the same line — the other providers' rows still land; the call itself is `200`. Waiting
    on the normal bucket (≤ `burst / rps`,
    i.e. ≤ 2 s at the default) is fine; waiting on a `429` pause (up to 300 s) is not — the SPA
    request would sit or die at a proxy timeout with no feedback.
- **RD9 — Per-provider circuit breaker (two trip conditions).** Within a sweep, one provider trips
  the breaker on **5 consecutive non-`429` failures** (5xx, timeout, transport) **or 3 consecutive
  `429` pauses** (the provider is saturated, not broken — but 212 × 300 s is still 17 h of waiting).
  A successful call resets both counters. Once tripped, the sweep stops calling that provider for its
  remainder and counts each remaining pair as `skipped`; the summary names the provider and the
  reason (`stopped responding` / `rate-limited`). The breaker is per-sweep state, not persisted — the
  next sweep tries again. Single owner clicks are not subject to it.
- **RD10 — The list-page status line reports only its own kind.** `/people` shows a people sweep;
  `/studios` a studios sweep; neither shows the other. Idle renders nothing. A finished sweep's line
  persists until *Dismiss* or leaving the route; a page-local `dismissedBatch` keeps a reload from
  resurrecting it.
- **RD11 — Counts are per pair, entities are per entity.** `done`/`total` count entities;
  `linked`, `needs_review`, `no_candidates`, `failed`, `skipped`, `stale_skipped` count pairs. The
  confirm's *due* number is entities with at least one pair that will be called.

---

## Requirements

### Must-have (P0)

- **P0-1 — Trigger + confirm on `/owner/status` → Actions.** Two bordered buttons after the existing
  ones, *Refresh all people…* / *Refresh all studios…*, rendered only when `library.<kind> > 0`;
  owner + Admin mode only.
  - Given the owner clicks *Refresh all people…*, When the confirm row opens, Then it reads
    `Refresh 212 people (148 due, 64 refreshed in the last 24 h)?` with an unchecked
    `☐ Refresh everything` checkbox, *Yes, refresh*, and *Cancel*.
  - Given the checkbox is ticked, When the confirm re-renders, Then the count reads `Refresh all 212 people?`.
  - Given the owner confirms, When the `POST` returns `202 {started:true}`, Then the toast reads
    `Refresh started.`; on `{started:false}`, `A refresh is already running.`; on a non-2xx, the
    `toMessage(e)` text — and `activity.refresh()` runs in every case.
  - Given a visitor, or the owner with Admin mode off, When `/owner/status` renders, Then neither
    button exists in the DOM.
- **P0-2 — `POST /api/v1/admin/enrich/sweep/{people|studios}` (owner-gated, RD1/RD3/RD4).**
  Body `{ "force": bool }` (default `false`). Returns `202 {status:"accepted", started:bool}`.
  - Given no sweep is running, When called, Then `started:true` and the job begins on a server-lifetime context.
  - Given a sweep of **either** kind is running, When called, Then `202 {started:false}` and nothing starts.
  - Given a person linked to provider A with `fetched_at` 3 h ago and unlinked to provider B, When
    swept without `force`, Then A is `stale_skipped` and B is resolved.
  - Given the same person swept with `force:true`, Then A refreshes and B is resolved.
  - Given an unlinked pair whose `/resolve` returns exactly one candidate at `>= 0.85`, When swept,
    Then `Enrich` applies it, the pair counts as `linked`, and the per-entity `JobRun` carries the
    sweep's `batch_id`.
  - Given two candidates `>= 0.85`, Then nothing applies and the pair counts as `needs_review`.
  - Given a dismissed pair, Then no `/resolve` is issued.
  - Given a provider whose `entity_types` excludes the kind, Then it is never called.
- **P0-3 — Activity read-model `sweep` block (RD5).** `kind` is always the **entity type**
  (`"person"` | `"studio"`, `model.EnrichEntity*`) — never the route plural; the route segment
  `{people|studios}` maps through the existing `"people" → EnrichEntityPerson` table
  (`internal/model/model.go`), and `SweepStatusLine kind="person"` compares against this field.
  `LibraryCounts` keeps its plural JSON keys (`library.people`, `library.studios`) — they are counts,
  not kinds. `GET /admin/activity` returns
  `sweep: { state:"idle"|"running", kind, started_at, batch_id, total, done, linked, needs_review,
  no_candidates, failed, skipped, stale_skipped, last_run: { kind, finished_at, duration_ms, batch_id,
  total, linked, needs_review, no_candidates, failed, skipped, stale_skipped, skipped_providers:[],
  error? } | null }`. `LibraryCounts` gains `studios`.
  - Given a running sweep, When polled, Then `done` is monotonic and `done <= total`.
  - Given the sweep finished, When polled, Then `state:"idle"` and `last_run` describes it until the
    next sweep starts or the process restarts.
  - Given the process restarted mid-sweep, When polled, Then `state:"idle"` and `last_run` is `null`.
- **P0-4 — Summary + per-entity job runs under one `batch_id` (RD6).**
  - Given a finished sweep, When history is read, Then exactly one `JobRun` of kind `enrich-sweep`
    exists for it with `Detail` like `people · 212 · linked 23 · 31 need review · 2 failed · 44 skipped
    (tmdb) · 64 recently refreshed` and its `batch_id`.
  - Given the sweep auto-applied a link, Then that entity's `enrich` run has the same `batch_id`.
  - Given `GET /admin/activity/history?batch=<id>`, Then only runs with that `batch_id` return.
- **P0-5 — Status-page running state.** Given a running people sweep, When Actions renders, Then the
  people button is `disabled` with label `Refreshing people 37 / 212…`, the studios button is
  `disabled` with `title` + `aria-describedby` `One refresh at a time`; Rescan/Reload are unaffected.
  When the poll observes running→idle, the digest reloads (existing `$effect`, extended).
- **P0-6 — List-page status line (RD10).** A shared `SweepStatusLine` on `/people` and `/studios`,
  between the header and `DuplicatesBanner`, owner + Admin mode only.
  - Given a running sweep of this kind, Then `role="status" aria-live="polite"`:
    `Refreshing people in the background — 37 of 212 · linked 4 · 6 need review · 2 failed. You can leave this page.`
  - Given it finished while this route was mounted, Then the page's `reload()` runs **once**, and the
    line reads `Refreshed 212 people in 6 m 40 s — Linked 23 · 31 need review · 2 failed · Skipped 44
    (tmdb stopped responding) · 64 recently refreshed · View in System Activity · Dismiss`, where
    *Linked N*, *N need review*, and *View in System Activity* link to `/owner/status?batch=<id>`.
  - Given *Dismiss*, Then the line disappears and a reload does not bring it back.
  - Given a sweep of the other kind, or idle with no finished sweep, Then nothing renders.
  - Given `last_run.error`, Then `role="alert"` `text-warn`: `Couldn't refresh people: <error>. Try again from System Activity.`
- **P0-7 — Job history batch filter.** Given `/owner/status?batch=<id>`, When `JobHistory` renders,
  Then a filter chip `batch <id> ×` scopes the list to that batch; `×` clears the query param.
- **P0-8 — Provider traffic contract (RD7–RD9).** Applies to **all** sidecar calls, not only sweeps.
  - Given default config and a sidecar with no `rate_limit`, When 10 calls are issued at once, Then at
    most 4 leave immediately and the rest are paced at 2/s.
  - Given `/describe.rate_limit {requests_per_second: 10, burst: 20}`, Then those values apply;
    given `{requests_per_second: 1000}`, Then it is clamped to 50 and a warning is logged.
  - Given `metadata-sources.yaml` sets `rate_limit: {requests_per_second: 1}` for a provider that
    declared 10, Then 1 applies.
  - Given a sidecar answers `429` with `Retry-After: 12`, Then that provider's bucket pauses 12 s,
    other providers continue, and the pair retries once after the pause.
  - Given `429` with no header, Then the pause is 30 s; given `Retry-After: 9999`, Then 300 s.
  - Given 5 consecutive `503`s from provider B during a sweep, Then B is not called again in that
    sweep, its remaining pairs count as `skipped`, `skipped_providers` includes `B`, and provider A's
    pairs keep completing.
  - Given the same provider B on the **next** sweep, Then it is called again (breaker not persisted).
  - Given provider C answers `429` on three consecutive pairs during a sweep, Then C trips the breaker,
    its remaining pairs count as `skipped`, and the summary says `rate-limited`.
  - Given C's bucket is paused (a `429` 40 s ago with `Retry-After: 120`), When the owner clicks
    Refresh on C for one person, Then the request answers `503` with `Retry-After: 80` within the
    normal bucket wait (≤ 2 s), and the inline status line reads `C is rate-limiting — try again in 80 s`.
  - Given `/healthz` and `/describe` calls at startup, Then none of them consume bucket tokens.
- **P0-9 — Contract doc amendment.** `metadata-provider-contract.md` gains §4.13 `/describe.rate_limit`
  (optional, additive, no `protocol_version` bump), a `429` row in §2.0/§2.5, and §4.4 is amended from
  "entirely provider-owned" to the shared posture. Sidecar sync is downstream (the sidecar repos own
  contract-sync).

### Nice-to-have (P1)

- **P1-1 — Attempt marker for unlinked pairs.** Persist `last_attempted_at` per `(entity, provider)` on
  every `/resolve` (sweep or click) so the 24 h skip also covers unlinked no-candidate pairs, and F47
  P2-2's "last checked" annotation becomes possible. Given a person unlinked to B with an attempt 3 h
  ago, When swept without `force`, Then B is `stale_skipped`.
- **P1-2 — Due-count preview endpoint.** `GET /admin/enrich/sweep/{kind}/preview` → `{total, due}` so
  the confirm's `148 due` is exact rather than derived client-side. P0 may compute *due* from the
  list payload's `fetched_at` if it is already there, else show only `total` until P1-2.
- **P1-3 — Reload on `/owner/status` when a sweep finishes** — the digest already reloads (P0-5); also
  reload the history list if it has been opened, as the scan edge does.

### Future considerations (P2)

- **P2-1 — Films.** Third button, third status line on `/films`; `{kind}` already admits it.
- **P2-2 — Undo this sweep.** Whole-batch revert of the links a sweep created (needs an enrichment
  snapshot design comparable to F30's writeback snapshots).
- **P2-3 — Scheduled sweeps** (nightly / on `reload-config`) — only after P2-2 exists, so an
  unattended schedule has an unattended undo.
- **P2-4 — Adaptive rate limits** — learn a provider's real ceiling from `429`s instead of a static bucket.
- **P2-5 — Sidecar-owned batching** — HOLODEX-422.

---

## Backend surface (summary for `/architecture`)

| Change | Where |
|---|---|
| `POST /admin/enrich/sweep/{people\|studios}` → 202, `TryLock` single-flight, `force` body | `internal/api/enrich_review.go` (route beside `enrichRefreshAll`), sweep runner in `internal/enrich` (mirrors `extract.BatchRunner`) |
| Shared per-entity step used by both the handler and the sweep | extract from `enrichRefreshAll` / `refreshOneProvider` into `enrich.Service` |
| `sweep` block on `activityResponse`; `LibraryCounts.Studios` | `internal/api/activity.go`, `internal/repo/jobruns.go` |
| `JobKindEnrichSweep = "enrich-sweep"`; `batch_id` threaded through `RecordSearched` | `internal/model/model.go`, `internal/enrich/service.go` |
| `?batch=` on history | `internal/api/activity.go`, `internal/repo/jobruns.go` |
| Token bucket + `429`/`Retry-After` + breaker | `internal/enrich/client.go`; limit resolution in `internal/enrich/service.go` (`Sources()` already holds per-provider config) |
| `rate_limit:` key | `internal/config` (sources YAML), `metadata-sources.yaml.example` |
| `/describe.rate_limit` sanitizer | `internal/enrich/enrich.go` (beside `SanitizeLinkTemplates`) |

No migration is required for P0 (`job_runs.batch_id` and `entity_enrichment.fetched_at` exist).
P1-1 adds a column or table for `last_attempted_at` — its own numbered migration.

---

## Provider traffic (the contract F47 was waiting on)

| Layer | Mechanism | Default |
|---|---|---|
| Core | Per-provider token bucket on every outbound call | `2 req/s`, burst `4` |
| Sidecar | `/describe.rate_limit {requests_per_second, burst}` — optional, clamped | absent → default |
| Operator | `metadata-sources.yaml` `rate_limit:` | absent → sidecar's declaration |
| Back-pressure | `429` + `Retry-After` pauses that provider's bucket (30 s default, 300 s cap), one retry | — |
| Fault isolation | 5 consecutive non-`429` failures **or** 3 consecutive `429` pauses → provider skipped for the rest of the sweep | — |
| Interactive calls | Never wait on a `429` pause — fail fast with `503` + `Retry-After`, surfaced inline | — |

Why a static bucket and not adaptive: a personal-scale library finishes a 212-entity sweep in about two
minutes per provider at 2 req/s. Getting it right adaptively is P2-4; getting it *safe* is a constant.

---

## Success metrics

Personal-scale product; metrics are checks the owner can read off System Activity, not dashboards.

**Leading (first sweep)**
- A 200-entity, 2-provider sweep **completes** (summary `JobRun` written) with `failed + skipped ≤ 5 %`
  of pairs when both providers are healthy.
- **Zero** provider bans or sustained `429` storms: at most one `429` pause per provider per sweep on
  the reference TMDB sidecar.
- `linked` on a first sweep of a mostly-unlinked library is **> 0** and every linked entity is reachable
  from the done line in **≤ 2 clicks**.

**Lagging (over the following month)**
- **Revert rate on sweep-created links < 5 %** (per-field Reverts on entities whose `enrich` run carries
  a sweep `batch_id`). Above that, the `>= 0.85` threshold or a provider's confidence is miscalibrated
  → revisit F47 P2-3, not this spec.
- F47 review-queue depth **drops** after a sweep and stays down (ambiguous pairs surface once, get
  decided, stay decided via RD4).
- A second sweep within 24 h of the first issues **< 10 %** of the first sweep's provider calls (the
  staleness skip working).

---

## Open questions

Resolved in-session 2026-09-19 (recorded so they are not re-asked): placement **D**; staleness skip
**24 h with a force checkbox**; films **deferred to P2**; `/resolve/batch` **deferred (HOLODEX-422)**.

Remaining — non-blocking, resolve during implementation:

- **[engineering]** Whether the confirm's *due* count ships in P0 from data the list payload already
  carries, or waits for P1-2's preview endpoint and shows only `total` until then.
- ~~**[engineering]** Whether the bucket's clock is wall-time or a monotonic ticker~~ — **resolved by
  ADR-103 D5:** `time.Now()` monotonic, injected (`now`/`sleep`) so tests assert computed delays.
- **[design, on evidence]** Whether *Skipped N (tmdb stopped responding)* should also surface on the
  status page's Recent-failures callout (HOLODEX-416) as one dismissible row per breaker trip. Not in
  P0; add if the first real trip is missed.

---

## Timeline / phasing

Ships as one PR ([#360](https://github.com/whoiskevinrich/holodex/pull/360), Draft until the gates
are green) in this order, each step independently testable:

1. **Contract** — ADR + `metadata-provider-contract.md` §4.13 + `rate_limit:` config + client token
   bucket / `429` / breaker (P0-8, P0-9). Landing this first makes every *existing* click safer.
2. **Job** — shared per-entity step, sweep runner, `enrich-sweep` kind, `batch_id` threading,
   `sweep` block, `LibraryCounts.Studios`, `?batch=` (P0-2, P0-3, P0-4).
3. **UI** — status-page buttons/confirm/running, `SweepStatusLine`, history chip (P0-1, P0-5, P0-6, P0-7).
4. **QA** — three skins, handoff checklist 1–10, mutation-check the `SingleStrongMatch` reuse.

Dependencies: none external. Nothing here blocks or is blocked by another in-flight epic; the
`refreshOneProvider` extraction touches the same file as HOLODEX-418's Re-match change (merged).
