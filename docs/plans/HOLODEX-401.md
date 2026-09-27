---
key: HOLODEX-401
status: in-progress
profile: full
depends-on: []
release_note:
---

# HOLODEX-401 · Write tags to the file as a set + per-tag on-file markers

Done means the writeback dialog writes a video's tags as one set whenever the file lags, showing each tag
as on file / will add / will drop, and the owner's tag chips on the media page carry an on-file glyph —
backed by the file's tag set recorded at every scan.

**Design package:** [spec F72](../specs/tag-set-writeback.md) · [ADR-111](../architecture/ADR-111-recorded-file-tag-set.md) · [handoff](../design/tag-set-writeback-handoff.md) + [mockup](../design/tag-set-writeback-mockup.svg) · [testing-strategy](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/ADR-*`
- [x] design `design-handoff` → `docs/design/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [backend] Migration 0054 `videos.file_tags` + UpsertVideo writes it; genres-row diff + `video.tags[].written/on_file` — `internal/api/genre_writeback.go`
2. [ ] [frontend] Tag-set row in the dialog + on-file glyph on owner `TagLinkChip` — `web/src/lib/components/writeback/WritebackFormDialog.svelte`
3. [ ] [testing] Round-trip integration: write → re-extract → `in_sync: true`; testing-strategy section — `docs/testing-strategy.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-26 · session
- skills: (design phase by hand — spec, ADR-111, handoff)
- decisions (owner, question cards over an inline mockup): Tags row writes on open; 2B glyph on every chip (replaces the ·source suffix); record file tags at every scan; unknown until rescan (no backfill)
- handoff: Design package written (spec F72, ADR-111, handoff + SVG); next is the /implement crossing and then the backend slice (migration 0054).

## Dropped — newest first (the reason is the point)
