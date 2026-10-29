<script lang="ts">
	// A People-List-shaped whole-list scroll for VirtualRows.svelte.test.ts: the page's row
	// and placeholder markup over a SparseList whose fetcher the test controls.
	// The app's stylesheet, so the row-height contract is measured on the real utilities.
	import '../../../../app.css';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import type { PageFetcher } from '#lib/pagedList.svelte.ts';
	import { RovingList, WindowVirtualizer } from '#lib/windowVirtualizer.svelte.ts';
	import VirtualRows from '../VirtualRows.svelte';

	type Row = { id: number; name: string };
	let { fetch, lanes = 1 }: { fetch: PageFetcher<Row>; lanes?: number } = $props();

	// svelte-ignore state_referenced_locally
	const list = new SparseList<Row>(fetch);
	const virt = new WindowVirtualizer(78);
	const roving = new RovingList(virt, () => list.total);
	list.load();
</script>

{#if list.total > 0}
	<VirtualRows {list} {virt} {roving} {lanes} label="People" noun="people" rowClass="grid gap-2 pb-2">
		{#snippet entry(p)}
			<div class="relative flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink">
				<a href={`/people/${p.id}`} class="flex min-w-0 flex-1 items-center gap-3">
					<span class="portrait-frame--1x1 block w-12 shrink-0 bg-surface-2"></span>
					<span class="flex-1 truncate">{p.name}</span>
				</a>
				<button type="button" class="text-xs">ring</button>
			</div>
		{/snippet}
		{#snippet placeholder()}
			<div class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5">
				<span class="portrait-frame--1x1 block w-12 shrink-0 border border-rule bg-surface-2"></span>
				<span class="h-2.5 w-2/5 rounded-theme bg-rule"></span>
			</div>
		{/snippet}
	</VirtualRows>
{/if}
