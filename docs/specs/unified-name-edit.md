# Unified name-edit mechanism (F56.2, HOLODEX-269)

Parent epic: [HOLODEX-267](https://whoiskevinrich.atlassian.net/browse/HOLODEX-267) — Entity
decision editing overhaul. Sibling stories: HOLODEX-268 (two-tier field editing model — this
spec treats the entity **name** as the Tier-1 in-place field for Person/Studio/Tag/Video,
complementing HOLODEX-268's Tier-1 treatment of Video's Title/People/Studio/Tags fields), and
HOLODEX-270 (video composite-key collision check, which plugs into this mechanism's collision
seam for Video Title specifically — see Non-Goals).

## Problem Statement

Renaming an entity works today, but through three divergent, bespoke implementations plus one
missing case entirely:

- **Person**: name renders through `SourceSelect`'s `onadopt` intercept
  (`people/[id]/+page.svelte:255-291`) — not visually distinct from the generic per-field
  source-of-truth chip row, and not co-located with the page's own `<h1>`.
- **Studio**: rename lives inside `AliasPanel`, buried in the Aliases section further down the
  page (`studios/[id]/+page.svelte:217-230`) — not co-located with the header either.
- **Tag**: rename/merge/near-miss only exists in the list page's "Manage tags" mode
  (`tags/+page.svelte:228-290`). Tag *does* have its own detail page (`tags/[id]/+page.svelte`,
  live since Phase 1, substantially built out by HOLODEX-259 three days before this spec —
  parent/children/categories/writeback toggle/sync are all there) — but that page has no rename
  or merge affordance at all today. The gap isn't a missing page, it's a missing control on an
  existing one.
- **Video Title**: no rename affordance near the `<h1>` at all
  (`media/[id]/+page.svelte:622` is a plain, non-editable heading). Title *is* technically
  editable today, but only by scrolling to the generic Metadata list and using the same
  generic source-of-truth control used for genres or overview — a location with no relationship to
  where the title is actually displayed.

An owner has to remember a different interaction per entity type to do the same conceptual
thing — rename this thing, and tell me if that name collides with something else. That
inconsistency is the direct target of this story, not a cosmetic unification: three of these four
mechanisms already share the same backend primitives (see **Existing State** below), so the gap
is almost entirely in the frontend surface, not the data model.

## Existing State (corrects the epic's original framing)

Codebase research for this spec found the current mechanisms are closer to each other than the
epic's originating brainstorm assumed, which changes the risk profile of "one shared component":

- **Person and Studio already render the identical collision/merge-offer card** on an exact
  name collision (both route into the confirm-card markup embedded in `AliasPanel`), and both
  already call the same backend primitive: `POST /{people|studios}/{id}/rename` →
  `Repo.RenameEntity` → `EntityConflict` (409) → the caller offers merge. The epic's "biggest
  open unknown" (whether Person's and Studio's collision handling can share one component) is
  **substantially de-risked** — they already do, just via two different UI entry points into the
  same card. The real gap is *where the rename trigger is anchored* (chip-row vs.
  buried-in-Aliases-panel), not the collision-handling logic itself.
- **Tag's exact-collision path uses the same primitive too** (`entity_identity.go`'s generic
  `mergeEntity`/`renameEntity`/`EntityConflict`), plus a **fuzzy near-miss** advisory layer
  (`looseKeyExpr`, `Repo.NearMiss`) that Person and Studio do not currently get —
  `api.nearMiss`'s type signature excludes `'person'` outright.
- **The merge primitive** (`Repo.MergeEntities`, `identity_ops.go:249-401`) is already
  entity-generic and config-driven per type, including the load-bearing "loser's name becomes an
  alias of the survivor" step (ADR-061 D6) — this story reuses it as-is, no backend change to
  the merge mechanics themselves.
- **A fourth, even-more-divergent precedent exists**: `categories/[id]/+page.svelte` already has
  an always-visible bordered pencil beside its `<h1>` opening an inline rename form — closest
  existing visual precedent for "docked pencil beside a name," even though Category isn't in
  this story's scope (it isn't on the ADR-061 identity spine).
- **Correction to this spec's own first draft**: an earlier version of this document claimed Tag
  had no detail page at all and proposed building one, reversing F43's RD7 non-goal
  (`docs/specs/entity-identity.md`). That was wrong — `tags/[id]/+page.svelte` has existed since
  Phase 1 and was expanded three days before this spec was written (HOLODEX-259, PR #221:
  parent/children/categories/writeback/sync). RD7's "no detail page" line was already stale
  relative to that shipped page and needs no action from this story either way; nothing here
  reverses or supersedes it. The actual, much smaller gap is that the existing page has no
  rename/merge control — this spec adds one, it doesn't create a page.

## Goals

1. One shared frontend component (a docked-pencil affordance + inline edit + pluggable
   collision/verdict handling) replaces the three divergent rename UIs and adds the missing
   Video Title case — an owner learns the interaction once, uses it everywhere.
2. Every entity's name-edit surface is co-located with where the name is actually displayed
   (the `<h1>` / pill), not buried in a secondary panel or a generic field list.
3. Tag's existing detail page (`tags/[id]`) gains the same docked-pencil rename affordance the
   other three entity types get — closing the gap where it's the only identity-spine entity whose
   own page has no rename control, without adding a page it already has.
4. No regression to existing collision/alias/merge correctness — this is a frontend
   consolidation over backend primitives that already work; the spec's P0 is "same guarantees,
   one surface," not new backend semantics.

## Non-Goals

- **Video Title's collision detection.** Video isn't on the identity spine (titles aren't
  deduped) — HOLODEX-270 defines a different collision surface entirely (composite
  `{title, people, date, studio}` match, verdict = view-existing/save-anyway, not
  keep-separate/merge). This story wires Video Title's commit action into the same
  pluggable-collision *seam* the shared component exposes, but ships a no-op checker for video
  in the interim — HOLODEX-270 plugs its real checker into that seam later. Building HOLODEX-270's
  checker itself is out of scope here.
- **Extending fuzzy near-miss to Person.** Person's `nearMiss` gap is pre-existing and not a
  regression this story introduces; closing it is a legitimate but separable follow-up (P2,
  see Requirements) — don't block this story's scope on it.
- **Removing or restructuring Tag's list-page bulk multi-select merge.** Bulk merge
  (`MergeCanonicalDialog`, 2+ pills selected in "Manage tags" mode) is a list-level operation
  distinct from one entity's own rename affordance — it stays on the list page exactly as it is
  today. The single-tag rename/alias/near-miss actions become available on the detail page too
  (additive, not a move — the list page's per-pill actions stay as a shortcut, see Requirements).
- **Restructuring anything already on the tag detail page.** Parent/children/categories/writeback
  toggle/sync (HOLODEX-259, ADR-075/077/078) are untouched — this story only adds the
  `NameEditControl` to the page's existing header, alongside the name it already renders.
- **Changing the merge primitive itself.** `Repo.MergeEntities` and `EntityConflict` are reused
  verbatim; this is a consumer-side consolidation.

## Resolved Decisions

- **No page is created or reversed for Tag.** `tags/[id]` already exists (Phase 1, expanded
  HOLODEX-259) — this story docks the shared `NameEditControl` on its existing header, exactly
  like Person/Studio. `docs/specs/entity-identity.md`'s RD7 stands unchanged: it governs the
  field-decision model (Tags don't get one) and was never actually about page existence in a
  load-bearing way, despite its "no detail page" phrasing having gone stale once Phase 1's
  browse-style tag page shipped. No edit to RD7 or ADR-061 is needed for this story.

## User Stories

- As an owner viewing a Person/Studio/Tag detail page, I want to click a pencil next to the
  entity's name so that I can rename it without hunting through a field list or a secondary
  panel.
- As an owner renaming an entity to a name another entity already has, I want a clear choice
  between keeping both as separate entities or merging them, so that I don't accidentally create
  a duplicate or accidentally destroy a distinct entity's identity.
- As an owner viewing a video's detail page, I want to rename the video the same way I rename a
  person or studio (pencil beside the title), so I don't have to remember that title editing
  lives in a different place than everything else.
- As an owner who renames a tag from its list page today, I want that same action available from
  a tag's own detail page too, so I don't have to leave the page I'm already looking at to rename
  what I'm looking at.
- As an owner, when I successfully rename an entity, I want the old name preserved as a
  searchable alias (already true today) so that existing references and scan-time matching keep
  working.

## Requirements

### Must-Have (P0)

- **Shared `NameEditControl` component** (or equivalent name, finalized in design-handoff):
  docked pencil (low-opacity at rest, brightens hover/focus — matching the discoverability
  language already set for HOLODEX-268's Tier-1 fields) → inline text edit → commit. Props:
  current name, an async `onCommit(value)` that performs the rename call and reports back
  `{ok}` or `{conflict: <entity>}`, and a slot/prop for the verdict panel shown on conflict.
  - *Acceptance*: renders identically at rest for a non-owner and an owner-not-hovering (no DOM
    or visual difference); pencil only appears for an owner on hover/focus.
- **Extract the identity-collision verdict card out of `AliasPanel`** into its own
  entity-generic component (e.g. `entity/MergeOfferCard.svelte`) so the new `NameEditControl`
  can show it directly on a 409, without needing an `AliasPanel` instance nearby. `AliasPanel`
  keeps using the extracted component for its own alias-collision case (no behavior change
  there).
  - *Acceptance*: `AliasPanel`'s existing alias-add collision flow is unchanged after the
    extraction (regression check, not new behavior).
- **Person**: `SourceSelect`'s `onadopt` intercept for `name` is removed; the shared control
  replaces it, docked beside the person's own heading. `name` no longer appears as a row in the
  generic resolved-fields list (Tier-1 exclusivity — one editing surface, not two).
  - *Acceptance*: renaming a person via the docked pencil produces the identical
    `POST /people/{id}/rename` call, 409-conflict-into-merge-offer, and alias-on-success
    behavior as today's flow (parity, not new semantics).
- **Studio**: the Rename trigger + inline form is removed from `AliasPanel` and replaced by the
  shared control docked beside the studio's heading. `AliasPanel` keeps its Add-alias
  functionality (unaffected) and the near-miss nudge on studio rename continues to fire.
  - *Acceptance*: same parity requirement as Person, against `POST /studios/{id}/rename`.
- **Tag detail page** (`tags/[id]/+page.svelte`, existing route — no new page or backend
  endpoint): the shared `NameEditControl` is added to its header, beside the `<h1>` that already
  renders `tag.name` (`api.getTag` already returns everything needed). The control's rename call
  reuses the existing `POST /tags/{id}/rename` → `EntityConflict` → merge-offer path; near-miss
  continues to fire the same non-blocking advisory it does on the list page today.
  - *Acceptance*: renaming a tag from its detail page produces identical backend behavior
    (alias-on-success, 409-into-merge-offer, near-miss nudge) as renaming it from the list
    page's existing manage-mode UI.
- **Video Title**: the shared control is docked beside the video's `<h1>`
  (`media/[id]/+page.svelte:622`), wired to the existing `PUT /media/{id}/fields/title/decision`
  (`source: 'manual'`) call — no new backend endpoint needed for the commit itself. `title` is
  removed from the generic Metadata `SourceSelect` list (Tier-1 exclusivity, same as Person's
  name). The collision seam is wired to a no-op checker for now (see Non-Goals) — every commit
  succeeds immediately, matching Video's current field-decision behavior; HOLODEX-270 replaces
  the no-op with its composite-key checker later.
  - *Acceptance*: renaming a video's title via the docked pencil produces the identical
    `field_source_decisions` row (`source: manual`) as today's buried Metadata-list flow.

### Nice-to-Have (P1)

- Extend fuzzy near-miss (`api.nearMiss`) to Person, closing the pre-existing gap noted in
  research (excluded today by `api.nearMiss`'s type signature and `AliasPanel.flagNearMiss`'s
  early return for `'person'`).

### Future Considerations (P2)

- A shared `NameEditControl` reuse for Category (`categories/[id]/+page.svelte`'s existing
  bespoke docked-pencil-and-form), even though Category isn't on the identity spine and has no
  collision/merge semantics today — purely a consistency cleanup, not correctness-motivated.
- Once HOLODEX-270 lands, revisit whether Video Title's "no-op collision checker" placeholder
  should instead surface a lightweight non-blocking signal (e.g., "this looks similar to another
  video") ahead of the full composite-key verdict panel.

## Success Metrics

This is an internal-tool consistency fix for a single-owner (or small-team) product with no
external usage analytics pipeline — success is qualitative and verified via the standard
`/testing-strategy` pass (Go + Vitest coverage for the parity requirements above) and live
3-skin QA, not a metrics dashboard. The bar: an owner can rename any of the four entity types
using the identical gesture, and every existing collision/alias/merge guarantee still holds
(zero regressions in the parity acceptance criteria above).

## Open Questions

- **Component prop shape** for `NameEditControl` (exact prop names, how the verdict-panel slot
  is expressed, whether Video's no-op checker is a real function prop or a config flag) —
  engineering, resolved during `/design-handoff` and implementation, not blocking here.
- **Does the list page's "Manage tags" mode stay the primary tag-admin surface** now that its
  rename/merge/near-miss actions are duplicated on the detail page, or does usage shift naturally
  once the detail-page control exists? — product/design, non-blocking; P0 scope above doesn't
  require an answer either way to ship correctly.

## Timeline Considerations

No hard deadline. Dependency note: this spec conceptually extends HOLODEX-268's Tier-1
in-place-editing model (entity name *is* the Tier-1 field for Person/Studio/Tag/Video, the same
way Title/People/Studio/Tags are Tier-1 for the Video page) — HOLODEX-268's own spec/design docs
are not yet merged to `main` as of this writing, but this story's requirements don't structurally
depend on that merge landing first; align visual language (hover-reveal pencil treatment) with
HOLODEX-268's conventions when both are implemented, whichever lands first.
