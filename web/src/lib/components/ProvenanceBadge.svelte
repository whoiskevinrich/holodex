<script lang="ts">
	// Provenance chip (F22.7 → HOLODEX-135). Labels where a resolved field value came
	// from. For a provider it now shows the provider's BRAND ICON (ADR-059) instead of
	// the repeated "from <provider>" text that out-shouted the value down a column; the
	// long form stays on the icon's title/alt (hover + screen readers still name the
	// source). The file baseline keeps a small muted pill — never --warn. Tokens only.
	import ProviderIcon from './ProviderIcon.svelte';
	import { providers } from '$lib/providers.svelte';

	let {
		provider = '',
		label = '',
		// F45 (ADR-063): the derived-field treatment. `computed` renders an icon-only
		// "calculated" glyph (no provider icon, no file pill); `derivedFrom` are the input
		// field LABELS for the transitive "calculated from …" hover/SR copy.
		computed = false,
		derivedFrom = []
	}: {
		provider?: string;
		label?: string;
		computed?: boolean;
		derivedFrom?: string[];
	} = $props();

	const text = $derived(provider ? `from ${label || provider}` : 'from file');

	// "calculated from Born" / "calculated from Born and Died" — serial-comma join
	// (at most two inputs today). Lives on title + aria-label only, never inline (D5).
	const derivedText = $derived(`calculated from ${joinSerial(derivedFrom)}`);

	function joinSerial(parts: string[]): string {
		if (parts.length <= 1) return parts[0] ?? '';
		if (parts.length === 2) return `${parts[0]} and ${parts[1]}`;
		return `${parts.slice(0, -1).join(', ')}, and ${parts[parts.length - 1]}`;
	}

	// Load the public provider directory once (idempotent + shared across every badge)
	// so the icon URL is available; until it resolves — or when the provider has no
	// cached icon — ProviderIcon shows the monogram fallback.
	$effect(() => {
		if (provider) void providers.load();
	});
</script>

{#if computed}
	<!-- F45 (ADR-063 D5): icon-only computed treatment — a muted "calculated" glyph, the
	     transitive phrase on title + the wrapping span's aria-label (never inline text).
	     text-muted only (never --accent/--warn); the inner SVG is decorative. -->
	<span class="ml-2 inline-flex align-middle text-muted" title={derivedText} aria-label={derivedText}>
		<svg
			class="h-4 w-4"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			stroke-linejoin="round"
			aria-hidden="true"
		>
			<rect width="16" height="20" x="4" y="2" rx="2" />
			<line x1="8" x2="16" y1="6" y2="6" />
			<line x1="16" x2="16" y1="14" y2="18" />
			<path d="M8 10h.01" />
			<path d="M12 10h.01" />
			<path d="M8 14h.01" />
			<path d="M12 14h.01" />
			<path d="M8 18h.01" />
			<path d="M12 18h.01" />
		</svg>
	</span>
{:else if provider}
	<span class="ml-2 inline-flex align-middle">
		<ProviderIcon name={provider} iconUrl={providers.iconUrl(provider)} title={text} size={16} />
	</span>
{:else}
	<span
		class="ml-2 inline-block rounded-full bg-surface-2 px-2 py-0.5 align-middle text-xs text-muted"
		aria-label={`source: ${text}`}
	>
		file
	</span>
{/if}
