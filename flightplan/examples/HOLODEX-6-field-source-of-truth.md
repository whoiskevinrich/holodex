---
# MIGRATION EXAMPLE (ADR-064 Action Item 2) — the F36 rollout expressed in the Flightplan schema.
# The authoritative live plan is docs/plans/field-source-of-truth-rollout.md; this is a worked
# example showing how that hand-rolled plan maps onto the four-section schema. Not a live worklog.
key: HOLODEX-6
status: done
depends-on: []
release_note: Pick which source (file, a provider, or a manual value) is the truth for each metadata field, per item.
---

# HOLODEX-6 · Per-field source-of-truth (F36 / ADR-051)

File is the baseline truth; provider enrichment is an additive shadow; a standing per-item, per-field
decision (`file` | `provider:<name>` | `manual`) overrides precedence at resolve time. Built
video-first but **entity-generic**, so People and Studio inherit it the moment it merges.

**Design package:** [spec F36](../../docs/specs/field-source-of-truth.md) · [ADR-051](../../docs/architecture/ADR-051-per-field-source-of-truth-decisions.md) · [ADR-052](../../docs/architecture/ADR-052-baseline-source-contract.md) · [handoff](../../docs/design/field-source-of-truth-handoff.md) · [testing-strategy §9](../../docs/testing-strategy.md)

## Gates — definition of done

- [x] architecture `architecture` → ADR-052 `BaselineSource` contract (video-first, entity-generic)
- [x] backend — migration 0016 `field_source_decisions`, resolver decision short-circuit + file-first default, owner-gated `PUT/DELETE …/decision`, writeback of decided value (one `WriteBatch`/file)
- [x] frontend — `SourceSelect` radiogroup, candidates, per-field + section sync indicators, 3-skin QA
- [x] testing `testing-strategy` → §9 F36 rows (one-`WriteBatch`-per-file, decisions-are-DB-only)
- [x] security `security-review` — owner gate + untrusted `manual_value` → file write (ran on the S1 backend diff, #72)
- [~] multi-provider — until: a second real provider exists (S4, [HOLODEX-9](https://whoiskevinrich.atlassian.net/browse/HOLODEX-9), P1/optional; sequence after the chip redesign that deletes the "providers differ" hint)

## Up next — ordered (position = priority)

<!-- All fast-follows were promoted out to their own issues once F36 merged — the worklog's up-next
     queue is the staging area, `→ KEY` is the promotion. -->

1. [x] [frontend] Chip redesign — unify source control on selectable chips → [HOLODEX-112](https://whoiskevinrich.atlassian.net/browse/HOLODEX-112) *(do first: S5/S6 inherit the pattern)*
2. [x] [backend] People through resolver + curation + decisions → [HOLODEX-10](https://whoiskevinrich.atlassian.net/browse/HOLODEX-10)
3. [x] [backend] Studio entity (table, scan resolve-or-create, FTS, facet) → [HOLODEX-11](https://whoiskevinrich.atlassian.net/browse/HOLODEX-11)
4. [~] [backend] Multi-provider trust order + per-provider Adopt → [HOLODEX-9](https://whoiskevinrich.atlassian.net/browse/HOLODEX-9)  ⛔ deferred (see gate)
5. [ ] [—] Extract `InlineEditInput` (Enter/Escape/blur commit) → [HOLODEX-111](https://whoiskevinrich.atlassian.net/browse/HOLODEX-111) *(chore; fold into the chip redesign)*

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-01 · S3 gate — integration + live QA
- skills: security-review
- handoff: QA §2/§3/§4 passed against backend-films (TMDB Dune); decision round-trips DB-only, all 3 skins clean. F36 ready to merge (#71).

### 2026-06-30 · S2 frontend — SourceSelect
- skills: design-handoff
- handoff: SourceSelect wired to the live decision API (dev mock deleted); frozen types matched S1 exactly. PR #71 ready for review.

### 2026-06-29 · S1 backend — the decision engine
- skills: architecture, testing-strategy
- handoff: migration 0016 + resolver short-circuit + owner-gated decision endpoints landed (#72); one-WriteBatch-per-file asserted; ① entity-generic resolver done as a byproduct — People/Studio unblocked.
