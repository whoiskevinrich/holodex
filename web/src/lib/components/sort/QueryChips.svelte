<script lang="ts">
	// An F87 query's chips, for a chips row: plain and NOT chips, OR groups, or the Advanced
	// query row (query-language-handoff.md). Browse and the playlist page both render it;
	// with no onremove the chips are read-only (F89 handoff).
	import FilterChip from './FilterChip.svelte';
	import QueryChipGroup from './QueryChipGroup.svelte';
	import AdvancedQueryRow from './AdvancedQueryRow.svelte';
	import type { ChipCond, ChipModel } from '#lib/query.ts';

	let {
		model,
		onremove,
		onedit,
		problem
	}: {
		model: ChipModel | null;
		/** Removes the node at this path in the query tree. */
		onremove?: (path: number[]) => void;
		/** The Advanced row's Edit as text. */
		onedit?: () => void;
		/** Whether a refusal names this chip. */
		problem?: (chip: ChipCond) => boolean;
	} = $props();
</script>

{#if model?.kind === 'advanced'}
	<AdvancedQueryRow text={model.text} {onedit} />
{:else if model}
	{#each model.items as item (item.kind === 'chip' ? item.chip.path.join('.') : `g${item.path.join('.')}`)}
		{#if item.kind === 'chip'}
			<FilterChip
				label={item.chip.label}
				negated={item.chip.negated}
				ownerOnly={item.chip.ownerOnly}
				problem={problem?.(item.chip)}
				onremove={onremove ? () => onremove(item.chip.path) : undefined}
			/>
		{:else}
			<QueryChipGroup
				members={item.members}
				{problem}
				onremove={onremove ? () => onremove(item.path) : undefined}
				onremovemember={onremove}
			/>
		{/if}
	{/each}
{/if}
