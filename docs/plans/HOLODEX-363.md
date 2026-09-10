---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-363
status: in-progress
release_note: The media page now puts file details with the enrichment dumps at the bottom, runs the "More with" rows the full width of the window, and shows visitors the metadata values again.
---

# HOLODEX-363 · Media detail stage layout — file joins enrichment, shelves break the width cap, visitor rail regains field values

Picks up queue item 1 from [HOLODEX-362](HOLODEX-362.md): the visitor right rail is ~70% empty
because `+page.svelte` gates the whole Metadata section on a bare `{#if isOwner}` (the comment
records it as deliberate — "visitors previously saw a filtered subset"). That was put to the owner
with three options drawn at ultrawide, 4K desktop and Pixel 7 Pro. The owner chose **option A**
(restore read-only values) and added two further layout calls: `File` moves down to sit with the
enrichment dumps, and the More-with shelves span the window rather than the stage. Done means all
three land, three-skin, owner and visitor, at all three viewports, with no horizontal page scroll.

**Two of the three decisions key off one threshold, so it is named once.** 2648px =
`--container-stage` (2600) + `main`'s 48px padding. Below it the stage fills the window and there is
no gutter, so "stage width" and "window width" are the same pixel and any justification-or-breakout
rule is a no-op; at or above it the stage stops growing, gutters open, the overview relocates to the
rail and the shelf centres. Writing `2648` twice — once for the shelf, once for the overview — is
the failure mode this note exists to prevent.

**The shelf rule was already implemented, and that is why it kept being re-decided.**
`.video-grid.stage-aligned` in `app.css` is exactly "left-justified under the cap, centred when
breaking it": `justify-content: start` + `min-width: min(var(--container-stage), 100%)` +
`max-width: 100%` + `margin-inline: auto`. Written for HOLODEX-331 §9.6 and wired to a **single**
call site — the film page's Scenes grid, via `VideoGrid`'s opt-in `stageAligned`. Nothing generalised
it, so the next component to face the question had nothing to inherit. The owner's note on this was
explicit ("I've made this decision more than once"), so the behavioural rule and its traps are now
recorded in [`web/src/lib/components/video/CLAUDE.md`](../../web/src/lib/components/video/CLAUDE.md)
rather than left in a chat answer.

**But it is not a one-prop change, twice over.** `RelatedShelf` is a `flex … overflow-x-auto`
scroller with fixed-width cards, borrowing `.video-grid` only to reset the Brutalist `reel` counter
and inherit `data-layout` — there is no `stageAligned` prop on it, so the class goes on directly and
`width: fit-content` against `overflow-x: auto` has to be verified live rather than assumed to match
the grid case. And `max-width: 100%` resolves against the *container*, so a shelf inside
`<article class="mx-auto max-w-stage">` caps at 2600 forever: the shelves must become **siblings** of
that article, which also drops them out of its `space-y-6` rhythm.

**Moving the shelves out fixes the one-column order for free.** Below `lg` the rail stacks under the
subject in DOM order, so a phone currently shows `… studio → shelves → tags → films/people →
metadata` — the recommendation shelves ahead of the entity links. As a sibling band after the grid it
becomes `… studio → tags → films/people → metadata → shelves → file+enrichment`. This was the main
objection to the rejected "collapse to one column" option, resolved as a side effect of a different
decision. No `order-*` anywhere; visual and focus order still match the DOM.

**The overview move is the expensive part, and the reason this carries an ADR.**
`#field-overview` is a deep-link anchor, and the codebase already guards this hazard for
`#field-actors` ("rendered in exactly one branch below, so the id is never duplicated"), so the
viewport-conditional move **cannot** be a second render behind a media query. It has to be CSS
placement — the two column wrapper `<div>`s become one flat grid with named areas so the block can be
*placed* into the rail. That restructures `stage-grid`, which the film detail page shares.

## Gates — definition of done

- [~] spec `write-spec` — n/a: no new capability or changed requirement. Every element already
  exists with the same semantics; this changes where three of them sit and who sees one of them
- [ ] architecture `architecture` — **required.** The named-area restructure of `stage-grid` is
  cross-cutting: the film detail page shares the primitive. ADR must cover the named-area contract,
  what the film page inherits, and whether it gets the same overview behaviour or opts out.
  Claim the number with `node scripts/adr-claims.mjs --reserve media-detail-stage-layout` — do not
  pick by eye
- [x] design `design-handoff` —
  [media-detail-stage-layout-handoff.md](../design/media-detail-stage-layout-handoff.md) + two
  committed SVGs: [block order and width scope](../design/media-detail-stage-layout-mockup.svg)
  across all three viewports, and the [shelf justification rule](../design/media-detail-stage-layout-shelf-rule.svg).
  Supersedes two points in [media-detail-reorder-handoff.md](../design/media-detail-reorder-handoff.md):
  File's position (§File-above-Completeness) and the owner-only Metadata re-gate (§2)
- [~] backend — n/a: frontend-only. The resolver already returns the fields; no endpoint, gate or
  payload changes. Visitors were already served the resolved fields by the API — only the template
  withheld them
- [ ] frontend — three moves in `web/src/routes/media/[id]/+page.svelte`, one class on
  `RelatedShelf.svelte`, and the `stage-grid` named-area restructure in `app.css`
- [ ] testing `testing-strategy` — geometry rungs for shelf overhang symmetry above the cap and
  left-alignment below it, plus a `#field-overview` uniqueness assertion at every viewport. Folds
  into [HOLODEX-359](HOLODEX-359.md)'s harness work rather than standing up a second one
- [~] security `security-review` — n/a: no auth, access or infrastructure change. Re-exposing
  resolved field values to visitors is a template gate, not an access-control one; `file_path`,
  codecs and byte size stay behind `isOwner` with `File` itself

## Up next — ordered (position = priority)

1. [ ] [—] **Confirm the overview-move scope before writing the ADR.** The owner said "for Ultrawide
   only, put the comments/overview above the tags in the second column" while answering the *visitor*
   rail question. Drawn for owner **and** visitor, on the reasoning that branching on viewport *and*
   role yields four arrangements to QA instead of two and the spare 1073px is there either way.
   Narrowing it to visitors is a change to the placement rule, not the mechanism — but it changes the
   ADR, so settle it first.
2. [ ] [M] ADR for the `stage-grid` named-area restructure (gate above). Blocks the overview move;
   does **not** block decisions 1 and 2, which are independent and could land first.
3. [ ] [S] Verify `width: fit-content` against `overflow-x: auto` on `RelatedShelf` live. If the
   flex-scroller case does not match the grid case, the shelf needs its own declaration instead of
   sharing `.video-grid.stage-aligned` — decide that before generalising the class.
4. [ ] [S] Restate the shelves' vertical rhythm at their new call site. Leaving `max-w-stage` also
   leaves the article's `space-y-6`, so the gap above and below the band has to be explicit.
5. [ ] [—] Sweep this issue **with its epic** ([HOLODEX-12](https://whoiskevinrich.atlassian.net/browse/HOLODEX-12)):
   CI transitions only the branch's own key, so a child of an epic never moves on its own. To
   `In Review` when the PR is marked ready, to `Done` on merge.
6. [ ] [—] `chore/flightplan-worklog-closeout` still holds one unmerged commit (`ee66874`, the
   HOLODEX-356 worklog closeout) and its remote is `[gone]`. Left untouched by this branch — needs
   either a fresh PR or a cherry-pick onto a live branch before it is lost.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-10 · critique to approved design; no implementation yet
- skills: design-critique, design-handoff
- **the critique found six things; two were fixed on other branches mid-session and merged before
  this branch existed** — the always-on expand chevron (#319, `ExpandableText` now gates on
  `clamps`) and the two equally-weighted destructive buttons (#320, permanent delete behind a menu,
  [HOLODEX-362](HOLODEX-362.md)). Re-read `origin/main` rather than trusting the session's earlier
  reading of those files.
- measured rather than eyeballed, from `stage-grid` itself: ultrawide 5120 → subject 1503 / rail
  1073 with 1260 dead each side; 4K@200% → 1078 / 770; Pixel 7 Pro is 412×892 **CSS** px at DPR 3.5,
  below `lg`, so one column of 364. `field-grid` column counts follow —
  `floor((container − 32) / 320)` — which is what killed the first read of option A: at ultrawide the
  visitor's three metadata fields resolve to a **single 3-across row**, so restoring them barely
  fills the rail. That is why the owner's overview move is load-bearing rather than cosmetic.
- contrast checked, not assumed: broadcast `--muted #6f7da6` is 4.90:1 on `--bg` and 4.67:1 on
  `--surface`. Both pass AA for the 12px section labels — reported as passing rather than manufactured
  into a finding.
- correction worth carrying: this session's first draft of decision 2 treated the justification rule
  as new CSS to design. It was not — `.video-grid.stage-aligned` had shipped it for HOLODEX-331 §9.6.
  Grep `app.css` for the *behaviour* before speccing a layout rule, the same lesson
  [HOLODEX-362](HOLODEX-362.md) recorded about control *shapes*.
- **the owner asked for the recurring decision to be written down, not just answered** — appended to
  `web/src/lib/components/video/CLAUDE.md` (threshold, behaviour in plain terms, both traps, current
  call sites), which auto-loads for whoever touches those components next.
- handoff: design gate is green and the Draft PR is open; **nothing is implemented**. The next
  session's first move is queue item 1 — settle whether the ultrawide overview move applies to
  owners as well as visitors — because that answer changes the ADR in item 2, and the ADR gates the
  only expensive part of the work. Decisions 1 and 2 are independent of both and can land first if
  you want something merged sooner.
