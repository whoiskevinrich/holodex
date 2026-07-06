<script lang="ts">
	// Owner enrich controls as compact per-provider chips (HOLODEX-136). Replaces the
	// old "Enrich from {p}" + "Clear {p} data" two-button wall (HOLODEX-119) that
	// repeated the provider name up to 4× and broke down with multiple providers. Each
	// provider is one chip: brand icon (ADR-059) + name + a primary "Enrich" action,
	// with "Clear" tucked into a ⋯ overflow menu shown only once the provider is linked.
	// Shared by the person, media, and studio detail pages. Tokens only.
	import ProviderIcon from './ProviderIcon.svelte';
	import { providers as providerDir } from '$lib/providers.svelte';

	let {
		providers,
		linked,
		busy = '',
		size = 'sm',
		onenrich,
		onclear
	}: {
		providers: string[];
		linked: (p: string) => boolean;
		busy?: string;
		size?: 'sm' | 'xs';
		onenrich: (p: string) => void;
		onclear: (p: string) => void;
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

	// While a menu is open, Escape closes it (returning focus) and an outside click
	// dismisses it (no focus move — the pointer already left). Mirrors the app's other
	// dismissable popovers without pulling in a full modal focus trap for one action.
	$effect(() => {
		if (!openMenu) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				e.stopPropagation();
				close();
			}
		};
		const onClick = (e: MouseEvent) => {
			const t = e.target as Node;
			if (!(t instanceof Element) || !t.closest('[data-enrich-chip]')) close(false);
		};
		window.addEventListener('keydown', onKey, true);
		window.addEventListener('click', onClick);
		return () => {
			window.removeEventListener('keydown', onKey, true);
			window.removeEventListener('click', onClick);
		};
	});
</script>

<div class="flex flex-wrap items-center gap-2">
	{#each providers as p (p)}
		<div
			data-enrich-chip
			class="relative inline-flex items-stretch rounded-theme border border-rule bg-surface {txt}"
		>
			<!-- Primary action: open the provider's enrich picker. Icon + name identify it
			     once; the accent verb signals the click target. -->
			<button
				type="button"
				onclick={() => onenrich(p)}
				title={`Enrich from ${p}`}
				class="inline-flex items-center gap-1.5 rounded-theme {pad} text-ink hover:bg-surface-2"
			>
				<ProviderIcon name={p} iconUrl={providerDir.iconUrl(p)} size={16} />
				<span class="font-medium">{p}</span>
				<span class="text-accent">Enrich</span>
			</button>

			{#if linked(p)}
				<!-- Overflow: Clear lives here so it only appears once the provider has
				     contributed data, and never competes with the primary Enrich action. -->
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
</div>
