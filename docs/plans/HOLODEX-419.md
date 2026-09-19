---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-419
status: in-progress
release_note: The media page now keeps tracking a file write until it finishes, however long it takes — the "writing to file" badge clears and the poster refreshes on its own instead of sticking until a reload after a slow (multi-GB) write.
---

# HOLODEX-419 · Media page stops polling a pending writeback after 120 s

Reported as a regression from PR #353 ("page never updates after Write decisions to file").
It is not: the path is unchanged since #303 (ADR-091). The media page's `$effect` waits on
`waitForVideoWriteback` with the helper's default 120 s cap; on timeout the helper resolves with
`pending: true` and the page's continuation returns without reloading or re-arming, and nothing
else ever re-runs the effect. Any write slower than 120 s strands the page on *writing to file* +
*out of sync* with a stale poster. With no `mkvpropedit` on PATH every MKV write is copy + full
ffmpeg remux (~4× file size in I/O), so the films testbed's multi-GB MKVs hit it routinely.

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/fire-and-forget-writeback.md` gains R2.5a (page-level wait has
  no time cap; unmount/navigation is the only cancel)
- [~] architecture `architecture` — n/a: no architecture change (ADR-091 stands)
- [~] design `design-handoff` — n/a: no visual change; the existing badges now clear as designed
- [x] frontend — `+page.svelte`'s poll passes `timeoutMs: Infinity`
- [x] testing `testing-strategy` — `writebackJob.test.ts` pins the uncapped wait under fake timers
  (mutation-checked: fails under the default cap); row added to `docs/testing-strategy.md`; live
  repro + re-verify on `backend-amv` with the helper cap shrunk to 400 ms
- [~] security `security-review` — n/a
- [x] `code-review high --fix` — one finding, fixed: the re-entrancy guard is now keyed by
  `pageGeneration` (a bare boolean still set by the previous video's winding-down loop — up to 5 s
  at the backoff ceiling, the steady state of a long write — would make the next video's effect
  return without polling)

## Up next — ordered (position = priority)

1. [ ] [—] On merge, HOLODEX-419 → Done via CI (branch-keyed)
2. [ ] [—] Pre-existing, not fixed here: a poll fetch that rethrows an HTTP-status error (owner
   session expiring mid-write) leaves `pollingWriteback` stuck `true` with an unhandled rejection —
   file if it bites

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · diagnosed, reproduced, fixed
- skills: code-review high --fix
- Read #353's diff (image `?v=` versioning only; nothing on the status path), traced the page's
  poll, ran an isolated stack (`backend-amv` :7810 / Vite :5174 — another session held :7800/:5173)
  and confirmed a 16 MB and an 86 MB write both settle and refresh the poster on `main@63f376a`.
  Shrinking `JOB_POLL_TIMEOUT_MS` to 400 ms reproduced the exact symptom: two poll ticks then
  silence, UI on *writing to file* while the server said `pending:false`. Fix at the page's call
  site; re-verified under the same shrunk cap (seven ticks, badge cleared, poster `?v=` advanced).
- handoff: fix + spec + test shipped; open PR (ready — all gates green) and let CI move Jira.
