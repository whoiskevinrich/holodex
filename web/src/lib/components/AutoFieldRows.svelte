<script lang="ts">
	// F39 (ADR-056): renders display-only auto-registered non-canonical fields as
	// read-only rows under an "Additional details" divider, shared by the video /
	// person / studio detail pages. No source chips, no curation — for owner and
	// visitor alike. Emits grid children directly (a divider spanning both columns,
	// then one row per field), so it drops straight into each page's Details `<dl>`.
	//
	// The backend already degrades a non-allowlisted image_url to text (ADR-039), so
	// the `display` here is trusted. Values are provider-sourced (auto-registered
	// fields never come from the file/record baseline), so a ProvenanceBadge always
	// shows the supplying provider.
	import type { ResolvedField } from '$lib/types';
	import { providerFromWinningSource } from '$lib/format';
	import ProvenanceBadge from './ProvenanceBadge.svelte';
	import UrlValueList from './UrlValueList.svelte';
	import ChipValueList from './ChipValueList.svelte';

	let { fields }: { fields: ResolvedField[] } = $props();

	const provider = (f: ResolvedField) => providerFromWinningSource(f.winning_source);
</script>

{#if fields.length}
	<div class="mt-1 border-t border-rule pt-3 sm:col-span-2">
		<p class="text-xs text-muted">Additional details</p>
	</div>
	{#each fields as f (f.canonical)}
		<div class={f.display === 'long_text' || f.display === 'chips' ? 'sm:col-span-2' : ''}>
			<dt class="inline text-muted">{f.label}:</dt>
			{#if f.display === 'long_text'}
				<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
			{:else if f.display === 'image_url'}
				<dd class="mt-1 block">
					<img
						src={f.values[0]}
						alt={f.label}
						class="max-h-64 rounded-theme border border-rule"
					/>
				</dd>
			{:else if f.display === 'chips'}
				<dd class="mt-1 block"><ChipValueList values={f.values} /></dd>
			{:else if f.display === 'url'}
				<!-- HOLODEX-137: provider icon + host in the link folds in provenance. -->
				<dd class="inline"><UrlValueList values={f.values} provider={provider(f)} /></dd>
			{:else}
				<dd class="inline text-ink">{f.values.join(', ')}</dd>
			{/if}
			{#if provider(f) && f.display !== 'url'}
				<ProvenanceBadge provider={provider(f)} label={provider(f)} />
			{/if}
		</div>
	{/each}
{/if}
