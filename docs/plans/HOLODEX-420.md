---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-420
status: in-progress
release_note: If the server refuses a status check while the media page is tracking a file write (for example the video was removed or re-indexed mid-write), the page now recovers cleanly instead of leaving the "writing to file" badge stuck until you navigate away.
---

# HOLODEX-420 · Media page writeback poll: an HTTP-status refusal mid-poll leaves the re-entrancy guard set

`pollUntilSettled` deliberately rethrows a fetch error that carries an HTTP `status`; the media
page chains its guard-clearing + `reloadDetail()` onto `waitForVideoWriteback(...)` with no
`.catch`. On that path the rejection was unhandled and `pollingGeneration` stayed set for the
page's lifetime, so any later `reloadDetail()` that came back `pending: true` returned without
re-arming the poll — *writing to file* until navigation. Pre-existing since #303 (ADR-091); noted
during the HOLODEX-419 review and left out of scope there.

Fix lands in the helper, not the page: `waitForVideoWriteback`'s own doc already promised "never
throws" and the page is its only caller, so the refusal now settles as the not-pending zero value
and the page's existing resolved-path code runs (guard clears, detail reloads). The page also
applies the settled state before the reload so a reload that fails — the same 404 — cannot leave
the badge on the stale pending object.

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/fire-and-forget-writeback.md` gains R2.5b (refusal settles
  the wait as not-pending; the follow-up reload decides what renders)
- [~] architecture `architecture` — n/a: ADR-091 stands
- [~] design `design-handoff` — n/a: no visual change; the existing badge stops sticking
- [x] frontend — `writebackJob.ts` catches the refusal; `+page.svelte` applies `settled` before
  `reloadDetail()`
- [x] testing `testing-strategy` — `writebackJob.test.ts` refusal case flipped from
  `rejects` to `resolves` not-pending after one fetch (mutation-checked: restoring the rethrow fails
  exactly that case); row added to `docs/testing-strategy.md`
- [~] security `security-review` — n/a
- [x] `code-review high --fix` — one finding, applied: the helper's try/catch collapsed to
  `.catch(() => null)` on the awaited call

## Up next — ordered (position = priority)

1. [ ] [—] On merge, HOLODEX-420 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · fixed at the helper, spec + test in lockstep
- skills: code-review high --fix, code-review
- Traced the page's `$effect` chain and the helper's contract; the existing
  `waitForVideoWriteback` test pinned the rethrow the page could not handle, so the contract moved
  to the helper (single caller, doc already said never-throws) rather than adding a `.catch` the
  harness cannot exercise.
- handoff: fix + spec + test shipped; open PR ready (all gates green) and let CI move Jira.
