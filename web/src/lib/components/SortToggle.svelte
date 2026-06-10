<script lang="ts">
	// Shared A–Z / Most-videos segmented control for the people & tags indexes.
	let { sort = $bindable() }: { sort: 'name' | 'count' } = $props();

	const cls = (active: boolean) =>
		active ? 'bg-accent px-3 py-1 text-accent-ink' : 'px-3 py-1 text-muted hover:text-ink';
</script>

<div class="flex overflow-hidden rounded-theme border border-rule text-sm">
	<button onclick={() => (sort = 'name')} class={cls(sort === 'name')}>A–Z</button>
	<button onclick={() => (sort = 'count')} class={cls(sort === 'count')}>Most videos</button>
</div>
