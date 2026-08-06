<script lang="ts">
	// List/Poster display-mode toggle for the People index (F55) — same segmented-control
	// shell as SortToggle, a 2-way switch instead of a 3-way sort so it carries aria-pressed
	// (a 3-way sort isn't a toggle in the ARIA sense; a 2-way view switch is).
	import type { PersonView } from '$lib/viewPreference.svelte';

	let { view = $bindable() }: { view: PersonView } = $props();

	const cls = (active: boolean) =>
		active ? 'bg-accent px-3 py-1 text-accent-ink' : 'px-3 py-1 text-muted hover:text-ink';
</script>

<div class="flex overflow-hidden rounded-theme border border-rule text-sm">
	<button onclick={() => (view = 'list')} class={cls(view === 'list')} aria-pressed={view === 'list'}>
		List
	</button>
	<button onclick={() => (view = 'poster')} class={cls(view === 'poster')} aria-pressed={view === 'poster'}>
		Poster
	</button>
</div>
