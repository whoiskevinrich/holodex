<script lang="ts">
	// A smart playlist's cap (F89 P0-2), a compact select with SortDropdown's compact look.
	// A cap set some other way (the API) stays selectable, so opening it never changes it.
	let { cap, onchange }: { cap: number | null; onchange: (cap: number | null) => void } = $props();

	const CHOICES = [10, 25, 50, 100, 250, 500, 1000];
	const choices = $derived(cap != null && !CHOICES.includes(cap) ? [...CHOICES, cap].sort((a, b) => a - b) : CHOICES);
</script>

<label class="min-w-0">
	<span class="sr-only">Cap</span>
	<select
		value={cap ?? ''}
		onchange={(e) => onchange(e.currentTarget.value === '' ? null : Number(e.currentTarget.value))}
		class="h-8 max-w-full min-w-0 rounded-theme border border-rule bg-surface px-3 py-0 text-sm text-ink outline-none focus:border-accent"
	>
		<option value="">Cap: keep all</option>
		{#each choices as n (n)}
			<option value={n}>Cap: first {n.toLocaleString()}</option>
		{/each}
	</select>
</label>
