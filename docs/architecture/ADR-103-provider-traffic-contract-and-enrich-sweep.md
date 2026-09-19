# ADR-103: Provider traffic contract — per-provider pacing in core, `429` as a typed pause, a per-sweep breaker — and the entity refresh sweep as a single-flight job

**Status:** Proposed
**Date:** 2026-09-19
**Deciders:** Project owner

**Extends:** [ADR-033](ADR-033-metadata-source-plugins.md) (the sidecar contract gains its first
traffic clause: pacing is core's, `429` is the sidecar's one back-pressure signal) ·
[ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md) D1 (the sweep reuses the single-strong-match
auto-apply unchanged — **no new matching rule**) · [ADR-080](ADR-080-configurable-provider-search-patterns.md)
D2 (the yaml → `/describe` → default precedence, reused verbatim for `rate_limit`) ·
[ADR-028](ADR-028-activity-surface-and-job-history.md) / [ADR-071](ADR-071-job-run-attribution-and-paginated-history.md)
(`job_runs.batch_id` becomes the sweep's audit key; history gains a batch filter) ·
[ADR-091](ADR-091-fire-and-forget-writeback-status.md) (progress is server truth on the existing
activity poll, never a browser-held promise).
**Issue:** [HOLODEX-421](https://whoiskevinrich.atlassian.net/browse/HOLODEX-421) (spec
[F66](../specs/entity-refresh-sweep.md); lifts F47's Non-Goal / P2-1; contract
[§4.13](../specs/metadata-provider-contract.md#413-rate-limit-declaration-describerate_limit)).

---

## Context

F47 (ADR-066) shipped per-entity enrichment with one explicit non-goal: *no bulk resolution until the
provider rate-limit contract exists as its own initiative*. Every provider call today is the direct
consequence of one owner click, so the fan-out factor is providers-per-entity, never
entities-in-the-catalog, and the contract could stay unwritten. F66 removes that guard: one owner
click now becomes a 212-entity × N-provider unattended sweep — the first bulk provider traffic in
the product. The sweep is only acceptable if the traffic posture is decided first, which is what
this ADR does. Three facts about the current code shape the decision:

1. **There is no place to hang a limiter.** `Service.client(provider)` builds a fresh `httpClient`
   (and a fresh `http.Client`) per call (`internal/enrich/service.go:230`); nothing per-provider is
   long-lived except the three `atomic.Pointer` caches beside it (`fieldHints`,
   `preferredPatterns`, `linkTemplates`). Pacing state cannot live on the client.
2. **A `429` is invisible.** The one HTTP seam, `(*httpClient).do`, maps every non-2xx to
   `fmt.Errorf("provider returned %d", code)` and never reads `Retry-After`. Upstream of that,
   `refreshOneProvider` collapses every error into `"no_candidates"` (`internal/api/enrich_review.go`),
   so an owner cannot tell "the provider found nothing" from "the provider told us to slow down" —
   tolerable for one click, disastrous for a sweep that would march on at full pace.
3. **The single-flight precedents expose no progress.** `extract.BatchRunner.TriggerAll` and
   `scanner.TriggerRescan` are both `sync.Mutex.TryLock` + goroutine + a `JobRun` on a detached 5 s
   context; only the scanner has a live `Status()`. `RecordSearched` writes one `enrich` run per
   provider × entity with `BatchID` unset, so nothing ties a sweep's runs together.

The design handoff and spec have already fixed the user-visible numbers (`2 req/s` burst `4`,
`[0.1, 50]`/`[1, 100]` clamps, 30 s / 300 s `Retry-After` defaults, breaker at 5 / 3, 24 h
staleness window). This ADR decides **where each mechanism lives and what shape it has** so the
backend can be built without re-deriving them; it does not reopen the numbers.

## Decision

**D1 — Pacing is a core concern, keyed by provider name, living on `enrich.Service`.** A
`map[string]*pacer` beside the existing per-provider caches, created on first use and **never
re-created** — each `Acquire` re-resolves the limit (D3) and adjusts the live bucket
(`SetLimit`/`SetBurst`) only when it changed, so in-flight tokens and an active pause survive a
`reload-config` or a changed `/describe` declaration. The pacer wraps
whatever `newClient` returns, so the test fakes injected through `NewServiceWithClient` are paced
too (with an injected clock, D5). `/resolve` and `/enrich` share one bucket per provider across all
entity types; `/describe` and `/healthz` are not paced (they are Holodex's own probes — pacing them
would slow readiness and `reload-config` for nothing, and `/describe` is already piggybacked on every
call by `verifiedClient`).

**D2 — The bucket is `golang.org/x/time/rate`; the pause and the retry policy are ours.** The token
bucket itself is the Go team's `rate.Limiter` (a ~200-line, dependency-free package from the `x/`
family already in `go.mod` via `x/image`/`x/text`), driven through `ReserveN(now, 1)` +
`DelayFrom(now)` so the clock is injectable. What `x/time/rate` does not model — a provider-declared
pause — is a `pausedUntil time.Time` on the pacer. `Acquire(ctx)`:

1. if `now < pausedUntil` → return `*ErrProviderPaused{Provider, RetryAfter: pausedUntil − now}`
   **immediately** — the pacer never sleeps through a pause (that is the caller's decision, D4);
2. else reserve one token and sleep its delay, context-aware (≤ `burst / rps`, 2 s at the default).

A `429` response sets `pausedUntil = now + retryAfter` (header parsed as delta-seconds only — the HTTP-date
form counts as unparseable; absent or unparseable → 30 s; capped at 300 s) and returns the same `*ErrProviderPaused`. The typed error is the
contract between the client and both callers; nothing else in the error surface changes (`"provider
returned %d"` stays for every other code, unlogged base URLs stay unlogged).

**D3 — `rate_limit` rides ADR-080's exact carriage.** `Source.RateLimit *RateLimit
yaml:"rate_limit"` (`{requests_per_second, burst}`) beside `search_pattern`; `Manifest.RateLimit
*RateLimit json:"rate_limit"` beside `preferred_search_pattern`, persisted by a `persistRateLimit`
twin of `persistPreferredPattern` (clamp to `[0.1, 50]`/`[1, 100]` and log; malformed → warn and
drop the whole object). Resolution is yaml → described → default `{2, 4}`, computed at
`Acquire` time from the caches so a change is live on the next call without a restart. No
`default_rate_limit` fleet key: the default is a safety constant, not an operator preference (an
operator who wants a different fleet pace sets it per source, which is the only place a limit is
meaningful anyway).

**D4 — Sweep calls wait on a pause; interactive calls fail fast. The policy lives in the caller, not
the client.** Both paths call the same `Service.Resolve`/`Enrich` and receive the same
`*ErrProviderPaused`. The **sweep runner** sleeps `RetryAfter` (context-aware) and retries the pair
**once**; a second pause counts the pair as `failed` and increments that provider's `429` streak.
The **interactive handlers** (`enrich`, `resolve`, `enrichRefreshAll`, re-match) map the error to
`503` + `Retry-After: <ceil(remaining)>` and the existing inline status line renders *"<provider> is
rate-limiting — try again in N s"*. Chosen over a context-value flag or a `wait bool` parameter on
`Resolve`/`Enrich`: the client stays uniform and never blocks longer than one bucket delay, and the
one place that may legitimately wait minutes is the one place that already owns a long-lived
context and a breaker.

**D5 — Clock: `time.Now()` monotonic, injected.** The pacer takes `now func() time.Time` and `sleep
func(ctx, time.Duration) error` (defaults `time.Now` and a timer). Go's `time.Now()` carries a
monotonic reading, so `pausedUntil` arithmetic and `x/time/rate`'s reservations are immune to wall
clock jumps; tests advance a fake `now` and assert computed delays instead of sleeping. This answers
the spec's open question (wall-time vs. ticker): monotonic, by construction of `time.Time`.

**D6 — The circuit breaker is per-sweep state in the runner, never in the client.** Two counters per
provider, reset by any success: `consecutiveFailures` (non-`429` error: 5xx, timeout, transport) trips
at **5**; `consecutivePauses` (a pair that was still paused after its one retry) trips at **3**. Once
tripped, the runner stops calling that provider for the sweep's remainder, counts each remaining
pair as `skipped`, and records the reason (`stopped responding` / `rate-limited`) on the summary.
Single owner clicks are never subject to it — there is no breaker outside a sweep — and the next
sweep starts clean. Chosen over a service-level breaker: a persisted or process-wide breaker would
make an owner's *click* fail because a *sweep* had a bad afternoon, with no UI to reset it.

**D7 — The per-entity step moves into `enrich.Service`; the handler and the sweep call the same
function.** `refreshOneProvider`'s routing (linked → `Enrich`; unlinked → dismissed? skip :
`Resolve` → `SingleStrongMatch` → `Enrich` + `RecordSearched`, else `needs_review`/`no_candidates`)
is extracted to `Service.RefreshPair(ctx, kind, id, provider, RefreshOpts{Force, BatchID}) PairOutcome`.
`PairOutcome` carries the status the handler renders today **plus the typed error**, so the sweep can
classify (`ErrProviderPaused` vs. other) where the handler keeps rendering `no_candidates`. The
**staleness skip** is a rule of this step, not of the sweep: a linked pair whose
`entity_enrichment.fetched_at` is younger than 24 h returns `stale_skipped` unless `Force`; the
interactive path always passes `Force: true` (a click means *now*). Unlinked pairs have no attempt
marker (F47 P2-2) and are always re-resolved in P0. ADR-066 D1's auto-apply is called, not copied.

**D8 — One `enrich.SweepRunner`, one `TryLock` across kinds, live status on the activity poll.**
Mirrors `extract.BatchRunner` (`TryLock` + goroutine + `defer Unlock`, base context set at startup)
with two additions it lacks: a `Status()` read-model (the scanner's `ScanStatus` pattern) and
per-kind dispatch. `Trigger(kind, force) (started bool)`; a second `POST` of *either* kind while
running answers `202 {started:false}` — informational, never an error. Entities run sequentially;
within one entity the existing per-provider fan-out runs concurrently (bounded by provider count, as
one click is today). `GET /admin/activity` gains the `sweep` block exactly as spec
P0-3 shapes it — `state: idle|running`, `kind` = the **entity type** (`person`/`studio`, never the
route plural), `started_at`, `batch_id`, `total`/`done` plus the six per-pair counts, and a
`last_run` object (same counts + `finished_at`, `duration_ms`, `skipped_providers[]`) that outlives
the run until the next sweep or a restart — and `busy` includes a running sweep. Read from memory
while the process lives; after a restart `last_run` is `null` and the sweep is reconstructible from
history via its batch (D9), which is why the block is not persisted.

**D9 — `batch_id` is the audit key: a new `enrich-sweep` summary run plus the same id on every
per-entity run.** `model.JobKindEnrichSweep = "enrich-sweep"`. The summary `JobRun` maps
`Seen = total entities`, `Updated = linked`, `Skipped = skipped + stale_skipped`, `Errors = failed`,
`Detail` = the human done-line (counts + breaker reasons). `RecordSearched` and `recordEnrichJob`
gain a `batchID` parameter (explicit, threaded through `RefreshOpts` — not a context value) so every
`enrich` run the sweep produces carries the sweep's id; interactive calls pass `""` as today.
`ListJobRuns` gains a `batch` filter (`WHERE batch_id = ?`, exposed as `?batch=` on history) — no
new index: `job_runs` is 30-day-pruned and the filter is an owner-only audit read. No migration:
`batch_id` (0028) and `fetched_at` already exist.

**D10 — Per-provider `429` becomes a sidecar obligation, not a courtesy.** Contract §4.13 already
says it; this ADR makes the in-repo TMDB sidecar comply: an upstream TMDB `429` is passed through as
`429` + `Retry-After` (today `providers/tmdb/tmdb.go:1206` folds it into a generic error). A sidecar
that answers `502`/`503` instead is not broken — it just gets counted as a failure and hits the
5-streak breaker rather than the pause — but it forfeits the graceful path the contract offers.

## Consequences

- **Every existing click gets safer first.** D1–D5 apply to all sidecar traffic regardless of the
  sweep, which is why the spec phases the limiter ahead of the sweep. A concurrent
  `enrichRefreshAll` fan-out is now paced per provider (one token each); an owner clicking during a
  sweep contends for the same bucket and waits at most one delay or fails fast on a pause.
- **`x/time/rate` is a new direct dependency** — first-party Go, no transitive deps. The alternative
  (a hand-rolled bucket) is ~40 lines whose reservation math is exactly the part that goes wrong.
- **A `429` surfaces as a distinct owner-visible state** for the first time (`503` +
  `Retry-After` inline; `Skipped N (<provider> rate-limited)` on the done line). `no_candidates`
  stops silently absorbing rate limits.
- **`refreshOneProvider` leaves the handler.** The API layer loses its one copy of the enrichment
  routing; `internal/api/enrich_review.go` becomes a thin renderer over `Service.RefreshPair`.
- **A running sweep survives navigation and reload but not a restart.** The summary run is written
  only at the end (like extraction); a sweep killed mid-way leaves per-entity runs with the batch id
  and no summary — history shows exactly what was done, and the next sweep's staleness skip makes
  re-running it cheap. Acceptable at personal-library scale; a resumable sweep is not designed.
- **The sweep never sees the 8 s ceiling relaxed.** A pause is a *pacer* state; the HTTP timeout is
  unchanged. A sidecar that wants Holodex to wait says `429`, never holds the connection.
- **Films are not wired** (`{kind}` accepts `people|studios` in P0); adding `films` is a dispatch
  entry, not a design change. `/resolve/batch` (HOLODEX-422) would sit *behind* the same pacer.
- **Revisit** if a provider ever needs adaptive pacing (spec P2-4) — the pacer's `SetLimit` is the
  hook — or if the breaker's per-sweep scope proves too forgiving (a persisted cool-down would need
  its own reset affordance, which is why it was not chosen now).

## Alternatives considered

| Alternative | Why not |
|---|---|
| **Pace in the sidecar only** (contract stays silent; each provider queues) | Puts the one bulk caller's safety in N third-party codebases; a non-compliant sidecar turns a sweep into a hammer. Core is the only place that sees *all* outbound traffic |
| **Limiter on `httpClient`** | Nothing per-provider outlives one call today (Context 1); would force a client cache that exists only to hold a bucket |
| **Hand-rolled token bucket** | Cheaper on the dependency list, dearer on correctness; `x/time/rate` is first-party and its `ReserveN(now, …)` API gives the injectable clock D5 needs for free |
| **Context value / `wait bool` to distinguish sweep from click** (D4) | Behaviour keyed off a context value is invisible at the call site; a parameter touches every `Resolve`/`Enrich` call. Returning the pause as a typed error keeps one client and moves the policy to the two callers that differ |
| **Service-level or persisted breaker** (D6) | A sweep's bad afternoon would fail the owner's next click with no UI to reset it; per-sweep state needs neither |
| **Retry-`429` inside `do`** (sleep and retry transparently) | Would block an interactive request for up to 300 s behind a proxy timeout with no feedback; the spec's RD8 exists precisely to forbid that |
| **Structured JSON `Detail` on the summary run** | The counts already live on the `sweep` block while it matters and in the per-entity runs (`?batch=`) forever; a second schema in `job_runs.detail` would drift from both |
| **SSE / dedicated progress endpoint** | ADR-091's rule — the 3 s activity poll is already on every owner page; one `sweep` block reaches both the status page and the list-page status line |
| **Persist `sweep` state for restart** | A resumable sweep needs a checkpoint table and a resume policy for a job that a personal library re-runs in ~2 min per provider; history + staleness skip cover the gap |

## Action items

1. [ ] `go get golang.org/x/time/rate`; `internal/enrich/pacer.go` — `pacer` (bucket + `pausedUntil`,
   injected `now`/`sleep`), `*ErrProviderPaused`, `Acquire`; unit tests with a fake clock (D2, D5)
2. [ ] `Source.RateLimit` / `Manifest.RateLimit` + `persistRateLimit` clamp/log; `metadata-sources.yaml.example`
   `rate_limit:` comment; `Acquire` re-resolves yaml → described → default and calls
   `SetLimit`/`SetBurst` on change (D1, D3)
3. [ ] `(*httpClient).do`: `429` → parse `Retry-After` → pause + typed error; pacer applied in
   `Service.client` for `/resolve` + `/enrich` only (D1, D2)
4. [ ] Extract `Service.RefreshPair` + `PairOutcome` from `refreshOneProvider`; staleness skip with
   `Force`; thread `batchID` through `RecordSearched`/`recordEnrichJob` (D7, D9)
5. [ ] Interactive handlers map `*ErrProviderPaused` → `503` + `Retry-After`; SPA inline line (D4)
6. [ ] `enrich.SweepRunner` (`TryLock`, `Status()`, breaker, retry-once); `POST /admin/enrich/sweep/{kind}`
   202; `sweep` block + `busy` on `/admin/activity`; `LibraryCounts.Studios`; `JobKindEnrichSweep`;
   `?batch=` on history (D6, D8, D9)
7. [ ] TMDB sidecar: pass upstream `429` through with `Retry-After` (D10)
8. [ ] `docs/architecture/README.md` row; contract §4.13 and F66 "ADR pending" → ADR-103; F47 Non-Goal
   note points here; `docs/testing-strategy.md` row (limiter/breaker/single-flight/`onfinished`-once)
