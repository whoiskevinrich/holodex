<script lang="ts">
	// Provenance chip (F22.7 → HOLODEX-135). Labels where a resolved field value came
	// from. For a provider it now shows the provider's BRAND ICON (ADR-059) instead of
	// the repeated "from <provider>" text that out-shouted the value down a column; the
	// long form stays on the icon's title/alt (hover + screen readers still name the
	// source). The file baseline keeps a small muted pill — never --warn. Tokens only.
	import ProviderIcon from './ProviderIcon.svelte';
	import { providers } from '$lib/providers.svelte';

	let { provider = '', label = '', manual = false }: { provider?: string; label?: string; manual?: boolean } =
		$props();

	const text = $derived(provider ? `from ${label || provider}` : manual ? 'from a custom value' : 'from file');

	// Load the public provider directory once (idempotent + shared across every badge)
	// so the icon URL is available; until it resolves — or when the provider has no
	// cached icon — ProviderIcon shows the monogram fallback.
	$effect(() => {
		if (provider) void providers.load();
	});
</script>

{#if provider}
	<span class="ml-2 inline-flex align-middle">
		<ProviderIcon name={provider} iconUrl={providers.iconUrl(provider)} title={text} size={16} />
	</span>
{:else}
	<span
		class="ml-2 inline-block rounded-full bg-surface-2 px-2 py-0.5 align-middle text-xs text-muted"
		aria-label={`source: ${text}`}
	>
		{manual ? 'custom' : 'file'}
	</span>
{/if}
