# Spec: Unified Studio edit affordance + Film-level cascade writeback (F57)

**Status**: Draft
**Phase**: New epic (Jira [HOLODEX-285](https://whoiskevinrich.atlassian.net/browse/HOLODEX-285))
**Owner**: Project owner
**Date**: 2026-08-25
**Feature block**: **F57** — one owner-gated Studio edit affordance, rendered identically on the
Media detail page (a single video's resolved `studio` field) and the Film detail page (the
derived-union `studio` display across every video attached to a film). On Film, setting a studio
from that affordance **cascades**: it sets a new `manual` studio decision on every attached video
and writes each change back to its file, reusing the ADR-077 write-queue/batch-status mechanism
for progress. This is the first case in the app where an owner action fans out a decision + a
writeback across N videos in one gesture.

**Depends on** (all shipped):
- the F36 decision model ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)) and
  entity-generic resolver ([ADR-052](../architecture/ADR-052-baseline-source-contract.md)) — Media's
  `studio` field already rides this; Film's cascade sets decisions through the same
  `field_source_decisions` table, just N rows in one action instead of one
- `StudioPicker.svelte` (HOLODEX-271) — the existing docked-pencil Studio edit popover on Media,
  including its known-candidate chips, full-library search, create-fallback, and the HOLODEX-270
  video composite-key collision check via its `decide`/`verdict` contract
- `NameEditControl.svelte`'s docked-pencil pattern (HOLODEX-269) and `.name-edit-row`/
  `.name-edit-pencil` CSS hooks (`app.css`) — the shared hover/focus-reveal affordance this spec
  extends to Film
- the tag writeback-sync mechanism (HOLODEX-239, [ADR-077](../architecture/ADR-077-tag-writeback-exclusion.md)) —
  `internal/writequeue.EnqueueMany` with a `batchID`, `api.writebackBatchStatus` polling, and
  `WritebackBatchDialog.svelte` rendering aggregate pending/running/done/failed progress. F57
  reuses this display/polling shape but needs a **new** backend action: ADR-077's
  `syncTagWriteback` only re-pushes an *already-decided* value, it never sets one — F57 must set
  a new decision on each video, then enqueue the write, in one server-side action
- the films entity ([F56/ADR-085](films-entity.md)) — `internal/repo.FilmStudios` (the SQL union
  this spec's cascade target reads from) and `film_videos` (the attachment list the cascade walks)
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`)

**ADR**: **[ADR-086](../architecture/ADR-086-film-studio-cascade-decide-and-writeback.md) (Proposed)**
records the bulk-decide-then-writeback mechanism: a shared `decideStudioForVideo` helper
(extracted from the existing single-video Studio-decide path) called once per video attached to
the film, with a **best-effort, not all-or-nothing** failure posture — a per-video collision or
error excludes only that video from the shared-batch writeback rather than aborting the rest,
since each video's decision-set is already an independent commit by the time a later one might
fail. Endpoint is Film-scoped for v1 (`POST /films/{id}/studio/cascade`), not a general N-video
primitive (deferred per P1-2 below); writeback progress reuses ADR-077's existing
`GET /writeback/batches/{batchID}/status` unchanged. Touches **access** (new owner-gated
bulk-mutation endpoint) → `/security-review` before merge.

**Design handoff**: pending `/design-handoff` — the shared Studio affordance's visual contract on
Media vs. Film (same docked pencil, same popover chrome, different trigger copy/placement), the
Film-side cascade confirmation and progress UI (reusing `WritebackBatchDialog`), and 3-skin QA.

**Related**: [films-entity.md](films-entity.md) (defines the read-only derived-union this spec
partially reverses), [../design/films-entity-handoff.md](../design/films-entity-handoff.md) RD2/RD3
(the design decision this spec supersedes for Studio only — see Non-Goals), [field-source-of-truth.md](field-source-of-truth.md)
(the decision model both Media's and the cascade's writes go through).

---

## Problem Statement

The owner's editing experience for a video's Studio is inconsistent depending on which page
they're on. On the Media detail page, an owner sees a docked pencil next to the studio name and
can pick, search, or create a replacement — the same affordance-family used for Person/Tag/Title.
On the Film detail page, the same underlying data — the studio(s) attached to that film's
videos — renders as plain text links with no edit affordance at all, because Film's Studio value
is currently defined as a read-only derived union (RD2/RD3, `films-entity-handoff.md`) with no
per-film storage.

This forces the owner into a page-by-page workaround: correcting a studio for a multi-scene film
today means opening each attached video individually and repeating the same edit N times, with no
guarantee all N end up consistent, and no bulk confirmation of what changed. The owner has stated
directly that this is the problem to solve: **edit vs. read-only should be determined solely by
owner-view state, not by which entity or page is showing the value** — every linked-entity display
of a studio should offer the same affordance when the owner is in owner view, and the same
read-only presentation when they are not.

## Goals

1. One shared Studio edit affordance component, visually and behaviorally identical wherever a
   studio value is shown to an owner — Media detail (single video) and Film detail (N videos) —
   and identical read-only presentation to a non-owner in both places.
2. Setting a studio from the Film page's affordance cascades a `manual` studio decision to every
   video currently attached to that film, unconditionally (overwrites any existing per-video
   studio decision, including manually-set ones — no partial/opt-in cascade).
3. Every cascaded decision is also written back to its video's file in the same action, using the
   existing write-queue batch mechanism, with the owner seeing aggregate pending/running/done/
   failed progress exactly as they do for the existing Tag writeback-sync batch.
4. A per-video failure (collision, unwritable file, unsupported container) during the cascade is
   visible per-video in the batch result and does not block or roll back the videos that
   succeeded — consistent with ADR-077's existing partial-failure posture for Tag sync.

## Non-Goals

1. **Cast/Tags on the Film page stay read-only derived unions.** RD2/RD3 in
   `films-entity-handoff.md` are superseded by this spec **for Studio only** — Person and Tag
   values on a film are unchanged and remain out of scope. Extending the same affordance to them
   is a plausible future spec, not part of F57.
2. **No new film-level Studio storage.** The cascade sets decisions on each attached *video*, the
   same as if the owner had edited each video individually — it does not introduce a
   `film.studio_id` column or any per-film Studio record. Film's Studio display remains the same
   derived SQL union it is today (`FilmStudios`); F57 changes who can trigger a write into that
   union's inputs, not how the union itself is computed or stored.
3. **No selective/partial cascade UI in v1.** The user has resolved this explicitly: overwrite
   every attached video, no exceptions, no per-video opt-out checklist before commit. A
   review-before-cascade UI (e.g. "3 of 10 videos already have a different manual studio — apply
   anyway?") is a plausible P1 addition, not required for v1.
4. **No change to how Media's own Studio affordance behaves today.** `StudioPicker` on Media
   already has the docked-pencil pattern, collision check, and decision write; F57 reuses it
   as-is for the single-video case and does not re-scope its existing behavior.

## Resolved Decisions

*(Locked with the owner across a multi-round brainstorm, 2026-08-25, via question cards.)*

**RD1 — Edit affordance is gated on owner view, not on entity/page.** The user's explicit
correction: "Edit affordance is tied to Owner view. Read-only is when Owner view is not active.
All linked entities should have the same edit/read affordances that are visually the same." This
reverses the premise of `films-entity-handoff.md` RD2/RD3 for Studio — that spec treated
Film-Studio as *structurally* read-only (no storage to write to); this spec treats it as
*presentationally* gated on owner view like every other editable field, and adds the missing
write path (the cascade) so the affordance has something to do.

**RD2 — A Film-page studio edit propagates to the underlying videos, not a new film-level
value.** Considered and rejected: introducing film-level Studio storage that Media pages would
then need to reconcile against. Chosen: the film has no Studio of its own — editing "the film's
studio" is shorthand for editing every attached video's studio in one gesture, which is exactly
what the derived-union display already implies to the owner today (they're looking at what the
attached videos say).

**RD3 — The cascade also writes back to file, not just to the decision table.** The user's
answer: "Studio edits should cascade to all videos attached to the film and write back to the
files." An in-app-only decision update (with file drift persisting until each video is later
manually synced) was considered and rejected — the owner working at the film level cares about
file-level consistency across the scene set, not just app-level.

**RD4 — The cascade overwrites unconditionally.** The user's answer: "Overwrite every attached
video, no exceptions." Considered and rejected: skipping videos with an existing divergent manual
decision, or prompting per-video before applying. Chosen: the film-level action is the owner
declaring a new source of truth for every video in the set — a partial cascade would leave the
film's own derived-union display inconsistent with itself immediately after the action the owner
just took to make it consistent.

## User Stories

- As the owner, I want to see the same docked-pencil Studio affordance on a film's detail page
  that I already see on a video's detail page, so I don't have to remember which pages are
  editable and which aren't.
- As the owner correcting a mislabeled multi-scene film, I want to set the studio once from the
  film page and have every attached video (and its file) updated, so I don't have to repeat the
  same edit N times and risk leaving one scene inconsistent.
- As the owner triggering a film-level studio cascade, I want to see per-video progress and any
  per-video failure, so a stuck or unwritable file doesn't silently fail without my knowing which
  video needs manual attention.
- As a non-owner (or the owner outside owner view) visiting a film's page, I want the studio to
  render as a plain link with no edit UI, unchanged from today.

## Requirements

### Must-have (P0)

- **P0-1**: Extract (or adapt) `StudioPicker`'s affordance into a form usable from both Media
  (single-video `decide`) and Film (N-video cascade) call sites, without changing Media's current
  behavior. Acceptance: Media's Studio edit flow (collision check, chip candidates, search,
  create-fallback) is bit-for-bit unchanged after the extraction.
- **P0-2**: `POST /films/{id}/studio/cascade` (ADR-086 D3), owner-gated: given a chosen studio
  (existing entity ID or new-studio name), sets a `manual` studio decision on every video in that
  film's `film_videos` set (via ADR-086 D1's `decideStudioForVideo`, best-effort per D2 — see
  P0-4) and enqueues a writeback job for each video whose decision succeeded, under one shared
  `batchID` compatible with the existing `writebackBatchStatus` polling. Acceptance: calling it
  against a film with N attached videos and no collisions results in N `field_source_decisions`
  rows (studio, manual) and N write-queue jobs sharing one batch ID.
- **P0-3**: Film detail page renders the same docked-pencil Studio affordance as Media when
  `isOwner`, opening the shared picker; committing triggers the P0-2 cascade and shows
  `WritebackBatchDialog` (or an equivalent using the same batch-status polling) for progress.
  Acceptance: an owner can set a studio from the film page and watch aggregate
  pending/running/done/failed counts resolve.
- **P0-4**: Per-video collision (HOLODEX-270 composite-key check) or write failure during the
  cascade surfaces in the batch result without aborting the remaining videos. Acceptance: seed a
  film where one attached video's post-edit `{title, people, date, studio}` would collide with
  another existing video; the cascade completes for all non-colliding videos and reports the
  collision distinctly for the one that didn't.
- **P0-5**: Non-owner and owner-outside-owner-view rendering of Film's Studio section is
  unchanged from today (plain links, no pencil).

### Should-have (P1)

- **P1-1**: A pre-cascade summary ("this will overwrite N videos, M of which currently have a
  different manually-set studio") shown before commit, informational only — no opt-out
  checklist (Non-Goal 3), just visibility into blast radius before the owner confirms.
- **P1-2**: Reuse the film cascade's decide-then-enqueue backend action as a general N-video
  primitive (not Film-specific) if a second caller emerges — deferred until one actually does,
  per "no speculative abstraction."

### Future considerations (P2)

- **P2-1**: Extending the same unified edit-affordance treatment to Cast/Tags on the Film page
  (explicitly out of scope per Non-Goal 1) — would need its own product decision on whether those
  cascade the same way or use a different relationship-edit shape (attach/detach vs. replace).
- **P2-2**: Per-video opt-out or selective cascade (explicitly out of scope per Non-Goal 3) if
  the unconditional-overwrite posture proves too blunt in practice.

## Behavior detail

- The cascade is **all-or-nothing at the decision-write layer, best-effort at the file-write
  layer** — mirroring ADR-077's existing Tag-sync posture (see the `tag_writeback_sync.go` doc
  comment: a *read* failure before any enqueue aborts the whole batch, since nothing has been
  committed yet; once decisions are set and jobs are enqueued, each video's writeback job
  succeeds or fails independently and is reported independently). The new mechanism must decide
  where "setting the decision" sits relative to that boundary — this is exactly the open
  engineering question the pending ADR needs to resolve (see Open Questions).
- The shared Studio affordance component's `decide` callback shape differs by caller: Media's is
  `(source, manualValue) => Promise<{ok:true} | {conflict}>` for one video; Film's cascade has no
  single conflict to resolve inline (conflicts land per-video in the batch result, not as a
  blocking `verdict` snippet). The component needs a caller-selectable commit path — inline
  single-decide vs. fire-cascade-and-show-batch-dialog — rather than forcing Film through the
  single-video `decide` contract.

## API

- **Existing, unchanged**: `PUT /media/{id}/fields/studio/decision` (Media's single-video path).
- **New**: `POST /films/{id}/studio/cascade` (ADR-086 D3) — owner-gated, accepts a studio
  selection (existing entity ID, or a name to create), sets a manual studio decision on every
  video in `film_videos` for that film via ADR-086 D1's shared `decideStudioForVideo`, enqueues a
  writeback job for each video whose decision succeeded under one batch ID (ADR-086 D2), and
  returns `{batch_id, results: [{video_id, status, conflict?, error?}]}` synchronously — the
  per-video `results` array covers the decision-set phase (fast, synchronous DB work); `batch_id`
  feeds the existing `api.writebackBatchStatus` polling for the writeback phase, following the
  `POST .../writeback/sync` → `{batch_id}` → poll shape from ADR-077 D3 unchanged.

## UI

Deferred to `/design-handoff` — placement/copy for the Film-side trigger, and the shared
picker's `PickerShell` popover treatment on Film (candidate chips make less sense as
"videos currently share this studio" vs. Media's simpler "known values for this field").

## Success Metrics

This is an internal-tool, single-owner feature — no adoption/retention metrics apply. Success is
binary: an owner can correct a film's studio in one action instead of N, and the resulting state
(decisions + files) is verifiably consistent across all attached videos afterward.

## Open Questions

*(Resolved by [ADR-086](../architecture/ADR-086-film-studio-cascade-decide-and-writeback.md):
failure-boundary semantics are best-effort-per-video, not all-or-nothing — a decision-set failure
for video K excludes only K's writeback while videos K+1..N proceed, since each video's decision
is already an independent commit by the time a later one might fail. Endpoint is Film-scoped for
v1, `POST /films/{id}/studio/cascade`, not a general N-video primitive.)*

- **[engineering]** Should the shared Studio affordance become a genuinely single component with
  a caller-selectable commit strategy (single-decide vs. cascade-and-batch-dialog), or a shared
  *presentational* shell (`StudioPicker`'s chips/search/create UI) wrapped by two thin
  page-specific callers? Route through `/design-handoff` and initial implementation together —
  affects the component seam more than the product behavior.
- **[design]** Visual spec for the Film-side trigger and the cascade confirmation/progress UI —
  pending `/design-handoff`.

## Timeline / routing

No hard deadline. Per this repo's change-routing rules, before implementation begins: `/architecture`
(new ADR for the decide-then-enqueue mechanism), `/design-handoff` (shared affordance + Film
trigger), `/testing-strategy` (this is a multi-file behavior change), and `/security-review`
(new owner-gated bulk-mutation endpoint) — all as gates on a Draft PR (ADR-069) opened once the
first of these lands, marked ready for review only once all four are green.
