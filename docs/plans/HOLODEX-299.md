---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-299                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — the film→video bulk attach dialog no longer renders an empty candidate list.
---

# HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list

Bug fix: `GET /films/{id}/video-candidates` serialized `already_attached` as `null` (a Go map
miss) for any video with no attachments elsewhere — the common default-scope case.
`FilmBulkAttachDialog.svelte`'s row template called `.filter()` on it unconditionally, which
throws on `null` and silently blanked the whole `<ul>` even though the "N candidates" count
above it (computed separately) was correct. Fixed by normalizing the field to `[]` on the wire
in `internal/api/film_videos.go`, matching its existing TS type contract
(`already_attached: FilmVideoCandidate[]`, never `| null`). Also added a poster thumbnail to
each candidate row (previously text-only), matching the visual vocabulary of other video-row
lists (`CompletenessQueueRow.svelte`).

**Design package:** none (bug fix, no spec/ADR/design churn) · found while visually verifying
HOLODEX-298 · verified live against `backend-films`/`web` dev servers + `npm run check` +
`go test ./internal/api/...`

## Gates — definition of done

- [~] spec `write-spec` — not applicable; no requirement/scope change
- [~] architecture `architecture` — not applicable; no data-model/seam change
- [~] design `design-handoff` — not applicable; no new UX, just fixing a broken existing one
- [x] frontend
- [x] testing — added a regression assertion (`internal/api/film_candidates_test.go`) that the
  wire response never contains `"already_attached":null`
- [~] security `security-review` — not applicable; no auth/access/infra touched

## Up next — ordered (position = priority)

1. [x] [backend] normalize `AlreadyAttached` to `[]repo.FilmAttachment{}` instead of `nil` — `internal/api/film_videos.go`
2. [x] [frontend] add poster thumbnail to candidate rows — `web/src/lib/components/film/FilmBulkAttachDialog.svelte`
3. [x] [testing] regression assertion against a `null` wire value — `internal/api/film_candidates_test.go`
4. [x] [—] push, open PR, sync Jira — [PR #279](https://github.com/whoiskevinrich/holodex/pull/279)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-30 · Fixed empty candidate list in the film→video bulk attach dialog
- handoff: found while visually verifying an unrelated HOLODEX-298 change — opening the "Attach
  videos…" dialog showed "3 candidates" but rendered zero rows. Traced to
  `attachedByVideo[v.ID]` returning Go's zero value (`nil`) for any video not attached to another
  film, which JSON-serializes as `null`; the frontend's `c.already_attached.filter(...)` throws
  on that, silently blanking the `{#each}` block while the sibling count text (computed from
  `results.length`, a separate reactive read) kept showing the right number. Fixed at the source
  by normalizing to `[]`. Also added a thumbnail per row since the list was otherwise text-only.
  Filed as HOLODEX-299 (bug) rather than folding into HOLODEX-298 since it's an unrelated defect;
  moved the fix off the HOLODEX-298 worktree branch onto its own branch before committing.
  Live-verified in the browser: dialog now lists all 8 unattached candidates with thumbnails.
  `npm run check`: 0 errors. `go test ./internal/api/...`: all pass. Next: push, open PR, sync
  Jira.
