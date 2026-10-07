---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-486
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: infra               # stack/deploy: a new runtime dependency in the image
depends-on: []
release_note: The container image now includes MKVToolNix on a Debian 13 base, so tag and poster writes to MKV files edit the file in place instead of rebuilding it, keeping chapters, fonts and other embedded data exactly as they were.
---

# HOLODEX-486 · Bundle MKVToolNix in the runtime image

This is done when the runtime image ships `mkvtoolnix`, so production MKV writeback edits tags in place
(mkvpropedit) instead of remuxing, and the media-tool suites pass **inside the built image** via
`make test-image`.

**Design package:** [ADR-117](../architecture/archive/ADR-117-bundle-mkvtoolnix-runtime.md) · spun out of HOLODEX-485 · CI wiring → HOLODEX-487

## Gates — definition of done

- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-117 (Proposed); amends ADR-007's "test-only" note **and runtime base** (D1b: bookworm → trixie, owner's choice)
- [x] backend → `{cmd,internal,providers}/**` — `Dockerfile` (trixie-slim + mkvtoolnix); D5 Title-`Delete` guard; tag names XML-escaped
- [x] testing `testing-strategy` — `make test-image` (writeback/thumbnail/metadata in the built image, all pass on trixie; 4 MKV round trips failed on bookworm); app boots healthy; testing-strategy row
- [x] security `security-review` — no findings ≥8; applied the one hardening note (escape `TagName`)

## Up next — ordered (position = priority)

1. [ ] [—] Merge PR #412 after CI (owner's call — base-image change); HOLODEX-486 → Done via jira-sync
2. [ ] [—] Watch the `:edge` canary for MKV `in_sync` / tag churn after a rescan (exiftool 12.57 → 13.25, ffmpeg 5.1 → 7.1)
3. [ ] [—] Run `make test-image` in CI → HOLODEX-487

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: architecture, implement, security-review, handoff
- handoff: Built and all gates settled. The in-image test caught bookworm's exiftool missing relocated MKV Tags, so the base moved to trixie (owner's choice) and every suite passes in-image. PR #412 marked ready; next is the owner's merge, then watch the canary.

## Dropped — newest first (the reason is the point)

- [~] [backend] Stay on bookworm + pinned upstream exiftool tarball — dropped 2026-09-28: owner chose trixie; keeps every package Debian-maintained, bookworm is oldstable
- [~] [backend] `exiftool -ee` on Matroska in the extractor — dropped 2026-09-28: parses every cluster of every file at scan time
- [~] [backend] Config toggle to choose the MKV backend — dropped 2026-09-28: a second selector that can disagree with what's installed; removing the package is the rollback
- [~] [backend] Upstream mkvtoolnix.download apt repo — dropped 2026-09-28: third-party signing key for no needed feature; Debian's package suffices
