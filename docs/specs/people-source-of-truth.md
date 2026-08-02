# Spec: People on the unified source-of-truth model (F37)

**Status**: Draft
**Phase**: F36 fast-follow ② (Jira [HOLODEX-10](https://whoiskevinrich.atlassian.net/browse/HOLODEX-10), epic HOLODEX-4)
**Owner**: Project owner
**Date**: 2026-07-01
**Feature block**: **F37** — wire the person detail page through the unified resolver +
value-level curation + per-field source decisions, making `person` the second entity (after
`video`) on the F36 decision model — and resolving the name-as-identity seam ADR-051 §6/§9
deliberately deferred to this refactor.

**Depends on** (all shipped):
- the F36 decision model ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md), migration 0016 `field_source_decisions` — already keyed by `entity_type`)
- the entity-agnostic resolver ([ADR-052](../architecture/ADR-052-baseline-source-contract.md), `BaselineSource` + `ResolveFields`, `internal/resolver`)
- the source-chip control ([F36 handoff](../design/field-source-of-truth-handoff.md), HOLODEX-112 `SourceSelect`/`CurationChip` radio mode, `web/src/lib/f36.ts`)
- person enrichment ([F22](metadata-plugins.md), `entity_enrichment` for `entity_type='person'`; TMDB supplies bio/birthdate/deathdate/nationality/website/photo)
- person aliases + merge ([F23](person-aliases.md) / [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md) — scan-time name routing)
- value-level curation ([F30](metadata-curation.md) / [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md), `metadata_curation` — already keyed by `entity_type`)
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)

**New ADR**: **None needed.** ADR-051 §9 (entity generalization, baseline-per-entity) and
ADR-052 (the `BaselineSource` contract) already decide the architecture; this spec applies them.
Touches **access** (new owner-gated mutation endpoints, including an identity rename) →
a `/security-review` sign-off is required before merge.

**Unblocks**: [F32 video credits](video-credits-people.md) (queued behind this refactor) and
the Studio promotion ([HOLODEX-11](https://whoiskevinrich.atlassian.net/browse/HOLODEX-11)),
which inherits this exact shape once `studio` is an entity.

---

## Problem Statement

After F36, the media page and the person page speak different languages. A video field is a
row of source-tagged chips — baseline anchored first, provider values as candidates, a standing
per-field decision, manual custom values, value-level curation on merge fields. A person field
is a raw enrichment dump: `GET /people/{id}` returns provider fields verbatim
(`internal/api/handlers.go` `getPerson` — no `resolved[]`), and the page renders them with a
provider badge and no owner control at all — no way to adopt, correct, suppress, or blank a
provider value, and no custom values. The decision/curation/enrichment stores are all already
entity-typed; persons simply aren't wired through them. Meanwhile the F36 promise ("the model
must not foreclose People/Studio") stays unproven until a second entity actually rides it.

## Goals

1. **One editable-field vocabulary across entities** — the person page renders the same
   source-chip radiogroup and curation chips as the media page; an owner who learned F36 on a
   video already knows how to curate a person.
2. **Per-field decisions for person fields** — adopt a provider's bio/birthdate/…, type a
   custom value, or pin a field blank; standing, source-pinned (a re-enrich flows through),
   DB-only.
3. **Resolve the name-as-identity seam (ADR-051 §6/§9)** — adopting a provider or custom
   *name* updates the person's real identity (rename + alias) instead of forking display from
   search and scan routing.
4. **Prove the model is entity-generic** — `person` flows through `ResolveFields` via a
   `personBaseline` with zero changes to the resolver core, the decision store, or the chip
   components.
5. **Close the merge gap** — a person merge cleans up the merged-away person's decision and
   curation rows (today it would orphan them).

## Non-Goals

- **Studio promotion** — `studio` is not an entity yet; it inherits this model in
  [HOLODEX-11](https://whoiskevinrich.atlassian.net/browse/HOLODEX-11), in parallel. *(Why:
  separate table/scan/page work; nothing here forecloses it.)*
- **Multi-provider match/enrich UI** — the page keeps the single person-capable-provider
  assumption (`web/src/routes/people/[id]/+page.svelte` ~line 67); widening it is
  [HOLODEX-119](https://whoiskevinrich.atlassian.net/browse/HOLODEX-119), deliberately deferred
  until a second real provider exists. The chips themselves are already per-provider.
- **Writeback / sync state for persons** — a person has no file. There is no "Write decisions
  to file" button, no out-of-sync pill, and `in_sync` is **omitted** (not `false`) from person
  resolved fields. *(Why: the concept has no referent; inventing a person writeback would be
  circular — the person record is derived from files, not a source of them.)*
- **Promoting provider aliases to routing aliases** — a provider `aliases` value stays
  display-side even when kept (RD2); a per-chip "promote to routing alias" affordance is a
  tracked P2. *(Why: display curation silently changing scan behavior is a footgun; promotion
  should be its own explicit act.)*
- **MCP parity** — `get_person` keeps its current shape; exposing `resolved[]` over MCP rides
  the existing deferred MCP-parity item (F22.5f).
- **Split / un-merge** — unchanged from F23.

## Resolved Decisions

*(Locked with the owner 2026-07-01 via brainstorm question cards.)*

- **RD1 — Name materializes; it never pins.** The name field renders source chips like any
  replace field, but selecting a non-record source does **not** write a decision row — it opens
  a **confirm dialog** and, on confirm, **renames the person and keeps the old name as an F23
  alias** (one transaction), so search, scan routing, and display remain one identity. There is
  no standing decision for `name`; after the rename the record chip simply carries the new
  name. *Identity fields materialize; metadata fields pin.* This realizes the ADR-051 §6 "may
  offer, never automatically" seam.
- **RD2 — Provider `aliases` = merge field with F30 curation, display-only.** The provider
  `aliases` enrichment field becomes a normal merge field (✕-chips: suppress / manual add),
  visually and semantically **separate** from the F23 routing-alias section, which is
  unchanged. Kept chips do not route at scan time.
- **RD3 — Empty baseline renders the `—` record chip.** Matching the video empty-file-baseline
  edge case: the record chip stays anchored first and selectable even when the record has no
  value, so "keep this field blank" is an explicit, standing decision (`source=record`) that
  suppresses a provider value without curation tricks.
- **RD4 — The person baseline provenance label is `·record`.** `·file` is factually wrong for
  persons. The baseline label becomes per-entity (video `·file`, person `·record`); chips,
  aria-labels, and the decision `source` value follow it (see API note below).
- **RD5 — Merge drops the duplicate's decision + curation rows.** Same rule as enrichment
  (F23.9 already drops the merged-away person's `entity_enrichment` rows): the canonical
  person's own decisions/curation win; nothing is migrated. Closes the orphaned-rows gap in
  `internal/api/aliases.go` `mergePersons`.
- **RD6 — Record-first default is additive for persons.** Under the F36 file-first default
  (RD4 there), an undecided field resolves to the baseline *when the baseline has a value* —
  and a person's baseline has only `name`. So every enrichment-only field (bio, birthdate, …)
  keeps resolving to the provider value until the owner decides otherwise: **no displayed
  value changes for undecided fields**. This is a design property, not an accident; tests
  assert it. The shared `sourceChips`/`selectedChipKey` implementation applies the same
  empty-baseline-wins rule to any entity (a video's `poster_url` with no embedded cover art
  hits it too), so `SourceSelect`'s radio chip reads selected for an undecided field whenever
  the baseline is empty. **Addendum (HOLODEX-245):** that chip now carries a distinct
  "pending" treatment (dashed ring, hollow dot, `·pending` suffix) instead of the filled
  decided styling, so an owner can tell an implicit winner from a standing decision at a
  glance. This changes only the chip's *selection-indicator styling* — the resolved value
  RD6 guarantees is unchanged; `in_sync`/`needsWriteback` and the writeback dialog's
  decided/undecided split are also unaffected (HOLODEX-213 stands as-is).
- **RD7 — Endpoint parity with video.** Person decision/curation endpoints mirror the media
  shapes exactly (paths below), behind the same `requireOwner` gate.

## User Stories

**Owner — curate a person like a video**
- As the owner, I want the person page's fields to be the same source chips I use on videos,
  so I don't learn a second editing model.
- As the owner, I want to fix a provider's wrong nationality with a custom value, so the page
  shows what I know to be true.
- As the owner, I want to pin a person's bio blank even though the provider has one, so a
  re-enrich never resurrects text I don't want.

**Owner — correct a name without breaking identity**
- As the owner, when I adopt TMDB's spelling of a person's name, I want the person actually
  renamed with the old spelling kept as an alias, so searching either name still finds them and
  the next scan doesn't resurrect a duplicate.
- As the owner, when the corrected name already belongs to another person, I want to be shown
  that person (with video counts) and offered the existing merge flow instead, so a rename
  never silently collides or auto-merges.

**Owner — curate provider aliases**
- As the owner, I want to drop a junk provider alias and add my own, so the aliases shown are
  the ones I consider real — without accidentally changing how scans route names.

## Requirements

### Must-have (P0)

- **P0-1 — `personBaseline`.** A `BaselineSource` implementation over the person record:
  `name` is the only baseline-backed field; every other canonical person field resolves an
  empty baseline. Person fields flow through `ResolveFields` with **no changes to the resolver
  core**.
  - Given a person with enrichment, When resolved with no decisions, Then every field's value
    equals today's raw-enrichment display (RD6 — additive).
- **P0-2 — Resolved payload.** `GET /people/{id}` gains `resolved[]` (same
  `ResolvedField` shape as media detail: value(s), provenance, `decision` marker, candidates;
  **no `in_sync`**), pre-loading decisions + curation like the media handler. The raw
  `enriched[]` block is retired in the same release (the SPA is its only consumer).
- **P0-3 — Person decision endpoints (RD7).**
  `PUT/DELETE /api/v1/people/{id}/fields/{canonical}/decision`, `requireOwner`, replace fields
  only, provider must be currently matched, `manual_value` sanitized as in F30/F36; 404 unknown
  person/field, 400 bad source. **`name` is rejected** (400) — it has no decision row (RD1).
- **P0-4 — Person curation endpoints (RD7).** `POST /api/v1/people/{id}/curation` and
  `…/curation/clear` mirroring the media handlers, so the `aliases` merge field gets F30
  add/suppress/nowrite semantics (RD2; `nowrite` is accepted-but-moot — no writeback).
- **P0-5 — Name rename flow (RD1).** A new owner-gated `POST /api/v1/people/{id}/rename
  {name}`: transactionally set `people.name`, insert the previous name as an F23 alias, refresh
  FTS. Selecting a provider/custom name chip in the UI opens the confirm dialog and calls this.
  - Given the new name equals another person's name (`people.name` UNIQUE), Then 409 with that
    person's id/name/video count, and the UI offers the existing F23 merge flow instead —
    never an auto-merge.
  - Given the rename succeeds, Then searching the old name still returns the person (alias
    routing) and a re-scan of a file crediting the old name links to them.
- **P0-6 — Merge cleanup (RD5).** `mergePersons` deletes the merged-away person's
  `field_source_decisions` and `metadata_curation` rows in the same transaction that drops its
  enrichment.
- **P0-7 — The page on chips.** The person detail Details section renders replace fields as
  the `SourceSelect` chip radiogroup (record chip anchored first, `·record`, `—` when empty per
  RD3) and `aliases` as a `CurationFieldRow` merge field — reusing the existing components with
  an entity-generic baseline label (RD4). No write button, no sync pills. Owner-gated; visitors
  see read-only resolved values. Tokens only; QA all three skins.

### Should-have (P1)

- **P1-1 — Long-text fit.** `bio` (display `long_text`) needs a treatment beyond the
  `max-w-[14rem]` chip clamp — chips carry a truncated preview and the selected value renders
  in full beneath the row. Settle exact form in the design-handoff addendum.
- **P1-2 — Rename affordance parity.** The rename confirm is also reachable from a plain
  "Rename" affordance next to the name (not only via adopting a candidate chip), replacing any
  ad-hoc rename path with the same alias-keeping transaction.

### Future considerations (P2)

- **P2-1 — Promote provider alias → routing alias.** A per-chip explicit action that inserts a
  kept provider alias into `person_aliases`.
- **P2-2 — Studio inherits** ([HOLODEX-11](https://whoiskevinrich.atlassian.net/browse/HOLODEX-11), specced as [F38](studio-entity.md)) — same chips, `·record` baseline, no writeback.
- **P2-3 — Multi-provider person UI** ([HOLODEX-119](https://whoiskevinrich.atlassian.net/browse/HOLODEX-119)).
- **P2-4 — MCP `resolved[]` parity** (rides deferred F22.5f).

## Behavior detail

### Resolution (person replace field)
Identical to F36 §Behavior with the baseline swapped: decision short-circuit first
(`record` → live record value; `provider:x` → shadow value; `manual` → literal); undecided →
record value if present, else first provider (RD6). Merge fields (`aliases`) are the F30 union +
curation, unchanged.

### The decision `source` vocabulary
The stored source stays `file | provider:<name> | manual` in `field_source_decisions` (the
column is entity-generic; renaming the enum is churn for no behavior). The **API and UI**
present it as the entity's baseline label — `record` for persons — mapped at the handler edge:
`PUT …/decision {source:"record"}` stores `file`; `resolved[].decision` echoes `record`.
*(Engineering may instead choose to store `record` literally and widen `fieldsource.Valid()`;
either way the payload vocabulary is per-entity and tests pin it.)*

### Name materialization (RD1)
Selecting a non-record name chip → confirm dialog ("Rename to X? 'Y' is kept as an alias —
scans and search still match") → `POST /people/{id}/rename` → refetch. On 409 (name taken),
surface the existing merge confirmation with both video counts (F23 "never auto-merge",
homonym-safe). The provider name chip remains visible after rename only if the provider value
now differs from the record.

### Merge (RD5)
`POST /people/{canonical}/merge {from_id}` additionally deletes `from_id`'s
`field_source_decisions` + `metadata_curation` rows (entity_type `person`) in the existing
transaction. The canonical person's rows are untouched.

## API

```
GET    /api/v1/people/{id}                                  + resolved[] (no in_sync); enriched[] retired
PUT    /api/v1/people/{id}/fields/{canonical}/decision      { source, manual_value? }   (requireOwner; name → 400)
DELETE /api/v1/people/{id}/fields/{canonical}/decision                                  (requireOwner → record default)
POST   /api/v1/people/{id}/curation                         { field, value, action }    (requireOwner)
POST   /api/v1/people/{id}/curation/clear                   { field, value, action }    (requireOwner)
POST   /api/v1/people/{id}/rename                           { name }                    (requireOwner; 409 name-taken → merge offer)
```

`source ∈ { "record", "provider:<name>", "manual" }`. Errors mirror the media decision
endpoints (400/401/403/404/409).

## UI (grounded in real components)

- **Details section** (`web/src/routes/people/[id]/+page.svelte`): replace fields (bio, born,
  died, nationality, website) render the `SourceSelect` chip radiogroup — `[● — ·record]`
  `[○ value ·tmdb]` `[＋ Custom]` — exactly per the [F36 handoff](../design/field-source-of-truth-handoff.md)
  (chip shell, fold/dedup, a11y radiogroup semantics), with the baseline label `·record` (RD4)
  and no out-of-sync pill or write button (Non-Goals).
- **Name row**: chips + the RD1 confirm dialog; the record chip always carries the live
  `people.name`.
- **Aliases row**: `CurationFieldRow` merge chips (RD2), placed so it cannot be confused with
  the F23 routing-alias management section (which is unchanged — the handoff addendum settles
  placement/labeling).
- **Website** keeps its `url` display treatment inside the chip value.
- Owner-gated via `activity.effectiveOwner`; refetch-after-mutate as on media. Tokens only;
  QA Cinémathèque / Broadcast / Brutalist.

## Success Metrics

Single-owner consistency feature:
- **Leading:** the person page exposes adopt/custom/blank per field and they survive re-enrich
  per the pin semantics (tests + manual QA); a rename keeps old-name search and scan routing
  working end-to-end (the F23 invariant, now reachable from a chip).
- **Leading:** zero resolver-core diffs in the implementing PR (entity-genericity proven).
- **Lagging:** F32 and HOLODEX-11 build on `personBaseline`/these endpoints without reopening
  the model.

## Open Questions

- **Q1 (engineering, non-blocking):** store `record` literally in `field_source_decisions.source`
  vs. map at the handler edge (see Behavior detail — either is acceptable; pick during
  implementation and pin with tests).
- **Q2 (design, non-blocking):** exact long-text (`bio`) treatment (P1-1) — settle in the
  design-handoff addendum.

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:
1. **`/design-handoff`** — an addendum to the [F36 handoff](../design/field-source-of-truth-handoff.md):
   person page layout, `·record` label, the rename confirm dialog, aliases-row placement vs.
   the F23 section, bio long-text treatment, 3-skin QA checklist items.
2. **`/testing-strategy`** — extend §9 (or a new §) with: `personBaseline` resolution + RD6
   additivity, decision short-circuit for person fields, name-decision rejection, rename
   transaction (+ 409 collision → no auto-merge), merge cleanup (RD5), endpoint auth, chips
   a11y/3-skin.
3. **`/security-review`** — new owner-gated surface: decisions/curation parity + the **rename**
   (identity mutation feeding FTS + scan routing) and untrusted `manual_value` (display-only
   here — no file writes).

Implementation mirrors the F36 rollout: S1 backend (`personBaseline`, payload, endpoints,
merge cleanup) ∥ S2 frontend (page on chips, against the frozen payload) → S3 QA + security.
Effort: L (per HOLODEX-10).
