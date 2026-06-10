<script lang="ts">
	import type { Snippet } from 'svelte';

	// Shared loading/error shell so route pages don't each re-implement the
	// "Loading…" + red error box choreography.
	let {
		loading,
		error,
		loadingText = 'Loading…',
		children
	}: { loading: boolean; error?: string; loadingText?: string; children: Snippet } = $props();
</script>

{#if loading}
	<p class="py-16 text-center text-sm text-muted">{loadingText}</p>
{:else if error}
	<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">{error}</p>
{:else}
	{@render children()}
{/if}
