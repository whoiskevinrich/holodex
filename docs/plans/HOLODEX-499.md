---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-499
status: in-review            # chore posture = zero gate rows, so this stored value is read
profile: chore               # dev-tooling guard only; no user-facing behaviour
depends-on: []
release_note: No change for users — the development build now keeps every video silent (volume 0) so background testing never plays sound; production is unaffected.
---

# HOLODEX-499 · Dev environment never emits audio

Done means no `<video>`/`<audio>` in the page can make a sound under the Vite dev server, and
the rule is written where agents will read it. Filed as a playlist auto-advance bug, which
couldn't be reproduced (Play all → `ended` → next item works) and was withdrawn by the owner.
The ticket was repurposed for the owner's standing rule from the same message.

**Change:** `web/src/lib/devSilence.ts` (wired in `web/src/routes/+layout.svelte`), plus a
`.claude/CLAUDE.md` Gotchas line. Volume 0, not `muted`, so the autoplay policy still applies.

## Gates — definition of done

<!-- chore posture: no gate rows. -->

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR once CI is green; jira-sync moves HOLODEX-499 to Done — `web/src/lib/devSilence.ts`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-30 · session
- skills: code-review (high --fix: added listener teardown; detached `new Audio()` gap noted, not fixed — no such use)
- Auto-advance repro on web-9300 + AMV media: Play all → seek to end → hands off to the next item on the same `<video>` node. Guard verified live: plays at volume 0 and stays unmuted, and a raise to 0.8 snaps back to 0.
- handoff: Dev audio guard built and verified, and the playlist bug is withdrawn (not reproducible). Next: merge the chore PR once CI is green.
