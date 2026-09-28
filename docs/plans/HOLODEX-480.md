---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-480                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: infra               # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Writing metadata to a fragmented MP4 now just works — Holodex remuxes the file as part of the write, keeping all its existing tags.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-480 · Remux fragmented MP4 on write

Done when a writeback to a fragmented MP4/M4V/MOV succeeds invisibly. The file is remuxed into the
job's temp copy, its existing tags (XMP included) are restored, and the batch is written and renamed
into place. HOLODEX-479's refusal remains only as the fallback for when the remux can't run.

**Design package:** [ADR-116](../architecture/ADR-116-remux-fragmented-mp4-on-write.md) · [spec R3.7a](../specs/fire-and-forget-writeback.md) · absorbs HOLODEX-481 (Won't Do)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — outside the `infra` posture, kept because it carries work: R3.7a amends HOLODEX-479's R3.7
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-116 (Proposed), which extends ADR-041's copy step
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [backend] Remux branch in `writeExiftoolBatch`, `-TagsFromFile`, temp re-check, ErrFragmentedMP4 fallback (ADR-116 D1–D4) — `internal/writeback/fragments.go`
2. [ ] [testing] fMP4 integration (tags kept, no moof, mp4/mov preserved) + XMP graft fixture + ffmpeg-failure fallback — `internal/writeback/fragments_test.go`
3. [ ] [security] `/security-review` of the new ffmpeg exec over user files — `internal/writeback/`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: architecture, implement
- handoff: Crossed into build (the posture has no approve gate; the owner said "implement" after reviewing ADR-116); draft PR open. Start at Up next #1: remux branch in `writeExiftoolBatch`.

## Dropped — newest first (the reason is the point)

- [~] [backend] Scan-time `fragmented` flag + writeback-dialog warning (HOLODEX-481) — dropped 2026-09-28: the owner wants the fix invisible, and with remux-on-write the flag has no consumer
- [~] [backend] Background sweep remuxing every fragmented file — dropped 2026-09-28: rewrites files the owner never asked to change, needs up to 2x disk, and changes bytes that seeding/checksums rely on
