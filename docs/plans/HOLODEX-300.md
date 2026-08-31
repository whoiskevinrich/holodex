---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-300                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — the film bulk-attach dialog now defaults its search to the film's title and attaches the selection unnumbered when no starting scene number is given.
---

# HOLODEX-300 · Film bulk-attach dialog: default search term + optional starting scene number

Bug fix: `FilmBulkAttachDialog.svelte` didn't match design handoff §4c on two points — the
search input started empty instead of seeded with the film's title, and the starting scene
number was required, blocking the documented "omitted means every selected video attaches
unnumbered" behavior. Threaded a nullable `*int64` starting scene number end-to-end
(Svelte state → `api.ts` → JSON body → Go handler → `BulkAttachFilmVideos`), reusing
`insertFilmVideo`'s existing nil-safe "unnumbered, never collides" handling rather than adding
new logic.

**Design package:** none (bug fix conforming to existing design handoff §4c, no spec/ADR/design churn)

## Gates — definition of done

- [~] spec `write-spec` — not applicable; conforms to existing `docs/design/films-entity-handoff.md` §4c, no requirement change
- [~] architecture `architecture` — not applicable; no data-model/seam change, reuses `insertFilmVideo`'s existing nil-scene handling verbatim
- [~] design `design-handoff` — not applicable; behavior already specified in §4c, no new markup/visual change
- [x] backend
- [x] frontend
- [x] testing `testing-strategy` — new repo- and API-layer test coverage for the nil/omitted-starting-scene-number path (mirrors existing `TestBulkAttachFilmVideos`/`TestFilmVideoSceneCollision`)
- [~] security `security-review` — not applicable; no auth/access/infra surface touched (existing owner-gated bulk-attach endpoint, only the scene-number field's nullability changed)

## Up next — ordered (position = priority)

1. [x] [backend] `BulkAttachFilmVideos` + `bulkAttachFilmVideos` handler accept nil starting scene number — `internal/repo/films.go`, `internal/api/film_videos.go`
2. [x] [frontend] seed search from `filmName`; blank starting-scene-number attaches unnumbered — `web/src/lib/components/film/FilmBulkAttachDialog.svelte`, `web/src/lib/api.ts`
3. [x] [testing] repo + API test coverage for the unnumbered path — `internal/repo/films_test.go`, `internal/api/films_test.go`
4. [x] [—] `/simplify` (reuse/simplification/efficiency/altitude, all clean save one non-blocking reuse note vs. `FilmAttachDialog.svelte`, skipped as premature abstraction) + live 3-skin QA
5. [x] [—] push, open PR (#280), sync Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-30 · Implemented, tested, and live-verified both fixes
- skills: simplify
- handoff: nullable `*int64` starting scene number threaded end-to-end; nil path reuses
  `insertFilmVideo`'s pre-existing "unnumbered, never collides" handling, so unnumbered
  attaches skip the per-video collision check entirely (a minor efficiency win, not just
  parity). Search input seeds from `filmName` prop. New Go tests
  `TestBulkAttachFilmVideosUnnumbered` (repo + API layers) cover the omitted-field path.
  `/simplify`'s four review agents came back clean except one non-blocking reuse note
  (scene-number blank-parsing duplicated between this dialog and `FilmAttachDialog.svelte`'s
  `confirm()`) — skipped as premature abstraction: the two call sites differ (this one has no
  `isFullFilm` interplay, different error copy) and the two dialogs are already documented as
  deliberately separate components. Live-verified against `backend-films`/Dune: default search
  populates "Dune", blank-scene attach produces unnumbered (`—`) scene cards, all three skins
  (Cinémathèque/Broadcast/Brutalist) render the changed elements with correct token contrast.
  `go test ./internal/repo/... ./internal/api/...` and `npm run check` both pass. Next: push,
  open PR, sync Jira.

### 2026-08-30 · `/code-review high --fix` caught and fixed a numbered-attach crash
- skills: code-review
- handoff: 8-angle review surfaced a real correctness bug the earlier `/simplify` pass and
  manual QA both missed (QA that session only exercised the blank/unnumbered path):
  `commit()`'s `startingSceneNumber.trim() !== ''` guard threw a `TypeError` as soon as a user
  typed a positive scene number, because Svelte's `bind:value` on the `type="number"` input
  coerces the bound value to a `Number`, not a string — `FilmAttachDialog.svelte`'s sibling
  `confirm()` already documents this exact gotcha and avoids `.trim()`. Fixed to check
  `startingSceneNumber !== ''` directly. Live re-verified against `backend-films`/Dune: typed
  scene `5`, attach succeeded (204), scene card rendered `#5` — no console error. Also added
  the `testing-strategy.md` update the conventions angle flagged as missing (the change-routing
  table's multi-file-change gate). One reuse finding (scene-number parsing duplicated with
  `FilmAttachDialog.svelte`) reconfirmed and again skipped as premature abstraction. All gates
  still green; pushed as a follow-up commit on the same PR (#280).
