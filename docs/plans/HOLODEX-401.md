---
key: HOLODEX-401
status: in-progress
profile: full
depends-on: []
release_note: The writeback dialog now writes a video's tags to the file, showing which tags are already on the file, which will be added and which will be dropped — and your tag chips show at a glance whether each tag is on the file.
approved:
  design:
    on: 2026-09-27
    at: e0802c2a
---

# HOLODEX-401 · Write tags to the file as a set + per-tag on-file markers

Done means the writeback dialog writes a video's tags as one set whenever the file lags, showing each tag
as on file / will add / will drop, and the owner's tag chips on the media page carry an on-file glyph —
backed by the file's tag set recorded at every scan.

**Design package:** [spec F72](../specs/tag-set-writeback.md) · [ADR-111](../architecture/archive/ADR-111-recorded-file-tag-set.md) · [handoff](../design/tag-set-writeback-handoff.md) + [mockup](../design/tag-set-writeback-mockup.svg) · [testing-strategy](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/ADR-*`
- [x] design `design-handoff` → `docs/design/**`
- [x] backend → `{cmd,internal,providers}/**`
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy`
- [~] security `security-review` — no auth, access or infra change: read-only fields on the existing `GET /media/{id}`, no new route; the write path is unchanged (server still recomputes the set) — until: a tag-set write endpoint or client-supplied values are trusted

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-401 to Done (branch key is lowercase `holodex-401`) — `.github/workflows/jira-sync.yml`
2. [ ] [backend] Fill `file_tags` for unchanged files (a routine scan skips them) → HOLODEX-468 — `internal/scanner/scanner.go`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-27 · session
- skills: implement, code-review, handoff
- build: migration 0054 `videos.file_tags` recorded in `UpsertVideo`; `repo.TagIdentityKeys` now shared by the writer's `tagKeeper` and the read-back; genres row carries `on_file`/`file_only`/set `in_sync`; `video.tags[].written/on_file`; dialog tag set row + `TagLinkChip` glyphs; `·tag` no longer reads as a provider
- verified live on `backend-amv` (real MP4): unknown → known on re-extract, dialog diff, Write → `Genre: action, heist`, row `=`; contrast ≥ 4.5:1 in all three skins; owner confirmed build matches the approved mockup
- found: a routine scan skips unchanged files, so `file_tags` fills only on re-extract (change, Refresh, write); docs corrected, library-wide fill spun out as HOLODEX-468
- handoff: All gates settled and PR #398 marked ready; next is merge, then HOLODEX-468 so pre-0054 videos get their on-file markers without a per-video Refresh.

### 2026-09-26 · session
- skills: implement (design phase by hand — spec, ADR-111, handoff)
- decisions (owner, question cards over an inline mockup): Tags row writes on open; 2B glyph on every chip (replaces the ·source suffix); record file tags at every scan; unknown until rescan (no backfill)
- handoff: Crossed into build — design signed off at e0802c2a; draft PR open. Start at the backend slice (migration 0054 `videos.file_tags`).

## Dropped — newest first (the reason is the point)
