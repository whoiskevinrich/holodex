---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-272
status: ready-for-review
depends-on: [HOLODEX-270]
release_note: Adding or removing a person on a video now goes through a picker on the video detail page — known candidates, full-library search, or an inline create-fallback — protected by the same duplicate-video safeguard as a title or studio change.
---

# HOLODEX-272 · People attach/detach + relationship picker (F56.5)

Done means the video detail page's People grid gets add/remove affordance via a new
`PersonPicker` popover (known-candidate chips, full-library search, inline create-fallback,
required Actor/Director role choice), committing through the existing curation model
(`POST /media/{id}/curation`, `action=add`/`suppress` on `actors`/`director` — the same mechanism
`CurationFieldRow.svelte` already uses in production; not writing `video_people` directly, and not
`SetDecision` either, since `actors`/`director` are `multi`/`merge` fields that path structurally
rejects — see spec § Resolved Decisions, corrected during backend implementation), and every
attach/detach runs through the HOLODEX-270 composite-key collision gate exactly as Title and Studio
already do. This is the last open story under epic HOLODEX-267.

**Design package:** [spec](../specs/people-relationship-picker.md) ·
[design handoff](../design/people-relationship-picker-handoff.md) · testing-strategy §5: written
(see below)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/people-relationship-picker.md`
- [~] architecture — not needed (resolved in spec § Resolved Decisions: attach/detach extends
      ADR-051/ADR-072 through the existing field-decision model rather than writing `video_people`
      directly, so no new persistence/API shape is introduced)
- [x] design `design-handoff` → `docs/design/people-relationship-picker-handoff.md`
- [x] backend — `internal/repo/video_collision.go` (`FindPeopleCollision`, sibling to
      `FindTitleCollision`/`FindStudioCollision`), wired into the existing curation add/suppress
      endpoint (`internal/repo/curation.go` `SetCurationChecked`, `internal/api/curation.go`) with
      the same 409+override gate — no new endpoint (see spec's corrected Resolved Decision #1)
- [x] frontend — `PersonPicker.svelte` (new, multi-select — `StudioPicker`/`EntityPickerDialog`
      are both single-select and not directly reusable), People-grid empty-state guard fix,
      hover-reveal remove control on `PersonPoster` call sites
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §5
- [x] security `security-review` — zero findings; `POST /media/{id}/curation` stays inside the
      same `requireOwner` group as every other mutation, `FindPeopleCollision` and the new
      `curation.go`/`repo.go` changes are fully parameterized, no unsafe Svelte sink introduced

## Up next — ordered (position = priority)

1. [x] [backend] `FindPeopleCollision` + collision gate on the existing curation endpoint
2. [x] [frontend] `PersonPicker.svelte` + People-grid wiring (empty-state guard, remove control)
3. [x] [testing] `docs/testing-strategy.md` §5 row
4. [x] [security] `/security-review` — new mutation surface, required before Draft → ready

All gates green — mark [PR #235](https://github.com/whoiskevinrich/holodex/pull/235) ready for
review (out of Draft) in the same push as this update.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-12 · PR #235 code-review findings applied
- skills: code-review (xhigh), simplify
- handoff: Ran the xhigh-effort `/code-review` on PR #235 (5+5 angles × 8 candidates, 1-vote
  verify, sweep) and got 15 findings. Applied all with minimal edits: the two real correctness
  bugs in the People collision gate itself — `proposedPeopleNames` (`internal/api/curation.go`)
  ignored `role`, so a dual-role suppress couldn't tell which `video_people` row was being
  removed and silently missed a collision; and it read `PeopleForVideos` unlocked before
  `writeMu`, letting two concurrent same-video commits race past the check — now recomputed
  inside the `writeMu`-locked `check()` closure. Root-caused both to `PeopleForVideos`
  (`internal/repo/repo.go`) dropping the `role` column entirely; added it. Fixed four frontend
  correctness bugs: `CurationFieldRow.svelte`'s `run()` silently swallowed a 409 `{conflict}`
  response instead of erroring; a stale `CollisionOfferCard` could resubmit a forgotten pending
  action after an unrelated commit; `PersonPicker`'s role toggle could resubmit an already-taken
  role within one session; and the page's `personBusyKey` and the picker's local `busyKey` were
  independent locks that could race on the same video — unified via a `$bindable` prop, following
  `PickerShell`'s existing `dialogEl` precedent. Deduped: extracted `sendConflictable` in `api.ts`
  (shared 409-parsing fetch wrapper for `curateMedia`/`setFieldDecision`), `personKey` in
  `format.ts` (was duplicated verbatim in `PersonPicker.svelte` and `+page.svelte`), a `roleToggle`
  Svelte snippet (was copy-pasted between the search-candidate and create-new rows), and removed
  `PersonPicker`'s dead `addTile` binding (redundant with `PickerShell`'s own focus-restore). On
  inspection, the reviewer's "triplicated ~30-40 line shell" finding for
  `Find{Title,Studio,People}Collision` (`internal/repo/video_collision.go`) overstated the gap —
  the actual scan/hydrate loop was already shared via `scanCollisionCandidates`/
  `compositeKeyCandidates`/`hydrateCollision`; what remained was one identical title+recorded_at
  query block duplicated between `FindStudioCollision` and `FindPeopleCollision`, extracted into
  `titleAndRecordedAtOf`. Skipped 3 findings as real-but-not-worth-it under a minimal-edits
  mandate: `FindPeopleCollision`'s redundant re-query of title/recorded_at (the caller already has
  the video via `GetVideo`, but fixing it needs a signature change or diverges from
  `FindStudioCollision`'s identical existing pattern); sharing `PersonPicker`'s debounced-
  search/keyboard-nav logic with `StudioPicker` (would require touching a previously-shipped,
  out-of-scope HOLODEX-271 component); and `curatePerson`'s full `reloadDetail()` after every
  attach/detach (confirmed this matches the page's existing full-reload-per-mutation pattern at 16
  other call sites — not a regression this PR introduced). Verified clean: `go build ./...`,
  `go test ./internal/repo/... ./internal/api/...`, `npm run check` (0 errors), `npm run test`
  (139/139). No gate changed — PR #235 was already ready for review; this just closes the review
  loop on it in the same push.

### 2026-08-12 · Frontend QA, testing-strategy, security-review — all gates green
- skills: testing-strategy, security-review
- handoff: `PersonPicker.svelte` + the People-grid wiring (`Person.Role`, repo query, types.ts,
  `+page.svelte`) had already been written and unit-verified (`go build`/`go test`/`npm run
  check`/`npm run test`) in the prior session; this session's job was live driven-browser QA
  against a real backend+frontend instance. Spent significant time on an environment problem
  before any QA could start: this session's Browser-preview tooling (`preview_start`) was bound
  to an unrelated, stale worktree (`heuristic-golick-1459ef`, still on the already-merged
  HOLODEX-268 branch) rather than this one — confirmed via `preview_list`'s `cwd` field, root-
  caused via `netstat`/process inspection after ruling out a stale Vite cache. Worked around it
  by stopping the wrongly-rooted servers and launching `go run`/`npm run dev` directly via Bash
  (correctly scoped to this worktree) with `run_in_background: true`, using the Browser pane only
  to drive `localhost` over HTTP. Also hit and fixed, incidentally: `MappedFacets.svelte` crashed
  on every route (breaking SvelteKit hydration app-wide, not just the video page) because
  `/api/v1/facets` marshals a Go nil slice as JSON `null` for a zero-value facet — same bug class
  as the HOLODEX-270 `VideoCollision.People` nil-slice fix. Patched with a one-line
  `facet.values?.length` guard to unblock QA; filed the backend-side root cause as a separate
  follow-up rather than fixing it here (out of this story's scope). With the environment fixed,
  QA'd the full interaction set against a fresh, unenriched `E:\TestCopy-Film` database (zero
  pre-existing people): 2-char search threshold, no-match + create-fallback with the Actor/
  Director role toggle, immediate-commit-without-closing (multi-select, per the design's explicit
  non-goal), grid↔picker live sync in both directions, re-search-finds-the-real-person with the
  role toggle correctly excluding an already-held role, the "Already attached as Actor, Director"
  disabled no-op row, detach from both the picker's own chip and the grid's remove control, zero
  console errors throughout. 3-skin token check (Cinémathèque/Broadcast/Brutalist) via
  `javascript_tool` computed-style reads (screenshots time out in this environment) — all
  token-driven, no hardcoded colors. The 409/`CollisionOfferCard` path was not re-driven through
  the browser (forcing an exact `{Title, Date, Studio}` match in this fresh dataset would need
  real fixture seeding) — covered instead by the backend's `TestCurationAPI_PeopleCollision`/
  `_Suppress` plus `CollisionOfferCard` being the same already-QA'd component from the
  HOLODEX-270/271 rows, same posture Studio's own row already took. Wrote the `docs/
  testing-strategy.md` §5 row and a §11 gap bullet documenting all of the above. Ran
  `/security-review` against the full diff (backend collision gate + frontend picker): zero
  findings — `POST /media/{id}/curation` stays inside the existing `requireOwner` group,
  `FindPeopleCollision` and the new repo/curation.go changes are fully parameterized (no
  deviation from the `FindTitleCollision`/`FindStudioCollision` precedent), and `PersonPicker.svelte`
  uses only normal Svelte text interpolation (no `{@html}`). All seven gates are now green; PR
  #235 moves from Draft to ready for review in this same push.

### 2026-08-11 · Backend: FindPeopleCollision + collision gate on the curation endpoint
- skills: simplify
- handoff: Set out to implement "attach/detach endpoint constructing an `actors`/`director` manual
  decision" per the spec, and found the spec's own premise wrong: `actors`/`director` are `multi:
  true`/`merge` fields, and `internal/api/decisions.go`'s `replaceField` gate rejects any
  `f.Multi || f.Merge` field with a 400 — `SetDecision` structurally cannot write these fields at
  all. Traced the actual mechanism: `metadata_curation` (F30/ADR-048) via `SetCuration`/
  `action=add`/`suppress`, already wired to `relinkIfEntity`, already exposed today through
  `CurationFieldRow.svelte`'s generic Metadata-fields curation row — confirmed by
  `internal/api/curation.go`'s own pre-existing comment ("a link IS a curation add, ADR-072 RD1")
  and by ADR-072 Action Item 9 ("reuses the F30 curation endpoint; no new API"). So the real gap
  was narrower than spec'd: `POST /media/{id}/curation` had no composite-key collision check on a
  person-typed add/suppress. Closed exactly that: added `FindPeopleCollision`
  (`internal/repo/video_collision.go`, sibling to `FindTitleCollision`/`FindStudioCollision` —
  Title/Studio/Date held fixed, People the varying dimension compared by normalized name since a
  newly-created person has no id yet); `SetCurationChecked`/`setCurationLocked`
  (`internal/repo/curation.go`, mirroring `decisions.go`'s `SetDecisionChecked` split for the same
  writeMu-locked check-then-write atomicity); and the registry-driven gate in
  `internal/api/curation.go`'s `setCuration` handler (`registry.Lookup(field).EntityKind ==
  EntityKindPerson`, not a hardcoded field list) plus a new `Override bool` on `curationBody` and a
  `proposedPeopleNames` helper (People-flavored sibling of `decisions.go`'s
  `resolveProposedStudioNames`). Wrote `TestFindPeopleCollision`
  (`internal/repo/video_collision_test.go`) and three API-level tests
  (`internal/api/curation_collision_test.go`): attach-collision 409 + override-persists,
  suppress-collision 409 + override-persists, and a non-person field (`genres`) confirming the
  gate is registry-driven rather than hardcoded. Corrected the spec's Resolved Decision #1 and P0
  backend requirement to describe the actual mechanism, and ran `/simplify` on the diff before
  committing: 4 parallel review agents found 3 real duplication findings, all fixed — `curation.go`'s
  person-typed branch collapsed into the generic tail (a `check` closure is nil when the gate
  doesn't apply, same as override, so `SetCurationChecked`/`relinkIfEntity`/204 run once instead of
  twice); the test file's `sendCuration` helper deleted in favor of the existing `sendDecision`;
  `postCurationRaw` deleted in favor of generalizing `decisions_collision_test.go`'s
  `putDecisionRaw` into a method-parameterized `rawRequest`. Skipped 3 lower-value findings: a
  `peopleDecisionServer`/`decisionServer` test-setup dedup (would touch a shared helper used by
  every other decision test — outside this diff's radius), extracting a shared scan helper across
  Title/Studio/People's collision functions (both agents flagged low-confidence), and reusing a
  `relinkContext` across the collision check and post-commit relink the way Studio's
  `resolveProposedStudioNames`/`relinkStudiosWithContext` does — investigated it, but People's
  `ReconcileVideoPeople` needs role-tagged links across *both* `actors` and `director` (not just the
  field being edited), so mirroring the precedent properly is a real follow-up, not a same-pass fix;
  filed as HOLODEX-274. `go build ./...` clean; full `go test ./...` green (28 packages). Next:
  frontend (`PersonPicker.svelte` + People-grid wiring) — now genuinely simpler than spec'd, since
  it POSTs to an endpoint that already exists rather than a new one.

### 2026-08-11 · Design handoff written
- skills: design-handoff, visualize, simplify
- handoff: Read `StudioPicker.svelte`/`PickerShell.svelte`/`PersonPoster.svelte` and the current
  (broken) People-grid markup in full before designing. Built a grounded mockup (Holodex's actual
  Cinémathèque tokens, `PersonPoster`-shaped cards) comparing the two live design questions side
  by side: role-choice UI (Option A — a global Actor/Director segmented toggle above search — vs
  Option B — a per-result-row role pill defaulting to Actor) and the hover-reveal remove control
  on grid cards. Resolved to Option B: a video's people list routinely mixes both roles in one
  sitting, and B lets an owner add all of them in a single open/close cycle without toggling a
  global mode between searches. Wrote
  `docs/design/people-relationship-picker-handoff.md` (Layout, Design Tokens, Components, States
  and Interactions, Responsive, Edge Cases, Animation, Accessibility) grounded in the real
  component props/state, not invented ones. Next: backend (`FindPeopleCollision` + attach/detach
  endpoint).

### 2026-08-11 · Spec written, epic-status review led here
- skills: write-spec, graphify
- handoff: Investigated "what's left to close HOLODEX-267" and found HOLODEX-272 was the only
  open child story with zero code/branch/PR started despite being marked In Progress. Also
  confirmed HOLODEX-267's own `needs-design`/`needs-spec` labels are stale bookkeeping (every
  other child story shipped its own spec+design; no separate epic-level umbrella doc was ever
  intended) — worth clearing once this story's gates land. Set up a dedicated worktree
  (`HOLODEX-272-people-picker`, branch `worktree-HOLODEX-272-people-picker` — key substring is
  enough for Jira linkage, no rename needed; Jira status was already In Progress, no transition
  needed). Ran an Explore agent to ground the spec in current code before writing it: confirmed
  `video_people` is fully derived (F40/ADR-072, rebuilt from resolved `actors`/`director` field
  decisions on every relink, zero other writers), no `FindPeopleCollision` exists, and neither
  `StudioPicker` nor `EntityPickerDialog` supports multi-select. Surfaced the two load-bearing
  open questions (attach mechanism, role-on-attach) via AskUserQuestion before writing
  requirements — resolved to "extend the field-decision model" (no new ADR needed) and "require
  an Actor/Director role choice at attach time." Wrote
  `docs/specs/people-relationship-picker.md`. Next: design handoff for `PersonPicker.svelte`.
