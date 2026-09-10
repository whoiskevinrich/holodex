# Video components

The video card/grid primitives and the horizontal shelves built on top of them.

| File | Purpose |
|---|---|
| `RecentlyAddedShelf.svelte` | The 20 newest videos, sliced from the already-loaded grid page (no extra request); landing view only. |
| `RelatedShelf.svelte` | One "More with &lt;name&gt;" shelf; self-omits when there are no items so there's never an empty rail. |
| `VideoCard.svelte` | Single video thumbnail card, with cache-busted thumbnail URL and 404-retry-with-backoff while a thumbnail is still generating. `sceneNumber`/`onEditScene` (Films-only, F56/HOLODEX-326) render a scene-number badge that becomes a real `<button>` — a sibling of the card's `<a>`, never nested inside it — when an edit callback is supplied. |
| `VideoGrid.svelte` | Responsive video grid — `min(density, capForWidth(viewport))` columns, 1 on a phone up to 16 on an ultrawide; card aspect ratio driven by `data-layout`. Opt-in `stageAligned` (film Scenes only) swaps `1fr` tracks for fixed ones sized off the full column count and caps the track count at the card count, so the grid holds the stage width for a short row and grows past it for a long one — track maths in `$lib/stageGrid`, CSS in `app.css`. |

## Stage cap vs. window width — decide it once

Recurring question, settled here so it stops being re-litigated per component: **does this
row stop at the page's width ceiling, or run the full window?**

The ceiling is `--container-stage` (2600px), applied as `max-w-stage` on the detail pages'
`<article>`. The window stops filling it at **2648px** — 2600 plus `<main>`'s 48px of
padding — and above that threshold gutters open either side. That one number is the whole
decision surface: below it "stage width" and "window width" are the same pixel, so any rule
phrased in terms of justification or breakout is a no-op.

**The rule.** A row of cards holds the stage for a short row and grows past it for a long
one; its cards keep one size regardless of count. Already encoded, with no per-call-site
arithmetic needed:

```css
.video-grid.stage-aligned {
	justify-content: start;                        /* left-justified inside the box */
	width: fit-content;
	min-width: min(var(--container-stage), 100%);  /* never narrower than the stage */
	max-width: 100%;                               /* grows to the container */
	margin-inline: auto;                           /* box centred → row centres past the cap */
}
```

Read as behaviour: **below 2648px the cards sit flush with the player's left edge; above it a
short row still pins to the stage and centres on the page, and a long row overhangs the stage
on both sides.** Track maths in `$lib/stageGrid` (`stageGridTracks` / `stageGridWidth`), pinned
by `stageGrid.test.ts` because the interesting card counts are beyond what the dev fixture
reaches. Prose lives beside the CSS in `app.css`; ADR/spec context is HOLODEX-331 §9.6.

**Two traps.**

- `max-width: 100%` resolves against the *container*, so a row inside `max-w-stage` caps at
  2600 and can never break out however the class is set. Breaking the cap means the element
  is a sibling of that `<article>`, not a descendant — which also drops it out of the
  article's `space-y-6` rhythm.
- `RelatedShelf` is **not** a `VideoGrid`. It is a `flex … overflow-x-auto` scroller with
  fixed-width cards that borrows the `.video-grid` class only to reset the Brutalist `reel`
  counter and inherit `data-layout` sizing. There is no `stageAligned` prop to pass; the
  class has to go on directly, and `width: fit-content` against `overflow-x: auto` is not
  the same layout as against a grid — verify it live before assuming parity.

**Current call sites.** `stageAligned` is opt-in and used by exactly one: the film page's
Scenes grid. `RelatedShelf` does not use it yet.
