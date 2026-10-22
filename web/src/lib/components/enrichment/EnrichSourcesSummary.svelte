<script lang="ts">
	// The media page's provider status (HOLODEX-220 Part A): one "{n} sources linked"
	// trigger whose popover holds a row per provider, instead of a chip per provider plus
	// "Refresh all". The page's single Sync verb does the re-pull, so per-provider actions
	// become exceptions inside the popover: Match… while unmatched; Refresh, Re-match… and
	// Clear behind ⋯ once linked. A sibling of EnrichProviderChips rather than a variant,
	// because the person and studio pages keep the chip row unchanged (handoff "Not in
	// scope"). The `footer` snippet carries the page's one-shot extras. Tokens only.
	import type { Snippet } from 'svelte';
	import ProviderIcon from './ProviderIcon.svelte';
	import { providers as providerDir } from '#lib/providers.svelte.ts';
	import { dismissable } from '#lib/actions/dismissable.ts';
	import { formatAgo } from '#lib/format.ts';

	let {
		providers,
		linked,
		syncedAt,
		busy = '',
		syncing = false,
		onenrich,
		onrefresh,
		onclear,
		footer
	}: {
		providers: string[];
		linked: (p: string) => boolean;
		// When this provider's stored enrichment was last fetched (ISO), for the row status.
		syncedAt: (p: string) => string | undefined;
		busy?: string;
		// The page Sync is running: every row action disables; the popover stays readable.
		syncing?: boolean;
		// Unmatched "Match…" and the linked "Re-match…" both open EnrichPicker; Re-match
		// flags itself so the picker shows the list even for a lone strong match (HOLODEX-418).
		onenrich: (p: string, opts?: { rematch: boolean }) => void;
		onrefresh: (p: string) => void;
		onclear: (p: string) => void;
		footer?: Snippet;
	} = $props();

	const panelId = $props.id();
	let open = $state(false);
	// The linked provider whose ⋯ menu is open inside the popover ('' = none).
	let openMenu = $state('');
	let trigger = $state<HTMLButtonElement | null>(null);

	const linkedProviders = $derived(providers.filter((p) => linked(p)));
	const unmatchedN = $derived(providers.length - linkedProviders.length);
	const label = $derived(
		providers.length === 0
			? 'More'
			: linkedProviders.length === 0
				? 'No source linked'
				: `${linkedProviders.length} source${linkedProviders.length === 1 ? '' : 's'} linked`
	);

	$effect(() => {
		void providerDir.load();
	});

	function close(returnFocus: boolean) {
		open = false;
		openMenu = '';
		if (returnFocus) trigger?.focus();
	}

	function act(fn: () => void) {
		close(false);
		fn();
	}
</script>

<div
	class="relative"
	data-enrich-sources
	use:dismissable={{ enabled: open, inside: '[data-enrich-sources]', onclose: close }}
>
	<button
		type="button"
		bind:this={trigger}
		onclick={() => (open ? close(false) : (open = true))}
		aria-expanded={open}
		aria-controls={panelId}
		aria-label={providers.length === 0 ? 'More metadata actions' : undefined}
		class="btn-quiet flex items-center gap-1.5 rounded-theme px-2 py-0.5 text-xs"
	>
		{#each linkedProviders.slice(0, 3) as p (p)}
			<ProviderIcon name={p} iconUrl={providerDir.iconUrl(p)} size={14} />
		{/each}
		<span>{label}</span>
		{#if unmatchedN > 0 && linkedProviders.length > 0}
			<span class="text-warn">· {unmatchedN} unmatched</span>
		{/if}
		<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"
			><path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6" /></svg
		>
	</button>

	<div
		id={panelId}
		hidden={!open}
		class="absolute left-0 top-full z-20 mt-1 w-72 max-w-[calc(100vw-2rem)] rounded-theme border border-rule bg-surface p-1 text-xs shadow-sm"
	>
		{#each providers as p (p)}
			{@const isLinked = linked(p)}
			<div class="relative flex items-center gap-2 rounded-theme px-2 py-1.5">
				<ProviderIcon name={p} iconUrl={providerDir.iconUrl(p)} size={16} />
				<span class="min-w-0 break-words font-medium text-ink">{p}</span>
				<span class="ml-auto shrink-0 whitespace-nowrap {isLinked ? 'text-muted' : 'text-warn'}">
					{#if busy === p}
						Working…
					{:else if isLinked}
						{syncedAt(p) ? `synced ${formatAgo(syncedAt(p))}` : 'linked'}
					{:else}
						not matched
					{/if}
				</span>
				{#if isLinked}
					<button
						type="button"
						onclick={() => (openMenu = openMenu === p ? '' : p)}
						disabled={syncing || busy === p}
						aria-haspopup="menu"
						aria-expanded={openMenu === p}
						aria-label={`More actions for ${p}`}
						class="btn-quiet rounded-theme px-1.5"
					>
						⋯
					</button>
					{#if openMenu === p}
						<div
							role="menu"
							class="absolute right-0 top-full z-30 mt-1 min-w-max rounded-theme border border-rule bg-surface p-1 shadow-sm"
						>
							<button
								role="menuitem"
								type="button"
								onclick={() => act(() => onrefresh(p))}
								class="block w-full rounded-theme px-3 py-1.5 text-left text-ink hover:bg-surface-2"
							>
								Refresh {p}
							</button>
							<button
								role="menuitem"
								type="button"
								onclick={() => act(() => onenrich(p, { rematch: true }))}
								class="block w-full rounded-theme px-3 py-1.5 text-left text-ink hover:bg-surface-2"
							>
								Re-match…
							</button>
							<button
								role="menuitem"
								type="button"
								onclick={() => act(() => onclear(p))}
								class="block w-full rounded-theme px-3 py-1.5 text-left text-ink hover:bg-surface-2"
							>
								Clear {p} data
							</button>
						</div>
					{/if}
				{:else}
					<button
						type="button"
						onclick={() => act(() => onenrich(p))}
						disabled={syncing || busy === p}
						class="btn-quiet rounded-theme px-1.5 text-accent"
					>
						Match…
					</button>
				{/if}
			</div>
		{/each}
		{#if footer}
			<div class="flex flex-col items-stretch {providers.length ? 'mt-1 border-t border-rule pt-1' : ''}">
				{@render footer()}
			</div>
		{/if}
	</div>
</div>
