# Spec: Enrichment review workflow — queue, confidence routing, unmatched flag, refresh

**Status**: Draft
**Phase**: Phase 3 (Enrichment / curation foundation)
**Owner**: Project owner
**Date**: 2026-07-12
**Feature block**: **F47** — a generalized, entity-agnostic **enrichment review workflow**: a
dedicated review queue for un-enriched People/Studios/Media, confidence-based auto-apply for
unambiguous matches, a durable **"not matched"** verdict so rejected candidates never re-prompt,
an optional provider-supplied **view-source link** for sparse candidates, and a **refresh
bypass** (single-provider and all-providers) so re-enriching an already-linked entity skips the
picker entirely.

**Depends on** (all shipped, except where noted):
- F22 Metadata source plugins ([metadata-plugins.md](metadata-plugins.md),
  [ADR-033](../architecture/ADR-033-metadata-source-plugins.md)) — the `entity_enrichment` shadow
  store, the per-entity `/enrich/resolve` · `/enrich` · `/enrich/{provider}` (DELETE) routes this
  spec extends, and the [provider HTTP contract](metadata-provider-contract.md) this spec amends.
- F36 Field source-of-truth ([field-source-of-truth.md](field-source-of-truth.md),
  ADR-051) — the per-field decision/provenance model that auto-apply's Keep/Revert rides
  unchanged; no new undo plumbing is introduced here.
- F37/F38 People/Studio source-of-truth (ADR-052) — the entity-generic resolver that already
  makes Person, Studio, and Video ride one `BaselineSource` model; this spec assumes enrichment
  is already wired symmetrically across all three (`/people`, `/studios`, `/media` all expose
  `enrich/resolve` · `enrich` · `enrich/{provider}` today).
- HOLODEX-136 `EnrichProviderChips` — the existing per-provider chip component (primary action +
  ⋯ overflow) this spec extends with a Refresh state and a Refresh-all action.
- F43 Entity identity / Duplicates queue ([entity-identity.md](entity-identity.md),
  ADR-061) — the direct UX precedent this spec's review queue mirrors (dense grouped rows,
  per-row verdicts, `/owner` hub tab, local-state removal on resolve without refetch).
- F35 Owner tooling hub ([owner-tooling-hub.md](owner-tooling-hub.md)) — hosts the new queue as
  a tab, alongside Duplicates.
- **ADR-055** Enrichment unique-key invariant (Proposed) — this spec's "linked ⇒ has a stored
  `external_id`" assumption (RD7/RD8, Refresh/Refresh-all) depends on every enrichable entity
  type conforming to id-based identity. **Studio and Video already conform; Person does not yet**
  (HOLODEX-125 tracks the fix). See Open Questions Q1.

**ADR**: [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) (Proposed) —
records the threshold-gated auto-apply routing change (amending the metadata provider contract's
stated v1 posture, "Holodex always shows the owner a picker and never auto-applies a candidate in
v1") and the new per-`(entity, provider)` `enrichment_dismissals` store, mirroring how ADR-061
preceded F43.

**Design handoff**: [enrichment-review-workflow-handoff.md](../design/enrichment-review-workflow-handoff.md)
— the Enrichment tab (grouped queue rows, resolving Q3's grouping/ordering question below), the
`EnrichPicker` "None of these match" + view-source-link additions, the provider chip's flipped
Refresh/Re-match/Clear states, and Refresh-all's partial-result surfacing.

---

## Problem Statement

Working through un-enriched entries today means opening each Person one at a time, clicking
Enrich, waiting for the picker's auto-search, and confirming — even when the result is an
unambiguous single match that required no judgment at all. There is no queue: the owner has to
remember (or re-browse) which entries still need attention, and the mechanism only exists for
People even though Studios and Media have the identical shadow-store/provider machinery
(F22/F26/F38). Two states are silently lost today: a candidate list the owner reviewed and
correctly rejected has nowhere to record that verdict, so the same wrong candidates resurface on
every future visit; and an already-linked entity's re-enrichment re-runs the full search+picker
flow even though the provider identity (`external_id`) is already known. The cost is a workflow
that scales with owner attention rather than with genuine ambiguity — the tedious part is the
volume of unambiguous confirmations, not the occasional real judgment call.

## Goals

1. **One click resolves the common case.** Opening a queue row for an entity with a single
   strong-confidence match applies it immediately — no separate confirm step — while remaining
   as easy to revert as any other field override (F36).
2. **One place to work the backlog, for all three entity types.** A review queue lists every
   Person/Studio/Media entry missing provider data, without the owner needing to revisit each
   entity page to discover it.
3. **A rejected match is never re-asked.** Telling the tool "none of these match" is durable —
   the same candidates do not resurface until the owner explicitly asks to try again.
4. **Re-enrichment costs one click, not a search.** An already-linked provider refreshes with a
   single action using the stored `external_id`; refreshing every provider on an entity is a
   second single action, not N repeated searches.
5. **No new bulk provider traffic.** None of the above triggers provider calls the owner did not
   directly ask for — the queue's list/count is a zero-cost DB signal, and Refresh-all is bounded
   to one entity's configured providers (small, fixed N), not the catalog.

## Non-Goals

- **Queue-wide bulk/background resolution** (auto-resolving every visible row on queue load).
  Explicitly deferred until the provider rate-limit contract exists as its own initiative — this
  spec's lazy, per-row-click model is chosen specifically to avoid needing it yet.
- **Cross-provider confidence calibration.** `confidence` stays provider-native and advisory
  (per the existing contract); this spec thresholds the same frontend value already computed by
  `EnrichPicker.matchLabel` (`>=0.85` strong) — it does not attempt to normalize confidence
  across different providers' scoring.
- **A new undo/audit log for enrichment.** Auto-apply reuses F36's existing per-field
  decision/provenance Keep/Revert; there is no separate enrichment-specific undo history.
- **An in-app candidate photo comparison.** The sparse-candidate problem is solved by an
  external, provider-supplied view-source link opened in a new tab — not an in-app image fetch
  or diff (candidates carry no image today; adding one was considered and rejected in favor of
  the link, which requires no new fetch/rate-limit surface).
- **Automatic re-surfacing of a "not matched" verdict.** Dismissal is durable, cleared only by an
  explicit "Try again" — no TTL/expiry, to avoid reintroducing background polling.
- **Person `external_id` identity conformance.** Assumed as a prerequisite (ADR-055,
  HOLODEX-125), not solved by this spec — see Open Questions Q1.

## Resolved Decisions

*(Captured from the product-brainstorming session, 2026-07-12.)*

- **RD1 — Auto-apply threshold reuses the existing frontend "strong match" cutoff.**
  `EnrichPicker.matchLabel`'s `>=0.85` threshold (already shipped, F22) is the sole auto-apply
  trigger: a `/resolve` call returning **exactly one** strong-confidence candidate applies it
  immediately via the existing `apply()` call; two-or-more strong, any possible-only, or
  weak-only results always require the owner's pick.
- **RD2 — Queue list/count is a zero-cost DB signal.** Membership = "this entity is missing an
  `entity_enrichment` row for at least one provider whose `entity_types` includes its type." No
  `/resolve` call happens until a row is opened; the badge count is the same query, count-only.
- **RD3 — Per-row resolve fires on click, not on queue load.** Clicking a row triggers
  `/enrich/resolve` for that row's outstanding provider(s); the outcome routes per RD1 (auto-apply
  vs. needs review) and updates the row in place, matching the Duplicates queue's
  resolve-without-refetch pattern (F43).
- **RD4 — "Not matched" is a durable, owner-set verdict, scoped per `(entity, provider)`.** A new
  **"None of these match"** action (in `EnrichPicker`, usable from both the queue and entity
  detail pages) records a dismissal. It excludes the row from "needs review" state and queue
  counts but stays reachable via a **Try again** action that clears the dismissal and re-resolves.
  No expiry — matches the on-demand model (RD2/Non-Goals).
- **RD5 — "No data yet" needs no persisted state.** It is the absence of both an
  `entity_enrichment` row and a dismissal — distinguished from "not matched" purely by whether an
  owner verdict exists, not by any stored timestamp (see Open Questions Q2 for the deferred
  "last checked" annotation).
- **RD6 — Candidates may carry an optional, provider-supplied `profile_url`.** A view-source
  link opened in a new tab, letting the owner verify a match against the provider's own (richer)
  page instead of the picker's three-field summary. Optional per provider — the contract states
  this explicitly, and both the queue and `EnrichPicker` degrade to label-only when absent.
  Scheme-validated server-side (`http`/`https` only) before it ever reaches the client, per the
  untrusted-provider-input posture already established for the rest of the contract (§6 of the
  provider contract spec).
- **RD7 — A linked provider's primary chip action flips to Refresh.** Once a provider chip is
  linked (an `external_id` is stored), `EnrichProviderChips`'s primary button changes from
  "Enrich" (open picker) to **"Refresh"** (call `apply()` directly with the stored id — no
  `/resolve`, no picker). **"Re-match…"** (reopen the picker to pick a different candidate) and
  **"Clear"** move into the ⋯ overflow alongside each other.
- **RD8 — Refresh-all is per-entity, bounded, and reuses RD1's routing.** A single **"Refresh
  all"** action fans out across only that entity's configured providers (today: 2–4, operator
  bounded, not catalog-sized). Each provider independently: refreshes if linked, or resolves +
  auto-applies if a single strong match is found, or surfaces inline for review if ambiguous —
  the action never silently drops an ambiguous provider. Explicitly distinct from queue-wide bulk
  (Non-Goals) — the fan-out factor is providers-per-entity, not entities-in-the-catalog.
- **RD9 — Queue rows show one status chip per provider, never a single collapsed flag.** Row-level
  action ("Review" / "Try again" / none) is derived from whichever provider chips still need
  attention; a row with one auto-applied and one needing-review provider still shows "Review."

## User Stories

**Owner — work through the backlog fast**
- As the owner, I want a queue of entities missing provider data, so I don't have to remember or
  re-browse which People/Studios/Media I haven't gotten to.
- As the owner, I want a single obvious match to apply itself when I open a row, so I'm not
  confirming the same clearly-right answer over and over.
- As the owner, I want to tell the tool "none of these are right" and have it remember, so I
  don't re-evaluate the same wrong candidates on my next pass.
- As the owner, I want a sparse or unfamiliar candidate to link out to the provider's own page,
  so I'm not guessing from three lines of text.

**Owner — keep already-matched entries fresh**
- As the owner, I want to refresh an already-linked provider's data with one click, so I don't
  have to re-search and re-pick someone I've already matched.
- As the owner, I want to refresh every provider on an entity at once, so I don't have to visit
  each chip separately when I know I want everything current.
- As the owner, I want to explicitly re-match an entity to a different candidate if the linked one
  turns out wrong, without losing the one-click refresh for everything else.

**Owner — trust the safety net**
- As the owner, I want an auto-applied match to be as easy to revert as any field override I'd
  make manually, so speed doesn't cost me safety.
- As the owner, I want each provider's status shown separately per entity, so a good match from
  one provider doesn't hide that a second provider still needs my attention.

## Requirements

### Must-have (P0) — queue, auto-apply, not-matched, refresh bypass

- **P0-1 — Generalized review queue (RD2/RD3/RD9).** `GET /owner/enrich-queue` lists entities
  (person/studio/media) missing enrichment from ≥1 supporting provider, grouped by entity type,
  each row carrying one status per provider. `/owner` hub gains an **Enrichment** tab alongside
  Duplicates.
  - Given a Person has no `entity_enrichment` row for `tmdb` (which supports `person`), When the
    queue loads, Then that Person appears with a `tmdb` chip in "not yet reviewed" state, with
    **zero** provider calls made.
- **P0-2 — Auto-apply on single strong match (RD1).** Opening a queue row (or its detail-page
  `EnrichPicker`) resolves the outstanding provider(s); a lone strong-confidence candidate applies
  immediately through the existing `apply()` path.
  - Given a queue row's only outstanding provider returns exactly one candidate with
    `confidence >= 0.85`, When the owner opens that row, Then the field values apply without a
    second confirmation click, and the row shows "auto-applied."
  - Given two candidates both score `>= 0.85`, When resolved, Then neither auto-applies — the row
    shows "needs review" and opens the picker as today.
- **P0-3 — Auto-applied results are revertable via the existing decision model (F36).** No new
  undo mechanism; Keep/Revert on the affected fields is unchanged.
- **P0-4 — "Not matched" dismissal (RD4/RD5).** `EnrichPicker` gains a **"None of these match"**
  action; committing it records a dismissal for that `(entity, provider)`. The queue and detail
  pages show it as "not matched" with a **Try again** action that clears the dismissal.
  - Given the owner dismisses all candidates for `(person:42, tmdb)`, When the queue reloads,
    Then that provider chip reads "not matched," is excluded from "needs review" counts, and
    reopening `EnrichPicker` for it does **not** re-run `/resolve` until "Try again" is clicked.
- **P0-5 — Refresh bypass (RD7).** `EnrichProviderChips`'s primary action becomes **Refresh**
  once a provider is linked, calling `apply()` directly with the stored `external_id`; "Re-match"
  and "Clear" move to the ⋯ overflow.
  - Given a Studio is already linked to `tmdb:174`, When the owner clicks the `tmdb` chip's
    primary action, Then `apply(tmdb, "tmdb:174")` is called directly — no `/resolve` request is
    made and no picker opens.
- **P0-6 — Per-provider status chips (RD9).** Every surface that shows enrichment status (queue
  rows, detail-page chips) represents state per `(entity, provider)`, never collapsed to one flag
  per entity.

### Should-have (P1) — view-source link, refresh-all

- **P1-1 — Optional `profile_url` on candidates (RD6).** `/resolve` candidates may carry
  `profile_url`; when present and scheme-valid (`http`/`https`), both `EnrichPicker` and the queue
  render it as a "view source ↗" link opening in a new tab. Absent or invalid → no link rendered,
  no error. [`metadata-provider-contract.md`](metadata-provider-contract.md) §2.3 documents it as
  optional.
  - Given a candidate carries `profile_url: "javascript:alert(1)"`, When Holodex serves it to the
    client, Then the link is dropped server-side (scheme validation) — never rendered as a
    clickable `javascript:` URL.
- **P1-2 — Refresh-all (RD8).** One action per entity that fans out across its configured
  providers, applying RD1's routing per provider and surfacing any ambiguous result inline rather
  than silently skipping it.
  - Given an entity has two providers, one already linked and one unlinked-but-resolves-to-two-
    candidates, When the owner clicks "Refresh all," Then the linked provider silently refreshes
    and the unlinked one surfaces an inline "N possible — Review" affordance for that provider only.

### Future considerations (P2)

- **P2-1 — Queue-wide bulk/background resolution** — once the provider rate-limit contract exists.
- **P2-2 — "Last checked" annotation for the no-data-yet state** — requires persisting an
  attempt marker on every resolve, not just dismissals; deferred pending real need (Open
  Questions Q2).
- **P2-3 — Cross-provider confidence calibration/normalization.**
- **P2-4 — Auto-resurfacing / TTL for dismissed verdicts.**

## Data model

```
enrichment_dismissals                    -- new; the "not matched" verdict (RD4)
  entity_type    TEXT      -- 'person' | 'studio' | 'video'
  entity_id      INTEGER
  provider       TEXT
  dismissed_at   DATETIME
  PRIMARY KEY (entity_type, entity_id, provider)
```

No other schema changes — auto-apply writes through the existing `entity_enrichment` shadow store
and F36 decision tables exactly as a manually-confirmed `apply()` does today; Refresh/Refresh-all
call the same `apply()` path with an `external_id` already on hand instead of one chosen from a
fresh `/resolve`.

## API

All under `/api/v1`, `requireOwner` (ADR-030) except the public provider directory. Mirrors the
existing per-entity-type route shape (`/people`, `/studios`, `/media`) rather than a generic
entity-kind parameter, matching today's `enrich/resolve` · `enrich` · `enrich/{provider}` routes.

```
GET    /owner/enrich-queue                                        → {rows:[{entity_type,entity_id,name,providers:[{provider,state}]}]}
                                                                       state: 'unreviewed' | 'auto_applied' | 'needs_review' | 'not_matched'

POST   /people|studios|media/{id}/enrich/{provider}/dismiss        → 204  (records not-matched, RD4)
DELETE /people|studios|media/{id}/enrich/{provider}/dismiss        → 204  (Try again — clears the verdict)
POST   /people|studios|media/{id}/enrich/{provider}/refresh        → 200 {enriched}   (direct apply w/ stored external_id, RD7 — 400 if not linked)
POST   /people|studios|media/{id}/enrich/refresh-all               → 200 {results:[{provider,status,enriched?}]}   (RD8 — status: 'refreshed'|'auto_applied'|'needs_review'|'no_candidates')
```

Existing `enrich/resolve`, `enrich` (apply), and `enrich/{provider}` (DELETE, clear) are unchanged;
`/resolve`'s response gains the optional `profile_url` field on each candidate (P1-1).

## Success Metrics

- **Leading:** owner clicks to enrich a single-strong-match entity drops from ~4 (open picker →
  wait for auto-search → click candidate → confirm) to **1** (open queue row).
  - Measurement: instrumented click count per successful enrichment, compared pre/post launch.
- **Leading:** zero repeat-prompts on a dismissed `(entity, provider)` pair in QA — reopening
  `EnrichPicker` for a dismissed pair does not re-run `/resolve` until "Try again."
- **Leading:** Studio and Media entries appear in the queue alongside People on first release
  (parity check — the mechanism is not Person-only).
- **Lagging:** queue backlog count (the P0-1 zero-cost signal) trends down across successive
  owner sessions.
- **Lagging:** enrichment coverage (% of entities with ≥1 linked provider) rises across all three
  entity types, not just Person.

## Open Questions

- **Q1 (engineering, blocking for Person):** ADR-055 flags Person as the one entity type whose
  `external_id` is stored but not yet used for identity (HOLODEX-125). RD7/RD8 assume "linked ⇒
  has a stored `external_id`" — confirm HOLODEX-125 lands (or scope Refresh/Refresh-all to
  Studio/Media only until it does).
- **Q2 (engineering, non-blocking):** is a "last checked: 3d ago" annotation on the *no-data-yet*
  state (not just the dismissed state) worth the write volume of an attempt-log row per resolve
  call? Deferred as P2-2; revisit if owners report confusion between "never tried" and "tried,
  found nothing."
- **Q3 (design, resolved):** review-queue row grouping/ordering — grouped by entity type in nav
  order (People → Studios → Media, not Duplicates' frequency-driven tags-first, since no entity
  type dominates the enrichment backlog the way tags dominate near-miss duplicates); within a
  group, rows with an actionable provider (`needs_review`/`unreviewed`) sort above fully
  `auto_applied`/`not_matched` rows. See the [design handoff](../design/enrichment-review-workflow-handoff.md#1-ownerenrichment--the-review-queue-tab).
- **Q4 (engineering, non-blocking):** the provider contract's current §2.3 text ("Holodex always
  shows the owner a picker and never auto-applies a candidate in v1") needs an explicit amendment
  once P0-2 ships. Confirm this is documentation-only (no wire-format change — `confidence`'s
  shape is unchanged, only how Holodex's *client* uses it) rather than a `protocol_version` bump.

## Timeline / routing

No hard deadline. Per the change-routing rules, before/with implementation:

1. ✅ **`/architecture`** — [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md)
   records the auto-apply-with-revert model (the provider-contract posture change, Q4) and the
   `enrichment_dismissals` store, mirroring how ADR-061 preceded F43.
2. ✅ **`/design-handoff`** — [enrichment-review-workflow-handoff.md](../design/enrichment-review-workflow-handoff.md):
   the Enrichment tab under `/owner`, per-provider status chips, the `EnrichPicker` "None of these
   match" + view-source link additions, the flipped-primary-action chip states (RD7), Refresh-all's
   inline partial-result UI (P1-2). Q3 resolved (grouped People → Studios → Media, actionable rows
   first). Tokens only; QA Cinémathèque / Broadcast / Brutalist.
3. ✅ **`/testing-strategy`** — [docs/testing-strategy.md](../testing-strategy.md) (§4/§5, "Critical
   invariants", Phase 3 F47 subsection): queue population/lazy-resolve correctness (P0-1 never
   triggers a provider call), auto-apply threshold + revert (P0-2/P0-3), dismissal persistence +
   non-renag (P0-4), `profile_url` scheme validation (P1-1), Refresh/Refresh-all idempotency and
   partial-failure handling (P0-5/P1-2). Written ahead of S1–S4; nothing automated yet.
4. ⬜ **`/security-review`** — `profile_url` is the one new externally-influenced surface
   (untrusted provider input rendered as a link — scheme validation is the mitigation, P1-1); new
   mutations (dismiss/undismiss/refresh/refresh-all) are `requireOwner` like all existing
   enrichment endpoints, no new ungated surface expected — confirm on the implementation diff.

**Slices:** **S1** data model + generalized dismiss/undismiss/refresh endpoints (`enrichment_dismissals`
table; per-entity-type route additions) → **S2** `GET /owner/enrich-queue` + the `/owner`
Enrichment tab (entity-generic, mirrors F43's Duplicates tab) → **S3** `EnrichPicker`: auto-apply
on a single strong match, "None of these match" action, view-source link rendering → **S4**
`EnrichProviderChips`: Refresh primary action + Refresh-all → **S5** provider contract doc
amendment (`profile_url`, updated auto-apply posture, Q4) → **S6** QA + security. P0 = S1–S4;
P1 = S3 (link)/S4 (refresh-all) once ratified. Effort: **L**.
