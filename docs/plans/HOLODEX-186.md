---
key: HOLODEX-186
status: in-progress
depends-on: []
release_note: Enrichment now queues every un-enriched person, studio, and media entry in one place, applies obvious matches automatically, remembers a rejected candidate so it never re-prompts, and lets you refresh an already-linked source with one click.
---

# HOLODEX-186 · F47 — Enrichment review workflow (queue, confidence routing, unmatched flag, refresh)

A generalized, entity-agnostic enrichment review workflow across Person/Studio/Media: a review
queue for un-enriched entries, confidence-based auto-apply for unambiguous single-strong-match
candidates, a durable "not matched" verdict, an optional provider view-source link, and a refresh
bypass (single-provider and all-providers). **Done** = P0–P1 slices (S1–S6) shipped: queue live in
the Owner hub, `EnrichPicker`/`EnrichProviderChips` extended, provider contract amended, QA +
security clean.

**Design package:** [spec](../specs/enrichment-review-workflow.md) · [ADR-065](../architecture/ADR-065-enrichment-auto-apply-and-dismissal.md) · [handoff](../design/enrichment-review-workflow-handoff.md) · [testing-strategy](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → [enrichment-review-workflow.md](../specs/enrichment-review-workflow.md)
- [x] architecture `architecture` → [ADR-065](../architecture/ADR-065-enrichment-auto-apply-and-dismissal.md) — auto-apply-with-revert posture change + new `enrichment_dismissals` store
- [x] backend — S1 (`enrichment_dismissals` migration + dismiss/undismiss/refresh/refresh-all endpoints + `/owner/enrich-queue`) landed
- [ ] frontend — [handoff](../design/enrichment-review-workflow-handoff.md) landed (Enrichment tab, `EnrichPicker`/`EnrichProviderChips` additions, Q3 resolved); build (S2–S4) not started
- [x] testing `testing-strategy` → [docs/testing-strategy.md](../testing-strategy.md) §4/§5/Phase 3 — written ahead of S1–S4, now exercised by S1's test suite
- [ ] security `security-review` — `profile_url` scheme validation is the one new externally-influenced surface

## Up next — ordered (position = priority)

1. [x] [backend] S1 data model + endpoints — `enrichment_dismissals` migration, `dismiss`/`undismiss`/`refresh`/`refresh-all` routes, `GET /owner/enrich-queue` — `internal/api`, `internal/db/migrations`
2. [ ] [frontend] S2 Enrichment tab — `owner/enrichment/+page.svelte`, `EnrichQueueRow.svelte`, `ProviderStatusChip.svelte` — `web/src/routes/owner`
3. [ ] [frontend] S3 `EnrichPicker`: auto-apply on single strong match, "None of these match", view-source link — `web/src/lib/components/EnrichPicker.svelte`
4. [ ] [frontend] S4 `EnrichProviderChips`: Refresh primary action + Refresh-all — `web/src/lib/components/EnrichProviderChips.svelte`
5. [ ] [—] S5 provider contract doc amendment (`profile_url`, auto-apply posture, Q4) — `docs/specs/metadata-provider-contract.md`
6. [ ] [testing] S6 QA (3-skin) + `/security-review` (`profile_url` scheme validation)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-12 · S1 backend — data model + endpoints
- skills: simplify
- backend: `internal/db/migrations/0024_enrichment_dismissals.{up,down}.sql` — the `enrichment_dismissals`
  table (D2) with real `AFTER DELETE` cascade triggers on `people`/`studios`/`videos` (stronger than
  `entity_enrichment`'s manual per-merge cleanup, since this table is new). `internal/repo/enrichment_dismissals.go`
  (Dismiss/Undismiss/EnrichmentDismissed) and `internal/repo/enrich_queue.go` (`EnrichQueue` — the
  zero-cost, batched, no-N+1 membership query: an entity qualifies only when it has ≥1 outstanding
  provider that is NOT dismissed; People → Studios → Media order per Q3). `internal/api/enrich_review.go`
  — `GET /owner/enrich-queue` and the entity-generic dismiss/undismiss/refresh/refresh-all handler
  factories (mirroring `enrich.go`'s resolve/apply/clear route shape, mounted in `mountEnrich`); wired a
  dismissal check into all three existing `/enrich/resolve` handlers (409 while dismissed, per RD4's
  block-until-"Try again" invariant). `internal/enrich`: `StrongMatchThreshold`/`SingleStrongMatch` (D1's
  reusable threshold primitive, mirroring `EnrichPicker.matchLabel`'s 0.85 cutoff) — used by
  refresh-all's server-side per-provider routing (refresh linked / auto-apply a single strong match /
  surface `needs_review` / skip a dismissed unlinked provider). Full test coverage: repo (membership/
  states/ordering/cascade), enrich (threshold boundary table), api (queue listing zero-provider-calls,
  dismiss blocks resolve + undismiss clears it, refresh 400-when-unlinked + zero-resolve-calls,
  refresh-all auto-apply + dismissed-skip) — all green, no regressions (`go test ./...`).
- note: D1's *client-triggered* auto-apply (opening a queue row auto-applies via the existing
  `/resolve`+`/enrich` calls, no new endpoint) is S3 frontend work, not this slice — only refresh-all
  needed the threshold check server-side (it's one atomic backend call fanning out over providers).
- next: S2 Enrichment tab (frontend) — `/owner/enrich-queue` is live and tested, unblocking S2–S4.

### 2026-07-12 · `/design-handoff` → Enrichment tab, picker + chip additions
- skills: design-handoff
- handoff: Wrote [enrichment-review-workflow-handoff.md](../design/enrichment-review-workflow-handoff.md)
  against the real `EnrichPicker`/`EnrichProviderChips`/`DuplicatePairRow`/`owner/+layout` source and
  the actual token values (no new tokens/fonts/radii introduced). Resolved spec Q3: queue groups by
  entity type in nav order (People → Studios → Media, not Duplicates' frequency-driven tags-first),
  actionable rows sort first within a group. Cross-referenced the handoff into the spec (pending-pointer
  + Timeline checkbox + Q3) and flipped Jira HOLODEX-186's Design handoff gate + cleared `needs-design`.
  Next: S1 backend (data model + endpoints) unblocks S2–S4 frontend build; Person's Refresh/Refresh-all
  scope still depends on HOLODEX-125 (ADR-055 gap) per the handoff's open question 3.
