<script lang="ts">
	import type { PeopleTagSort } from '$lib/types';

	// Shared A–Z / Most-videos / Random segmented control for the people & tags
	// indexes. "Random" is a seeded client-side shuffle (ADR-045).
	let { sort = $bindable() }: { sort: PeopleTagSort } = $props();

	const cls = (active: boolean) =>
		active ? 'bg-accent px-3 py-1 text-accent-ink' : 'px-3 py-1 text-muted hover:text-ink';
</script>

<div class="flex overflow-hidden rounded-theme border border-rule text-sm">
	<button onclick={() => (sort = 'name')} class={cls(sort === 'name')}>A–Z</button>
	<button onclick={() => (sort = 'count')} class={cls(sort === 'count')}>Most videos</button>
	<button onclick={() => (sort = 'random')} class={cls(sort === 'random')}>Random</button>
</div>
