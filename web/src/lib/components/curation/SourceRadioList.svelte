<script lang="ts">
	// Stacked full-width candidate rows for a `long_text` replace field (HOLODEX-400). Lifted
	// out of SourceEditModal's body so the writeback dialog can embed the same chooser per row:
	// one native radio row per SourceChip plus an inline Custom textarea. Paragraph-length
	// values need their own line each — a chip row truncates the very text being compared
	// (HOLODEX-303). Selecting a row STAGES (binds `stagedKey` / `stagedCustomValue`); the
	// embedding component owns Confirm/Save. Tokens only; QA 3 skins.
	import type { SourceChip } from '$lib/f36';
	import type { ResolvedField } from '$lib/types';
	import SourceValueClamp from './SourceValueClamp.svelte';

	let {
		field,
		chips,
		stagedKey = $bindable(),
		stagedCustomValue = $bindable(''),
		disabled = false,
		baselineKey = 'file',
		baselinePlaceholder = 'No value',
		onstage
	}: {
		field: ResolvedField;
		chips: SourceChip[];
		stagedKey: string | null;
		stagedCustomValue?: string;
		disabled?: boolean;
		baselineKey?: string;
		// What the baseline row reads as when its value is empty ("No value" by default; the
		// writeback dialog passes "Not read back from this file" for an ADR-093 field).
		baselinePlaceholder?: string;
		// Fires after every staged change (radio pick, Custom focus, Custom edit).
		onstage?: () => void;
	} = $props();

	// Radios share a name per field so the browser groups them; two chooser instances for
	// different fields on one page never collide.
	const name = $derived(`source-edit-${field.canonical}`);

	// onclick as well as onchange: clicking the ALREADY-checked radio fires no change event,
	// but that click is how the owner confirms an RD6 pending pick in the writeback dialog.
	function setStaged(key: string) {
		stagedKey = key;
		onstage?.();
	}
</script>

<fieldset class="space-y-2" {disabled}>
	<legend class="sr-only">Source for {field.label}</legend>
	{#each chips as chip (chip.key)}
		{#if chip.key === 'custom'}
			<label
				class="block rounded-theme border p-2 {stagedKey === 'custom'
					? 'border-accent bg-accent/10'
					: 'border-rule'}"
			>
				<span class="flex items-center gap-2">
					<input
						type="radio"
						{name}
						class="accent-accent"
						value="custom"
						checked={stagedKey === 'custom'}
						onchange={() => setStaged('custom')}
						onclick={() => setStaged('custom')}
					/>
					<span
						class="text-xs uppercase tracking-wide {stagedKey === 'custom' ? 'text-accent' : 'text-muted'}"
					>
						Custom
					</span>
				</span>
				<!-- value + oninput rather than bind:value, so `onstage` always observes the
				     already-updated literal on the same keystroke (handler order is not guaranteed
				     between a bind listener and a sibling oninput). -->
				<textarea
					value={stagedCustomValue}
					onfocus={() => setStaged('custom')}
					oninput={(e) => {
						stagedCustomValue = e.currentTarget.value;
						onstage?.();
					}}
					rows="5"
					placeholder={`Write a custom ${field.label.toLowerCase()}…`}
					class="mt-1 ml-6 block w-[calc(100%-1.5rem)] resize-none rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent disabled:cursor-not-allowed"
				></textarea>
			</label>
		{:else}
			{@const value = chip.value.trim()}
			<label
				class="block cursor-pointer rounded-theme border p-2 {stagedKey === chip.key
					? 'border-accent bg-accent/10'
					: 'border-rule hover:bg-surface-2'}"
			>
				<span class="flex items-center gap-2">
					<input
						type="radio"
						{name}
						class="accent-accent"
						value={chip.key}
						checked={stagedKey === chip.key}
						onchange={() => setStaged(chip.key)}
						onclick={() => setStaged(chip.key)}
					/>
					<span
						class="text-xs uppercase tracking-wide {stagedKey === chip.key ? 'text-accent' : 'text-muted'}"
					>
						{chip.labels.join(' + ')}
					</span>
				</span>
				<!-- Paragraph-length values clamp to four lines (SourceValueClamp, HOLODEX-417) so every
				     candidate — and whatever footer the embedder has — stays on screen together. -->
				<span class="mt-1 block pl-6 text-sm {value ? 'text-ink' : 'text-muted'}">
					{#if value}
						<SourceValueClamp text={value} label={chip.labels.join(' + ')} />
					{:else}
						{chip.key === baselineKey ? baselinePlaceholder : 'No value'}
					{/if}
				</span>
			</label>
		{/if}
	{/each}
</fieldset>
