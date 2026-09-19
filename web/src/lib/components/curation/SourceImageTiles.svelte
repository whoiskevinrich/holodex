<script lang="ts">
	// Image-tile chooser for an `image_url` replace field (HOLODEX-403, ADR-101 D3) — the
	// third chooser shape beside SourceChipRow (chips) and SourceRadioList (stacked rows).
	// One tile per candidate: the entity's own image (`·file` — the video's cover art) and
	// each provider's; no Custom tile, because a pasted URL would have to pass the provider
	// asset-host allowlist (ADR-039) and an owner upload has its own control on the page.
	// Same staged contract as the other two: selecting a tile STAGES (binds `stagedKey`),
	// never commits; the embedder owns Confirm. Keyboard is the chip row's, verbatim:
	// roving tabindex, arrows move AND stage, Space/Enter stage. Tokens only; QA 3 skins.
	import type { SourceChip } from '$lib/f36';
	import type { ResolvedField } from '$lib/types';

	let {
		field,
		chips,
		selection,
		stagedKey = $bindable(),
		disabled = false,
		baselineKey = 'file',
		baselineImage,
		baselinePlaceholder,
		onstage
	}: {
		field: ResolvedField;
		// sourceChips(field) minus the Custom chip — the embedder filters it out.
		chips: SourceChip[];
		selection: { key: string; pending: boolean };
		stagedKey: string | null;
		disabled?: boolean;
		baselineKey?: string;
		// What the baseline tile SHOWS. The baseline chip's own value is often '' (a mapping
		// with no `file:` source), but the entity still has an image of its own — the video's
		// served poster — so the embedder passes that URL here. undefined → placeholder.
		baselineImage?: string;
		// Two short lines for the placeholder tile when there is no baseline image to show
		// (e.g. an owner upload overwrote the extracted cover on disk — ADR-049 / ADR-101 D4).
		baselinePlaceholder?: [string, string];
		onstage?: () => void;
	} = $props();

	let groupEl = $state<HTMLElement | null>(null);

	function focusTile(key: string) {
		groupEl?.querySelector<HTMLElement>(`[data-seg="${key}"]`)?.focus();
	}
	function stage(key: string) {
		if (disabled) return;
		stagedKey = key;
		onstage?.();
	}
	function onGroupKey(e: KeyboardEvent) {
		if (disabled) return;
		const focused = (e.target as HTMLElement | null)?.closest?.('[data-seg]') as HTMLElement | null;
		const i = chips.findIndex((c) => c.key === focused?.dataset.seg);
		if (i < 0) return;
		const n = chips.length;
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (delta) {
			e.preventDefault();
			const target = chips[(i + delta + n) % n];
			focusTile(target.key);
			stage(target.key);
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			stage(chips[i].key);
		}
	}

	// The image a tile shows: the candidate URL for a provider; for the baseline, the
	// embedder-supplied image (the entity's own), falling back to the chip's value.
	function tileSrc(chip: SourceChip): string {
		return chip.key === baselineKey ? (baselineImage ?? chip.value) : chip.value;
	}
</script>

<div
	bind:this={groupEl}
	role="radiogroup"
	aria-label={`Source of truth for ${field.label}`}
	aria-disabled={disabled}
	tabindex={-1}
	class="flex flex-wrap items-start gap-3"
	class:pointer-events-none={disabled}
	onkeydown={onGroupKey}
>
	{#each chips as chip (chip.key)}
		{@const checked = stagedKey === chip.key}
		{@const pending = checked && selection.pending && stagedKey === selection.key}
		{@const src = tileSrc(chip)}
		{@const label = chip.labels.join(' + ')}
		<!-- Selection is never colour alone: border weight/dash + the dot + aria-checked. -->
		<button
			type="button"
			role="radio"
			data-seg={chip.key}
			aria-checked={checked}
			tabindex={checked ? 0 : -1}
			aria-label={`${field.label} from ${label}${pending ? ', pending' : ''}`}
			{disabled}
			onclick={() => stage(chip.key)}
			class="group flex flex-col items-center gap-1 rounded-theme focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
		>
			{#if src}
				<img
					{src}
					alt=""
					class="h-20 w-14 shrink-0 rounded-theme border object-cover {checked
						? `border-2 border-accent ${pending ? 'border-dashed' : ''}`
						: 'border-rule'}"
				/>
			{:else}
				<span
					class="flex h-20 w-14 shrink-0 flex-col items-center justify-center rounded-theme border border-dashed text-center text-[0.65rem] leading-tight text-muted {checked
						? 'border-accent'
						: 'border-rule'}"
				>
					{#if baselinePlaceholder}
						<span>{baselinePlaceholder[0]}</span><span>{baselinePlaceholder[1]}</span>
					{:else}
						<span>—</span>
					{/if}
				</span>
			{/if}
			<span class="inline-flex items-center gap-1 text-[0.65rem] {checked ? 'text-accent' : 'text-muted group-hover:text-ink'}">
				<span
					class="h-2 w-2 shrink-0 rounded-full border {checked ? `border-accent ${pending ? '' : 'bg-accent'}` : 'border-current'}"
					aria-hidden="true"
				></span>
				·{label}{pending ? ', pending' : ''}
			</span>
		</button>
	{/each}
</div>
