<script lang="ts">
	// A person's nationality flag beside the hero name (HOLODEX-139). Resolves the
	// `nationality` field's values to countries, shows the primary flag with a "+N" when
	// there is more than one, and renders NOTHING when none resolve (no flag, no layout
	// gap). The flag is imagery; the "+N" chrome uses tokens. alt/title name the country
	// (or the full list) for the tooltip and screen readers.
	import { countriesFromNationality } from '$lib/nationality';
	import { flagUrl } from '$lib/flags';

	let { values }: { values: string[] } = $props();

	// Only keep countries whose flag actually bundled — a resolved-but-missing asset
	// degrades to no flag rather than a broken image.
	const countries = $derived(countriesFromNationality(values).filter((c) => flagUrl(c.code)));
	const primary = $derived(countries[0]);
	const label = $derived(countries.map((c) => c.name).join(', '));
</script>

{#if primary}
	<span class="inline-flex shrink-0 items-center gap-1 align-middle">
		<img
			src={flagUrl(primary.code)}
			alt={label}
			title={label}
			width="20"
			height="15"
			class="h-4 w-auto rounded-theme border border-rule"
		/>
		{#if countries.length > 1}
			<span class="text-xs text-muted" aria-hidden="true">+{countries.length - 1}</span>
		{/if}
	</span>
{/if}
