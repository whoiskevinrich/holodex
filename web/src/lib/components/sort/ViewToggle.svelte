<script lang="ts" generics="V extends AnyView">
	// The list toolbar's View slot (F55, F82): one segmented control for every list page,
	// over that page's views from viewPreference's PAGE_VIEWS. Each cell carries
	// aria-pressed. Below `sm` a cell is a 32×32 icon (the F73 toolbar's 375px budget has no
	// room for the words); the aria-label keeps the name at every width. `onchange` fires only
	// on a click, so the page records an explicit choice and nothing else (F73 R7).
	import { segmentedToggleWrapperClass } from '#lib/components/sort/segmentedToggle.ts';
	import type { AnyView } from '#lib/viewPreference.svelte.ts';

	let {
		options,
		view = $bindable(),
		onchange
	}: { options: readonly V[]; view: V; onchange?: (v: V) => void } = $props();

	const LABELS: Record<AnyView, string> = {
		list: 'List',
		poster: 'Poster',
		grid: 'Grid',
		table: 'Table',
		chips: 'Chips',
		posters: 'Posters'
	};

	// segmentedToggleClass's px-3 can't be overridden per breakpoint (Tailwind orders
	// conflicting utilities by stylesheet, not class order), so the cell spells its own
	// padding: none in the 32px icon cell, px-3 once the word shows.
	const cls = (active: boolean) =>
		`inline-flex h-8 w-8 items-center justify-center sm:w-auto sm:px-3 ${
			active ? 'bg-accent text-accent-ink' : 'text-muted hover:text-ink'
		}`;

	function choose(v: V) {
		if (v === view) return;
		view = v;
		onchange?.(v);
	}
</script>

<div class={segmentedToggleWrapperClass}>
	{#each options as v (v)}
		<button
			onclick={() => choose(v)}
			class={cls(view === v)}
			aria-pressed={view === v}
			aria-label="{LABELS[v]} view"
		>
			<svg class="h-3.5 w-3.5 sm:hidden" viewBox="0 0 14 14" fill="none" aria-hidden="true">
				{#if v === 'list'}
					<path d="M1 3h12M1 7h12M1 11h12" stroke="currentColor" stroke-width="1.5" />
				{:else if v === 'poster' || v === 'grid'}
					<path d="M1.5 1.5h4.5v4.5H1.5zM8 1.5h4.5v4.5H8zM1.5 8h4.5v4.5H1.5zM8 8h4.5v4.5H8z" stroke="currentColor" stroke-width="1.3" />
				{:else if v === 'table'}
					<path d="M0.5 0.5h13v13h-13zM0.5 5h13M0.5 9h13M5 0.5v13" stroke="currentColor" stroke-width="1.3" />
				{:else if v === 'chips'}
					<rect x="0.75" y="3.5" width="12.5" height="7" rx="3.5" stroke="currentColor" stroke-width="1.3" />
				{:else if v === 'posters'}
					<path d="M0.5 2.5h13v8h-13zM0.5 13h13" stroke="currentColor" stroke-width="1.3" />
				{/if}
			</svg>
			<span class="hidden sm:inline">{LABELS[v]}</span>
		</button>
	{/each}
</div>
