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

**Exactly one rule is viewport-conditional, and it owns the threshold alone.** 2648px =
`--container-stage` (2600) + `main`'s 48px padding. Below it the stage fills the window and there is
no gutter, so "stage width" and "window width" are the same pixel and any justification-or-breakout
rule is a no-op; at or above it the stage stops growing, gutters open and the shelf box centres.
The first spec had the overview move keyed to the same number; it no longer is, so 2648 belongs to
the shelf and must not be promoted into a page-wide breakpoint.

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

**The overview move was the expensive part until the owner made it unconditional.** Specced first
as "ultrawide only", it could not have been a second render — `#field-overview` is a deep-link
anchor and the codebase already guards that hazard for `#field-actors` — so it would have had to be
CSS placement: the two column wrappers collapsed into one flat grid with named areas, restructuring
`stage-grid`, which the film detail page shares. That carried the ADR. **"The overview moves to the
second column" as a flat statement deletes all of it**: the block is simply the rail's first child,
`stage-grid` is untouched, and the anchor is unique by construction. `needs-adr` cleared. The cost
is one line of DOM order on phones — the synopsis now reads after the studio card.

**It also put the media page in conflict with the film page, which had answered the same question
the other way.** `films/[id]` renders its description as `ExpandableText` in the header and gates
the whole `Details` section owner-only, reasoned in a comment as "the description a visitor wants
already renders in the header above". Coherent alone, incoherent once the sibling page does the
opposite. The owner's call is to generalise rather than accept the split:
[HOLODEX-364](HOLODEX-364.md) moves the film page onto the same rule, and the column contract now
lives in [`web/src/routes/CLAUDE.md`](../../web/src/routes/CLAUDE.md) so that work inherits it.

## Gates — definition of done

- [~] spec `write-spec` — n/a: no new capability or changed requirement. Every element already
  exists with the same semantics; this changes where three of them sit and who sees one of them
- [~] architecture `architecture` — n/a **as of the unconditional overview rule**. It was required
  while the move was viewport-conditional, because that forced a named-area restructure of
  `stage-grid`, a primitive the film detail page shares. An unconditional move is a DOM relocation:
  no grid change, nothing for the film page to inherit structurally. No ADR number was claimed, so
  none needs releasing
- [x] design `design-handoff` —
  [media-detail-stage-layout-handoff.md](../design/media-detail-stage-layout-handoff.md) + two
  committed SVGs: [block order and width scope](../design/media-detail-stage-layout-mockup.svg)
  across all three viewports, and the [shelf justification rule](../design/media-detail-stage-layout-shelf-rule.svg).
  Supersedes two points in [media-detail-reorder-handoff.md](../design/media-detail-reorder-handoff.md):
  File's position (§File-above-Completeness) and the owner-only Metadata re-gate (§2)
- [~] backend — n/a: frontend-only. The resolver already returns the fields; no endpoint, gate or
  payload changes. Visitors were already served the resolved fields by the API — only the template
  withheld them
- [ ] frontend — three moves in `web/src/routes/media/[id]/+page.svelte` (File to the bottom,
  overview into the rail, Metadata un-gated for visitors), the shelves lifted out of the
  `max-w-stage` article, and one class on `RelatedShelf.svelte`. **No `app.css` structural change** —
  `stage-grid` is untouched
- [ ] testing `testing-strategy` — geometry rungs for shelf overhang symmetry above the cap and
  left-alignment below it, a `#field-overview` uniqueness assertion at every viewport, and an
  overflow check on the overview in the 397px rail at the narrowest two-column width (1024). Folds
  into [HOLODEX-359](HOLODEX-359.md)'s harness work rather than standing up a second one
- [~] security `security-review` — n/a: no auth, access or infrastructure change. Re-exposing
  resolved field values to visitors is a template gate, not an access-control one; `file_path`,
  codecs and byte size stay behind `isOwner` with `File` itself

## Up next — ordered (position = priority)

1. [ ] [M] **Implement.** Nothing is gated any more — the design is settled, the ADR is gone and all
   three decisions are independent of each other. Suggested order: File to the bottom (smallest,
   self-contained), then the shelves out of the article, then the overview + visitor Metadata
   together since they share the rail.
2. [ ] [S] Verify `width: fit-content` against `overflow-x: auto` on `RelatedShelf` live. If the
   flex-scroller case does not match the grid case, the shelf needs its own declaration instead of
   sharing `.video-grid.stage-aligned` — decide that before generalising the class.
3. [ ] [S] Restate the shelves' vertical rhythm at their new call site. Leaving `max-w-stage` also
   leaves the article's `space-y-6`, so the gap above and below the band has to be explicit.
4. [ ] [S] [HOLODEX-364](HOLODEX-364.md) — film page onto the same overview rule, after this merges.
   Watch the header/banner `-mb-14` overlap: the header loses a block, so how much band shows
   changes.
5. [ ] [—] Sweep this issue **with its epic** ([HOLODEX-12](https://whoiskevinrich.atlassian.net/browse/HOLODEX-12)):
   CI transitions only the branch's own key, so a child of an epic never moves on its own. To
   `In Review` when the PR is marked ready, to `Done` on merge.
6. [ ] [—] `chore/flightplan-worklog-closeout` still holds one unmerged commit (`ee66874`, the
   HOLODEX-356 worklog closeout) and its remote is `[gone]`. Left untouched by this branch — needs
   either a fresh PR or a cherry-pick onto a live branch before it is lost.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-10 (later) · one sentence deleted the ADR
- skills: design-handoff
- the owner's "maybe it would be easier to say the overview moves to the second column as a general
  statement" removed the single most expensive item in this ticket. Everything that made the move
  hard was the *viewport condition*, not the move: `#field-overview` is a deep-link anchor, so a
  media-query branch could not be a second render, so it had to be CSS placement, so `stage-grid`
  had to become named areas, so a primitive shared with the film page changed, so it needed an ADR.
  Unconditional, it is a DOM relocation. **architecture gate → n/a, `needs-adr` cleared, no ADR
  number was ever claimed so none needs releasing.**
- checked before agreeing, which is what caught the real cost: `films/[id]` already answers this
  question the other way and says so in a comment — description in the header for all roles, whole
  `Details` section owner-only, "the description a visitor wants already renders in the header
  above". Adopting the rule on the media page alone would have left two pages sharing `stage-grid`
  and disagreeing about it. Owner chose to generalise: [HOLODEX-364](HOLODEX-364.md) filed.
- measure checked rather than asserted: the rail gives the overview ~52 characters per line at the
  narrowest two-column width (1024 → 397px rail), ~100 at 1920 and ~140 at ultrawide, against 73 /
  140 / 200 in the subject column. So the rail is the better measure wide and the *worse* one narrow
  — recorded in the handoff as a simplicity win, explicitly not a typographic one, so nobody later
  defends it on grounds that do not hold.
- the column contract went to [`web/src/routes/CLAUDE.md`](../../web/src/routes/CLAUDE.md) rather
  than into the video components doc: it is a route-level decision about which zone holds what, and
  it auto-loads for whoever opens either detail page next. The stage-cap/width-scope rule stays in
  the video doc and the two cross-reference.
- handoff: design gate green, all three decisions unblocked and mutually independent, nothing
  implemented. Next session just builds — start with File, it is the smallest and touches nothing
  else.

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
