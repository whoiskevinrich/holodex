<script lang="ts">
	// Renders a `url`-display field's values (F27) as a comma-separated list of links.
	// Each value is scheme-gated: only http(s) becomes a clickable, new-tab link —
	// anything else falls back to inert text, since the value is provider-supplied and
	// Svelte does not sanitize `href` (see isHttpUrl). Tokens only.
	//
	// `hostname` opt-in (F38): show the bare host (e.g. "legendary.com") as the link
	// text instead of the full URL — friendlier for long provider URLs like a studio's
	// TMDB fallback page. The full URL stays the href + title.
	//
	// `provider` opt-in (HOLODEX-137, ADR-059): lead the link with the provider's brand
	// icon and show the destination HOST as the text — so the website row reads
	// "[icon] themoviedb.org" instead of a long raw URL, folding provenance into the
	// link (the caller then drops the separate ProvenanceBadge). A provider implies the
	// hostname text. Falls back to the raw value if it doesn't parse.
	import { isHttpUrl } from '$lib/format';
	import ProviderIcon from '../enrichment/ProviderIcon.svelte';
	import { providers as providerDir } from '$lib/providers.svelte';

	let {
		values,
		hostname = false,
		provider = ''
	}: { values: string[]; hostname?: boolean; provider?: string } = $props();

	const branded = $derived(!!provider);

	// Load the provider directory once so the link can lead with the real icon
	// (monogram until it resolves / when the provider has none).
	$effect(() => {
		if (provider) void providerDir.load();
	});

	function linkText(url: string): string {
		if (!hostname && !branded) return url;
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
			title={url}
			class="inline-flex items-center gap-1 break-all align-middle text-accent hover:underline"
			>{#if branded}<ProviderIcon
					name={provider}
					iconUrl={providerDir.iconUrl(provider)}
					size={14}
				/>{/if}{linkText(url)}<span class="sr-only"> (opens in a new tab)</span></a
		>
	{:else}
		<span class="break-all text-ink">{url}</span>
	{/if}
{/each}
