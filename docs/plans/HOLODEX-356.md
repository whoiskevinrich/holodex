---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-356
status: in-review
depends-on: [HOLODEX-355]
release_note: Fixed the header and long titles pushing pages sideways on tablet and phone widths.
---

# HOLODEX-356 · the header nav at 768px, and an unbroken name on a detail page

The last two horizontal-overflow sources behind the `no-horizontal-page-overflow` assertion, filed
alongside [HOLODEX-355](HOLODEX-355.md) but with different causes. Done means the assertion goes
green on all 180 checks and its `blockedBy` marker comes off entirely, arming it against the next
regression.

Four causes, not the two the ticket named — the third surfaced only once the first two were fixed
and the harness could see past them, and the fourth only under the `code-review high` pass, in the
two places the harness cannot see at all:

1. **The header does not fit at 768px.** ~800px of content into 705px of usable width. Not a shrink
   guard: the search box had *already* collapsed to its 42px min-content and the nav still ran 47px
   over. **Fix:** `flex-wrap` on the header and on the nav.
2. **The person hero's identity column had no `min-w-0`.** As a flex item its automatic minimum size
   was its min-content — 1403px on a 60-character name — so it could not shrink and the `truncate`
   on the `h1` inside never got the chance to clip anything. **Fix:** `min-w-0` on that column.
3. **Media, studio and tag detail headings had no wrap guard at all.** Person was the only caller
   passing `min-w-0 truncate` to `NameEditControl`; the other three passed a bare `headingClass`,
   so an unbreakable title simply painted past the page edge. **Fix:** `min-w-0 break-words` inside
   `NameEditControl` itself, so it is the row's own invariant rather than a thing each caller must
   remember. `break-words` rather than `truncate` because a media title is content — it should wrap,
   not be hidden — and both are inert against an ordinary name, and inert again under person's
   `truncate`, whose `white-space: nowrap` takes `overflow-wrap` out of play.
4. **The film and category detail headings are outside that component.** `films/[id]` renders a bare
   `h1` in a `flex-1` column, and `categories/[id]` hand-rolls NameEditControl's heading+pencil shape
   without using it — so neither inherited fix 3. Measured by hand on `/films/9000` at 768px: 251px
   of page overflow before, 0 after. **Fix:** the same two classes, restated at both sites. Not a
   refactor of the hand-rolled row — that belongs to the component-reuse work in
   [HOLODEX-287](https://whoiskevinrich.atlassian.net/browse/HOLODEX-287), still unmerged.

**Design package:** [`docs/design/header-narrow-width-handoff.md`](../design/header-narrow-width-handoff.md)
with its committed mockup — the header wrap is a visible change to global chrome, so it was put to
the owner as wrap vs. collapse-into-a-menu before implementing. Wrap was chosen: two classes, nothing
hidden, desktop untouched. Nothing else here changes a requirement or a seam.

## Gates — definition of done

- [~] spec `write-spec` — n/a: a defect fix, no requirement or scope change
- [~] architecture `architecture` — n/a: no seam, stack or data-model decision
- [x] design `design-handoff` — `docs/design/header-narrow-width-handoff.md` + committed SVG mockup;
  the header's narrow-width behaviour was a real either/or and was decided by the owner, not assumed
- [~] backend — n/a: frontend-only
- [x] frontend — `flex-wrap` on the header and nav; `min-w-0` on the person hero's identity column;
  `min-w-0 break-words` on `NameEditControl`'s heading
- [x] testing `testing-strategy` — `no-horizontal-page-overflow` armed: `blockedBy: 'HOLODEX-356'`
  removed from `web/geometry/assertions.mjs`. 180/180 across three skins and both widths
- [~] security `security-review` — n/a: no auth, access or infrastructure surface

## Up next — ordered (position = priority)

1. [ ] [—] Sweep to `Done` on merge — CI transitions only the branch's own key
2. [ ] [P2·M] **`blockedBy` mutes an assertion across every page it runs on** — carried over from
   [HOLODEX-355](HOLODEX-355.md) item 3, and now *unblocked*: that item was deferred because
   narrowing a marker needs 356's real failing set measured first, and this session measured it.
   The set was `text|studiotext|tagtext / unbroken` at `narrow` in `cinematheque` and `brutalist`
   only — 6 of 180 checks — which is exactly the per-cell granularity the item argues for
3. [ ] [P2·S] **The text ladder has no film or category rung**, so cause 4's two fixes are not
   regression-guarded — dropping either class again would leave the harness at 216 passed. Raised by
   the `code-review high` pass and deliberately not fixed there: it is a `testdata/stressseed`
   change (a `filmtext` dimension, its manifest entries, and the assertion's `when`), not a
   frontend one
4. [ ] [P3·S] `tag-chips-stay-tappable` measures the inner `<a>` and becomes a trap if HOLODEX-357
   is resolved by padding `.curation-chip` instead — carried over from HOLODEX-355 item 4
5. [ ] [P3·S] The header is 46px taller at 768px and 72px at 375px. If that reads as too much
   vertical chrome in real use, the follow-up is collapsing the nav at narrow widths — declined
   here on purpose (handoff §2a), not overlooked

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-09 · all three causes fixed, harness armed
- skills: design-handoff, code-review
- verified: reproduced both ticket measurements live against the stress fixture before touching
  anything — `/people/20001` at 1440 (`scrollWidth` 1456 vs 1425) with the overflowing box walked
  up to its ancestor, and the header at 768 with each child's min-content probed (`logo 70 · form
  42 · nav 608` into 705). That probe is what reclassified cause 1 from "missing shrink guard" to
  "does not fit": the form was already at its floor. Cause 3 was invisible until 1 and 2 were
  fixed and the harness could report past them. After: harness `--only no-horizontal-page-overflow`
  180/180 with a *newly-passing* line, full harness 216 passed / 18 known-open (HOLODEX-357 and
  -354, both pre-existing) / 1 skipped, `npm run check` 0 errors, vitest 272/272. `code-review high`
  then found cause 4 — two headings the harness does not address — and both were fixed and the film
  one measured live; its third finding, the missing ladder rung, is item 3 in *Up next*.
- then: opened **[PR #315](https://github.com/whoiskevinrich/holodex/pull/315)** ready for review
  (not draft) — every gate in the routing table is satisfied, so the Draft posture ADR-069 asks for
  does not apply and CI fires `In Review` on the ready state.
- handoff: the assertion is armed — `blockedBy` is gone, so the next sideways-scrolling regression
  on a text-ladder page fails the run instead of being absorbed. The one judgement call worth
  re-examining is `break-words` vs `truncate` on the three detail headings: `truncate` would match
  the person page but would clip ordinary long multi-word titles to one line, which for a media
  title is a content regression. If a one-line title turns out to be the house style, that is a
  design call, not a bug.
