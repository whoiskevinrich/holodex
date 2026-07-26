<script lang="ts">
	// A person's nationality flag beside the hero name (HOLODEX-139). Resolves the
	// `nationality` field's values to countries, shows the primary flag with a "+N" when
	// there is more than one, and renders NOTHING when none resolve (no flag, no layout
	// gap). The flag is imagery; the "+N" chrome uses tokens. alt/title name the country
	// (or the full list) for the tooltip and screen readers.
	import { countriesFromNationality } from '$lib/nationality';
	import { flagUrl } from '$lib/flags';

	let { values }: { values: string[] } = $props();

	// Resolve each nationality to a country + its bundled flag URL in one pass, dropping any
	// whose flag asset is missing (degrades to no flag rather than a broken image). Pairing
	// the URL here avoids a second flagUrl() lookup in the template.
	const flags = $derived(
		countriesFromNationality(values)
			.map((c) => ({ name: c.name, url: flagUrl(c.code) }))
			.filter((f): f is { name: string; url: string } => !!f.url)
	);
	const primary = $derived(flags[0]);
	const label = $derived(flags.map((f) => f.name).join(', '));
</script>

{#if primary}
	<span class="inline-flex shrink-0 items-center gap-1 align-middle">
		<img
			src={primary.url}
			alt={label}
			title={label}
			width="20"
			height="15"
			class="h-4 w-auto rounded-theme border border-rule"
		/>
		{#if flags.length > 1}
			<span class="text-xs text-muted" aria-hidden="true">+{flags.length - 1}</span>
		{/if}
	</span>
{/if}
