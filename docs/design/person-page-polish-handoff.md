# Design Handoff: Person-page polish — taller parallax banner · inline poster · list scroll-restore

**Status**: Implemented (developer handoff, reverse-documented from the change)
**Date**: 2026-06-24
**Spec**: [`docs/specs/people-images.md`](../specs/people-images.md) (F25 hero) — see the **F25.26–28 follow-ups** section
**Architecture**: [ADR-038](../architecture/ADR-038-person-images.md) (person images / hero), [ADR-032](../architecture/ADR-032-browse-state-preservation.md) (browse-state preservation — the pattern reused for the people list)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [`theming.md`](theming.md) — **tokens only, QA all three skins**

Three small, related changes to the **person experience**:

1. **Taller hero banner with parallax** — the banner band doubles in height (5:1 → **5:2**) and its
   image drifts opposite to page scroll for depth.
2. **Poster on the person page** — the 2:3 poster now renders inline in the hero (it previously only
   appeared on a video's credits surface), so you don't have to leave the page to see it.
3. **List scroll-restoration** — returning from a person detail page to `/people` lands you back at the
   row you were on, not the top.

All markup is **tokens only** — no `zinc-*`, `sky-*`, hex, or fixed `rounded-lg`/`px` radii. The banner
parallax adds **no color/styling literals** (transform + aspect only); skin flourishes stay in `app.css`.

> **Skin reminders that bite these surfaces:** Broadcast & Brutalist set `--radius: 0` (everything
> `rounded-theme` is square). Broadcast washes scanlines over `.portrait-frame::after` — that includes
> the banner and poster. The hero images already route through `.portrait-frame`, so these flourishes
> apply automatically; no per-skin markup was added.

---

## Surface 1 — Hero banner: doubled height + scroll parallax

**Files:** [`web/src/lib/components/PersonBanner.svelte`](../../web/src/lib/components/PersonBanner.svelte),
[`web/src/app.css`](../../web/src/app.css) (`.portrait-frame--banner` block).

### Layout / measurements

| Property | Before | After | Token / source |
|---|---|---|---|
| Aspect ratio | `5 / 1` | **`5 / 2`** | `.portrait-frame--banner` (app.css) |
| Max height | `270px` | **`540px`** | `.portrait-frame--banner` (app.css) |
| Frame class | `aspect-[5/1] max-h-[270px] w-full` | **`portrait-frame--banner w-full`** | PersonBanner |
| Crop-editor frame | `.crop-frame--banner` `5/1` | **`5/2`** | kept in lockstep so the crop preview matches the hero |

The 5:2 ratio is **shared** between the rendered banner (`.portrait-frame--banner`) and the crop-editor
preview (`.crop-frame--banner`) — what you crop is what you see. Changing one without the other is a bug.

The avatar/poster row keeps its existing negative-margin overlap (`-mt-10 sm:-mt-12`); it still tucks the
1:1 headshot over the banner's lower-left, now against a taller band.

### Motion — parallax

| Element | Trigger | Animation | Range | Easing |
|---|---|---|---|---|
| Banner `<img>` | page scroll | `translateY` drift | `-4%` → `-24%` of the image's own height | linear (rAF-sampled) |

- The image is rendered **140% of the frame height** and clipped by the frame's `overflow: hidden`, so it
  always covers the frame across the whole drift range (no edge reveal at either extreme).
- The shift is driven by a **CSS variable** `--banner-shift` that PersonBanner updates from a **passive,
  `requestAnimationFrame`-throttled scroll listener** as the band passes the viewport
  (`progress = (innerHeight − frameTop) / (innerHeight + frameHeight)`, clamped 0–1).
- **Why JS, not a CSS `view()` timeline:** the image's nearest scroll container is its own
  `overflow: hidden` frame, which never scrolls — so a pure-CSS scroll-driven timeline on the image is
  inert. A rAF listener is also the broadest-compatibility option (works in Firefox/Safari, which lack
  scroll-driven CSS timelines).

### States / edge cases

- **Reduced motion** (`prefers-reduced-motion: reduce`): the parallax CSS block and the JS listener both
  **no-op** — the banner is a static 5:2 cover crop. (CSS drops the `height: 140%` + transform; the
  effect returns early.)
- **Before first paint / JS unavailable:** the variable's default (`-14%`) centers the crop — fully
  covered, just no live drift. The component also **seeds** the correct position synchronously on mount.
- **Empty slot** (no uploaded banner): the backend serves the themed placeholder; parallax still applies
  (harmless on a flat placeholder).
- **Image decode error:** unchanged — `PersonImageFrame` hides the `<img>`, leaving the framed well.

---

## Surface 2 — Poster on the person page

**File:** [`web/src/routes/people/[id]/+page.svelte`](../../web/src/routes/people/[id]/+page.svelte) (hero row).

The standalone owner-only **"Replace poster"** button (which let you replace a poster you couldn't see)
is replaced by an actual **2:3 poster card** sitting in the hero row, to the right of the headshot, with
the replace affordance **overlaid** — mirroring the headshot's `Edit` overlay.

### Layout

| Property | Value | Token |
|---|---|---|
| Component | `PersonPoster` → `PersonImageFrame role="poster"` | `.portrait-frame--2x3` |
| Width | `w-20` (mobile) → `sm:w-24` | Tailwind width utilities |
| Aspect | `2 / 3` (e.g. 96×144 at `w-24`) | `.portrait-frame--2x3` |
| Position | hero row, after the avatar, `items-end` (bottom-aligned) | existing `-mt-10 sm:-mt-12` row |
| Replace button (owner) | `absolute bottom-1 right-1`, `bg-bg/70 text-ink hover:text-accent rounded-theme` | tokens |

### Visibility rule (states)

| Viewer | Poster present | Poster absent |
|---|---|---|
| **Owner** | shows the poster + `Edit` overlay | shows the **placeholder** + `Edit` overlay (so they can add one) |
| **Visitor** | shows the poster | **hidden** (no placeholder clutter) |

Condition: `images.roles.poster?.present || isOwner`. The `Edit` overlay button text is `…` while
uploading (`uploadBusy === 'poster'`), else `Edit`; it reuses the page's existing single hidden file
input retargeted via `pickCore('poster')`.

### Accessibility

- Replace button: `aria-label="Replace poster"` + `title="Replace poster"` (icon-only affordance).
- Poster `<img>` `alt` is the person's name (via `PersonImageFrame`).

---

## Surface 3 — People-list scroll restoration

**Files:** [`web/src/lib/peopleScroll.svelte.ts`](../../web/src/lib/peopleScroll.svelte.ts) (new),
[`web/src/routes/people/+page.svelte`](../../web/src/routes/people/+page.svelte).

Mirrors the browse-grid pattern ([ADR-032](../architecture/ADR-032-browse-state-preservation.md)). Because
Holodex is an SPA (`ssr=false`), the list component is destroyed when you open a person and SvelteKit
resets scroll to the top on in-app navigation — so `← Back` dumped you at the top of a long A–Z list.

### Behavior

| Step | What happens |
|---|---|
| Leaving `/people` (open a person, or any nav away) | `beforeNavigate` saves `{ sort, scrollY }` to a module-scoped cache |
| Returning to `/people` (first load of the new instance) | after the re-fetched list paints (`tick()`), restore `window.scrollTo(0, y)` **iff** the saved `sort` matches |
| Sort change / post-merge reload | **no** restore — intentionally stays at the top |
| Full page reload | cache is empty (session-scoped) — starts at the top |

The cache is **one-shot** (cleared on read) and **keyed by sort** (a sort change reorders the list, so the
saved offset is dropped). No persistence, no URL change.

### Edge cases

- **Failed list fetch:** the saved offset is consumed but there's no list to scroll — lands at top (the
  error message renders). Acceptable.
- **List shorter than before** (e.g. a merge removed people while you were away): `scrollTo` clamps to the
  new max — no error.

---

## Design tokens used (all three surfaces)

| Token class | CSS var | Usage here |
|---|---|---|
| `.portrait-frame` (+`--banner`/`--2x3`) | `--surface-2`, `--rule`, `--radius` | banner & poster wells, aspect ratios |
| `bg-bg/70` | `--bg` @ 70% | replace-button scrim over images |
| `text-ink` / `hover:text-accent` | `--ink` / `--accent` | replace-button label |
| `rounded-theme` | `--radius` | replace-button corners (square in Broadcast/Brutalist) |
| `border-rule` | `--rule` | (unchanged surrounding cards) |

No new colors, fonts, or radii were introduced.

---

## Accessibility notes

- Parallax respects `prefers-reduced-motion` (no transform, no listener).
- Scroll restoration uses instant `scrollTo` (no smooth-scroll animation to fight reduced-motion).
- No focus-order or tab-stop changes; the poster's only interactive element is the owner replace button,
  already in DOM order after the headshot's.
