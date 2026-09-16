<script lang="ts">
	// Films row (F56, design handoff §5): a small horizontal shelf of poster-thumb cards
	// on person/studio/tag detail pages, rendered through EntityVideos' `footer` snippet.
	// Renders nothing when empty — callers only pass films once films_enabled is on and
	// the entity's film union is non-empty, so there's never an informational dead end.
	//
	// HOLODEX-384: a film tile is a video card's sibling, not a chip. Its poster is exactly
	// as tall as the `.video-frame`s in the grid above (the width follows from 2:3), it wears
	// the frame's border/radius and VideoCard's caption block, and it lifts on hover through
	// the same `.media-lift` hook as the person hero. The sizing maths lives in app.css
	// (`.films-shelf`); this component only supplies the two inputs the grid renders with —
	// the column count and the card layout — so the shelf and the grid can't disagree.
	// EntityVideos' grid is never stage-aligned (that mode is the film page's Scenes list
	// only), so the plain `repeat(cols, minmax(0,1fr))` track width is the right target.
	import { monogram } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import { effectiveDensity } from '$lib/density.svelte';
	import type { Film } from '$lib/types';

	let { films }: { films: Film[] } = $props();

	const cols = $derived(effectiveDensity());
</script>

{#if films.length}
	<section class="films-shelf" style="--cols: {cols}" data-layout={activity.cardLayout}>
		<h2 class="text-xs uppercase tracking-wide text-muted">Films</h2>
		<!-- The shelf scrolls horizontally, and a scroll container clips its children on
		     every edge — without slack the 6% lift is cut off. `--lift-slack` (app.css) scales
		     with the tile so a 900px poster gets the same proportional room as a 160px one.
		     The negative side/bottom margins hand that space back so the tiles still sit on
		     the grid's left edge and the row's footprint doesn't grow; the top padding doubles
		     as the heading gap (no `space-y` here — a negative top margin would collapse
		     against it and pull the tiles into the heading). -->
		<ul class="-mx-(--lift-slack) -mb-(--lift-slack) flex gap-3 overflow-x-auto p-(--lift-slack)">
			{#each films as f (f.id)}
				<li class="media-lift group w-(--film-w) shrink-0">
					<a href={`/films/${f.id}`} class="block text-ink" title={f.name}>
						<!-- HOLODEX-383: same defect as HOLODEX-318 on the films index — this drew
						     the monogram unconditionally, so a film with a poster showed art on
						     /films and a letter here. `poster_url` is populated on the list read
						     (types.ts), so the monogram is the empty state, not the default. -->
						<div
							class="flex aspect-[2/3] items-center justify-center overflow-hidden rounded-theme border border-rule bg-logo-plate transition group-hover:border-accent group-focus-within:border-accent"
						>
							{#if f.poster_url}
								<img src={f.poster_url} alt="" loading="lazy" class="h-full w-full object-cover" />
							{:else}
								<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true"
									>{monogram(f.name)}</span
								>
							{/if}
						</div>
						<!-- VideoCard's title block (p-3 / text-sm font-medium line-clamp-2) so the two
						     rows' bottoms align as well as their frames; horizontal padding is trimmed
						     because the tile is ~3/8 of a card wide. `break-words` so a long single
						     word wraps inside the clamp instead of being cut mid-glyph. -->
						<div class="px-1 py-3">
							<span
								class="skin-title line-clamp-2 break-words text-sm font-medium text-ink group-hover:text-accent group-focus-within:text-accent"
								>{f.name}</span
							>
						</div>
					</a>
				</li>
			{/each}
		</ul>
	</section>
{/if}
