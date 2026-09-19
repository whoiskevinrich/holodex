---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-421
status: in-progress
release_note: Owners can refresh every person or studio against their metadata providers in one background sweep from System Activity — linked providers refresh, unlinked ones auto-link on exactly one strong match, everything else lands in the review queue — with live progress on the People and Studios pages and a per-sweep audit trail.
---

# HOLODEX-421 · Entity refresh sweep — "Refresh all people / studios" from System Activity

Bringing a whole catalog current after a provider change means opening every person/studio page
and clicking *Refresh all*. This adds one background **sweep** per kind that runs the *same*
per-entity step (`refreshOneProvider`: linked → refresh; unlinked → resolve, auto-apply exactly one
`>= 0.85` match, else needs review; dismissed pairs skipped) over every entity, sequentially, and
reports what it did. **No new matching rule.**

Decisions locked 2026-09-19 (in-session, Kevin):
- **Placement D** — trigger on `/owner/status` → Actions beside Rescan; People/Studios list pages
  render only a live status line (running / done) for their own kind. A–C, E mocked and rejected
  (table in the handoff).
- **Traffic posture** — core per-provider token bucket (`2 req/s, burst 4` default) on *all* sidecar
  calls; `/describe.rate_limit` self-declaration (clamped); `metadata-sources.yaml` `rate_limit:`
  override with the yaml → `/describe` → default precedence `search_pattern` already uses; honour
  `429 Retry-After`; per-provider circuit breaker. This is the "provider rate-limit contract" F47
  deferred (Non-Goals / P2-1) → needs an ADR that amends it. `/resolve/batch` deferred → HOLODEX-422.
- **Audit** — sweep gets a `batch_id`; every auto-apply it performs records with the same id;
  history gains `?batch=`; the done line's counts link there.

## Gates — definition of done

- [x] design `design-handoff` — `docs/design/entity-refresh-sweep-handoff.md` +
  `entity-refresh-sweep-mockup.svg` (Option D, seven states, backend contract the UI needs)
- [x] spec `write-spec` — `docs/specs/entity-refresh-sweep.md` (**F66**; RD1–RD11, P0-1…P0-9);
  F47 `enrichment-review-workflow.md` Non-Goal + P2-1 amended to point here; contract doc
  `metadata-provider-contract.md` §2.0/§2.2/§2.5/§4.4 amended + new §4.13 `/describe.rate_limit`
- [x] architecture `architecture` — **ADR-103** `provider-traffic-contract-and-enrich-sweep.md`: pacer on
  `enrich.Service` (D1, `x/time/rate` D2, ADR-080 carriage D3), typed `*ErrProviderPaused` the caller
  decides to wait on (D4), monotonic injected clock (D5), per-sweep breaker in the runner (D6),
  `Service.RefreshPair` shared step (D7), `SweepRunner` + `sweep` block (D8), `batch_id` (D9), TMDB
  `429` pass-through (D10); README row; spec / contract §4.13 / F47 pointers → ADR-103
- [ ] backend — `POST /admin/enrich/sweep/{people|studios}` (202), `sweep` on activity read-model,
  `LibraryCounts.Studios`, `?batch=` on history, rate limiter in `internal/enrich/client.go`
- [ ] frontend — status page buttons + confirm; shared `SweepStatusLine.svelte` on both list
  pages; `JobHistory` batch chip; `activity.busy` includes sweep
- [ ] testing `testing-strategy` — handler 202/single-flight, `SingleStrongMatch` path reuse,
  limiter + breaker unit tests, `SweepStatusLine` running→idle edge fires `onfinished` once,
  row in `docs/testing-strategy.md`
- [ ] security `security-review` — owner-gated mutation; no new outbound hosts (limiter only
  slows existing allowlisted calls); `/describe.rate_limit` clamped like other untrusted fields
- [ ] `code-review high --fix` before each commit
- [ ] three-skin QA (checklist in the handoff, items 1–11)

## Up next — ordered (position = priority)

1. [x] [—] `/write-spec` — F66 shipped; F47 + contract doc amended
2. [x] [—] `/architecture` — ADR-103 (number 102 went to HOLODEX-425 mid-session; claims file reserved 103)
3. [ ] [—] Backend, in ADR-103's action-item order (pacer → `rate_limit` carriage → `429` in `do` →
   `RefreshPair` extraction → `503` mapping → `SweepRunner`/activity/history → TMDB `429`), then
   frontend → tests, per the gates above
4. [x] [—] `/resolve/batch` sidecar endpoint follow-up filed → HOLODEX-422
5. [ ] [—] Mark the **new** PR ready only when every gate is green; CI moves 421 → In Review / Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · design handoff + decisions
- skills: design-handoff, code-review
- Created HOLODEX-421, renamed branch, fired In Progress. Explored list headers, per-entity
  Refresh-all (`refreshOneProvider`), activity read-model, extract-all/rescan precedents. Rendered
  five placements + two done-state revisions in-session; Kevin chose **D** and asked for a default
  rate limit with sidecar self-declaration → traffic posture above.
- handoff: design gate closed; open Draft PR; next session starts at `/write-spec`.

### 2026-09-19 · spec gate (F66)
- skills: write-spec
- `feature-claims.mjs` said F65 was free — it wasn't (HOLODEX-412 claimed it as an in-place
  amendment with no H1); pinned F65 in `.feature-claims`, reserved **F66**, filed the scanner gap
  as HOLODEX-423. Two question cards → Kevin: **24 h staleness skip + "Refresh everything"
  checkbox** (RD2/RD3); **films deferred to P2** (`{kind}` left open). Unlinked pairs have no
  attempt marker (F47 P2-2), so the skip covers linked pairs only in P0 — P1-1 adds the marker.
- Amended F47 (Non-Goal + P2-1 → F66) and the provider contract (§4.13 + 429 semantics; §4.4 from
  "entirely provider-owned" to shared). Handoff + SVG synced (confirm row: due count + checkbox).
- `code-review high --fix` on the docs: 7 findings, all applied — interactive calls must not wait on a
  `429` pause (fail fast), consecutive `429`s trip the breaker too, `sweep.kind` pinned to the entity
  type, RD7 excludes `/healthz`+`/describe`, handoff synced (clamp, breaker N, `busy` change).
- handoff: spec gate closed; next = `/architecture` for the rate-limit contract ADR.

### 2026-09-19 · architecture gate (ADR-103) — after a premature merge
- skills: architecture
- PR #360 was merged to main (6fe1cf9) with only the design + spec gates green; Jira had moved 421 to
  In Review. Continued on a fresh worktree branch `HOLODEX-421-refresh-sweep-adr`, moved 421 back to
  In Progress by hand. Explore sweep of the seams found what the ADR had to answer: no per-provider
  client outlives one call, `429` is an untyped string folded into `no_candidates`, `BatchRunner` has
  no live progress, `RecordSearched` never sets `BatchID`, no `x/time/rate` dep.
- ADR-103 written (D1–D10) + README row; "ADR pending" pointers in F66, contract §4.13 and F47
  resolved; spec's clock open question closed (D5). Shape choices worth knowing before coding: the
  pacer returns the pause as a typed error and *never* sleeps through it — the sweep runner is the
  one that waits/retries; the breaker is runner state, not service state; `batchID` is an explicit
  parameter, not a context value.
- handoff: architecture gate closed; next session starts backend at ADR-103 action item 1 (pacer).
