<script lang="ts">
	// Image-tile chooser for an `image_url` replace field (HOLODEX-403, ADR-101 D3) — the
	// third chooser shape beside SourceChipRow (chips) and SourceRadioList (stacked rows).
	// F85 (docs/design/writeback-poster-chooser-handoff.md) groups the tiles by where each image
	// came from: In the file (`·file`, the file's own cover art), Holodex (`·frame`, `·upload`),
	// Automatic, then one group per matched provider — `no poster` text for one with none, never
	// a dashed slot. No Custom tile, because a pasted URL would have to pass the provider
	// asset-host allowlist (ADR-039). Same staged contract as the other two: selecting a tile
	// STAGES (binds `stagedKey`), never commits; the embedder owns Confirm. The groups are
	// layout only: the whole chooser is one radiogroup with the chip row's keyboard — roving
	// tabindex, arrows move AND stage across every tile in reading order, Space/Enter stage.
	// Tokens only; QA Cinémathèque.
	import type { SourceChip } from '#lib/f36.ts';
	import type { ResolvedField } from '#lib/types.ts';
	import { imageTileGroups, tileImage, tileTitle } from './imageTileGroups.ts';

	let {
		field,
		chips,
		providers = [],
		selection,
		stagedKey = $bindable(),
		disabled = false,
		onstage
	}: {
		field: ResolvedField;
		// sourceChips(field) minus the Custom chip — the embedder filters it out.
		chips: SourceChip[];
		// The video's matched providers, so one with no poster still gets its group.
		providers?: string[];
		selection: { key: string; pending: boolean };
		stagedKey: string | null;
		disabled?: boolean;
		onstage?: () => void;
	} = $props();

	const groups = $derived(imageTileGroups(chips, providers));
	// Keyboard order: every tile in reading order across groups.
	const tiles = $derived(groups.flatMap((g) => g.chips));

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
		const i = tiles.findIndex((c) => c.key === focused?.dataset.seg);
		if (i < 0) return;
		const n = tiles.length;
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (delta) {
			e.preventDefault();
			const target = tiles[(i + delta + n) % n];
			focusTile(target.key);
			stage(target.key);
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			stage(tiles[i].key);
		}
	}
</script>

<div
	bind:this={groupEl}
	role="radiogroup"
	aria-label={`Source of truth for ${field.label}`}
	aria-disabled={disabled}
	tabindex={-1}
	class="flex flex-wrap items-start gap-x-5 gap-y-3"
	class:pointer-events-none={disabled}
	onkeydown={onGroupKey}
>
	{#each groups as group (group.key)}
		<div data-tile-group={group.key}>
			<p class="mb-1 whitespace-nowrap text-[0.65rem] uppercase tracking-wide text-muted">{group.caption}</p>
			{#if group.chips.length}
				<div class="flex flex-wrap gap-3">
					{#each group.chips as chip (chip.key)}{@render tile(chip)}{/each}
				</div>
			{:else}
				<p class="text-[0.65rem] text-muted">no poster</p>
			{/if}
		</div>
	{/each}
</div>

{#snippet tile(chip: SourceChip)}
	{@const checked = stagedKey === chip.key}
	{@const pending = checked && selection.pending && stagedKey === selection.key}
	{@const src = tileImage(field, chip)}
	{@const label = chip.labels.join(' + ')}
	<!-- Selection is never colour alone: border weight/dash + the dot + aria-checked. -->
	<button
		type="button"
		role="radio"
		data-seg={chip.key}
		aria-checked={checked}
		tabindex={checked ? 0 : -1}
		aria-label={`${field.label} from ${label}${pending ? ', pending' : ''}`}
		title={tileTitle(field, chip)}
		{disabled}
		onclick={() => stage(chip.key)}
		class="group flex flex-col items-center gap-1 rounded-theme focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
	>
		{#if src}
			<img
				{src}
				alt=""
				class="h-20 w-auto min-w-14 shrink-0 rounded-theme border object-contain {checked
					? `border-2 border-accent ${pending ? 'border-dashed' : ''}`
					: 'border-rule'}"
			/>
		{:else}
			<!-- Only `·file` can lack an image: the file has no cover art. -->
			<span
				class="flex h-20 w-14 shrink-0 flex-col items-center justify-center rounded-theme border border-dashed text-center text-[0.65rem] leading-tight text-muted {checked
					? 'border-accent'
					: 'border-rule'}"
			>
				<span>no cover</span><span>art</span>
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
{/snippet}
