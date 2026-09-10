---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-358
status: done
depends-on: [HOLODEX-356]
release_note: Fixed a long field value scrolling the media page sideways on a phone.
---

# HOLODEX-358 · a long field value still widens the media page below ~440px

The tail of [HOLODEX-356](HOLODEX-356.md), found by a `code-review medium` pass after that PR had
already merged. Done means `/media/202` is clean at 375px and the committed header mockup matches
what actually ships.

A `SourceBadge` value is a flex item, so it is floored at its own min-content and an unbreakable
token widens the row past the viewport — 133px of page scroll at 375px, from `#field-overview`.
HOLODEX-356's `min-w-0` + `break-words` pair cannot fix this shape: `overflow-wrap: break-word`
deliberately does not reduce min-content, so the floor survives it. Only `overflow-wrap: anywhere`
collapses it. That pair is right where 356 used it — a *block* heading inside a column `min-w-0`
has already narrowed — and wrong here; the distinction is the thing to remember.

Tracked under its own key rather than reopening HOLODEX-356: CI transitions the branch's key, so
reusing 356 would have dragged a `Done` issue back through `In Review` and out again.

**Design package:** no new design. It corrects HOLODEX-356's own
[`header-narrow-width-handoff.md`](../design/header-narrow-width-handoff.md) and its committed
mockup, which drew the wrapped header's search box adjacent to the logo when `justify-between`
actually ships it against the right edge with a 187px gap. The handoff now records the gap and why
closing it would move the desktop header.

## Gates — definition of done

- [~] spec `write-spec` — n/a: a defect fix, no requirement or scope change
- [~] architecture `architecture` — n/a: no seam, stack or data-model decision
- [x] design `design-handoff` — HOLODEX-356's handoff and mockup corrected to match the shipped
  layout; no new decision was taken
- [~] backend — n/a: frontend-only
- [x] frontend — `wrap-anywhere` on `SourceBadge`'s value span
- [x] testing `testing-strategy` — no new assertion is possible yet: the harness runs at 768 and
  1440, and this only reproduces below ~440px. Verified by hand instead; the missing rung is item 1
- [~] security `security-review` — n/a: no auth, access or infrastructure surface

## Up next — ordered (position = priority)

1. [ ] [—] Harness rung for the phone width and a long-text film/category fixture →
   **[HOLODEX-359](https://whoiskevinrich.atlassian.net/browse/HOLODEX-359)** — promoted out of
   this epic now that it has merged, because both worklogs carrying it (356 and this one) are closed
   and it would otherwise be lost

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-09 · PR #316 merged — worklog closed out
- skills: handoff
- verified: [PR #316](https://github.com/whoiskevinrich/holodex/pull/316) merged to `main` as
  `8cebb82` with all 11 checks green, and Jira `HOLODEX-358` is `Done` — CI fired it on the merge,
  so the by-hand sweep this queue was holding never needed doing: 358 is a standalone Bug with no
  epic parent, and the child-stays-`In Progress` trap only bites an epic's children. Working tree
  clean, branch in sync with `origin`.
- then: promoted the one surviving queue item to its own ticket,
  **[HOLODEX-359](https://whoiskevinrich.atlassian.net/browse/HOLODEX-359)**, linked to 349/356/358.
  It is the only thing that outlives this epic, and it was being carried by two worklogs that are
  both now closed.
- handoff: HOLODEX-358 is finished, merged and `Done` — nothing here is left to build. The next
  move on this thread is HOLODEX-359: add a 375px cell to `WIDTHS` in `web/geometry/browser.mjs`
  (decide the cost first — `matrix()` is `SKINS x WIDTHS`, so it takes the run from 6 cells to 9)
  plus a `filmtext` dimension in `testdata/stressseed`, so the three overflow fixes from PRs
  #314/#315/#316 finally have a test that can reproduce them.

### 2026-09-09 · `code-review medium --fix` over PR #315, both findings applied
- skills: code-review, handoff
- verified: a second pass at a viewport nothing else tests. **A field value still overflowed the
  page below ~440px** — `/media/202` at 375px, 133px of scroll from `#field-overview`'s value span.
  The fix is not the one HOLODEX-356 used, and that is the finding worth remembering:
  `overflow-wrap: break-word` **deliberately does not reduce min-content**, so on a *flex item*
  (which a `SourceBadge` value is) `max-w-full` + `break-words` left all 133px in place. Only
  `overflow-wrap: anywhere` (`wrap-anywhere`) collapses it. Put on the value span rather than the
  wrapper so it does not inherit into the provider chip row. HOLODEX-356's heading fixes are a
  different shape — the `h1` there is a *block* inside a shrunk column, so
  `break-words` is enough once `min-w-0` has narrowed the box. Person, studio and tag pages are
  clean at 375, so this was one element, not a family.
  Second finding: **the committed mockup disagreed with what ships.** `justify-between` spreads the
  wrapped first line, so the search box sits against the right edge with a 187px gap after the logo,
  not adjacent to it as drawn. The mockup and handoff now show the real layout and name the gap —
  every way of closing it also moves the search box at desktop width, which this fix does not touch.
  Green after: `npm run check` 0 errors · vitest 272/272 · full harness 216 passed / 18 known-open /
  1 skipped, unchanged.
- then: opened **[PR #316](https://github.com/whoiskevinrich/holodex/pull/316)** ready for review —
  every gate satisfied, so no Draft posture applies and CI fires `In Review` on the ready state.
- handoff: nothing deferred. Both findings landed after PR #315 had merged, so they moved to this
  key on a branch off the new `main` rather than reopening a `Done` issue. The `SourceBadge` change
  reaches into a shared component — isolated to one class on one span, and easy to drop if that
  scope is unwanted. It is unguarded for the same reason HOLODEX-356's film and category fixes are:
  the harness runs at 768 and 1440, and this only reproduces below ~440px.
