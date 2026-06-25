<script lang="ts">
	// Renders a `url`-display field's values (F27) as a comma-separated list of links.
	// Each value is scheme-gated: only http(s) becomes a clickable, new-tab link —
	// anything else falls back to inert text, since the value is provider-supplied and
	// Svelte does not sanitize `href` (see isHttpUrl). Tokens only.
	import { isHttpUrl } from '$lib/format';

	let { values }: { values: string[] } = $props();
</script>

{#each values as url, i (i)}
	{#if i > 0}<span class="text-muted">, </span>{/if}
	{#if isHttpUrl(url)}
		<a
			href={url}
			target="_blank"
			rel="noopener noreferrer"
			class="break-all text-accent hover:underline">{url}<span class="sr-only"> (opens in a new tab)</span></a>
	{:else}
		<span class="break-all text-ink">{url}</span>
	{/if}
{/each}
