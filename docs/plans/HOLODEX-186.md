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

**Design package:** [spec](../specs/enrichment-review-workflow.md) · [ADR-065](../architecture/ADR-065-enrichment-auto-apply-and-dismissal.md) · [handoff](../design/enrichment-review-workflow-handoff.md) · testing-strategy §pending

## Gates — definition of done

- [x] spec `write-spec` → [enrichment-review-workflow.md](../specs/enrichment-review-workflow.md)
- [x] architecture `architecture` → [ADR-065](../architecture/ADR-065-enrichment-auto-apply-and-dismissal.md) — auto-apply-with-revert posture change + new `enrichment_dismissals` store
- [ ] backend — S1 (`enrichment_dismissals` migration + dismiss/undismiss/refresh/refresh-all endpoints + `/owner/enrich-queue`) not started
- [ ] frontend — [handoff](../design/enrichment-review-workflow-handoff.md) landed (Enrichment tab, `EnrichPicker`/`EnrichProviderChips` additions, Q3 resolved); build (S2–S4) not started
- [ ] testing `testing-strategy` — not started
- [ ] security `security-review` — `profile_url` scheme validation is the one new externally-influenced surface

## Up next — ordered (position = priority)

1. [ ] [backend] S1 data model + endpoints — `enrichment_dismissals` migration, `dismiss`/`undismiss`/`refresh`/`refresh-all` routes, `GET /owner/enrich-queue` — `internal/api`, `internal/db/migrations`
2. [ ] [frontend] S2 Enrichment tab — `owner/enrichment/+page.svelte`, `EnrichQueueRow.svelte`, `ProviderStatusChip.svelte` — `web/src/routes/owner`  ⛔ blocked on #1
3. [ ] [frontend] S3 `EnrichPicker`: auto-apply on single strong match, "None of these match", view-source link — `web/src/lib/components/EnrichPicker.svelte`  ⛔ blocked on #1
4. [ ] [frontend] S4 `EnrichProviderChips`: Refresh primary action + Refresh-all — `web/src/lib/components/EnrichProviderChips.svelte`  ⛔ blocked on #1
5. [ ] [—] S5 provider contract doc amendment (`profile_url`, auto-apply posture, Q4) — `docs/specs/metadata-provider-contract.md`
6. [ ] [testing] S6 QA (3-skin) + `/security-review` (`profile_url` scheme validation)

## Session log — append-only (cap: last 8 sessions; older → archive/)

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
