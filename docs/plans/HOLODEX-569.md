---
key: HOLODEX-569
status: in-progress
profile: backend
depends-on: []
release_note: "Fix: flipping a video horizontally no longer fails with \"the new file's tags didn't match the original\" on ordinary MP4 files."
---

# HOLODEX-569 · Flip horizontally fails verification on real MP4s

F77 (HOLODEX-556) shipped the flip, but its fixtures were written by ffmpeg, so they already
carried what ffmpeg adds. Every real library MP4 failed rule 4 ("tags"). A throwaway harness ran
Rebuild → Verify on testbed files, writing only to scratch. It found three causes:
- an `encoder=Lavc libx264` stamp on the re-encoded track;
- a dropped track udta `name`;
- ffmpeg's own `ftyp` brands.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — rule 4 gains its second exception, MP4 `ftyp` brands (owner's call, 2026-10-12)
- [x] backend → `{cmd,internal,providers}/**` — `args.go` encoder + name→title, `verify.go` brand exemption
- [x] testing `testing-strategy` — args/verify unit tests, integration green, and a 2026-10-12 real-library run recorded in `docs/testing-strategy.md`

## Up next — ordered (position = priority)

1. [ ] [—] Open the PR, then squash-merge on green CI with subject `fix(media): let a horizontal flip pass verification on real MP4 files`
2. [ ] [—] Duplicate `covr` items lose cover art on flip → HOLODEX-570

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · flip tag-verification fix
- skills: code-review high --fix, handoff
- handoff: Flip now verifies on the AMV testbed: 31 of 33 sampled files pass. The 2 that fail carry duplicate `covr` items, tracked as HOLODEX-570. Next: open the PR and merge it on green CI.
