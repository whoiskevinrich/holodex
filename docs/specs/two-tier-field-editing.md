# Spec: Two-Tier Field Editing Model (F56)

**Status**: Draft
**Phase**: 4 (Curation tooling)
**Issue**: [HOLODEX-268](https://whoiskevinrich.atlassian.net/browse/HOLODEX-268)
**Epic**: [HOLODEX-267](https://whoiskevinrich.atlassian.net/browse/HOLODEX-267) — Entity decision editing overhaul (this spec covers the first of six linked stories; see § Non-Goals for the rest)
**Depends on**:
- Per-field source-of-truth decisions ([F36](field-source-of-truth.md) / [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)) — the decision model, API, and RD1–RD5 rules this spec re-presents but does not change.
- People decisions ([F37](people-source-of-truth.md)) — Person fields ride the same decision primitive.
- The baseline-source contract ([ADR-052](../architecture/ADR-052-baseline-source-contract.md)).
- The owner gating seam ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)).
- Frontend theming ([ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).
- Existing components: `SourceSelect.svelte`, `CurationChip.svelte` (`radio` mode), `ProvenanceBadge.svelte`, `f36.ts` (`resolveSelection`, `standing`, `isPendingSelection`, `outOfSync`).
- **Precedent, not a hard dependency**: the F51/ADR-079 image-edit-overlay pattern and the existing Tags hover-chip pattern — both already Tier-1-shaped and cited throughout as the visual language to extend.
- **New ADR**: none. This spec restructures *presentation* of the existing ADR-051 decision model — same API, same persistence, same RD1–RD5 rules. No new architectural decision is introduced.

---

## Origin

Surfaced during a product-brainstorming session (2026-08-09) on making metadata decisions from
entity pages instead of only the owner batch/field-claims list, and aligning owner editing
visually with the visitor view. That session found a confirmed bug along the way and the owner
explicitly chose to fold it into this redesign rather than patch it standalone — see § Problem
Statement.

## Problem Statement

Every scalar ("replace") field with more than one candidate source renders an always-on
`SourceSelect` radiogroup next to it — `Keep file / Adopt <provider> / Custom` — regardless of
whether the owner is actively deciding anything. This is visually nothing like what a visitor
sees (a plain resolved value), forcing constant context-switching between "what am I editing"
and "what does this actually look like."

Worse, it's broken for the single most common case. When a field's baseline is empty, the
resolver's implicit winning provider value renders as a selected-looking chip but is **not** a
standing decision (RD6, `standing: false` — [F36 §Behavior detail](field-source-of-truth.md)).
`SourceSelect.svelte`'s `activate()` has a no-op guard —

```js
if (chip.key === committedKey) {
    pendingKey = null;
    return;
}
```

— that fires on exactly this chip, because `resolveSelection()` (`f36.ts:138-167`) already
reports its key as `committedKey`. The chip that looks selected on the Person page (and anywhere
else a field has never been decided) cannot be clicked to confirm it. There is no way, short of
picking a *different* chip and back, to turn a pending implicit winner into a standing decision.

## Goals

1. **Fix the pending-chip bug structurally**, not by patching the guard. An explicit Confirm
   action, distinct from "select a chip," makes the ambiguity the bug lives in impossible to
   reintroduce.
2. **Tier-1 fields render identically to the visitor view at rest** on Video/Person/Studio pages
   — no permanent radiogroup, no owner-only chrome unless hovering/focused.
3. **Tier-2 fields collapse to the same badge a visitor already sees** (`ProvenanceBadge`) and
   expand into the decision UI only on demand, owner-only.
4. **Zero change to the underlying decision model.** Same API (`PUT`/`DELETE
   .../fields/{canonical}/decision`), same RD1–RD5 rules, same writeback behavior. This is a
   presentation-layer restructuring of F36, not a new feature.
5. **One consistent interaction across every Tier-2 field** on Video, Person, and Studio — no
   per-entity variation in how expand/select/confirm behaves.

## Non-Goals

*(Each of these is a separate, already-filed story under the same epic — do not fold into this one.)*

- **The docked-pencil name-edit mechanism** for Video Title / Person / Studio / Tag names —
  [HOLODEX-269](https://whoiskevinrich.atlassian.net/browse/HOLODEX-269). This spec explicitly
  leaves the name field's *existing* mechanism untouched (Person's `onadopt`-intercepted
  `SourceSelect`, Studio's `AliasPanel`) — it is Tier-1 by classification but out of scope to
  rebuild here.
- **The video composite-key ({title, people, date, studio}) collision check** —
  [HOLODEX-270](https://whoiskevinrich.atlassian.net/browse/HOLODEX-270).
- **The Studio relationship-edit popover** (known-candidates + search-any-studio + create-new) —
  [HOLODEX-271](https://whoiskevinrich.atlassian.net/browse/HOLODEX-271). Video's Studio field is
  classified Tier-1 (F56.6) and is excluded from this spec's conversion — it keeps its current
  `SourceSelect` presentation until 271 replaces it with the relationship popover.
- **People add/remove on video** — [HOLODEX-272](https://whoiskevinrich.atlassian.net/browse/HOLODEX-272). The People grid is untouched by this spec.
- **The writeback dialog's "Select all undecided" bug** —
  [HOLODEX-273](https://whoiskevinrich.atlassian.net/browse/HOLODEX-273). That story's fix
  *routes through* this spec's new Confirm/commit path — it depends on this shipping first, but
  this spec does not touch `WritebackFormDialog.svelte`.
- **Merge ("union") fields are unaffected.** Actors, genres, aliases, tags, etc. keep the
  existing F30 `CurationFieldRow` chip UI unchanged (RD1: the segmented source control only ever
  applied to *replace* fields; this spec doesn't widen that).
- **Image fields are unaffected.** `poster_url`, person `photo`, studio `branding_image` already
  use the F51/ADR-079 hover-edit-overlay pattern, not `SourceSelect` — out of scope, already
  Tier-1-shaped.
- **Tag entity pages are unaffected.** Tag has no `resolver` baseline and no field-conflict
  machinery — confirmed via `internal/resolver` (only `person_baseline.go` and
  `studio_baseline.go` exist). There is nothing for this spec to convert on a Tag page.
- **No new backend surface.** No new endpoint, no new DB column, no new access-control shape.

## Personas

- **Owner** — the only persona who sees any of this. Confirms pending fields, expands Tier-2
  badges to change a source, edits Tier-1 fields via their existing/future dedicated mechanisms.
- **Viewer** — unaffected, and that's the point: a Tier-2 field at rest must render byte-for-byte
  the same markup path a viewer already sees (`ProvenanceBadge` + resolved value), with owner
  interactivity layered on top via `{#if isOwner}`, never a different DOM structure.

## User Stories

1. **As the owner, I want to click the badge next to a field that has more than one candidate
   source**, so I can see and choose among them without a permanent control cluttering every
   field on the page.
2. **As the owner looking at a field with an unconfirmed implicit winner (RD6 pending)**, I want
   an unambiguous Confirm action, so clicking the thing that looks chosen actually chooses it —
   fixing the bug where this currently does nothing.
3. **As the owner, when I expand a field and change my mind**, I want to collapse without
   committing (Escape or clicking away), so exploring the candidates carries no risk of an
   accidental decision.
4. **As the owner viewing a Tier-1 field** (Title, People, Studio, Tags on Video; name on
   Person/Studio), I want it to look exactly like the visitor view, so I'm confident I'm seeing
   what visitors see while I decide whether to edit it via its own mechanism.
5. **As a viewer**, I should see no difference at all before or after this ships — same badge,
   same value, same layout.

## Interaction design

### At rest

A **Tier-2 field** (any replace field with 2+ candidate sources, per RD1's replace/merge split)
renders exactly what a viewer sees today: the resolved value plus `ProvenanceBadge` (provider
brand icon, or a muted "file" pill). No `SourceSelect` radiogroup. Owner-only: the badge gets a
`cursor: pointer` affordance and a subtle hover/focus treatment (tokens only — no new color, an
existing `--rule`/`--surface-2` hover state) so it reads as interactive without adding visual
weight at rest.

A replace field with only **one** candidate source (nothing to decide) renders the same way but
is **not** interactive — no badge click target, matching a viewer's experience exactly, since
there is no decision to expose.

### Expand

Clicking the badge expands the field in place into a chip row — reusing `CurationChip`'s
existing `radio` mode, one chip per candidate source (file, each matched provider, Custom) — plus
an explicit **Confirm** and **Cancel** action. Clicking a chip changes the *staged* selection
only; it does not call `decide()`. This is the structural fix for the bug: there is no chip whose
click both looks like a selection and silently does nothing, because no chip click commits
anything by itself.

- **Confirm** calls the existing `PUT .../fields/{canonical}/decision` (or `DELETE` if Custom was
  cleared back to a candidate) exactly as `SourceSelect` does today — no API change.
- **Cancel**, clicking outside, or **Escape** collapses back to the at-rest badge with no
  decision change, discarding the staged selection.
- **Custom** opens the existing inline input (unchanged from F36/F30's pattern) in place of the
  chip row; Confirm still gates the commit.

### The pending (RD6) case, explicitly

Given a field whose baseline is empty and an implicit provider winner is showing (RD6,
`standing: false`): the badge still renders (this *is* a decision worth making), the expanded
chip row shows that provider's chip already staged/highlighted as the current effective value,
and clicking **Confirm** with no further chip click **creates a standing decision** for that
source. This is the one new piece of behavior this spec introduces to the decision model — not a
new API shape, but the first UI path that can actually commit an RD6 pending value with a single,
unambiguous click.

### Sync indicator

Unchanged. The existing per-field out-of-sync `text-warn` pill (RD2) continues to render
independent of expand/collapse state — this spec restructures the source-selection affordance
only, not the sync signal.

## Requirements

### Must-have (P0)

| ID | Requirement | Acceptance criteria |
|----|---|---|
| F56.1 | Every replace field with 2+ candidate sources renders at rest as `ProvenanceBadge` + value only — no visible `SourceSelect` radiogroup. | Loading a video/person/studio detail page as owner shows no permanent segmented control anywhere; loading as a viewer shows an identical DOM shape (minus the owner-only click affordance). |
| F56.2 | Clicking the badge expands the field into a `CurationChip` `radio`-mode row with Confirm/Cancel, staged (not committed) selection. | Clicking a chip changes its visual state; no network request fires until Confirm is clicked. |
| F56.3 | Confirm commits via the existing decision API; Cancel/Escape/click-away discards the staged change with zero API calls. | Confirming calls `PUT .../decision` with the staged source; Cancel makes no request and the field returns to its pre-expand rendered value. |
| F56.4 | **Bug fix**: confirming an RD6-pending field's current implicit winner creates a standing decision. | Given a field with `isPendingSelection() === true`, when the owner expands and clicks Confirm without changing the staged chip, then `GET` on that field afterward reports `standing: true` for that source — previously this action was a no-op. |
| F56.5 | A replace field with exactly one candidate source renders the value with no badge interactivity. | A field with only a file value (no matched providers) shows the value, non-interactive, matching the viewer's rendering exactly. |
| F56.6 | Tier-1 fields (Title/People/Studio/Tags on Video; name on Person/Studio; all image fields) are unaffected by this spec — no wrapping in the new Tier-2 markup. | Video Title, Person/Studio name headers, and every image slot render exactly as they do before this change; their existing mechanisms (or lack thereof, pending HOLODEX-269–272) are untouched. |
| F56.7 | Merge fields are unaffected. | Actors/genres/tags/aliases continue to render via `CurationFieldRow`, unchanged by this spec. |
| F56.8 | Owner-gated, themed, accessible. | Expand/collapse and chip selection are keyboard-operable (roving tabindex within the expanded row, matching `SourceSelect`'s existing pattern); QA in Cinémathèque, Broadcast, and Brutalist; `rg 'zinc-\|sky-\|emerald-\|amber-\|rounded-(lg\|md\|sm\|xl)'` over new/changed components is empty. |

### Nice-to-have (P1)

| ID | Requirement | Acceptance criteria |
|----|---|---|
| F56.9 | Expanding one field's badge collapses any other currently-expanded field on the same page. | Expanding field B while field A is expanded (with an uncommitted staged change) collapses A back to its badge, discarding A's staged change — never two fields open at once. |
| F56.10 | A brief, dismissable confirmation affordance (e.g. a momentary highlight) on the field after a successful Confirm, so the owner has positive feedback the click did something — directly answering the "does clicking this do anything?" doubt that motivated this spec. | After Confirm, the field's badge/value briefly highlights (reusing an existing token transition, no new animation system) before settling to its new at-rest state. |

### Future considerations (P2)

| ID | Requirement | Notes |
|----|---|---|
| F56.11 | Extend the Tier-2 pattern to any future replace field automatically, with no per-field UI work. | Should already be true by construction (the pattern is field-shape-driven, not per-field-coded) — call out explicitly as a design constraint for implementation, not a new requirement. |

## Data, storage & serving

No changes. This spec reads and writes exactly the same `field_source_decisions` rows, through
exactly the same API, that F36 already ships. `resolveSelection`, `standing`,
`isPendingSelection`, and `outOfSync` in `f36.ts` are reused as-is — the fix in F56.4 is a
frontend commit-path change (Confirm now calls `decide()` in the pending case, where
`SourceSelect.activate()` currently no-ops), not a change to how those functions resolve state.

## Frontend / theming requirements

- New: a Tier-2 field wrapper component (owns badge-at-rest / expand / chip-row / Confirm-Cancel
  state) — reuses `CurationChip` (`radio` mode) and `ProvenanceBadge` internally rather than
  duplicating their rendering.
- `SourceSelect.svelte` is retired for every field this spec converts. It is **not** deleted
  outright in this story if Person's name field (`onadopt` intercept, HOLODEX-269's concern) still
  depends on it — confirm at implementation time whether a shared component or two call sites
  remain until HOLODEX-269 lands.
- Tokens only — no hardcoded palette/radii on the new component.
- QA all three skins for every state: at-rest (single-source, multi-source-undecided,
  multi-source-decided, RD6-pending), expanded, post-Confirm, post-Cancel.

## Access control & security

No new surface. This spec reuses F36's already-reviewed owner-gated API verbatim — no new
endpoint, no new mutation shape, no new persisted field. Per the project's change-routing rules,
`/security-review` is triggered by auth/access/infrastructure changes; this story introduces
none. **No security review required for this story.** (Contrast with HOLODEX-272, which adds a
genuinely new mutation surface and does need one.)

## Success Metrics

Single-owner correctness/UX feature, not a funnel — mirrors F36's framing:

- **Leading**: the bug is verifiably gone — an RD6-pending field can be confirmed with exactly
  one Confirm click, in every skin (manual QA + a regression test asserting F56.4's acceptance
  criterion).
- **Leading**: a page with N fields shows zero always-on radiogroups — visual parity with the
  logged-out view, confirmed by a snapshot/DOM comparison in tests.
- **Lagging**: no more "does clicking this do anything?" — qualitative, checked by the owner
  actually using the redesigned Person page for a normal curation session post-ship.

## Resolved Decisions

*(Carried over from the brainstorming session that produced this epic — see [[project-holodex-entity-decision-overhaul]] memory / HOLODEX-267.)*

- **Per-field editing, not bulk.** Confirm commits one field at a time. Bulk stays scoped to the
  separate writeback-to-file action (HOLODEX-273), never to accepting multiple pending decisions
  in this UI.
- **Fold the bug fix into this redesign, don't patch `SourceSelect` standalone** — the explicit
  Confirm step is what makes the ambiguous "click the selected-looking chip" interaction
  structurally impossible to reintroduce.
- **Tag is excluded entirely** — no field-conflict machinery exists for it; nothing to convert.

## Open Questions

- **[engineering, non-blocking]** Does `SourceSelect.svelte` get deleted in this story, or does it
  stay alive solely for Person's `onadopt`-intercepted name field until HOLODEX-269 replaces that
  mechanism too? Leaning: keep it alive, scoped down to that one remaining caller, and delete it
  as part of HOLODEX-269 — avoids a dangling half-migration inside this story.
- **[design, non-blocking]** Exact visual treatment of the "click me" affordance on an
  at-rest `ProvenanceBadge` (hover-only vs. a persistent subtle indicator) — resolve in
  `/design-handoff`, consistent with whatever discoverability language HOLODEX-269 settles on for
  the docked-pencil affordance, so the two read as one visual system rather than two.
- **[engineering, non-blocking]** F56.9 (single-field-expanded-at-a-time) — confirm this doesn't
  conflict with a field that has an in-flight Confirm request when another badge is clicked; likely
  resolved by simply disabling other badges while a commit is in flight.

## Timeline / routing

No hard deadline. Per the project's change-routing rules:

1. **`/design-handoff`** — required (`needs-design` on HOLODEX-268). Covers the Tier-2 wrapper
   component across all three skins and all four at-rest states, plus reconciling its
   discoverability language with HOLODEX-269's docked-pencil affordance so they read as one
   system (see Open Questions).
2. **`/testing-strategy`** — add an F56 block: the RD6-confirm regression test (F56.4), viewer/owner
   DOM-parity assertion (F56.1), staged-vs-committed selection (F56.2/F56.3), three-skin QA.
3. **`/security-review`** — not required (see § Access control & security).

Suggested build order (informal, non-gating):
1. Tier-2 wrapper component (badge/expand/chip-row/Confirm-Cancel), built and tested against one
   field on one entity page.
2. F56.4's RD6-confirm fix, verified with a regression test reproducing the original bug report.
3. Roll out to every remaining replace field across Video/Person/Studio.
4. Retire `SourceSelect` for all converted call sites (see Open Questions on the Person-name
   exception).
5. Three-skin QA pass.

## Artifacts to produce (project working agreements)

- [x] This spec (`docs/specs/two-tier-field-editing.md`).
- [ ] **Design handoff** — Tier-2 wrapper component, all states, all three skins. (`needs-design` on HOLODEX-268.)
- [ ] **Testing strategy** — add an F56 block to `docs/testing-strategy.md`.
- [x] **Security review** — not required, see § Access control & security.
- [ ] Add this spec to the `docs/architecture/README.md` phase-specs index.
