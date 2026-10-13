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
- [x] backend → `{cmd,internal,providers}/**` — `orderBy` added = `id`, `updated_*` on `indexed_at`; `ValidSort` accepts both; `UpsertVideo` keeps `indexed_at` unless size or mtime changed; completeness ties fall to `id DESC` (spec F55.5 "the type's default order")
- [x] frontend → `web/src/**` — Updated pair in `MEDIA_SORTS` after Date added; owner confirmed the build matches the approved mockup (2026-10-12)
- [x] testing `testing-strategy` — `added_updated_sort_test.go` (mutation-checked), `listState.test.ts` S4, `docs/testing-strategy.md` §5 row; live check: an unchanged-file refresh on the AMV testbed kept `indexed_at`

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge PR #499 on green CI, then confirm CI moved HOLODEX-567 to Done
2. [ ] [—] HOLODEX-568 will edit F12.7 / F12.8 in place to drop the "until HOLODEX-568" clauses once this merges; nothing to do here

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · brainstorm, probe, spec, design
- skills: product-brainstorming, write-spec, design-handoff, code-review high --fix, implement, handoff
- shipped: drift probe `scripts/detect_indexed_at_drift.sql` (f0ab1aae); spec edits (ebb00d36); design handoff + mockup (3740a807); design sign-off (bdaf2188); build (842c78ed). Spun out HOLODEX-568 (Relates). Worklog written by hand at the owner's request: the branch was renamed mid-session, so SessionStart never scaffolded it.
- handoff: Every gate settled and the build matches the approved mockup; PR #499 marked ready. Start at squash-merge on green CI.
