---
key: HOLODEX-502
status: in-progress
profile: full
depends-on: []
approved:
  design:
    on: 2026-10-09
    at: 00e12565
release_note: "Behind a single-sign-on proxy, an expired sign-in no longer throws away what you were editing. If you have unsaved input or a save fails, the page stays put and a notice offers to sign you in again in a new tab, then tells you if a change still needs saving. Regenerating a thumbnail now recovers the same way instead of failing with a generic error."
---

# HOLODEX-502 · Hold the auth-proxy re-auth reload while editing

When the ForwardAuth proxy's sign-in lapses, HOLODEX-127 recovers by reloading the page. That
reload wiped unsaved input, and a save the proxy redirected looked like it had succeeded. Worse
than the ticket said: the layout's 3 s activity poll is authed, so the reload fired within
seconds even if the owner never pressed Save. Option B (owner's pick, 2026-10-09): **hold the
reload** when a write fails or the page has unsaved input, and show a sticky banner whose "Sign
in again ↗" opens a new tab. An idle page still recovers silently. `regenerateThumbnail` now
goes through the same redirect handling.

**Design package:** [spec OS8](../specs/owner-session-persistence.md) · [handoff](../design/session-expired-banner-handoff.md) + [mockup](../design/session-expired-banner-mockup.svg) · [testing-strategy](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — OS8 in owner-session-persistence.md
- [~] architecture `architecture` — no technology fork: same `redirect: 'manual'` detection, a held branch in `triggerReauth`
- [x] design `design-handoff` → `docs/design/**` — session-expired-banner handoff + SVG (owner sign-off on the sticky z-[70] refinement pending)
- [~] backend — frontend-only change; no server route or behavior touched
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy`
- [ ] security `security-review` — touches the auth-recovery path

## Up next — ordered (position = priority)

1. [ ] [security] `/security-review` ran clean on 00e12565 (no findings) — `/handoff` to settle the gate, then mark the PR ready
2. [ ] [—] On merge, confirm CI moved HOLODEX-502 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-09 · held re-auth + session-expired banner
- skills: code-review high --fix, security-review, implement
- handoff: Crossed into build — design signed off at 00e12565; draft PR open. Start at settling the security gate (review ran clean) and marking the PR ready.
