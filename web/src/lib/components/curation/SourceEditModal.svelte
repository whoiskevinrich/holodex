<script lang="ts">
	// Tier-2 per-field source-of-truth control, modal variant (HOLODEX-303). SourceBadge's
	// inline click-to-expand chip row (F56) doesn't work for paragraph-length values —
	// expanding a multi-line source comparison inline either truncates the very thing being
	// compared, or blows out the surrounding layout. This gives each candidate its own
	// full-width row instead, inside a modal. Adopted as the standard pattern for `long_text`
	// tier-2 fields going forward (Person bio; Video overview since HOLODEX-365). Same
	// staged-then-Confirm contract as SourceBadge: nothing calls
	// `decide` until Save. Entity-generic like SourceBadge (`baselineKey`: 'file' for videos,
	// 'record' for persons/studios). Modal chrome delegates to ConfirmDialog (focus trap, Esc,
	// backdrop, focus-return), mirroring MergeCanonicalDialog's own radio-body usage of it.
	import type { DecisionSource, ResolvedField } from '$lib/types';
	import { resolveSelection, sourceChips } from '$lib/f36';
	import { toMessage } from '$lib/format';
	import ConfirmDialog from '../shared/ConfirmDialog.svelte';
	import SourceRadioList from './SourceRadioList.svelte';

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

	// Seeded once from the field's current decision — reactive re-seeding would clobber an
	// in-progress edit (same reasoning as SourceBadge's own staged state). This does NOT
	// guarantee `stagedKey` always matches a live entry in `chips`: the caller gates mount
	// with `{#if bioEditOpen && field}`, which does not remount on a `field` reference change
	// alone, so an unrelated reloadDetail() elsewhere on the page (e.g. another field's own
	// decide, or a promotion edit) can recompute `chips` while this modal stays open. `save()`
	// below treats a stagedKey that's fallen out of `chips` as an error, not a silent no-op.
	let stagedKey = $state<string | null>(resolveSelection(field, chips, baselineKey).key);
	let stagedCustomValue = $state(
		field.decision?.source === 'manual' ? (field.decision.manual_value ?? '') : ''
	);
	let busy = $state(false);
	let error = $state('');

	async function save() {
		if (busy) return;
		const chip = chips.find((c) => c.key === stagedKey);
		if (!chip) {
			error = 'This field changed while the dialog was open — close it and try again.';
			return;
		}
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
		<!-- The candidate rows live in SourceRadioList (HOLODEX-400) so the writeback dialog can
		     embed the same chooser; this modal only supplies the chrome and the Save commit. -->
		<SourceRadioList {field} {chips} bind:stagedKey bind:stagedCustomValue disabled={busy} />
	{/snippet}
</ConfirmDialog>
