---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-285
status: in-progress
depends-on: []
release_note: Owners can now change a film's studio once and have it cascade to every attached video's decision and file.
---

# HOLODEX-285 · Unified Studio edit affordance + Film-level cascade writeback

Done means: the Film detail page's Studios row has the same owner-gated docked-pencil edit
affordance as the Media page (RD1), and setting a studio there sets a new manual decision plus a
file writeback across every video attached to the film in one action (RD2-RD4), reusing the
ADR-077 write-queue/batch-status mechanism for progress.

**Design package:** [spec](../specs/film-studio-cascade-writeback.md) · [ADR-086](../architecture/ADR-086-film-studio-cascade-decide-and-writeback.md) · [handoff](../design/film-studio-cascade-writeback-handoff.md) · [testing-strategy](../testing-strategy.md#4-backend-strategy-by-component)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/film-studio-cascade-writeback.md`
- [x] architecture `architecture` → `docs/architecture/ADR-086-film-studio-cascade-decide-and-writeback.md`
- [x] design `design-handoff` → `docs/design/film-studio-cascade-writeback-handoff.md`
- [x] backend
- [x] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (§4 two rows, §5 one row, §10 adversarial block, §11 gap entry)
- [x] security `security-review` → no code in diff yet (docs-only); ADR-086 Action Item 7 checklist carried forward for the implementation PR

## Up next — ordered (position = priority)

1. [x] [backend] extract `decideStudioForVideo` (ADR-086 D1) — `internal/api/decisions.go`
2. [x] [backend] `VideoIDsForFilm` repo func + `cascadeFilmStudio` (ADR-086 D2) — `internal/repo/films.go`, `internal/api/film_studio_cascade.go` (new)
3. [x] [backend] mount `POST /films/{id}/studio/cascade` (ADR-086 D3)
4. [x] [frontend] `FilmStudioCascadeDialog.svelte` (handoff §3) — `web/src/lib/components/film/`
5. [x] [frontend] `initialBatch` prop on `WritebackBatchDialog` (handoff §4) — `web/src/lib/components/writeback/WritebackBatchDialog.svelte` (shipped as `autostart` first, then reshaped into `initialBatch` per the `/simplify` altitude finding below — see session log)
6. [x] [frontend] Film page Studios row pencil (handoff §2) — `web/src/routes/films/[id]/+page.svelte`

All six implementation items are done. Nothing left in this list — the epic is implementation-complete pending final PR review.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-25 · spec, ADR, and design handoff landed
- skills: write-spec, architecture, design-handoff, security-review, simplify
- handoff: three of four pre-implementation gates are green (spec, ADR-086, design handoff all
  merged into this session's docs). Next session should run `/testing-strategy`, then
  `/security-review` — no implementation code should be written until both land, per ADR-069.

### 2026-08-25 · testing-strategy landed; mockup persistence established as a standing rule
- skills: testing-strategy
- also: persisted this epic's design-handoff mockup as a committed SVG
  (`docs/design/film-studio-cascade-writeback-mockup.svg`) instead of leaving it as an ephemeral
  `show_widget` artifact, per the owner's explicit ask this session — and encoded that as a
  standing rule in `.claude/CLAUDE.md` for every future `/design-handoff` in this repo.
- handoff: four of seven gates are now green (spec, architecture, design, testing). Next session
  should run `/security-review` — no implementation code should be written until it lands, per
  ADR-069.

### 2026-08-25 · security-review landed — all five pre-implementation gates green
- skills: security-review
- also: reviewed the branch diff (docs-only — no backend/frontend code exists yet) and found no
  findings in-diff; carried ADR-086 Action Item 7's checklist (owner-gating, parameterized query,
  zero-video no-op, error-string exposure, hardcoded `override=false`, `batch_id` entropy) forward
  as the review checklist for whichever pass covers the implementation PR.
- handoff: all five pre-implementation gates (spec, architecture, design, testing, security) are
  now green. Implementation (`Up next` #1-6) is unblocked per ADR-069, but should only start once
  the owner explicitly asks — do not start coding proactively. Once implementation lands and this
  PR is ready, mark it ready for review (drops Draft) to fire Jira's In Review transition.

### 2026-08-25 · implementation landed — all seven gates green
- skills: simplify
- backend: extracted `decideStudioForVideo` out of `setFieldDecision`'s Studio branch (ADR-086
  D1, no single-video behavior change), added `VideoIDsForFilm` + `cascadeFilmStudio`'s
  best-effort per-video decide-then-enqueue loop (D2), mounted `POST
  /films/{id}/studio/cascade` (D3). Five new Go tests in `internal/api/film_studio_cascade_test.go`
  (partial-collision best-effort, same-value redecide, all-collide/zero-video no-ops, owner
  gating) — all pass.
- frontend: `FilmStudioCascadeDialog.svelte` (picker step mirroring `StudioPicker`, results step
  with Enqueued/Collision/Error `<details>` groups), the Film page's owner-gated Studios-row
  pencil, and a way for `WritebackBatchDialog` to show progress for a batch a caller already
  started server-side.
- also: live-QA'd the full flow in the browser against `backend-films` (with a local-only
  `FILMS_ENABLED=true` added to `.claude/launch.json` — untracked, not part of this PR) and found
  + fixed one real bug (the first `autostart` cut set the initial phase to `'starting'` but never
  actually called `start()` — the dialog hung on a disabled "Starting…" button). Confirmed the
  writeback job that looked stalled during QA (`pending` stuck at 1 for several minutes) was not a
  bug: `job_runs` shows both real cascade batches against the local 4K "Dune" test file completed
  successfully in ~3m15s each — genuinely slow real `exiftool`/`mkvpropedit` I/O, not a stuck
  worker. Confirmed via `writeback_queue`/`job_runs` row inspection, not guessed.
- also: ran `/simplify` on the full diff (4 parallel review angles). Reuse/efficiency passes found
  nothing actionable. One simplification finding applied: `attachedVideoCount` on the Film page was
  a redundant `Set`-union of the same arrays `videoTitles` already maps — replaced with
  `videoTitles.size`. One altitude finding applied: the `autostart` prop's caller-side `trigger={async
  () => cascadePending as {...}}` was faking a "start" call around already-known data (forced
  `as` cast, `phase='starting'` firing for a batch that wasn't starting). Replaced `autostart`
  with a real `initialBatch: {batch_id, enqueued}` prop that seeds `phase='progress'` directly and
  polls via a shared `watch()` helper, without going through `trigger()` at all for that caller.
  Re-verified live in the browser post-refactor: pencil → picker → results → "View writeback
  progress →" still lands directly in `'progress'` phase with no confirm/starting flash.
- also: 3-skin QA via computed-style contrast checks (this repo's established technique — browser
  screenshots aren't reliable here) on the new pencil affordance: Brutalist 5.73:1, Cinémathèque
  6.31:1, Broadcast 4.90:1 against body background — all comfortably above the 3:1 WCAG AA bar
  for UI components. `npm run check`: 0 errors (pre-existing unrelated a11y warnings only, plus
  one expected `state_referenced_locally` warning on `initialBatch`'s one-time initial-value read,
  same pattern the removed `autostart` prop already had).
- handoff: implementation is complete, all seven gates green, backend tests and frontend
  type-check both pass. Next step is Jira sync + push + marking Draft PR #254 ready for review.
  Not yet exercised live: a real Collision-scenario render in the browser UI specifically (the
  backend path is covered by
  `TestCascadeFilmStudio_PartialCollision_BestEffort`, and the Collision/Error `<details>`
  rendering was verified by direct source read, not a live browser trigger) — worth a follow-up
  spot-check if a reviewer wants to see it rendered, not considered blocking since the same
  component code renders both branches from the same `results` array already exercised for the
  Enqueued branch.
