<script lang="ts">
	// One F87 OR group in the chips row (query-language-handoff.md, approved Option A): an
	// inset box of member chips joined by `or`, an AND set wrapped in muted ( ) with `+`
	// between, and one trailing × that removes the whole group. It wraps as one unit. With
	// no onremove it is read-only (F89), and `problem` marks a member a refusal names.
	import FilterChip from './FilterChip.svelte';
	import { groupLabel, type ChipCond } from '#lib/query.ts';

	let {
		members,
		onremove,
		onremovemember,
		problem
	}: {
		/** Each member is one chip or an AND set of chips. */
		members: ChipCond[][];
		/** Removes the whole group. */
		onremove?: () => void;
		/** Removes one member chip, by its path in the query tree. */
		onremovemember?: (path: number[]) => void;
		problem?: (chip: ChipCond) => boolean;
	} = $props();

	const spoken = $derived(groupLabel(members));
</script>

<span
	role="group"
	aria-label={`Any of: ${spoken}`}
	class="inline-flex shrink-0 items-center gap-1 rounded-theme border border-rule bg-surface-2 p-1 text-xs"
>
	{#each members as set, i (set.map((c) => c.path.join('.')).join('|'))}
		{#if i > 0}<span class="text-muted">or</span>{/if}
		{#if set.length > 1}<span class="text-muted" aria-hidden="true">(</span>{/if}
		{#each set as c, j (c.path.join('.'))}
			{#if j > 0}<span class="text-muted" aria-hidden="true">+</span>{/if}
			<FilterChip
				label={c.label}
				negated={c.negated}
				ownerOnly={c.ownerOnly}
				problem={problem?.(c)}
				onremove={onremovemember ? () => onremovemember(c.path) : undefined}
			/>
		{/each}
		{#if set.length > 1}<span class="text-muted" aria-hidden="true">)</span>{/if}
	{/each}
	{#if onremove}
		<button
			type="button"
			onclick={onremove}
			aria-label={`Remove group: ${spoken}`}
			class="rounded-theme px-1 leading-none text-ink hover:bg-surface"
		>
			×
		</button>
	{/if}
</span>
