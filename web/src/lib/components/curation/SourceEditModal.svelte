<script lang="ts">
	// Tier-2 per-field source-of-truth control, modal variant (HOLODEX-303). SourceBadge's
	// inline click-to-expand chip row (F56) doesn't work for paragraph-length values —
	// expanding a multi-line source comparison inline either truncates the very thing being
	// compared, or blows out the surrounding layout. This gives each candidate its own
	// full-width row instead, inside a modal. Adopted as the standard pattern for `long_text`
	// tier-2 fields going forward (today: Person bio; Video overview is a candidate follow-up,
	// not yet migrated). Same staged-then-Confirm contract as SourceBadge: nothing calls
	// `decide` until Save. Entity-generic like SourceBadge (`baselineKey`: 'file' for videos,
	// 'record' for persons/studios). Modal chrome delegates to ConfirmDialog (focus trap, Esc,
	// backdrop, focus-return), mirroring MergeCanonicalDialog's own radio-body usage of it.
	import type { DecisionSource, ResolvedField } from '$lib/types';
	import { resolveSelection, sourceChips } from '$lib/f36';
	import { toMessage } from '$lib/format';
	import ConfirmDialog from '../shared/ConfirmDialog.svelte';

	let {
		field,
		decide,
		baselineKey = 'file',
		onclose
	}: {
		field: ResolvedField;
		decide: (source: DecisionSource, manualValue?: string) => Promise<void>;
		baselineKey?: string;
		onclose: () => void;
	} = $props();

	const chips = $derived(sourceChips(field, baselineKey));

	// Seeded once from the field's current decision — this component is mounted fresh each
	// time the modal opens (the caller gates it with an {#if}), so a plain initializer is
	// enough; no reactive re-seeding, which would otherwise clobber an in-progress edit.
	let stagedKey = $state<string | null>(resolveSelection(field, chips, baselineKey).key);
	let stagedCustomValue = $state(
		field.decision?.source === 'manual' ? (field.decision.manual_value ?? '') : ''
	);
	let busy = $state(false);
	let error = $state('');

	async function save() {
		const chip = chips.find((c) => c.key === stagedKey);
		if (!chip || busy) return;
		if (chip.key === 'custom' && !stagedCustomValue.trim()) {
			error = 'Enter a custom value, or choose another source.';
			return;
		}
		busy = true;
		error = '';
		try {
			await decide(chip.decisionSource, chip.key === 'custom' ? stagedCustomValue.trim() : undefined);
			onclose();
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

<ConfirmDialog
	title={`Edit ${field.label}`}
	confirmLabel="Save"
	variant="accent"
	{busy}
	{error}
	onconfirm={save}
	oncancel={onclose}
>
	{#snippet body()}
		<fieldset class="space-y-2">
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
								name={`source-edit-${field.canonical}`}
								class="accent-accent"
								value="custom"
								checked={stagedKey === 'custom'}
								onchange={() => (stagedKey = 'custom')}
							/>
							<span
								class="text-xs uppercase tracking-wide {stagedKey === 'custom' ? 'text-accent' : 'text-muted'}"
							>
								Custom
							</span>
						</span>
						<textarea
							bind:value={stagedCustomValue}
							onfocus={() => (stagedKey = 'custom')}
							rows="3"
							placeholder={`Write a custom ${field.label.toLowerCase()}…`}
							class="mt-1 ml-6 block w-[calc(100%-1.5rem)] resize-none rounded-theme border border-rule bg-bg px-2 py-1 text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent"
						></textarea>
					</label>
				{:else}
					<label
						class="block cursor-pointer rounded-theme border p-2 {stagedKey === chip.key
							? 'border-accent bg-accent/10'
							: 'border-rule hover:bg-surface-2'}"
					>
						<span class="flex items-center gap-2">
							<input
								type="radio"
								name={`source-edit-${field.canonical}`}
								class="accent-accent"
								value={chip.key}
								checked={stagedKey === chip.key}
								onchange={() => (stagedKey = chip.key)}
							/>
							<span
								class="text-xs uppercase tracking-wide {stagedKey === chip.key ? 'text-accent' : 'text-muted'}"
							>
								{chip.labels.join(' + ')}
							</span>
						</span>
						<span class="mt-1 block pl-6 text-sm {chip.value.trim() ? 'text-ink' : 'text-muted'}">
							{chip.value.trim() || 'No value'}
						</span>
					</label>
				{/if}
			{/each}
		</fieldset>
	{/snippet}
</ConfirmDialog>
