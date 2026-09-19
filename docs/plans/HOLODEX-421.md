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
- [ ] spec `write-spec` — new spec (claim F## via `node scripts/feature-claims.mjs` at scaffold);
  amends F47 `enrichment-review-workflow.md` Non-Goals / P2-1; contract doc
  `metadata-provider-contract.md` gains `/describe.rate_limit`
- [ ] architecture `architecture` — ADR: provider rate-limit contract (token bucket, `/describe`
  declaration, yaml override precedence, 429/Retry-After, circuit breaker) + sweep job shape
  (`enrich-sweep` JobKind, `sweep` block on `/admin/activity`, `TryLock` single-flight)
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
- [ ] three-skin QA (checklist in the handoff, items 1–10)

## Up next — ordered (position = priority)

1. [ ] [—] `/write-spec` — scaffold with `feature-claims.mjs`, fold in the handoff's "Backend
   contract this UI needs" §1–5, amend F47 Non-Goals/P2-1
2. [ ] [—] `/architecture` — rate-limit contract ADR (`node scripts/adr-claims.mjs` for the number)
3. [ ] [—] Backend → frontend → tests, per the gates above
4. [x] [—] `/resolve/batch` sidecar endpoint follow-up filed → HOLODEX-422
5. [ ] [—] Mark PR ready only when every gate is green; CI moves 421 → In Review / Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · design handoff + decisions
- skills: design-handoff
- Created HOLODEX-421, renamed branch, fired In Progress. Explored list headers, per-entity
  Refresh-all (`refreshOneProvider`), activity read-model, extract-all/rescan precedents. Rendered
  five placements + two done-state revisions in-session; Kevin chose **D** and asked for a default
  rate limit with sidecar self-declaration → traffic posture above.
- handoff: design gate closed; open Draft PR; next session starts at `/write-spec`.
