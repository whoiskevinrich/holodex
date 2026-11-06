<script lang="ts">
	// The playlist editor's live preview (F89 P0-4): the unsaved definition read from GET
	// /media as a whole-list scroll, the cap applied here, since a cap isn't a browse key
	// (F89 RD3). Saving goes through the owner's PATCH, whose check is the real one.
	import { api } from '#lib/api.ts';
	import type { MediaFilters, Video } from '#lib/types.ts';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import ScrollPositionChip from '../shared/ScrollPositionChip.svelte';
	import VirtualVideoGrid from './VirtualVideoGrid.svelte';

	let {
		filters,
		cap,
		signature,
		onmatch
	}: {
		/** The draft's filters and sort. */
		filters: MediaFilters;
		cap: number | null;
		/** Restarts the list when the draft changes. */
		signature: string;
		/** The matches before the cap, null while reading. */
		onmatch: (matches: number | null) => void;
	} = $props();

	const videos = new SparseList<Video>(async (offset, limit) => {
		const c = cap;
		if (c != null && offset >= c) return { items: [], total: c, limit, offset };
		const res = await api.listMedia({ ...filters, offset, limit: c != null ? Math.min(limit, c - offset) : limit });
		onmatch(res.total);
		return { ...res, total: c != null ? Math.min(res.total, c) : res.total };
	});
	const opening = Math.random().toString(36).slice(2); // crypto.randomUUID needs HTTPS
	let listEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(240);
	const roving = new RovingList(virt, () => videos.total);
	wholeListPage({
		id: 'playlist-preview',
		list: videos,
		virt,
		roving,
		// Unique per opening: a draft has no saved position to return to, and landing on one
		// would scroll the editor off the screen.
		key: () => `${opening}:${signature}`,
		restartOn: () => {
			onmatch(null);
			void signature;
		},
		root: () => listEl
	});
</script>

{#if videos.error}
	<p class="py-16 text-center text-sm text-muted">This definition can’t be previewed: {videos.error}</p>
{:else if videos.total > 0}
	<ScrollPositionChip {virt} total={videos.total} />
	<VirtualVideoGrid list={videos} {virt} {roving} label="Preview of this definition" bind:root={listEl} />
{:else if !videos.loading}
	<p class="py-16 text-center text-sm text-muted">Nothing matches this definition yet.</p>
{/if}
