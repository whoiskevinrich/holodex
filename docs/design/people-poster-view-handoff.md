# Design Handoff: Poster View for the People list page (F55)

**Status**: Draft (pre-implementation handoff)
**Date**: 2026-08-05
**Spec**: [`docs/specs/people-poster-view.md`](../specs/people-poster-view.md) (F55, Jira [HOLODEX-255](https://whoiskevinrich.atlassian.net/browse/HOLODEX-255))
**Architecture**: none required — see the spec header ("New ADRs required: none")
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [`theming.md`](theming.md) — **tokens only, QA all three skins**

A new **Poster View** for `/people`, toggled alongside the existing List view: a `PersonPosterGrid`
of `PersonPosterCard`s (the 2:3 `.portrait-frame` well + name/count below, mirroring `VideoCard`'s
title-below-thumbnail layout), a shared density control, and one small List-view padding fix (RD7).
All markup is **tokens only** — no `zinc-*`, `sky-*`, hex, or fixed `rounded-lg`/`px` radii; hover/focus
add transform + `ring-accent`/`shadow` only, no new color literals.

> **Corrections to the spec, found while grounding this handoff in the real component source.**
> The spec's RD5/RD6/RD3 describe the mockup's simulated CSS, not what `app.css` actually does
> today. None of these change what ships — they simplify it:
> - **RD5 (Cinémathèque bar):** `app.css` scopes the letterbox `::before`/`::after` bars to
>   `.video-grid … .video-frame` only (`app.css:265-280`). **`.portrait-frame` has never had this
>   bar.** The mockup added a parallel bar to `.portrait-frame` purely to preview what a shared
>   flourish would look like. Nothing to suppress — `PersonPosterCard` needs **zero** Cinémathèque
>   CSS.
> - **RD6 (Brutalist counter):** same story — `[data-theme='brutalist'] .video-frame::before`
>   (`app.css:449`) is the *only* Brutalist counter rule in the file; there is no existing
>   `.portrait-frame` equivalent to "carry over." The outcome RD6 wanted (no new per-skin branch)
>   still holds — there's just nothing to carry over, either. Only **Broadcast's scanline**
>   (`[data-theme='broadcast'] .portrait-frame::after`, `app.css:381-391`) is a real, existing
>   `.portrait-frame` flourish, and it applies to `PersonPosterCard` automatically since the card
>   reuses `.portrait-frame` as-is.
> - **RD3 ("14px grid gap"):** `VideoGrid` uses Tailwind `gap-4` (16px), not 14px
>   (`VideoGrid.svelte:22`). Use `gap-4` on `PersonPosterGrid` too, for the same reason RD3 gives
>   (gap alone separates borderless tiles) and for visual consistency with the video grid.

---

## Surface 1 — View toggle

**File:** [`web/src/routes/people/+page.svelte`](../../web/src/routes/people/+page.svelte) (header
actions row, `+page.svelte:117-146`) · **new module**
`web/src/lib/viewPreference.svelte.ts` · **new component**
`web/src/lib/components/person/PersonViewToggle.svelte`.

### Component

A 2-segment control, visually and structurally identical to
[`SortToggle.svelte`](../../web/src/lib/components/sort/SortToggle.svelte) — same container, same
active/inactive class pairs, just a different button set:

```svelte
<!-- PersonViewToggle.svelte -->
<script lang="ts">
	let { view = $bindable() }: { view: 'list' | 'poster' } = $props();
	const cls = (active: boolean) =>
		active ? 'bg-accent px-3 py-1 text-accent-ink' : 'px-3 py-1 text-muted hover:text-ink';
</script>

<div class="flex overflow-hidden rounded-theme border border-rule text-sm">
	<button onclick={() => (view = 'list')} class={cls(view === 'list')} aria-pressed={view === 'list'}>List</button>
	<button onclick={() => (view = 'poster')} class={cls(view === 'poster')} aria-pressed={view === 'poster'}>Poster</button>
</div>
```

`aria-pressed` is new relative to `SortToggle` (a 3-way sort isn't a toggle in the ARIA sense; a
2-way List/Poster switch is) — add it here, don't backport it to `SortToggle` in this change.

### Persistence module (P0-2, RD1)

Mirrors `sortPreference.svelte.ts`'s `readSort`/`writeSort` shape exactly, one key:

```ts
// viewPreference.svelte.ts
const KEY = 'holodex:view:people';
const VALUES = ['list', 'poster'] as const;
type View = (typeof VALUES)[number];

export function readView(): View {
	if (typeof localStorage === 'undefined') return 'list';
	try {
		const v = localStorage.getItem(KEY);
		if (v && (VALUES as readonly string[]).includes(v)) return v as View;
	} catch {
		// Malformed or unavailable storage — fall through to the default.
	}
	return 'list';
}

export function writeView(value: View): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(KEY, value);
	} catch {
		// Storage full/unavailable — the preference just won't persist (non-fatal).
	}
}
```

A dedicated module (not folded into `sortPreference.svelte.ts`) because the allowed-value set and
default are different in kind (2 fixed literals vs. a per-page `allowed` array) — same shape,
deliberately not the same function, consistent with `density.svelte.ts` also being its own module
next to `sortPreference.svelte.ts` rather than merged into it.

### Wiring into `+page.svelte`

```svelte
let activeView = $state<'list' | 'poster'>(readView());
$effect(() => writeView(activeView));
```

Header order (left → right), inserted after the existing `SortToggle`:

```
[Merge people… (owner only)]  [SortReroll if random]  [SortToggle]  [PersonViewToggle]  [Density slider (poster view only)]
```

Render split, replacing the page's existing single list branch:

```svelte
{#if activeView === 'poster'}
	<PersonPosterGrid people={displayed} />
{:else}
	<!-- existing A–Z nav + <ul> list markup, unchanged except RD7 -->
{/if}
```

**The A–Z jump-nav stays List-only**, gated the same as today
(`sort === 'name' && !q.trim()`) — its anchors (`#pl-{letter}`) are set on `<li>` elements that
only exist in the list branch. Rendering it above the poster grid too would jump to nothing.

**Merge people… / select mode (Q1 — resolution for this handoff):** the spec left the exact
button behavior open (Q1, non-blocking) with a recommendation. This handoff locks in that
recommendation so the component is buildable — **flagged, not silently promoted to "resolved" in
the spec**:

```svelte
onclick={() => {
	activeView = 'list';   // auto-switch so the checkbox affordance (list-only, RD2) is visible
	selecting = true;
}}
```

If the Merge button is clicked while in Poster view, the page switches to List and enters select
mode in one action — no dead click, no hidden checkbox UI to design for Poster view. Confirm
before merging; if the answer changes, only this one `onclick` handler changes.

---

## Surface 2 — `PersonPosterCard`

**New file:** `web/src/lib/components/person/PersonPosterCard.svelte`. Update that folder's
`CLAUDE.md` table in the same change (per the folder's own convention).

### Markup

```svelte
<script lang="ts">
	import type { Person } from '$lib/types';
	import PersonImageFrame from './PersonImageFrame.svelte';

	let { person, eager = false }: { person: Person; eager?: boolean } = $props();
	const hasPoster = $derived((person.poster_version ?? 0) > 0);
</script>

<a href={`/people/${person.id}`} class="poster-card group relative block">
	<PersonImageFrame
		personId={person.id}
		role="poster"
		name={person.name}
		version={person.poster_version}
		{eager}
		frameClass={`portrait-frame--2x3 w-full poster-card-frame ${hasPoster ? 'poster-card-frame--photo' : ''}`}
	/>
	<div class="space-y-0.5 pt-1.5">
		<h3 class="skin-title line-clamp-1 text-sm font-medium text-ink" title={person.name}>
			{person.name}
		</h3>
		<span class="text-xs text-muted">{person.video_count}</span>
	</div>
</a>
```

`poster-card` carries `group relative` (needed for the hover z-lift and `group-focus-visible`
below); `poster-card-frame`/`poster-card-frame--photo` are new, scoped classes added to `app.css`
(next to the existing `.portrait-frame` block) — **not** modifications to `.portrait-frame`
itself, which stays exactly as-is for every other caller (`PersonAvatar`, `PersonBanner`, the
person-detail `PersonPoster`, none of which get this card's hover/border behavior).

### CSS (new, additive — `app.css`, filed next to the `.portrait-frame` block)

```css
/* ---- Person poster-grid card (F55) — hover/focus/conditional-border layered onto
   .portrait-frame, scoped to this card only so PersonAvatar/PersonBanner/the
   person-detail PersonPoster are unaffected. */
.poster-card-frame {
	border-color: var(--rule); /* placeholder default — RD3 */
	transition: transform 0.15s ease-out, box-shadow 0.15s ease-out;
}
.poster-card-frame--photo {
	border-color: transparent; /* RD3: no border once a real poster exists */
}
.poster-card:hover .poster-card-frame,
.poster-card:focus-visible .poster-card-frame {
	transform: scale(1.045);
	box-shadow: 0 8px 20px -4px rgba(0, 0, 0, 0.35);
}
.poster-card:hover,
.poster-card:focus-visible {
	z-index: 1; /* lifted card should overlap neighbors, not sit under them */
}
.poster-card:focus-visible .poster-card-frame {
	outline: 2px solid var(--accent);
	outline-offset: 2px;
}
```

**Why `border-color: transparent`, not `border: none`, for the photo state:** removing the border
entirely shifts the box by 2px (border-box sizing), which would nudge every adjacent card at
whatever moment the poster image finishes loading (headshot vs. poster load timing can differ).
Keeping a transparent 1px border keeps the box stable; RD3's "no border" is a visual statement,
not a box-model one.

**Why the frame scales, not the `<a>`:** scaling the outer link would also scale the text block
below it (typography growing on hover reads as a bug, not a lift). Scaling only `.poster-card-frame`
keeps the name/count static while the image lifts — this matches the mockup's final "Conditional +
lift" style, where only the image plate moves.

### States

| State | Visual |
|---|---|
| Default, poster present (`poster_version > 0`) | No border (`border-color: transparent`); flat `.portrait-frame` background never shows through since the image covers it |
| Default, no poster yet (`poster_version` `0`/absent) | `1px solid var(--rule)` border around the themed placeholder |
| Hover (mouse, either border state) | `scale(1.045)` + soft shadow, `z-index: 1` |
| `:focus-visible` (keyboard, either border state) | Same lift as hover **plus** `2px solid var(--accent)` outline, `2px` offset |
| Regular `:focus` (mouse click) | No visible ring — `:focus-visible` only, so a mouse click doesn't leave a ring sitting on the card (matches the rest of the app's focus-ring convention) |

### Accessibility

- The whole card is one `<a>` — no separate focusable image and text target. Tab order follows
  DOM order (grid reading order, left-to-right/top-to-bottom, same as any CSS grid).
- `alt` text on the poster image comes from `PersonImageFrame`'s default (`name`) — **not** `""`;
  unlike `PersonPoster` on the person-detail page (which sets `alt=""` because a headshot already
  announces the name alongside it), this card has no sibling headshot, so the poster image itself
  must announce the person.
- This is a **new** affordance, not a restyle: no poster-style `.portrait-frame` consumer has a
  `:focus-visible` ring today. Verify with a real Tab pass, not just inspecting the CSS — this is
  the one item in this feature with a correctness bar beyond "looks right."

---

## Surface 3 — `PersonPosterGrid`

**New file:** `web/src/lib/components/person/PersonPosterGrid.svelte`. Mirrors
[`VideoGrid.svelte`](../../web/src/lib/components/video/VideoGrid.svelte) structurally.

```svelte
<script lang="ts">
	import type { Person } from '$lib/types';
	import PersonPosterCard from './PersonPosterCard.svelte';
	import { mediaDensity, viewportTierCap } from '$lib/density.svelte';

	let { people, empty = 'No people.' }: { people: Person[]; empty?: string } = $props();

	// RD8: same shared density value as VideoGrid, doubled — People's 2:3 poster
	// reads fine at roughly half a 16:9 thumbnail's width, so doubling the column
	// count keeps both grids feeling comparably dense at any slider position.
	const cols = $derived(Math.min(mediaDensity.value * 2, viewportTierCap.value * 2));
</script>

{#if people.length === 0}
	<p class="py-16 text-center text-sm text-muted">{empty}</p>
{:else}
	<div
		class="people-poster-grid grid gap-4"
		style={`grid-template-columns: repeat(${cols}, minmax(0, 1fr))`}
	>
		{#each people as person, i (person.id)}
			<PersonPosterCard {person} eager={i < 12} />
		{/each}
	</div>
{/if}
```

**Q2 resolution for this handoff:** derive the doubled cap from `viewportTierCap.value * 2` at
read time (shown above) rather than hand-maintaining a second tier table. The spec left this an
open, low-stakes engineering call; deriving it means `PersonPosterGrid`'s columns can't drift out
of the stated 2:1 ratio if `density.svelte.ts`'s `TIERS` ever change, at the cost of no
independent control if a future design ever *wants* People's ratio to stop being exactly 2×. No
independent control has been requested — derive it.

`eager={i < 12}` (vs. `VideoGrid`'s implicit lazy-by-default via `VideoCard`'s own `loading`
attribute) — pick a number that covers the widest single-viewport tier's first row-and-a-half
(12 cols × ~1.5 rows) so above-the-fold posters decode eagerly; everything below stays
`loading="lazy"` (the default on `<img>` inside `PersonImageFrame` when `eager` is unset).

### Load-in animation

Reuse `reel-rise` (`app.css:472-479`) exactly — add the same stagger rule for
`.people-poster-grid > *`, mirroring `.video-grid > *`'s existing block (`app.css:461-471`) rather
than inventing a second keyframe:

```css
@media (prefers-reduced-motion: no-preference) {
	.people-poster-grid > * {
		animation: reel-rise 0.5s cubic-bezier(0.2, 0.7, 0.2, 1) both;
	}
	.people-poster-grid > *:nth-child(1) { animation-delay: 0ms; }
	.people-poster-grid > *:nth-child(2) { animation-delay: 40ms; }
	/* … same nth-child stagger table as .video-grid, through :nth-child(6) then :nth-child(n+7) */
}
```

### Responsive behavior

| Breakpoint | `viewportTierCap.value` (video) | People cap (`× 2`) | Density-clamped `cols` |
|---|---|---|---|
| ≥1536px | 6 | **12** | `min(mediaDensity.value × 2, 12)` |
| 1280–1535px | 4 | **8** | `min(mediaDensity.value × 2, 8)` |
| 1024–1279px | 3 | **6** | `min(mediaDensity.value × 2, 6)` |
| 480–1023px | 2 | **4** | `min(mediaDensity.value × 2, 4)` |
| <480px | 1 (fallback) | **2** | `min(mediaDensity.value × 2, 2)` |

`density.svelte.ts` itself is **not modified** — `viewportTierCap` and `mediaDensity` are read
as-is; `PersonPosterGrid` only doubles the cap locally, per RD8/the corrected Q2 above.

---

## Surface 4 — Density slider (poster view only)

**File:** `web/src/routes/people/+page.svelte`, rendered only when `activeView === 'poster'`.
Exact markup, copied from the real slider on the media list
([`web/src/routes/+page.svelte:343-366`](../../web/src/routes/+page.svelte)) with only the wrapper
width changed (the header row here is a flex row of controls, not a filter sidebar column):

```svelte
{#if activeView === 'poster'}
	<div class="flex items-center gap-2">
		<svg class="h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
			<rect x="3" y="3" width="7" height="7" rx="1" />
			<rect x="14" y="3" width="7" height="7" rx="1" />
			<rect x="3" y="14" width="7" height="7" rx="1" />
			<rect x="14" y="14" width="7" height="7" rx="1" />
		</svg>
		<input
			type="range"
			min={DENSITY_MIN}
			max={DENSITY_MAX}
			step="1"
			aria-label="Grid density"
			value={invertDensity(mediaDensity.value)}
			oninput={(e) => (mediaDensity.value = invertDensity(Number(e.currentTarget.value)))}
			class="accent-accent"
		/>
		<svg class="h-4 w-4 shrink-0 text-muted" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
			<rect x="4" y="4" width="16" height="16" rx="2" />
		</svg>
	</div>
{/if}
```

Same `min`/`max`/`invertDensity()` as the media list — **the range input's own scale is
unchanged** (still 2–6, the raw `mediaDensity` value); only `PersonPosterGrid`'s consumption of
that value is doubled (Surface 3). Dragging the People slider changes the *same* stored number the
media list slider changes — moving it on `/people` and then visiting `/` shows the video grid
reacting too, which is the intended RD8 behavior, not a bug to guard against.

No new icon assets — reuses the two inline `<svg>` blocks verbatim (small 4-square / large
1-square), since they're generic "density" glyphs with no People-specific meaning to redraw.

---

## Surface 5 — List view: flush avatar padding (RD7)

**File:** `web/src/routes/people/+page.svelte`, the `<a>`/`<label>` row wrapper
(`+page.svelte:202` and `:217-222`).

| Property | Before | After |
|---|---|---|
| Row wrapper padding | `px-4 py-2.5` (all sides) | `py-0 pl-0 pr-4` — top/left/bottom flush, only right padding (before the text column) remains |
| Avatar | unchanged (`PersonAvatar size="sm"`, `w-12`) | unchanged |
| Gap between avatar and text (`gap-3`) | unchanged | unchanged |

```svelte
<!-- both the selecting (label) and default (a) row wrappers get the same class change -->
class="flex items-center gap-3 rounded-theme border border-rule bg-surface py-0 pl-0 pr-4 text-ink hover:border-accent"
```

The border/background/hover stay on the outer wrapper exactly as today — only the padding
shorthand changes. The avatar now sits flush in the row's top-left corner, butting directly
against the rounded corner and the row's border, with the name/count text still comfortably
inset on the right via `pr-4` + the existing `gap-3`.

---

## Surface 6 — Backend: `poster_version` (P0-6)

**Files:** `internal/repo/repo.go` (`ListPeople`), `internal/model/model.go` (`Person`),
`web/src/lib/types.ts` (`Person`).

Mirror the existing `headshot_id`/`HeadshotVersion` plumbing exactly, for `role = 'poster'`:

```go
// internal/repo/repo.go — ListPeople, alongside the existing headshot_id subquery
(SELECT id FROM person_images WHERE person_id = e.id AND role = 'poster') AS poster_id
```

```go
// internal/model/model.go — Person struct, alongside HeadshotVersion
PosterVersion int64 `json:"poster_version,omitempty"`
```

```ts
// web/src/lib/types.ts — Person interface
poster_version?: number;
```

`PersonPosterCard`'s `hasPoster` check (Surface 2) reads `person.poster_version`, never
`person.headshot_version` — the two roles are independently fillable (spec P0-6/RD3's whole
point); using the wrong field would render a person with only a headshot as borderless while
still showing the placeholder image, which is the exact bug this field exists to prevent.

---

## Design tokens used (all surfaces)

| Token class | CSS var | Usage here |
|---|---|---|
| `.portrait-frame--2x3` | `--surface-2`, `--rule`, `--radius` | poster card image well (existing, unmodified) |
| `.poster-card-frame` / `--photo` (new) | `--rule` | conditional border (RD3) |
| `border-accent` outline / `--accent` | `--accent` | `:focus-visible` ring (RD4) |
| `bg-accent` / `text-accent-ink` | `--accent`, `--accent-ink` | `PersonViewToggle` / `SortToggle`-style active segment |
| `border-rule` / `text-muted` / `hover:text-ink` | `--rule`, `--muted`, `--ink` | toggle inactive/hover segments |
| `rounded-theme` | `--radius` | toggle container corners (square in Broadcast/Brutalist) |
| `accent-accent` (native range input) | `--accent` | density slider thumb/track |
| `skin-title` | per-skin heading font | card name (mirrors `VideoCard`'s `<h3>`) |

No new color literals anywhere in this feature — the only new CSS (`.poster-card-frame` block,
Surface 2) references `var(--rule)`/`var(--accent)`, and the hover shadow reuses the same
un-tokenized black-opacity convention already established by `VideoCard`'s `shadow-xs`/
`drop-shadow-lg` (Holodex doesn't tokenize shadow color today — not introduced here either).

## Edge cases

- **Person with neither headshot nor poster:** placeholder + border (Surface 2's "no poster yet"
  row) — same server-guaranteed placeholder contract as today, no broken-image state possible.
- **Very long name:** `line-clamp-1` on the card title (tighter than `VideoCard`'s
  `line-clamp-2`, since the card is narrower and denser) — `title={person.name}` gives the full
  name on hover via native tooltip, same pattern as `VideoCard`.
- **`video_count` of 0:** renders as `0`, not hidden — matches today's List view, which shows the
  count unconditionally.
- **Density slider dragged to an extreme while few people exist:** `PersonPosterGrid`'s
  `grid-template-columns` still creates the requested column count; extra tracks just have no
  card in them (CSS grid default behavior) — no empty-cell placeholder needed, matches
  `VideoGrid`'s existing behavior at low item counts.
- **Reduced motion:** `reel-rise` and the hover-lift transform both respect
  `prefers-reduced-motion` — the stagger animation is already gated (`@media
  (prefers-reduced-motion: no-preference)`); the hover-lift `transition` in Surface 2's CSS should
  gain the same guard (wrap the `transition` declaration in the same media query, matching
  `.portrait-frame > img.is-loaded`'s existing pattern at `app.css:352-356`) so a reduced-motion
  user still gets the border/ring state changes instantly, without the scale/shadow animating in.

## Gate status (mirrors the spec)

- [x] `/design-handoff` — this document
- [ ] `/testing-strategy`
- [x] `/security-review` — not required (see spec routing rationale)
- [x] `/architecture` — not required (see spec routing rationale)
