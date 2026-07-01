# Spec: Per-field source-of-truth decisions (F36)

**Status**: Draft
**Phase**: Post–Phase 3 (enrichment follow-up; standalone owner-facing feature)
**Owner**: Project owner
**Date**: 2026-06-29
**Feature block**: **F36** — Per-field source-of-truth. The product-level fix for the F31
refresh-masking bug: a standing, per-item, per-field decision that names which source is *true*
for a field, defaulting to the file baseline, driving both display and writeback.

**Depends on**: Only shipped surfaces —
- the unified field resolver with `file:`/`{provider}:` provenance ([F27](metadata-plugins.md), `internal/resolver`)
- value-level curation + the durable write queue ([F30](metadata-curation.md) / [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md), `metadata_curation`)
- per-item refresh / forced re-extract + re-enrich ([F31](metadata-refresh.md) / [ADR-047](../architecture/ADR-047-per-item-metadata-refresh.md))
- the enrichment shadow store + per-provider matches ([F22](metadata-plugins.md) / [ADR-033](../architecture/ADR-033-metadata-source-plugins.md), `entity_enrichment`)
- configurable field mapping / precedence ([ADR-013](../architecture/ADR-013-metadata-field-mapping.md), `internal/mapping`)
- metadata writeback ([F28](metadata-curation.md) / [ADR-041](../architecture/ADR-041-metadata-writeback.md))
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)

**New ADR**: **Already written** — [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)
(supersedes the deferred F31.11 slice of [ADR-047](../architecture/ADR-047-per-item-metadata-refresh.md)).
Touches **access + persisted owner input that feeds file writeback** → a `/security-review`
sign-off is required before merge.

> **Pickup status (2026-06-29):** design **complete** — ADR-051, this spec (RD1–RD5),
> [handoff](../design/field-source-of-truth-handoff.md) + [QA checklist](../design/field-source-of-truth-qa-checklist.md),
> and [testing-strategy §9](../testing-strategy.md) F36 block are all done. **Implementation not started.**
> Next: the `[P2·XL]` ADR-051 parent task in the main `TASKS.md` ("Entity-generic source-of-truth"
> section). Recommended first step — a short `/architecture` addendum pinning the `BaselineSource`
> contract so the resolver decision short-circuit (here) and the entity-agnostic-resolver fast-follow
> ① don't fight over `resolver.Resolve`'s signature. `/security-review` is the remaining gate and runs
> against the implementation diff, not these docs.

---

## Problem Statement

Holodex merges three metadata layers per canonical field — **file**, **provider** enrichment,
and **manual** curation — by global mapping precedence, taking the first non-empty source in
mapping order ([F27](metadata-plugins.md)). The shipped example mappings list providers before
`file:`, so a provider silently outranks the owner's own file tags. The reported bug followed
directly: editing an MKV externally and running per-item Refresh ([F31](metadata-refresh.md))
**correctly re-reads and persists the file layer**, but the displayed value never changes
because a higher-precedence source masks it — and it surfaced on an instance using a **custom,
non-film provider**, where "provider wins by default" is simply wrong. Worse, writeback embeds
the *winning* value into the file, so adopting a provider value and writing it back overwrites
the owner's file tags. There is no per-item, per-field way to say "for this field, the **file**
is the truth" or "adopt **this** provider" — the owner cannot decide what is true, only watch a
global rule decide for them.

## Goals

1. **Let the owner decide the source of truth per field, per item** — a standing choice of
   `keep file` / `adopt <provider>` / `custom`, defaulting to the file baseline.
2. **Make the file the default authority** — an undecided field shows the file value; a provider
   is a *candidate*, never an automatic winner. This is the direct fix for the F31 bug.
3. **One decision drives display and writeback** — what the owner sees is what gets written; no
   divergence between the resolved view and the write payload.
4. **Pin the source, not the value** — a `keep file` or `adopt provider` decision follows the
   live layer, so a later Refresh file-edit or re-enrich flows straight through; only `custom` is
   frozen.
5. **Surface conflict and sync state** — when matched providers disagree, say so; when a decided
   value differs from what's embedded in the file, show it, per field and in aggregate.
6. **Keep it legible and safe** — owner-gated, themed across all three skins, reusing the
   existing curation components and provenance vocabulary.

## Non-Goals

- **Bulk / library-wide decisions** — applying a decision across many items, and conflict-triage
  queues, are the F31.11 batch op. Single-item only here. *(Why: batch needs the `plan`/`apply`
  fan-out and a confidence model that don't exist yet.)*
- **People / Studio entity pages** — this spec is **video fields only**. Person/Studio decisions
  ride the entity-agnostic-resolver and Studio-promotion fast-follows. *(Why: the resolver is
  video-shaped today; generalizing it is separate work.)* The model must not foreclose them.
  *(Update 2026-07-01: the People slice is now specced — [F37](people-source-of-truth.md).)*
- **F23 alias/merge offer on person fields** — adopting a provider's *person name* is also an
  identity question; that flow is deferred to the People refactor. We note the seam, build no copy.
- **Synonym / fuzzy dedup** (`Sci-Fi` ≡ `Science Fiction`) — out of scope, as in [F30](metadata-curation.md).
- **Reinventing merge-field curation** — merge fields keep the F30 per-value chips unchanged; the
  new source control is **replace-field only**.

## Resolved Decisions

*(Locked with the owner 2026-06-29; the leaning options in [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) were all confirmed.)*

- **RD1 — Replace-only source control.** The segmented `Keep file / Adopt <provider> / Custom`
  selector appears **only on scalar (replace) fields**. Merge fields keep the existing F30 chips
  (union of sources, per-value include/exclude, manual-add = "custom"); no field-level custom is
  added to merge fields. One system per field shape, no overlap.
- **RD2 — Sync surfaced per-field *and* in aggregate.** An out-of-sync field shows a per-field
  chip, **and** the Metadata section shows an "N fields out of sync" summary by the Write button.
- **RD3 — Person-field F23 offer deferred.** This spec covers video fields; adopting a provider
  value for a person field does **not** offer alias/merge here. The seam is noted; the flow lands
  with the People refactor.
- **RD4 — File-first global default + escape hatch.** Ship `default_source: file` (file beats
  providers when undecided), with `default_source: mapping` as a documented opt-out for a film
  instance that wants provider-first. Single-owner project; this fixes the bug as the default.
- **RD5 — Writes are atomic and batched per file (NON-NEGOTIABLE).** Setting or clearing a
  decision is a **DB-only** operation with **zero file I/O**. File tags are written **only** by the
  explicit "Write decisions to file" action, which collects **all** of an item's decided +
  out-of-sync fields and performs **one atomic `WriteBatch` per file** through the existing durable
  write queue — copy→write→rename ([ADR-041](../architecture/ADR-041-metadata-writeback.md)), one
  job per file ([ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)). **No
  per-field, per-toggle, or per-decision file writes.** Writing tags is I/O-intensive; collapsing
  to a single invocation per file is required, not optional.

## User Stories

**Owner — fix a masked file value (the bug)**
- As the owner, when I edit an MKV's tag externally and Refresh, I want the field to show my file
  value by default, so a provider never silently hides my own edit.
- As the owner, I want to see the provider's value as a *candidate* next to my file value, so I can
  choose to adopt it deliberately rather than have it imposed.

**Owner — decide the truth per field**
- As the owner, I want to pick `keep file` / `adopt <provider>` / `custom` per field, so each
  field reflects the source I trust for *this* item.
- As the owner, I want my choice to persist and keep following that source (a later file edit or
  re-enrich updates the field), so I decide once, not every time.
- As the owner, I want to type a custom value (e.g. fix a misspelled performer), so I can correct
  a field neither the file nor a provider gets right.

**Owner — multiple providers**
- As the owner with IMDB and TMDB both matched, I want one `Adopt` option per provider and a hint
  when they disagree, so I can resolve the conflict explicitly.

**Owner — write to file with confidence**
- As the owner, I want "Write decisions to file" to write exactly the values I decided, and to
  show me which fields are out of sync with the file, so writeback is predictable.

## Requirements

### Must-have (P0)

- **P0-1 — Standing per-field decision.** Persist `(video, canonical_field) → {file | provider:<name> | manual(+value)}`. Absence of a row = `file` (baseline). Owner-gated set/clear.
  - Given an undecided replace field with both a file and a provider value, When the page renders, Then the **file** value is shown and the provider appears as a candidate (RD4).
  - Given the owner selects `Adopt <provider>`, When the page reloads, Then the field shows that provider's value with provider provenance and the choice persists.
  - Given the owner selects `Custom` and enters a value, Then the field shows that literal with `manual` provenance.
  - Given the owner clears the decision, Then the field returns to the file baseline.
- **P0-2 — Source-pin, not value-pin.** A `file`/`provider` decision reflects the *current* layer.
  - Given a field decided `keep file`, When a later Refresh re-extracts a changed file value, Then the displayed value updates with no re-decision.
  - Given a field decided `adopt tmdb`, When a re-enrich changes TMDB's value, Then the field updates.
  - Given a field decided `custom`, When the file or provider changes, Then the custom value is unchanged.
- **P0-3 — Decision drives display.** The resolver consults the decision **before** mapping order, pre-loaded with the curation/enrichment maps (pure, no new I/O). A decided field ignores mapping precedence.
- **P0-4 — Decision drives writeback, atomically and batched per file (RD5, NON-NEGOTIABLE).** "Write decisions to file" writes the **decided** value per replace field and the curated set per merge field, as a **single atomic `WriteBatch` per file** via the durable queue (copy→write→rename, [ADR-041](../architecture/ADR-041-metadata-writeback.md) / one-job-per-file, [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)). The write payload equals the displayed truth.
  - Given several fields are decided/edited, When the owner writes, Then exactly **one** queued job runs **one** `WriteBatch` invocation for that file — never one write per field.
  - Given the owner sets or clears a decision, Then **no file is touched** (DB only); the file changes only on the explicit write action.
  - Given a write fails part-way, Then the original file is byte-for-byte intact (temp discarded) and the job is replayable on restart.
- **P0-5 — File-first default + escape hatch (RD4).** Undecided replace fields resolve file-first; `default_source: file|mapping` config (default `file`); shipped `metadata-mappings.yaml.example` reordered file-first.
- **P0-6 — Replace-only control (RD1).** Segmented source selector renders only on scalar fields; merge fields render the existing F30 chips unchanged.
- **P0-7 — Owner-gated API.** `PUT /media/{id}/fields/{canonical}/decision {source, manual_value?}` and `DELETE …/decision`, behind `requireOwner`; untrusted `manual_value` sanitized like F30 manual add.
- **P0-8 — Themed + accessible.** Tokens only; QA all three skins; the segmented control is keyboard-operable with clear selected state; candidates and provenance are screen-reader labeled.

### Should-have (P1)

- **P1-1 — Multi-provider control.** One `Adopt` option per **matched** provider (`Keep file / IMDB / TMDB / Custom`).
  - Given two providers supply different values for a replace field, Then a per-field "sources disagree" hint shows and each provider is selectable.
- **P1-2 — Inter-provider trust order.** A global config orders providers for the *undecided* winner among providers (file still ahead of all). Per-field decision overrides it.
- **P1-3 — Sync indicator (RD2).** Per-field out-of-sync chip when the decided value ≠ the value embedded in the file, plus an "N fields out of sync" summary by the Write button. After a successful write, sync clears without flipping the decision.
- **P1-4 — Candidate visibility.** Under a replace field, show the available candidates (file value, each provider value) so the choice is informed, not blind.

### Future considerations (P2)

- **P2-1 — Entity generalization.** People/Studio decisions reuse the same primitive once the resolver is entity-agnostic and Studio is an entity (fast-follows; design the table/UI so `entity_type` is the only difference). *People: specced as [F37](people-source-of-truth.md) (HOLODEX-10).*
- **P2-2 — Person-field F23 offer.** Adopting a provider person value offers alias/merge (deferred, RD3).
- **P2-3 — Bulk decisions / conflict triage.** Library-wide "prefer file / prefer provider X" via the F31.11 batch seam + `sources_disagree`.
- **P2-4 — Commit-and-detach affordance.** An explicit "bake provider value into the file and revert to file" shortcut (today: adopt → write → clear).

## Behavior detail

### Resolution (replace field)
1. If a decision row exists → return the decided source's **current** value (`file` baseline / `provider:<name>` shadow value / `manual` literal). Stop.
2. Else if `default_source: file` (default) → file value if present; else the inter-provider-trust-ordered first provider; else empty.
3. Else (`default_source: mapping`) → today's first-non-empty in mapping order.
Merge fields are unchanged: F30 union + per-value curation; decisions do not apply.

### Writeback
- **Decisions are DB-only; the file is touched only by the explicit write action (RD5).** Toggling a source, typing a custom value, clearing a decision → no file I/O.
- The write action collects **all** of the item's decided + out-of-sync fields and issues **one** durable job → **one atomic `WriteBatch` per file** (copy→write→rename; all tags in a single tool invocation). Never per-field. This rides the existing `internal/writeback.WriteBatch` + the F30 queue unchanged — F36 adds *which values* to write, not a new write mechanism.
- The write payload per replace field = the decided value (P0-4). Merge fields = the curated write-enabled set (F30).
- After write, recompute per-field sync (decided value vs. the tag now in the file). The decision is **not** mutated (aligned with ADR-051 §5).

### Sync state
- A field is *out of sync* when its decided value differs from the value currently embedded in the file's tag (read via the existing extract path / last writeback audit). Surfaced per-field + aggregate (RD2).

## API

```
PUT    /api/v1/media/{id}/fields/{canonical}/decision   { source, manual_value? }   (requireOwner)
DELETE /api/v1/media/{id}/fields/{canonical}/decision                                (requireOwner → file default)

200/204 on success · 400 bad source/canonical · 401/403 owner gate · 404 unknown id/field · 409 soft-deleted
```
`source ∈ { "file", "provider:<name>", "manual" }`; `manual_value` required (and sanitized) iff `source="manual"`; a `provider:<name>` must be a currently-matched provider. The media detail payload gains a per-field `decision` marker (chosen source + standing flag) and per-field `in_sync`.

## UI (grounded in real components)

- The **Metadata** section (`web/src/routes/media/[id]/+page.svelte`, `dl`/`dt`/`dd`) gains, on each **replace** field, a segmented source selector: `Keep file · Adopt <provider>…(one per matched provider) · Custom`, with the selected segment in the accent treatment. Candidate values render beneath, muted (P1-4).
- **Chips/provenance** reuse `CurationChip` (`·source` suffix; accent for provider, muted for file/manual) and `ProvenanceBadge`. Merge fields keep `CurationFieldRow` unchanged.
- **Custom** opens the existing inline input (reuses the F30 add/edit field pattern).
- **Write to file** → **"Write decisions to file"**, with an out-of-sync count beside it (RD2) and a per-field out-of-sync chip (a quiet `text-warn` pill — distinct from accent).
- Visible only to `effectiveOwner`. Tokens only; QA Cinémathèque / Broadcast / Brutalist (badge/counter collisions and accent-on-accent are the usual skin regressions).

## Success Metrics

This is a single-owner correctness/control feature, not a funnel. Success =
- **Leading:** the bug is gone — an external file edit + Refresh shows the file value by default in every skin; adopting a provider then editing the file behaves per the decision (manual QA + tests).
- **Leading:** a decided field's writeback writes the decided value (audited via `file_writebacks`).
- **Lagging:** the owner stops needing the `metadata-mappings.yaml` source-order workaround; provider drift no longer silently overrides curated fields.

## Open Questions

- **Q1 (engineering, non-blocking):** Sync read source — derive "value in file" from a fresh extract on demand, or from the `file_writebacks` audit + file mtime? (Audit is cheaper; an external edit since the last write would make it stale — Refresh reconciles. Decide in implementation.)
- **Q2 (engineering, non-blocking):** Should `DELETE decision` on a field that was `custom` also clear any F30 manual-add row for that field, or are they independent stores? (Lean: independent; deleting a decision reverts source selection only.)
- **Q3 (design) — ✅ resolved in [handoff](../design/field-source-of-truth-handoff.md).** Only the out-of-sync pill is `text-warn` (value row); "providers differ" is a **muted** informational hint on the candidates line (different rows, different weights) — so the two never read as one alarm.

## Timeline / routing

No hard deadline. Remaining artifacts before/with implementation, per the project change-routing rules:
1. **`/design-handoff`** — ✅ done: [handoff](../design/field-source-of-truth-handoff.md) + [QA checklist](../design/field-source-of-truth-qa-checklist.md) (the new `SourceSelect` control, sync indicators, 3-skin QA).
2. **`/testing-strategy`** — ✅ done: F36 block added to [testing-strategy.md](../testing-strategy.md) §9 (decision short-circuit + merge-untouched regression, file-first default + escape hatch, source-pin, **one-`WriteBatch`-per-file** assertion, DB-only decisions, sync recompute, multi-provider, API auth, `SourceSelect` a11y/3-skin) — mapped to the QA §2 smoke items.
3. **`/security-review`** — owner gate + untrusted `manual_value` feeding file writeback.

Implementation lands video-first ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) parent task, migration 0016); People/Studio inherit via the tracked fast-follows.
