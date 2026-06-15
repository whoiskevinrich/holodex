<script lang="ts">
	import type { Snippet } from 'svelte';

	// Generic status card (F21.4): a labelled surface panel; callers supply the body.
	let { title, children }: { title: string; children: Snippet } = $props();
</script>

<div class="space-y-2 rounded-theme border border-rule bg-surface p-4">
	<h2 class="text-xs uppercase tracking-wide text-muted">{title}</h2>
	{@render children()}
</div>
