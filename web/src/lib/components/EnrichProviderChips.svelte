<script lang="ts">
	// Owner enrich controls as compact per-provider chips (HOLODEX-136). Replaces the
	// old "Enrich from {p}" + "Clear {p} data" two-button wall (HOLODEX-119) that
	// repeated the provider name up to 4× and broke down with multiple providers. Each
	// provider is one chip: brand icon (ADR-059) + name + a primary action, with
	// per-provider controls tucked into a ⋯ overflow menu once linked (F47 S4,
	// ADR-066 RD7/RD8). Shared by the person, media, and studio detail pages. Tokens only.
	import ProviderIcon from './ProviderIcon.svelte';
	import { providers as providerDir } from '$lib/providers.svelte';
	import { dismissable } from '$lib/actions/dismissable';

	let {
		providers,
		linked,
		busy = '',
		refreshingAll = false,
		size = 'sm',
		onenrich,
		onrefresh,
		onclear,
		onrefreshall
	}: {
		providers: string[];
		linked: (p: string) => boolean;
		busy?: string;
		refreshingAll?: boolean;
		size?: 'sm' | 'xs';
		// Unlinked primary click, and the ⋯ "Re-match…" item once linked — both open
		// EnrichPicker for a fresh /resolve (RD7: the linked case is just a relabel of
		// what used to be the only primary action).
		onenrich: (p: string) => void;
		// Linked primary click — calls apply() directly against the stored external_id,
		// no picker (RD7/P0-5).
		onrefresh: (p: string) => void;
		onclear: (p: string) => void;
		// "Refresh all" — fans out over every provider in one call (RD8/P1-2).
		onrefreshall: () => void;
	} = $props();

	// The provider whose ⋯ menu is open ('' = none) — only one at a time.
	let openMenu = $state('');
	// ⋯ trigger refs (per provider) so focus returns to the opener on close (a11y), and
	// the menu's first item ref to move focus in on open.
	let triggers = $state<Record<string, HTMLButtonElement | null>>({});
	let firstItem = $state<HTMLButtonElement | null>(null);

	const txt = $derived(size === 'xs' ? 'text-xs' : 'text-sm');
	const pad = $derived(size === 'xs' ? 'px-2 py-1' : 'px-2.5 py-1.5');

	// Load the public provider directory once so the chip can lead with the real icon
	// (monogram until it resolves / when a provider has none).
	$effect(() => {
		void providerDir.load();
	});

	async function open(p: string) {
		openMenu = p;
		await Promise.resolve();
		firstItem?.focus();
	}

	function close(returnFocus = true) {
		const p = openMenu;
		openMenu = '';
		if (returnFocus && p) triggers[p]?.focus();
	}

	function toggle(p: string) {
		if (openMenu === p) close();
		else open(p);
	}
</script>

<div
	class="flex flex-wrap items-center gap-2"
	use:dismissable={{ enabled: openMenu !== '', inside: '[data-enrich-chip]', onclose: close }}
>
	{#each providers as p (p)}
		{@const isLinked = linked(p)}
		<div
			data-enrich-chip
			class="relative inline-flex items-stretch rounded-theme border border-rule bg-surface {txt}"
		>
			<!-- Primary action: unlinked opens the picker ("Enrich"); linked calls apply()
			     directly against the stored id ("Refresh", RD7) — no picker round trip.
			     Icon + name identify it once; the accent verb signals the click target. -->
			<button
				type="button"
				onclick={() => (isLinked ? onrefresh(p) : onenrich(p))}
				disabled={busy === p}
				title={isLinked ? `Refresh ${p}` : `Enrich from ${p}`}
				class="inline-flex items-center gap-1.5 rounded-theme {pad} text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				<ProviderIcon name={p} iconUrl={providerDir.iconUrl(p)} size={16} />
				<span class="font-medium">{p}</span>
				<span class="text-accent"
					>{isLinked ? (busy === p ? 'Refreshing…' : 'Refresh') : 'Enrich'}</span
				>
			</button>

			{#if isLinked}
				<!-- Overflow: Re-match/Clear live here so they only appear once the provider
				     has contributed data, and never compete with the primary Refresh action. -->
				<button
					type="button"
					bind:this={triggers[p]}
					onclick={() => toggle(p)}
					disabled={busy === p}
					aria-haspopup="menu"
					aria-expanded={openMenu === p}
					aria-label={`More actions for ${p}`}
					class="inline-flex items-center border-l border-rule px-1.5 text-muted hover:text-ink disabled:opacity-60"
				>
					{busy === p ? '…' : '⋯'}
				</button>
				{#if openMenu === p}
					<div
						role="menu"
						class="absolute right-0 top-full z-10 mt-1 min-w-max rounded-theme border border-rule bg-surface p-1 shadow-sm"
					>
						<button
							bind:this={firstItem}
							role="menuitem"
							type="button"
							onclick={() => {
								onenrich(p);
								close();
							}}
							class="block w-full rounded-theme px-3 py-1.5 text-left {txt} text-ink hover:bg-surface-2"
						>
							Re-match…
						</button>
						<button
							role="menuitem"
							type="button"
							onclick={() => {
								onclear(p);
								close();
							}}
							class="block w-full rounded-theme px-3 py-1.5 text-left {txt} text-ink hover:bg-surface-2"
						>
							Clear {p} data
						</button>
					</div>
				{/if}
			{/if}
		</div>
	{/each}

	{#if providers.length > 0}
		<!-- Refresh-all (RD8/P1-2): acts on every configured provider at once, not just
		     one chip — sits as a trailing item in the same flex-wrap row rather than a
		     new container, so it wraps together with the chips on narrow widths. -->
		<button
			type="button"
			onclick={onrefreshall}
			disabled={refreshingAll}
			class="rounded-theme border border-rule {pad} {txt} text-accent hover:bg-surface-2 disabled:opacity-60"
		>
			{refreshingAll ? 'Refreshing…' : 'Refresh all'}
		</button>
	{/if}
</div>
