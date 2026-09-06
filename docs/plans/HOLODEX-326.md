---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-326                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — a film scene's number can now be corrected in place (film detail page and media detail page) instead of requiring detach + reattach.
---

# HOLODEX-326 · Film/Media detail pages: no way to edit a scene number after attach

Bug fix: F56 (HOLODEX-279) shipped attach/detach for film↔video scene numbers but never an
in-place edit — the only way to correct a scene number was detach + reattach, and both the film
detail page's scenes grid and the media detail page's Films chip badge were read-only. Added
`PATCH /films/{filmId}/videos/{videoId}` + `internal/repo/films.go`'s `UpdateFilmVideoScene`
(reusing `insertFilmVideo`'s existing `filmSceneOccupant` collision check — same 409-naming-the-
occupant rule as attach, except re-setting a video's own current number is a no-op, not a
collision), and a shared `EditSceneNumberDialog` mounted from both pages, opened by turning the
existing scene-number badge into a real `<button>`.

**Design package:** small addendum to `docs/design/films-entity-handoff.md` (§8, plus notes in
§2c/§3a) — conforms to the already-documented F56 attach/detach model, no new architecture.

## Gates — definition of done

- [~] spec `write-spec` — not applicable; conforms to existing `docs/design/films-entity-handoff.md`, no requirement change beyond closing an attach/detach gap
- [~] architecture `architecture` — not applicable; no data-model/seam change, reuses `filmSceneOccupant`'s existing collision rule verbatim
- [x] design `design-handoff` — §8 added (shared dialog spec), §2c/§3a updated, QA checklist items 2.5/3.11 added
- [x] backend
- [x] frontend
- [x] testing `testing-strategy` — new repo/API test coverage (`TestUpdateFilmVideoScene`) + a new adversarial Given/When/Then scenario (self-number re-save is not a collision)
- [x] security `security-review` — new owner-gated mutation route; reviewed, no findings (PATCH mounted in the same `requireOwner` group as existing attach/detach/bulk-attach; `film_id`/`video_id` bound exclusively from validated URL params; parameterized queries throughout)

## Up next — ordered (position = priority)

1. [x] [backend] `UpdateFilmVideoScene` repo method + `updateFilmVideoScene` handler + PATCH route — `internal/repo/films.go`, `internal/api/film_videos.go`
2. [x] [frontend] `EditSceneNumberDialog` + `sceneNumber.ts` shared validator, wired into both detail pages, `VideoCard`/`VideoGrid`'s `onEditScene` threading — `web/src/lib/components/film/`, `web/src/lib/components/video/`, both `+page.svelte` routes
3. [x] [testing] `TestUpdateFilmVideoScene` (happy path, self-number no-op, collision, clear-to-unnumbered, 404, 401/403) — `internal/api/films_test.go`
4. [x] [—] `/simplify` (extracted shared `parseSceneNumberInput`, switched films/[id]'s save handler from a full reload to a local array patch, consolidated the button/span badge markup via `svelte:element`, fixed a stale comment) + `/security-review` (clean) + live 3-skin QA
5. [ ] [—] push, open PR, sync Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-06 · Implemented, tested, and live-verified both edit surfaces
- skills: simplify, security-review
- handoff: filed HOLODEX-326 against epic HOLODEX-279 (F56), renamed branch, fired In Progress.
  Backend: `PATCH /films/{filmId}/videos/{videoId}` reuses `filmSceneOccupant` (already shared by
  `insertFilmVideo`), with the occupant-is-self case treated as a no-op success rather than a
  409 — new `TestUpdateFilmVideoScene` covers all five branches plus the auth gate. Frontend:
  `EditSceneNumberDialog` (shared by the film detail page's scenes grid and the media detail
  page's Films chip row) reuses `ConfirmDialog` chrome and the same inline-error-string collision
  convention `FilmAttachDialog` already uses. `VideoCard.svelte`'s scene-number badge moved to a
  sibling of the card's `<a>` (was nested inside it) so the new owner-only edit affordance can be
  a real `<button>` without a nested-interactive-element violation — hit and fixed the same
  `bind:value`-on-`type="number"` coercion gotcha `FilmAttachDialog`/`FilmBulkAttachDialog` had
  already documented (`.trim()` on a value that's sometimes a `number` at runtime). `/simplify`'s
  four review agents found: (1) the two dialogs' scene-number validation was now duplicated —
  extracted `web/src/lib/components/film/sceneNumber.ts`'s `parseSceneNumberInput`, used by both;
  (2) films/[id]'s save handler did a full `reloadDetail()` where a local array patch (already
  used by the media page's equivalent handler) is sufficient since the scenes list's sort is
  itself `$derived` — switched to match; (3) a stale `('' = none open)` comment — fixed to
  `null`; (4) VideoCard's button/span badge branches were near-duplicate markup — consolidated
  via `svelte:element` (needed an explicit `role`/`type` on the dynamic tag to satisfy both
  svelte-check's a11y linter and TypeScript's widened `string | number` type for the bound
  value). Declined one altitude finding (generalize `VideoCard`'s `onEditScene` into a generic
  badge-snippet slot to keep Films vocabulary out of the shared component) as out of scope — the
  component already carries Films-specific `sceneNumber` vocabulary from F56, so this doesn't
  compound a previously-clean primitive. `/security-review`: no findings — PATCH inherits the
  same `requireOwner` group as POST/DELETE on the same router, ids are URL-bound only, all SQL
  parameterized. Live-verified against `backend-films` (real "Dune" scenes): renumber from both
  pages, self-number no-op, 409 collision naming the correct occupant, badge/button rendering
  correct across Cinémathèque/Broadcast/Brutalist. `go build`/`go test ./...` and
  `npm run check`/`npm run test` all pass. Next: push, open PR, sync Jira status.
