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

**Design package:** [spec](../specs/film-studio-cascade-writeback.md) · [ADR-086](../architecture/ADR-086-film-studio-cascade-decide-and-writeback.md) · [handoff](../design/film-studio-cascade-writeback-handoff.md) · testing-strategy §pending

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/film-studio-cascade-writeback.md`
- [x] architecture `architecture` → `docs/architecture/ADR-086-film-studio-cascade-decide-and-writeback.md`
- [x] design `design-handoff` → `docs/design/film-studio-cascade-writeback-handoff.md`
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [—] `/testing-strategy` — test plan for `decideStudioForVideo`, `cascadeFilmStudio`, and the new frontend results list
2. [ ] [—] `/security-review` — new owner-gated bulk-mutation endpoint (`POST /films/{id}/studio/cascade`)
3. [ ] [backend] extract `decideStudioForVideo` (ADR-086 D1) — `internal/api/decisions.go`
4. [ ] [backend] `VideoIDsForFilm` repo func + `cascadeFilmStudio` (ADR-086 D2) — `internal/repo/films.go`, `internal/api/film_studio_cascade.go` (new)  ⛔ blocked on #3
5. [ ] [backend] mount `POST /films/{id}/studio/cascade` (ADR-086 D3)  ⛔ blocked on #4
6. [ ] [frontend] `FilmStudioCascadeDialog.svelte` (handoff §3) — `web/src/lib/components/film/`  ⛔ blocked on #5
7. [ ] [frontend] `autostart` prop on `WritebackBatchDialog` (handoff §4) — `web/src/lib/components/writeback/WritebackBatchDialog.svelte`
8. [ ] [frontend] Film page Studios row pencil (handoff §2) — `web/src/routes/films/[id]/+page.svelte`  ⛔ blocked on #6, #7

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-25 · spec, ADR, and design handoff landed
- skills: write-spec, architecture, design-handoff
- handoff: three of four pre-implementation gates are green (spec, ADR-086, design handoff all
  merged into this session's docs). Next session should run `/testing-strategy`, then
  `/security-review` — no implementation code should be written until both land, per ADR-069.
