---
key: HOLODEX-567
status: in-progress
profile: feature
depends-on: []
release_note: "Date added now lists files in the order they arrived, so editing or refreshing a file no longer moves it to the top. A new Updated sort (newest or oldest) shows the files that actually changed on disk."
approved:
  design:
    on: 2026-10-12
    at: 3740a807
---

# HOLODEX-567 · Separate "Date added" from a new "Updated" sort pair on the Media page

Done means "Date added" sorts by arrival order (`id`) and never moves when a file is edited, refreshed
or re-read; a public "Updated — newest / oldest" pair sorts by when the file itself last changed; and
`indexed_at` bumps only when the file's size or modified time changed. Production showed 49 of the
first 50 under "Date added — newest" were re-indexed files, not new ones
(`scripts/detect_indexed_at_drift.sql`). Built in parallel with HOLODEX-568 (move detection) under two
agreements: the oldest row's id survives a move, and a move never touches `indexed_at`.

**Design package:** [spec phase-1 F3.4](../specs/phase-1-mvp.md) · [spec phase-2 F12.1, F12.3, F12.7, F12.8](../specs/phase-2-mcp-polish.md#f12-browse-ui-polish) · [handoff](../design/media-sort-updated-handoff.md) + [mockup](../design/media-sort-updated-mockup.svg). No architecture doc: no technology fork (no new column or storage), and the `feature` posture carries no architecture gate.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — phase-1 F3.4; phase-2 F12.1, F12.3, F12.7, F12.8 edited in place
- [x] design `design-handoff` → `docs/design/**` — `media-sort-updated-handoff.md` + SVG mockup
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [backend] Date added sorts by `id DESC` / `id ASC`; add `updated_desc` / `updated_asc` on `indexed_at` with `id` as the tiebreak; accept both in `repo.ValidSort` (playlists) — `internal/repo/repo.go`
2. [ ] [backend] `UpsertVideo` bumps `indexed_at` only when size or modified time changed (manual refresh, file_tags backfill and re-extract of an unchanged file must not bump it) — `internal/repo/repo.go`, `internal/scanner/refresh.go`
3. [ ] [frontend] Add "Updated — newest" / "Updated — oldest" after "Date added — oldest" in `MEDIA_SORTS`, plus the `SortOrder` / `PlaylistSort` types — `web/src/lib/filters.ts`, `web/src/lib/types.ts`
4. [ ] [testing] Sort-order tests (id order, Updated order, stable ties), a no-op refresh leaves `indexed_at` alone, a writeback bumps it; update `docs/testing-strategy.md`
5. [ ] [—] HOLODEX-568 will edit F12.7 / F12.8 in place to drop the "until HOLODEX-568" clauses once this merges; nothing to do here

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · brainstorm, probe, spec, design
- skills: product-brainstorming, write-spec, design-handoff, code-review high --fix, implement
- shipped: drift probe `scripts/detect_indexed_at_drift.sql` (f0ab1aae); spec edits (ebb00d36); design handoff + mockup (3740a807). Spun out HOLODEX-568 (Relates). Worklog written by hand at the owner's request: the branch was renamed mid-session, so SessionStart never scaffolded it.
- handoff: Crossed into build — design signed off at 3740a807; draft PR open. Start at the backend sort keys (Up next 1).
