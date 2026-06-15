# Design Handoff: Quick Wins — overlay fix, search history, "More with …" shelves

**Status**: Draft (developer handoff)
**Date**: 2026-06-14
**Spec**: [`docs/specs/quick-wins.md`](../specs/quick-wins.md) (overlay bugfix · QW1 · QW2/QW3)
**Architecture**: [ADR-031](../architecture/ADR-031-related-media-endpoint.md) (related-media endpoint)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [`theming.md`](theming.md) — **tokens only, QA all three skins**

This handoff covers the three UI surfaces in the Quick Wins batch: the **media-page
atmosphere overlay fix**, the **search-history dropdown** (QW1), and the shared
**"More with …" shelf** (QW3). All markup below is **tokens only** — no `zinc-*`,
`sky-*`, hex, or fixed `rounded-lg`/`px` radii. Skin flourishes live in `app.css`
gated by `[data-theme]`, never in component markup.

> **Skin reminders that bite these surfaces:** Broadcast & Brutalist set `--radius: 0`
> (everything `rounded-theme` is square). Broadcast appends a `▮` caret after every
> `.skin-title` and washes scanlines over `.app-atmosphere::after` and `.video-frame`.
> Brutalist numbers `.video-frame` cards via a CSS `counter()` reset on `.video-grid`.
> Each surface below calls out where these interact.

---

## Surface 1 — Atmosphere overlay fix (media playback)

**The bug.** `.app-atmosphere::after` ([`app.css:112`](../../web/src/app.css)) is a
`position: fixed; inset: 0; z-index: 40; pointer-events: none` pseudo-element on
`<body class="app-atmosphere">`. Its skin flourishes — Cinémathèque grain + vignette,
**Broadcast scanlines + CRT vignette** (worst), Brutalist none — paint over the entire
viewport, *including the playing `<video>`* on the media detail page. There is no way
to "watch cleanly": the scanlines/vignette sit on top of the picture.

**Decision — suppress the atmosphere while a media video plays, pure-CSS-gated.**
The detail-page `<video>` toggles a single state class on `<body>`; `app.css` owns the
hide rule. No per-component overlay markup, no z-index war.

**CSS (add to the `@layer components` atmosphere block in [`app.css`](../../web/src/app.css), right after the `.app-atmosphere::after` base rule):**
```css
/* While a media video is playing, drop the atmosphere so the picture is clean.
   Toggled by the media detail page via the .is-playing class on <body>. */
.app-atmosphere.is-playing::after {
	display: none;
}
```
> `display: none` (not `opacity: 0`) so the Broadcast `box-shadow` vignette is fully
> gone, not just faded. The rule is skin-agnostic — it applies to all three because it
> targets the shared `.app-atmosphere::after`, so each skin's flourish disappears
> together.

**Handler wiring in [`media/[id]/+page.svelte`](../../web/src/routes/media/[id]/+page.svelte)**
— add `onplay` / `onpause` / `onended` to the existing `<video>` (lines 73–82) and a
teardown so navigating away mid-play restores the overlay:
```svelte
<script lang="ts">
	// …existing state…

	function setPlaying(on: boolean) {
		// Guard for SSR / no-document; the class lives on <body class="app-atmosphere">.
		document.body?.classList.toggle('is-playing', on);
	}

	// Restore the atmosphere if we unmount (route change) while still "playing".
	$effect(() => () => setPlaying(false));
</script>

<video
	src={api.streamURL(video.id)}
	poster={…}
	controls
	preload="metadata"
	class="aspect-video w-full bg-black"
	onplay={() => setPlaying(true)}
	onpause={() => setPlaying(false)}
	onended={() => setPlaying(false)}
	onerror={() => (playFailed = true)}
></video>
```
> The `playFailed` fallback branch has no `<video>`, so no handler is needed there.
> Only the media detail page toggles `.is-playing`; no other page references it.

**States**

| State | Atmosphere | Trigger |
|-------|-----------|---------|
| Not playing / paused / ended | Visible (per skin) | `onpause`, `onended`, initial |
| Playing | Hidden (`display:none`) | `onplay` |
| Navigate away mid-play | Restored | `$effect` teardown |

**3-skin QA**
- **Cinémathèque:** grain + vignette gone during playback; returns on pause.
- **Broadcast:** scanlines **and** CRT vignette gone during playback (the load-bearing
  case) — picture fully clean; both return on pause/end.
- **Brutalist:** no atmosphere flourish to begin with — confirm the toggle is a no-op
  (no layout shift, no flicker) and nothing else moves.
- All skins: start play → overlay off; pause → overlay on; let it end → overlay on;
  hit ← Back mid-play → overlay on at the grid.

---

## Surface 2 — Search-history dropdown (QW1)

A dropdown anchored under the header search `<form>` in
[`+layout.svelte`](../../web/src/routes/+layout.svelte) (lines 46–53). The `<form>` is
already `relative max-w-md flex-1`, so the panel is `absolute` + full-width beneath the
input. The history list itself (read/write, dedupe, cap 10) lives in a small module
(`web/src/lib/searchHistory.ts` — `localStorage` key `holodex-search-history`); this
handoff covers only the **panel UI**.

**Behavior**

| Property | Value |
|----------|-------|
| Opens | On input **focus** *and* the input is **empty** *and* `history.length > 0` |
| **Hides on typing** | The moment `searchTerm` is non-empty, the panel closes — history is for recalling from an empty box, **not** filter-as-you-type. Clearing the box and re-focusing reopens it. *(Leaves the "typing" state free for a future autocomplete surface to own without reworking history.)* |
| Closes | Typing (above), Esc, blur (with a click-guard so clicking a row registers first), or after a selection |
| Keyboard | ↓/↑ move highlight; Enter runs highlighted (or submits the typed term if none highlighted); Esc closes |
| Rows | Up to 10, most-recent-first; each = query text + `×` remove control |
| Footer | "Clear history" row |
| Empty history | **No panel at all** (never an empty floating box) |

**Positioning & z-index.** The panel sits in normal header flow (the header is **not**
inside `.app-atmosphere`'s stacking trap — the overlay is `position: fixed` and
`pointer-events: none`, so it never intercepts clicks). A local `z-50` on the panel
keeps it above page content and above the `z-40` atmosphere overlay; `pointer-events`
on the overlay is `none` regardless, so interaction is safe either way.

**Markup (token-only) — replaces the bare `<form>` block:**
```svelte
<form onsubmit={submitSearch} class="relative max-w-md flex-1">
	<input
		bind:this={searchInput}
		bind:value={searchTerm}
		onfocus={openHistory}
		onkeydown={onSearchKeydown}
		placeholder="Search everything…  (Ctrl-K)"
		class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
	/>

	<!-- Open only on a focused, EMPTY box: `!searchTerm.trim()` hides the panel the
	     instant the user types (no filter-as-you-type), per the resolved design Q. -->
	{#if historyOpen && !searchTerm.trim() && history.length}
		<ul
			role="listbox"
			class="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-theme border border-rule bg-surface shadow-lg"
		>
			{#each history as q, i (q)}
				<li role="option" aria-selected={i === active}>
					<div
						class="flex items-center gap-2 px-3 py-1.5 text-sm {i === active
							? 'bg-surface-2 text-ink'
							: 'text-ink hover:bg-surface-2'}"
					>
						<button
							type="button"
							class="flex-1 truncate text-left"
							onmousedown={() => runQuery(q)}
						>
							{q}
						</button>
						<button
							type="button"
							aria-label={`Remove "${q}" from history`}
							class="shrink-0 text-muted hover:text-ink"
							onmousedown={(e) => { e.stopPropagation(); removeQuery(q); }}
						>
							×
						</button>
					</div>
				</li>
			{/each}
			<li>
				<button
					type="button"
					class="block w-full border-t border-rule px-3 py-1.5 text-left text-xs text-muted hover:text-ink"
					onmousedown={clearHistory}
				>
					Clear history
				</button>
			</li>
		</ul>
	{/if}
</form>
```
> Use `onmousedown` (not `onclick`) for the row actions so the action fires **before**
> the input's `blur` closes the panel. `runQuery(q)` sets `searchTerm = q` then
> `goto(\`/search?q=…\`)` (same path as `submitSearch`) and closes the panel.

**States**

| State | Visual |
|-------|--------|
| Default row | `text-ink`, transparent bg |
| Hover | `bg-surface-2` |
| Keyboard-active | `bg-surface-2 text-ink` (matches hover; single highlight model) |
| `×` remove | `text-muted` → `text-ink` on hover |
| Clear footer | `text-xs text-muted`, separated by `border-t border-rule` |

**Accessibility.** Input is the combobox; `role="listbox"` on the panel, `role="option"`
+ `aria-selected` on rows. ↓/↑ adjust `active`; Enter runs; Esc closes and returns focus
to the input.

**3-skin QA**
- **Cinémathèque:** rounded panel + rows (`rounded-theme` honored), accent border on
  input focus reads.
- **Broadcast / Brutalist:** panel and input are **square** (`--radius: 0`) — confirm the
  overflow-hidden corners look intentional, not clipped. The rows are **not** `.skin-title`,
  so the Broadcast `▮` caret must **not** appear on any query text — verify.
- All skins: active-row `bg-surface-2` is distinguishable from the `bg-surface` panel;
  the `×` and "Clear history" are legible against the panel; nothing collides with the
  Ctrl-K placeholder.

---

## Surface 3 — "More with …" shelf (`RelatedShelf.svelte`, QW3)

A new shared component rendered **twice** at the bottom of
[`media/[id]/+page.svelte`](../../web/src/routes/media/[id]/+page.svelte) — person shelf
first, then tag shelf — fed by `GET /api/v1/media/{id}/related` (ADR-031). It closely
mirrors the existing [`RecentlyAddedShelf.svelte`](../../web/src/lib/components/RecentlyAddedShelf.svelte):
a `.skin-title` heading over a horizontal scroll of [`VideoCard`](../../web/src/lib/components/VideoCard.svelte).

### Props
| Prop | Type | Description |
|------|------|-------------|
| `title` | `string` | Entity name — rendered as "More with `{title}`" |
| `href` | `string` | Entity page link (`/people/{id}` or `/tags/{id}`) |
| `items` | `Video[]` | Up to 5 related items (already excludes the current item) |

### Omission rule (load-bearing)
The shelf renders **only** when `items.length > 0`. A null block or empty `items`
renders **nothing** — no heading, no skeleton-forever, no "nothing here" text. The
parent decides per block; the component self-omits as a guard.

### The Brutalist counter fix (must-do)
The Brutalist catalog number comes from `counter-reset: reel` on `.video-grid` +
`counter-increment` on `.video-frame` ([`app.css:191`](../../web/src/app.css)). If a
shelf's cards are **not** wrapped in a `counter-reset` context, the numbering
**continues from the main grid** (or from the previous shelf) — e.g. the tag shelf would
start at `06`. **Wrap each shelf's card row in `.video-grid`** so the counter restarts
per shelf (`01…05`). `RecentlyAddedShelf` predates this and is a known minor
inconsistency; `RelatedShelf` does it right.

### Markup (token-only)
```svelte
<script lang="ts">
	import type { Video } from '$lib/types';
	import VideoCard from './VideoCard.svelte';

	let { title, href, items }: { title: string; href: string; items: Video[] } = $props();
</script>

{#if items.length > 0}
	<section class="space-y-2">
		<h2 class="skin-title text-sm font-semibold uppercase tracking-wide text-muted">
			More with
			<a {href} class="text-ink hover:text-accent">{title}</a>
		</h2>
		<!-- .video-grid resets the Brutalist `reel` counter so numbering restarts at 01 per shelf. -->
		<div class="video-grid flex gap-4 overflow-x-auto pb-2">
			{#each items as video (video.id)}
				<div class="w-52 shrink-0 sm:w-56">
					<VideoCard {video} />
				</div>
			{/each}
		</div>
	</section>
{/if}
```

### Parent wiring (`media/[id]/+page.svelte`)
Fetch `/related` **non-blocking** in a separate `$effect` (do not gate the primary
detail render on it). Reuse the page's existing card shimmer indirectly — `VideoCard`
already shimmers each thumbnail while it loads, so no shelf-level skeleton is needed; if
the whole `/related` call is in flight, simply render nothing until it resolves.
```svelte
<script lang="ts">
	import RelatedShelf from '$lib/components/RelatedShelf.svelte';
	let related = $state<{ person: RelatedBlock | null; tag: RelatedBlock | null } | null>(null);

	$effect(() => {
		const current = id;
		related = null;
		api.related(current)
			.then((r) => (related = r))
			.catch(() => (related = null)); // silently omit on error — non-blocking
	});
</script>

<!-- …after the existing detail sections, still inside <article>… -->
{#if related?.person}
	<RelatedShelf title={related.person.name} href={`/people/${related.person.id}`} items={related.person.items} />
{/if}
{#if related?.tag}
	<RelatedShelf title={related.tag.name} href={`/tags/${related.tag.id}`} items={related.tag.items} />
{/if}
```
> **Stable per page view (resolved design Q).** The `$effect` must track **only `id`** —
> read `const current = id;` first and reference nothing else reactive — so the fetch
> runs **once per media-page view** and the shelves do **not** reshuffle on incidental
> re-renders (skin switch, thumbnail regenerate). Navigating to a different item changes
> `id` → one fresh fetch → a new draw. The server stays per-request random
> ([ADR-031](../architecture/ADR-031-related-media-endpoint.md)); holding the result
> client-side is what makes the shelf stable while viewing. *(A hard reload is a new page
> view, so it re-draws — accepted.)*

### States
| State | Behavior |
|-------|----------|
| Loading (`/related` in flight) | Render nothing; primary detail content fully usable |
| Per-card thumbnail loading | `VideoCard` shimmer (existing) |
| Empty / null block | Shelf omitted entirely |
| Error | `related = null` → both shelves omitted; page unaffected |
| Populated | Heading + up to 5 cards, horizontal scroll |

### 3-skin QA
- **Cinémathèque:** cards show letterbox bars (`.video-frame::before/::after`); heading
  uses the display face; rounded corners honored.
- **Broadcast:** scanline wash reads over each card still; `▮` caret appears after the
  heading (it **is** a `.skin-title`) — confirm it sits after "More with `{title}`" and
  doesn't crowd the link; square corners.
- **Brutalist:** **catalog counter restarts at `01` on each shelf** (the whole point of
  the `.video-grid` wrap) — verify the person shelf is `01…05` and the tag shelf is
  **also** `01…05`, not `06…`. Heading uppercased; square corners.
- All skins: cards are **visually identical** to the browse grid (same `VideoCard`);
  horizontal scroll works; clicking a card → `/media/{id}`; clicking the heading link →
  entity page.

---

## Surface 4 — Fluid Back navigation (QW4)

Mostly behavioral (see [ADR-032](../architecture/ADR-032-browse-state-preservation.md)),
but it carries a hard **UX contract** worth pinning here because it's the difference
between "feels native" and "feels like a website reload":

- **No loading state on Back.** Returning to the browse grid must **not** flash the
  `Loading…` text ([`+page.svelte:250`](../../web/src/routes/+page.svelte)) or the
  empty-grid placeholder, and must **not** show per-card thumbnail shimmer for cards that
  were already loaded. The grid paints from cache fully formed.
- **No scroll jump.** The restore lands at the saved Y directly — no visible "top, then
  scroll down to position." Restore happens in `afterNavigate`, after layout, so there's
  one paint at the right place.
- **Filter change is *not* a Back restore** — changing a filter/sort legitimately resets
  to the top of a fresh result set (cache invalidated). That reset is expected and should
  feel like a new query, distinct from Back.
- **Skin-agnostic.** This surface adds **no markup and no styling** — it touches only the
  grid's data/scroll lifecycle. There is nothing per-skin to theme; the 3-skin QA below
  is just confirming the *absence* of regressions (no flicker, grid flourishes still
  render after a cached restore).

> Implementation note for the developer: the only visible-state risk is the existing
> `loading`/`loadingMore` flags briefly flipping true on a cached return. Seed the grid
> from the `browse.svelte.ts` store **before** the mount `$effect` would set
> `loading = true`, and short-circuit the page-0 fetch when the cache signature matches
> (per ADR-032). Verify with the network panel: **zero** `GET /api/v1/media` on Back.

---

## QA checklist (all three surfaces × three skins)

> Switch skins via the header picker. Tick each per skin.

### Overlay fix
- [ ] **Cinémathèque** — grain/vignette hidden during playback, restored on pause/end.
- [ ] **Broadcast** — scanlines **and** vignette hidden during playback (clean picture), restored on pause/end.
- [ ] **Brutalist** — toggle is a no-op; no layout shift / flicker on play/pause.
- [ ] **All** — navigate ← Back mid-play → overlay restored on the grid.

### Search-history dropdown
- [ ] **Cinémathèque** — rounded panel/rows; focus accent border reads.
- [ ] **Broadcast** — square panel; **no `▮` caret** on any query row; active row distinguishable.
- [ ] **Brutalist** — square panel; rows legible; `×` and "Clear history" legible.
- [ ] **All** — opens on focus only when non-empty; ↓/↑/Enter/Esc work; click runs query before blur closes it; empty history shows no panel.

### "More with …" shelves
- [ ] **Cinémathèque** — letterbox bars on cards; heading display face.
- [ ] **Broadcast** — scanline wash on cards; `▮` caret after heading; square cards.
- [ ] **Brutalist** — **counter restarts `01…05` per shelf** (person and tag both); uppercased heading.
- [ ] **All** — cards identical to browse grid; empty/null block omits the shelf; loading/error never blocks the primary detail content; heading + card links navigate correctly.

### Fluid Back (QW4)
- [ ] **All skins** — scroll the grid, open an item, Back → **same scroll position**, no jump-to-top.
- [ ] **All skins** — "Load more" ×2 (150 items), open an item, Back → all 150 still rendered, item on screen.
- [ ] **All skins** — Back shows **no `Loading…` flash** and fires **zero** `GET /api/v1/media` (check network panel).
- [ ] **All skins** — change a filter → resets to top of a fresh set (cache invalidated); hard reload → top of page 0.
- [ ] **All skins** — grid flourishes (letterbox / scanline / counter) still render correctly after a cached restore (no flicker).
