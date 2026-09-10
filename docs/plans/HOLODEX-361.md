---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-361
status: in-review
depends-on: [HOLODEX-325]
release_note: A short synopsis or bio no longer shows an expand arrow that does nothing.
---

# HOLODEX-361 · ExpandableText renders its chevron even when the text does not clamp

`ExpandableText` (added in [HOLODEX-325](HOLODEX-325.md)) rendered its expand/collapse chevron
unconditionally — there was no overflow check at all, so every short value got a control that could
not change what you see, and `aria-expanded` announced a collapsed region that was never there.
Found during a `design-critique` of the media detail page against the
[HOLODEX-342](HOLODEX-342.md) stress fixture: in visitor view a one-line overview sat under an
orphan chevron, because visitors get `ExpandableText` where owners get the ADR-051 `SourceBadge`
row. Done means the chevron appears exactly when clamping actually hides a line, at all three
skins, on all three call sites (media overview, film description, person bio).

**The measurement is against the clamp *budget*, not `clientHeight`.** The obvious check —
`scrollHeight > clientHeight` — only holds while the prose is collapsed: once expanded those two
are equal, so it reports "nothing hidden" and unmounts the very control the reader needs to collapse
again. Comparing full prose height against `line-height x lines` holds in *both* states, because
`scrollHeight` reports full content height whether or not the clamp is cutting it off. That is the
thing to remember here.

**Design package:** no new design. The behaviour is "remove a control that does nothing"; no new
surface, affordance or decision. The rule now lives in the component's own header comment, where
the next reader of `ExpandableText` will find it.

## Gates — definition of done

- [~] spec `write-spec` — n/a: a defect fix, no requirement or scope change
- [~] architecture `architecture` — n/a: no seam, stack or data-model decision
- [~] design `design-handoff` — n/a: no new surface. Hiding a dead control restores the
  component's documented intent rather than taking a design decision
- [~] backend — n/a: frontend-only
- [x] frontend — measured gate in `ExpandableText.svelte`; all three call sites inherit it
- [x] testing `testing-strategy` — verified by hand across skins and widths (see session log).
  No automated assertion added: the repo has no component-test harness (every `*.test.ts` is pure
  logic) and the gate is a DOM measurement, so the honest coverage is the harness rung queued below
- [~] security `security-review` — n/a: no auth, access or infrastructure surface

## Up next — ordered (position = priority)

1. [ ] [S] Fold a chevron-gate case into the geometry harness →
   **[HOLODEX-359](https://whoiskevinrich.atlassian.net/browse/HOLODEX-359)** (commented there, not
   re-ticketed). The harness already needs a long-text fixture; it also wants a *short*-text one, so
   the assertion is "no chevron when the prose fits" — the exact regression this fix closes.
2. [ ] [—] `web/node_modules` in this worktree is a partial install: `playwright` is a declared
   devDependency but absent, so `npm run geometry` cannot run here at all. Needs `npm ci` in the
   worktree before the harness rung above is workable.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-10 · chevron gated on a real overflow measurement
- skills: design-critique, code-review, handoff
- verified: the gate is right on both sides and on both clamp depths. Media overview (`lines`
  default 5) at p-width 696: a 1-line value gets **no** chevron, a 666-char value gets one, expands
  5 → 8 lines, and **keeps** the chevron while expanded (the budget-vs-`clientHeight` distinction
  above, caught before it shipped). Person bio (`lines={4}`, budget 91px): 2 lines → no chevron at
  781px, 5 lines → chevron at 320px. Crossover on identical content: no chevron at viewport 1280
  (3 lines), chevron at 360 (7 lines clamped to 5).
  **Three-skin QA is load-bearing here, not a formality:** the same 241-char value at the same
  312px width clamps in broadcast and brutalist (7 lines) but *not* in cinémathèque (5 lines), so
  the skins genuinely disagree about whether the control belongs. All three matched an
  independently-computed predicate. Green: `npm run check` 0 errors (15 warnings, all pre-existing,
  none in this file) · vitest 272/272.
- gotcha, the expensive kind: **`ResizeObserver` does not fire at all in the agent browser pane** —
  not even the initial observation the spec guarantees on `observe()`. That silently invalidates any
  width-reactivity conclusion drawn there, and it sent this session down a wrong path: reading a
  stale value, I wrongly concluded `bind:clientWidth` was dead, replaced it with a hand-rolled
  observer, then "fixed" a feedback loop that my own debug counter had caused (a rendered `$state`
  written from inside a tracked effect trips `effect_update_depth_exceeded` at ~1000 iterations).
  The shipped code is the simple `bind:clientWidth` after all. **Measure width behaviour with fresh
  page loads at different viewports, never by resizing in the pane** — which also matches
  HOLODEX-331's reload-don't-resize note, for a second, unrelated reason.
- then: the testbed carried no `overview` or `bio` on any entity, so all four values were seeded
  through the owner curation API and cleared again afterwards (`ClearCuration` is scoped to
  field+value+action, so it removed exactly what was added; re-verified `overview: none` on all
  three media).
- handoff: HOLODEX-361 is complete and pushed ready for review — nothing left to build. Two things
  are worth knowing before the next session touches this component. First, `expanded` deliberately
  survives `clamps` going false, so widening past the threshold and narrowing back leaves the prose
  open with the chevron reading "Collapse"; that is sticky-preference behaviour, not a bug, and
  resetting it would collapse text a reader opened on an incidental reflow. Second, the gate is
  unguarded by tests for the reason recorded in the testing gate, and the queue's item 2 blocks
  item 1 — the harness cannot run in this worktree until `npm ci` completes there.
