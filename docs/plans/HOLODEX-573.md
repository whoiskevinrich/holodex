---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-573
status: in-review
profile: ui                  # frontend bug fix; restore-on-return rule, no new surface
depends-on: []
release_note: Following a link to a list you visited earlier now starts it at the top; only Back, Forward or a reload return you to where you were, on Media too.
---

# HOLODEX-573 · List scroll snapshot restores on a plain link arrival, not only on Back

`navSnapshot`'s two `take`s (the Media grid's `browseCache` and every `listScroll` slot) spend the
snapshot on any arrival but return it only for `popstate` or `enter`. The root layout records the
type in `onNavigate` (`web/src/lib/lastNavigation.ts`), because the Media grid takes its snapshot
in its first `$effect`, which can run ahead of `afterNavigate`. The owner chose to include Media
(2026-10-13).

**Design package:** none (no new surface). The rule is in `docs/specs/list-toolbar.md` (Goals 4),
and the mechanism is in `docs/architecture/browse-and-list-state.md`.

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no new surface; the owner picked Media-in-scope on a question card
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` — §29 + `navSnapshot.test.ts` + nav harness `link-arrival-starts-fresh`; mutation-tested (no gate; afterNavigate instead of onNavigate)

## Up next — ordered (position = priority)

1. [ ] [—] After merge, cite the squash commit in browse-and-list-state.md's snapshot section ("Decided in"), as #508 did for HOLODEX-190
2. [ ] [—] On merge, confirm CI moved HOLODEX-573 to Done
3. [ ] [testing] Harness `back-after-write-refreshes-grid` hangs in `route.fetch` on this machine (fails on `main` too) → HOLODEX-574

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · restore a list's place only on a return
- skills: code-review (high --fix), handoff
- handoff: Fixed and verified on `backend-stress`. The nav harness is 9/10; the one failure is pre-existing on `main` and spun out as HOLODEX-574. Next: open the PR, merge, then cite the squash commit in the architecture doc.

## Dropped — newest first (the reason is the point)
