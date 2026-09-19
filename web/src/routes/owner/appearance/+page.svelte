<script lang="ts">
	// Appearance — the instance skin picker (F66 S2, ADR-102 D1/D3, design handoff
	// docs/design/instance-skin-handoff.md). One radio card per shipped skin plus the
	// owner's custom palette when one is configured; picking a card applies it for
	// every viewer and saves instantly (selection *is* the save). Each card renders in
	// its own tokens: `data-theme` on the button re-scopes every custom property, and
	// the two preview tiles are real `.video-frame`s so the skin's flourishes draw
	// themselves (`.skin-card` is the fence app.css uses so the page's skin doesn't
	// bleed into a card). Owner-only via the /owner layout gate.
	import { activity } from '$lib/activity.svelte';
	import { theme, THEMES, THEME_LABELS } from '$lib/theme.svelte';
	import type { ShippedTheme, ThemeCustom, ThemeId } from '$lib/types';

	// Descriptive subline per shipped skin: display font · UI font · radius (content,
	// not styling — the card's own tokens do the rendering).
	const SUBLINES: Record<ShippedTheme, string> = {
		cinematheque: 'Fraunces · Archivo · 2px',
		broadcast: 'VT323 · Share Tech Mono · 0px',
		brutalist: 'Spline Sans Mono · 0px'
	};

	const DOCS_URL =
		'https://github.com/whoiskevinrich/holodex/blob/main/docs/reference/configuration.md#appearance';

	interface Card {
		id: ThemeId;
		label: string;
		base: ShippedTheme;
		subline: string;
		custom: ThemeCustom | null;
	}

	const cards = $derived.by<Card[]>(() => {
		const shipped: Card[] = THEMES.map((id) => ({
			id,
			label: THEME_LABELS[id],
			base: id,
			subline: SUBLINES[id],
			custom: null
		}));
		const c = theme.custom;
		if (!c) return shipped;
		return [
			...shipped,
			{
				id: 'custom',
				label: c.name,
				base: c.base,
				subline: `based on ${THEME_LABELS[c.base]} · holodex.yaml`,
				custom: c
			}
		];
	});

	let error = $state('');
	let buttons: HTMLButtonElement[] = $state([]);

	async function choose(id: ThemeId) {
		error = '';
		if (id === theme.active) return;
		if (!(await theme.select(id))) {
			error = 'Couldn’t save the skin — the previous one is still active. Try again.';
		}
	}

	// Roving tabindex: Tab lands on the active card, arrows move between cards,
	// Space/Enter (the button's own activation) select.
	function onKey(e: KeyboardEvent, i: number) {
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown'
				? 1
				: e.key === 'ArrowLeft' || e.key === 'ArrowUp'
					? -1
					: 0;
		if (!delta) return;
		e.preventDefault();
		const n = cards.length;
		buttons[(i + delta + n) % n]?.focus();
	}
</script>

<div class="space-y-4">
	<p class="text-sm text-muted">Click a card to apply it for every viewer. Saved instantly.</p>

	{#if error}
		<p class="text-sm text-warn" role="alert">{error}</p>
	{/if}

	<div role="radiogroup" aria-label="Instance skin" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
		{#each cards as c, i (c.id)}
			{@const active = theme.active === c.id}
			<button
				type="button"
				role="radio"
				aria-checked={active}
				aria-label={c.label}
				tabindex={active ? 0 : -1}
				bind:this={buttons[i]}
				data-theme={c.base}
				style:--bg={c.custom?.tokens.bg}
				style:--ink={c.custom?.tokens.ink}
				style:--accent={c.custom?.tokens.accent}
				style:--muted={c.custom?.tokens.muted}
				style:--warn={c.custom?.tokens.warn}
				onclick={() => choose(c.id)}
				onkeydown={(e) => onKey(e, i)}
				class="skin-card relative rounded-theme border bg-bg p-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent {active
					? 'border-accent'
					: 'border-rule hover:border-ink/40'}"
			>
				{#if active}
					<span
						aria-hidden="true"
						class="btn-accent absolute right-2 top-2 px-1.5 py-0.5 text-xs uppercase tracking-wide"
					>
						Active
					</span>
				{/if}
				<span class="block pr-14 font-display text-base text-ink">{c.label}</span>
				<!-- Two real .video-frame tiles at the instance's card_layout aspect; the
				     skin's own flourishes render into them. Decorative only. -->
				<span
					aria-hidden="true"
					class="video-grid mt-2 grid grid-cols-2 gap-1.5"
					data-layout={activity.cardLayout}
				>
					{#each [false, true] as accentMeta (accentMeta)}
						<span class="video-frame block">
							<span class="absolute inset-x-1.5 bottom-1.5 flex flex-col gap-1">
								<span class="block h-1 w-3/4 rounded-full bg-ink"></span>
								<span class="block h-0.5 w-1/2 rounded-full {accentMeta ? 'bg-accent' : 'bg-muted'}"
								></span>
							</span>
						</span>
					{/each}
				</span>
				<span class="mt-2 block text-xs text-muted">{c.subline}</span>
			</button>
		{/each}

		{#if !theme.custom}
			<!-- The tab's only authoring affordance, on purpose (ADR-102 D2): the palette
			     is declared in config, not here. -->
			<div class="rounded-theme border border-dashed border-rule p-3 text-sm text-muted">
				No custom palette. Define <code class="text-ink">theme.custom</code> in
				<code class="text-ink">holodex.yaml</code> and restart — see
				<a href={DOCS_URL} target="_blank" rel="noopener" class="text-accent hover:underline"
					>Configuration › Appearance</a
				>.
			</div>
		{/if}
	</div>
</div>
