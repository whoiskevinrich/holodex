<script lang="ts">
	// Renders a `url`-display field's values (F27) as a comma-separated list of links.
	// Each value is scheme-gated: only http(s) becomes a clickable, new-tab link —
	// anything else falls back to inert text, since the value is provider-supplied and
	// Svelte does not sanitize `href` (see isHttpUrl). Tokens only.
	//
	// `hostname` opt-in (F38): show the bare host (e.g. "legendary.com") as the link
	// text instead of the full URL — friendlier for long provider URLs like a studio's
	// TMDB fallback page. The full URL stays the href + title. Falls back to the raw
	// value if it doesn't parse.
	import { isHttpUrl } from '$lib/format';

	let { values, hostname = false }: { values: string[]; hostname?: boolean } = $props();

	function linkText(url: string): string {
		if (!hostname) return url;
		try {
			return new URL(url).hostname.replace(/^www\./, '');
		} catch {
			return url;
		}
	}
</script>

{#each values as url, i (i)}
	{#if i > 0}<span class="text-muted">, </span>{/if}
	{#if isHttpUrl(url)}
		<a
			href={url}
			target="_blank"
			rel="noopener noreferrer"
			title={hostname ? url : undefined}
			class="break-all text-accent hover:underline">{linkText(url)}<span class="sr-only">
				(opens in a new tab)</span></a>
	{:else}
		<span class="break-all text-ink">{url}</span>
	{/if}
{/each}
