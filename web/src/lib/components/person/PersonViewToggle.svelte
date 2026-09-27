<script lang="ts">
	// List/Poster display-mode toggle for the People index (F55) — a 2-way switch, so it
	// carries aria-pressed. Below `sm` each cell is a 32×32 icon (the F73 toolbar's 375px
	// budget has no room for the words); the aria-label keeps the name at every width.
	import type { PersonView } from '$lib/viewPreference.svelte';
	import { segmentedToggleWrapperClass } from '$lib/components/sort/segmentedToggle';

	let { view = $bindable() }: { view: PersonView } = $props();

	// segmentedToggleClass's px-3 can't be overridden per breakpoint (Tailwind orders
	// conflicting utilities by stylesheet, not class order), so the cell spells its own
	// padding: none in the 32px icon cell, px-3 once the word shows.
	const cls = (active: boolean) =>
		`inline-flex h-8 w-8 items-center justify-center sm:w-auto sm:px-3 ${
			active ? 'bg-accent text-accent-ink' : 'text-muted hover:text-ink'
		}`;
</script>

<div class={segmentedToggleWrapperClass}>
	<button
		onclick={() => (view = 'list')}
		class={cls(view === 'list')}
		aria-pressed={view === 'list'}
		aria-label="List view"
	>
		<svg class="h-3.5 w-3.5 sm:hidden" viewBox="0 0 14 14" aria-hidden="true">
			<path d="M1 3h12M1 7h12M1 11h12" stroke="currentColor" stroke-width="1.5" />
		</svg>
		<span class="hidden sm:inline">List</span>
	</button>
	<button
		onclick={() => (view = 'poster')}
		class={cls(view === 'poster')}
		aria-pressed={view === 'poster'}
		aria-label="Poster view"
	>
		<svg class="h-3.5 w-3.5 sm:hidden" viewBox="0 0 14 14" fill="none" aria-hidden="true">
			<path d="M1.5 1.5h4.5v4.5H1.5zM8 1.5h4.5v4.5H8zM1.5 8h4.5v4.5H1.5zM8 8h4.5v4.5H8z" stroke="currentColor" stroke-width="1.3" />
		</svg>
		<span class="hidden sm:inline">Poster</span>
	</button>
</div>
