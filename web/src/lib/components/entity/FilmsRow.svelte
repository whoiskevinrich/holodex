<script lang="ts">
	// Films row (F56, design handoff §5): a small horizontal shelf of poster-thumb cards
	// on person/studio/tag detail pages, rendered through EntityVideos' `footer` snippet.
	// Renders nothing when empty — callers only pass films once films_enabled is on and
	// the entity's film union is non-empty, so there's never an informational dead end.
	import { monogram } from '$lib/format';
	import type { Film } from '$lib/types';

	let { films }: { films: Film[] } = $props();
</script>

{#if films.length}
	<section class="space-y-1.5">
		<h2 class="text-xs uppercase tracking-wide text-muted">Films</h2>
		<ul class="flex gap-3 overflow-x-auto pb-1">
			{#each films as f (f.id)}
				<li class="w-20 shrink-0">
					<a href={`/films/${f.id}`} class="block space-y-1 text-ink" title={f.name}>
						<div
							class="flex aspect-[2/3] items-center justify-center overflow-hidden rounded-theme bg-logo-plate"
						>
							<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true"
								>{monogram(f.name)}</span
							>
						</div>
						<span class="line-clamp-2 text-xs text-muted hover:text-accent">{f.name}</span>
					</a>
				</li>
			{/each}
		</ul>
	</section>
{/if}
