# Video composite-key collision check (F56.3, HOLODEX-270)

Parent epic: [HOLODEX-267](https://whoiskevinrich.atlassian.net/browse/HOLODEX-267) — Entity
decision editing overhaul. Depends on [HOLODEX-269](https://whoiskevinrich.atlassian.net/browse/HOLODEX-269)
(unified name-edit mechanism — this story's Title trigger point plugs into `NameEditControl`'s
existing conflict slot) and on HOLODEX-272 (people add/remove on video — this story's People
trigger point plugs into that story's still-unbuilt attach/detach surface). Sibling: HOLODEX-271
(studio relationship popover — this story's Studio trigger point plugs into that story's
still-unbuilt picker).

## Problem Statement

A video's real-world identity is the combination of **{Title, People, Date, Studio}** — not the
title string alone. Two videos with the same title but different people or a different studio are
legitimately distinct (a rerelease, a different session). Two videos that end up sharing the full
composite key are usually a mistake — a duplicate import, or an owner editing one field on the
wrong video — but the resolver and scanner have no way to flag it today: every in-place edit
(Title via HOLODEX-269, Studio via HOLODEX-271, People via HOLODEX-272) commits immediately with
no check against the rest of the library. The cost of not solving this is silent, hard-to-notice
duplication that only surfaces later as visitor confusion ("why are there two identical entries?")
with no record of which edit caused it.

## Existing State (grounded in code, this session)

- **Schema.** `videos.title` and `videos.recorded_at` (ISO date text) are columns
  (`internal/db/migrations/0001_init.up.sql:8,12`). People and Studio are **not** columns — both
  are derived join tables (`video_people`, `0001_init.up.sql:33-38`; `video_studios`,
  `0017_studios.up.sql:15-19`, explicitly "no column on videos" per its own comment) reconciled
  from the *resolved* field decision, never edited directly
  (`internal/api/person_links.go:105-162` `RelinkVideoPeople`/`ReconcileVideoPeople`; the studio
  counterpart follows the same pattern).
- **The three trigger points, and where each stands today:**
  - *Title*: HOLODEX-269 wires `NameEditControl` into the video header, committing through
    `decideField('title', 'manual', value)` → `PUT /media/{id}/fields/title/decision`
    (`internal/api/decisions.go:38-83`). That code's own comment says Video "isn't on the identity
    spine... no verdict snippet, no conflict state" — this story fills that gap.
  - *Studio*: reassignment goes through the same `setFieldDecision` endpoint today (canonical=
    `studio`) via `SourceSelect`'s radiogroup; HOLODEX-271 replaces the picking UI but not the
    commit path, so this is the future Studio hook point.
  - *People*: **no owner-mutable attach/detach endpoint exists at all** — `video_people` is only
    ever a side effect of resolving `actors`/`director` fields. HOLODEX-272 is building the first
    real surface; that's the future People hook point.
- **What already exists that's adjacent but doesn't cover this.** The F43/ADR-061 Duplicates
  system (`internal/repo/identity.go`, `internal/api/duplicates.go`,
  `DuplicatePairRow.svelte`) only detects **entity-identity** collisions — two Person/Studio/Tag
  records with the same normalized name. It has no concept of a video's composite key, and its
  verb set (Merge / Keep separate) doesn't transfer: two video **files** can't be folded into one
  record. This story's verdict panel borrows the two-item comparison *layout* from
  `DuplicatePairRow`, not its backend query or its verbs.
- **Normalization precedent.** `internal/repo/identity.go:60-71` (`nameKeyExpr()`, SQL
  `lower(trim(col))`) and `internal/repo/curation.go:26` (`curationNorm()`, the Go-side
  equivalent) are the established pattern for name comparison; this story reuses that shape for
  Title comparison. Date comparison has no existing helper — `recorded_at` is ISO text, so exact
  string equality is sufficient and needs no new normalization logic.

## Goals

1. An owner attempting a Title edit (this story's only wired trigger) that would produce a
   composite-key collision with another active video sees a verdict panel before the edit commits
   — zero silent duplicate-key commits via the Title path.
2. The collision-check mechanism (query + response shape) is entity-agnostic enough that
   HOLODEX-271 and HOLODEX-272 can call it from their own trigger points later without backend
   rework — this story ships the shared machinery once, not three times.
3. An owner who intends to keep two videos that happen to share a composite key (distinct
   encodes/cuts of one session) can say so in one click, with no data loss and no re-prompt on the
   same pair.

## Non-Goals

- **Studio and People trigger-point UI** — out of scope here; no picker/attach-surface exists yet
  for either (HOLODEX-271, HOLODEX-272 build those and wire this story's already-shared backend
  check into their own commit paths).
- **A "merge" verb for colliding videos** — video files are not entity records; there is nothing
  to fold together. The verdict is binary: view the existing one, or keep both.
- **Retroactive scan of the existing library for composite-key collisions** — this story is a
  write-time gate, not a backfill/audit tool. A library-wide sweep is a plausible fast-follow but
  isn't needed to ship the gate itself.
- **Fuzzy/near-miss matching on the composite key** (e.g. titles that differ by punctuation only)
  — this story checks for *exact* normalized-key collisions only, mirroring the entity-identity
  spine's own exact-match-first posture (near-miss surfaces separately, on its own review queue,
  for entities — this story doesn't invent an equivalent for videos).

## User Stories

- As an owner renaming a video's Title, I want to be warned if another active video already has
  the same title, people, date, and studio, so I don't accidentally create a confusing duplicate
  entry.
- As an owner who gets the collision warning, I want to jump straight to the existing video to
  compare, so I can decide with full context instead of guessing from the summary shown.
- As an owner who confirms the two videos are legitimately distinct (e.g. two cuts of one
  session), I want to save my edit anyway in one click, with no further nagging on that same pair.
- As an owner, I want the check to only fire on a genuine exact-key collision, not on
  near-miss text, so I'm not interrupted for videos that are obviously different.

## Requirements

### Must-Have (P0)

- **Shared collision-check mechanism.** A single Go-level function/query, callable from the Title
  commit path today and from the Studio/People commit paths later, that takes a video's proposed
  composite key (title, studio_id, people_ids, recorded_at) and returns any other **active**
  video sharing the same normalized key. Acceptance: given two videos with identical normalized
  title/date and identical studio+people sets, the check returns the second as a collision of the
  first; given any one field differing, it returns no collision.
- **Title commit path blocks on collision.** `setFieldDecision` for canonical=`title` runs the
  check before persisting; on a hit, returns a 409-style response carrying the colliding video's
  id/title/people/date/studio (enough to render the verdict panel without a follow-up fetch) and
  does **not** persist the edit. Acceptance: editing a title into collision with another active
  video returns 409 and the original title is unchanged in the DB.
- **Verdict panel — two choices, no default destructive action.** Shown inline in
  `NameEditControl`'s existing conflict-snippet slot on the Video Title mount. "View existing
  video" navigates away and discards the pending edit (no commit). "Save anyway, keep both"
  re-submits the same edit with an explicit override flag, which persists it and does not
  re-trigger the check for that same pair. Acceptance: neither action can be triggered by a single
  accidental click with no confirmation; "Save anyway" always requires the owner to have already
  seen the collision info.
- **Exact-match only.** The check compares normalized-exact keys (case/whitespace-insensitive
  title, exact date, exact people-set, exact studio) — no fuzzy/near-miss logic. Acceptance: a
  one-character title difference does not trigger the panel.

### Nice-to-Have (P1)

- **Thumbnail in the verdict panel.** Shown alongside the colliding video's title/people/date/
  studio for faster visual disambiguation — deferred if it adds meaningful backend work beyond
  what the existing video-list thumbnail endpoint already provides.
- **Deep link from "View existing video" preserves return context** — after navigating away, an
  owner can get back to the edit-in-progress video without re-navigating from a list.

### Future Considerations (P2)

- Library-wide composite-key collision report (retroactive sweep), once real usage data shows
  whether write-time gating alone is sufficient or a backlog of pre-existing collisions needs
  surfacing too.
- Extending the "save anyway, don't re-ask" suppression to be visible/reversible from an owner
  settings surface, if repeat false-positive collisions turn out to be common for a given pair.

## Success Metrics

**Leading:** collision-panel trigger rate on Title edits (sampled weekly post-launch — a
personal-library-scale product, so this is a qualitative "did it fire when expected, not more"
check rather than a statistical target); zero reports of the panel blocking a title edit that
should have gone through cleanly.

**Lagging:** no new duplicate-looking video pairs traceable to a Title edit, verified by spot
check a month after launch.

## Open Questions

- **Real-world false-positive rate is unverified** (engineering, non-blocking). No live DB exists
  in this worktree to check whether two legitimately distinct videos commonly already share the
  full composite key (e.g. multi-part recordings from one session, which might share date+people
  +studio and differ only by a title suffix). Recommendation: ship the exact-match gate with the
  "save anyway" escape hatch as designed (P0 already assumes false positives will occur and
  handles them non-destructively), and run the sample query the research pass drafted
  (`GROUP BY normalized title/date/studio/people HAVING count > 1`) against a real `holodex.db`
  before or shortly after rollout to confirm the escape hatch stays rare rather than routine.
