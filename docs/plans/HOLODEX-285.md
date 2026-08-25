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
- [ ] backend
- [ ] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (§4 two rows, §5 one row, §10 adversarial block, §11 gap entry)
- [x] security `security-review` → no code in diff yet (docs-only); ADR-086 Action Item 7 checklist carried forward for the implementation PR

## Up next — ordered (position = priority)

1. [ ] [backend] extract `decideStudioForVideo` (ADR-086 D1) — `internal/api/decisions.go`
2. [ ] [backend] `VideoIDsForFilm` repo func + `cascadeFilmStudio` (ADR-086 D2) — `internal/repo/films.go`, `internal/api/film_studio_cascade.go` (new)  ⛔ blocked on #1
3. [ ] [backend] mount `POST /films/{id}/studio/cascade` (ADR-086 D3)  ⛔ blocked on #2
4. [ ] [frontend] `FilmStudioCascadeDialog.svelte` (handoff §3) — `web/src/lib/components/film/`  ⛔ blocked on #3
5. [ ] [frontend] `autostart` prop on `WritebackBatchDialog` (handoff §4) — `web/src/lib/components/writeback/WritebackBatchDialog.svelte`
6. [ ] [frontend] Film page Studios row pencil (handoff §2) — `web/src/routes/films/[id]/+page.svelte`  ⛔ blocked on #4, #5

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-25 · spec, ADR, and design handoff landed
- skills: write-spec, architecture, design-handoff, security-review
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
