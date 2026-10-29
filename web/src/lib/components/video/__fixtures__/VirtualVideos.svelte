<script lang="ts">
	// The whole-list video grid for VirtualVideoGrid.svelte.test.ts: the real card and
	// placeholder over a SparseList whose fetcher the test controls, with the app's
	// stylesheet so the row-height contract is measured on the real utilities.
	import '../../../../app.css';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import type { PageFetcher } from '#lib/pagedList.svelte.ts';
	import type { Video } from '#lib/types.ts';
	import { RovingList, WindowVirtualizer } from '#lib/windowVirtualizer.svelte.ts';
	import VirtualVideoGrid from '../VirtualVideoGrid.svelte';

	let { fetch }: { fetch: PageFetcher<Video> } = $props();

	// svelte-ignore state_referenced_locally
	const list = new SparseList<Video>(fetch);
	const virt = new WindowVirtualizer(240);
	const roving = new RovingList(virt, () => list.total);
	list.load();
</script>

{#if list.total > 0}
	<VirtualVideoGrid {list} {virt} {roving} />
{/if}
