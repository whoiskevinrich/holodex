<script lang="ts">
	// The Tier-2 chip row as a staged, embeddable radiogroup (HOLODEX-400). Lifted from
	// SourceBadge's expanded state so the writeback dialog can render one chooser per row
	// without re-implementing the roving-tabindex / Custom-draft machinery. Selecting a chip
	// STAGES (binds `stagedKey` / `stagedCustomValue`) and never commits — the owner of this
	// component decides what Confirm is (SourceBadge's Confirm button; the dialog's Write
	// button). SourceBadge itself still carries its own copy of this markup for now — folding
	// it onto this component is a follow-up, not part of HOLODEX-400.
	//
	// Keyboard: arrow keys move focus AND stage within the group; Space/Enter stage the
	// focused chip; Escape inside the open Custom input cancels just the input and stops
	// propagating, so an enclosing dialog's own Escape handler does not fire on the same
	// keystroke (handoff §6). Tokens only; QA 3 skins.
	import { tick } from 'svelte';
	import { chipToResolvedValue, type SourceChip } from '$lib/f36';
	import type { ResolvedField } from '$lib/types';
	import CurationChip from './CurationChip.svelte';

	let {
		field,
		chips,
		selection,
		stagedKey = $bindable(),
		stagedCustomValue = $bindable(''),
		disabled = false,
		onstage
	}: {
		field: ResolvedField;
		chips: SourceChip[];
		// resolveSelection(field, chips) — the committed key + whether it is an RD6 implicit
		// winner, so the pending chip renders with the dashed ring while it is the staged one.
		selection: { key: string; pending: boolean };
		stagedKey: string | null;
		stagedCustomValue?: string;
		disabled?: boolean;
		// Fires after every staged change (chip, arrow key, committed Custom draft) so an
		// embedding row can react — the dialog promotes a matching row to will-write here.
		onstage?: () => void;
	} = $props();

	let editing = $state(false); // Custom inline input open
	let draft = $state('');
	let groupEl = $state<HTMLElement | null>(null);

	function focusChip(key: string) {
		groupEl?.querySelector<HTMLElement>(`[data-seg="${key}"]`)?.focus();
	}

	function setStaged(key: string) {
		stagedKey = key;
		onstage?.();
	}

	// Stage a chip now (click / Space / Enter) — never commits. The Custom chip opens the
	// inline input instead; its own commit (Enter/blur) stages the typed literal.
	function stage(chip: SourceChip) {
		if (disabled) return;
		if (chip.key === 'custom') {
			startCustom();
			return;
		}
		setStaged(chip.key);
	}

	function startCustom() {
		draft = stagedKey === 'custom' ? stagedCustomValue : (field.decision?.manual_value ?? field.values[0] ?? '');
		editing = true;
	}
	function commitCustomDraft() {
		const v = draft.trim();
		editing = false;
		if (!v) return; // nothing typed — leave whatever was staged before untouched
		stagedCustomValue = v;
		setStaged('custom');
	}
	async function cancelCustomEdit() {
		editing = false;
		await tick(); // let the chip button re-render before moving focus back onto it (a11y)
		focusChip('custom');
	}
	async function onCustomKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitCustomDraft();
			// The input unmounts on commit; keep keyboard focus in the group. Only on the
			// Enter path — a blur-commit must never pull focus back from wherever it went.
			await tick();
			focusChip('custom');
		} else if (e.key === 'Escape') {
			// Cancel just the input. stopPropagation keeps the keystroke from reaching an
			// enclosing dialog's Escape handler (first Escape closes the editor, second the dialog).
			e.preventDefault();
			e.stopPropagation();
			cancelCustomEdit();
		}
	}

	// Roving radiogroup arrow keys move focus AND stage. Space/Enter stage the focused chip.
	function onGroupKey(e: KeyboardEvent) {
		if (disabled) return;
		const focused = (e.target as HTMLElement | null)?.closest?.('[data-seg]') as HTMLElement | null;
		const i = chips.findIndex((c) => c.key === focused?.dataset.seg);
		if (i < 0) return; // key came from the inline input (no data-seg) — leave it alone
		const n = chips.length;
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (delta) {
			e.preventDefault();
			const target = chips[(i + delta + n) % n];
			focusChip(target.key);
			if (target.key === 'custom') {
				setStaged('custom');
				startCustom();
			} else setStaged(target.key);
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			stage(chips[i]);
		}
	}
</script>

<!-- Roving-tabindex radiogroup — identical shell to SourceBadge's expanded row (CurationChip's
     radio mode); selecting a chip stages, it never commits. -->
<div
	bind:this={groupEl}
	role="radiogroup"
	aria-label={`Source of truth for ${field.label}`}
	aria-disabled={disabled}
	tabindex={-1}
	class="flex flex-wrap items-center gap-1.5"
	class:pointer-events-none={disabled}
	onkeydown={onGroupKey}
>
	{#each chips as chip (chip.key)}
		{#if chip.key === 'custom'}
			{#if editing}
				<!-- svelte-ignore a11y_autofocus -->
				<input
					bind:value={draft}
					onkeydown={onCustomKey}
					onblur={commitCustomDraft}
					autofocus
					{disabled}
					aria-label={`Custom value for ${field.label}`}
					placeholder="Custom value…"
					class="w-32 rounded-full border border-accent bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent"
				/>
			{:else if stagedKey === 'custom' || chip.value}
				<!-- Staged/decided manual literal: a value chip (·manual) that re-opens the editor. -->
				<CurationChip
					item={stagedKey === 'custom'
						? { value: stagedCustomValue, sources: ['manual'], manual: true }
						: chipToResolvedValue(chip)}
					isOwner={false}
					radio={{
						key: 'custom',
						checked: stagedKey === 'custom',
						tabindex: stagedKey === 'custom' ? 0 : -1,
						onselect: () => stage(chip)
					}}
				/>
			{:else}
				<!-- Opener: choosing it opens the inline input; on commit it becomes the chip above. -->
				<button
					type="button"
					role="radio"
					data-seg="custom"
					aria-checked={stagedKey === 'custom'}
					tabindex={stagedKey === 'custom' ? 0 : -1}
					aria-label={`Set a custom value for ${field.label}`}
					{disabled}
					onclick={() => stage(chip)}
					class="curation-chip inline-flex items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-muted hover:text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
				>
					<svg class="h-3 w-3 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 5v14M5 12h14" />
					</svg>
					Custom
				</button>
			{/if}
		{:else}
			<CurationChip
				item={chipToResolvedValue(chip)}
				isOwner={false}
				radio={{
					key: chip.key,
					checked: stagedKey === chip.key,
					tabindex: stagedKey === chip.key ? 0 : -1,
					onselect: () => stage(chip),
					pending: selection.pending && stagedKey === selection.key
				}}
			/>
		{/if}
	{/each}
</div>
