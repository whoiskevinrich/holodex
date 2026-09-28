---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-486
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: infra               # stack/deploy: a new runtime dependency in the image
depends-on: []
release_note:
---

# HOLODEX-486 · Bundle MKVToolNix in the runtime image

This is done when the runtime image ships `mkvtoolnix`, so production MKV writeback edits tags in place
(mkvpropedit) instead of remuxing, and the writeback integration suite passes **inside the built
image** via `make test-image`.

**Design package:** [ADR-117](../architecture/ADR-117-bundle-mkvtoolnix-runtime.md) · spun out of HOLODEX-485 · CI wiring → HOLODEX-487

## Gates — definition of done

- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-117 (Proposed); amends ADR-007's "test-only" note
- [ ] backend → `{cmd,internal,providers}/**` — `Dockerfile` + D5 Title-`Delete` guard
- [ ] testing `testing-strategy` — `make test-image` (integration suite in the built image) + testing-strategy row
- [ ] security `security-review` — new parser binaries in the image

## Up next — ordered (position = priority)

1. [ ] [backend] Add `mkvtoolnix` to the runtime stage; guard Title `Delete` → `--delete title` — `Dockerfile`, `internal/writeback/writeback.go`
2. [ ] [testing] `make test-image` + run it; testing-strategy row — `Makefile`, `scripts/`
3. [ ] [security] `/security-review`

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: architecture, implement
- handoff: Crossed into build: architecture (ADR-117) settled, no approve gate in the infra posture; draft PR open. Start at the Dockerfile + D5 Title-Delete guard.

## Dropped — newest first (the reason is the point)

- [~] [backend] Config toggle to choose the MKV backend — dropped 2026-09-28: a second selector that can disagree with what's installed; removing the package is the rollback
- [~] [backend] Upstream mkvtoolnix.download apt repo — dropped 2026-09-28: third-party signing key for no needed feature; Debian's v74 suffices
