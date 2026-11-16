<script lang="ts">
	// The owner's top of every list's Filters panel (F90 handoff, Layout 2): Provider link
	// (Any · Linked · Not linked), then the Fields rows. State is the page's: Media keeps it in
	// ?expr=, People and Studios in their own params. It loads its list's fields.
	import { loadFilterFields, type FieldCond, type FilterField, type FilterList, type LinkState } from '#lib/fieldConditions.ts';
	import FieldConditions from './FieldConditions.svelte';
	import SegmentedRadio from './SegmentedRadio.svelte';

	let {
		list,
		link,
		conds,
		onlink,
		onconds
	}: {
		list: FilterList;
		link: LinkState;
		conds: readonly FieldCond[];
		onlink: (v: LinkState) => void;
		onconds: (c: FieldCond[]) => void;
	} = $props();

	const LINKS = [
		{ value: 'any', label: 'Any' },
		{ value: 'linked', label: 'Linked' },
		{ value: 'unlinked', label: 'Not linked' }
	] as const;

	let fields = $state<FilterField[] | null>(null);
	$effect(() => {
		void loadFilterFields(list).then((f) => (fields = f));
	});
</script>

<div>
	<span class="mb-1 block text-xs text-muted">Provider link</span>
	<SegmentedRadio options={LINKS} value={link} label="Provider link" onchange={onlink} />
</div>
<FieldConditions {fields} {conds} onchange={onconds} />
