---
key: HOLODEX-468
status: in-review
profile: backend
depends-on: []
release_note: Videos indexed before on-file tag markers existed now get their markers on the next background scan, with no per-video Refresh needed.
---

# HOLODEX-468 · Fill videos.file_tags for unchanged files after migration 0054

Done means a routine background scan fills `videos.file_tags` for a pre-0054 row whose file is
unchanged, once, without a boot backfill, so the whole existing library gets on-file tag glyphs.

**Context:** spun out of HOLODEX-401 · [ADR-111](../architecture/ADR-111-recorded-file-tag-set.md) D1 follow-up · [spec F72](../specs/tag-set-writeback.md) · [testing-strategy](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`
- [x] backend → `{cmd,internal,providers}/**`
- [x] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-468 to Done — `.github/workflows/jira-sync.yml`
2. [ ] [—] After deploy, check that the first scan's `updated` count is about the pre-0054 row count, and that the next scan skips them again — `/owner` activity

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-27 · session
- skills: code-review, handoff
- decision (owner, question card): option 1: the scanner treats `file_tags IS NULL` as changed. Rejected an owner-triggered batch and a tags-only exiftool pass
- build: `VideoStat.FileTagsKnown` (from `StatByPath`); the scanner's unchanged-file fast path needs it true. The one-time fill skips the thumbnail and filename-extraction hooks (a code-review finding: without that, every older video would get new thumbnails and a filename-extraction pass)
- docs: spec F72 non-goal and acceptance item 5; ADR-111 D1 follow-up note; testing-strategy row
- branch renamed `claude/holodex-468-410d0a` → `HOLODEX-468-file-tags-fill` (unpushed) so Flightplan's `HOLODEX-\d+` key matches
- handoff: Every gate is settled and the PR is open. Next: merge, confirm CI moved 468 to Done, then check the first post-deploy scan's `updated` count.

## Dropped — newest first (the reason is the point)
