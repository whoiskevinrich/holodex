<script lang="ts">
	// A playlist's tiles as one whole-list scroll (F80 EL11/EL14, HOLODEX-604): smart and
	// snapshot alike, a page at a time from GET /playlists/{id} where it's viewed. The page
	// owns the header read, which settles the seed; mount this once the playlist has loaded,
	// so every page reads under that one shuffle. A sort or seed change starts over at the
	// top; bump `refreshKey` after a remove or a Freeze to re-read in place, keeping the scroll.
	import { untrack } from 'svelte';
	import { api } from '#lib/api.ts';
	import type { Video } from '#lib/types.ts';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import ScrollPositionChip from '../shared/ScrollPositionChip.svelte';
	import VirtualVideoGrid from './VirtualVideoGrid.svelte';

	let {
		playlistId,
		sort,
		seed,
		refreshKey = 0,
		onRemove
	}: {
		playlistId: number;
		sort: string;
		seed?: number;
		refreshKey?: number;
		onRemove?: (video: Video) => void;
	} = $props();

	const videos = new SparseList<Video>((offset, limit) => api.getPlaylist(playlistId, seed, { limit, offset }));
	let listEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(240);
	const roving = new RovingList(virt, () => videos.total);
	wholeListPage({
		id: 'playlist',
		list: videos,
		virt,
		roving,
		key: () => `${playlistId}:${sort}`,
		restartOn: () => {
			void playlistId;
			void sort;
			void seed;
		},
		root: () => listEl
	});

	let lastRefresh = untrack(() => refreshKey);
	$effect(() => {
		if (refreshKey !== lastRefresh) {
			lastRefresh = refreshKey;
			untrack(() => videos.refresh());
		}
	});
</script>

{#if videos.error}
	<p class="py-16 text-center text-sm text-warn">Couldn’t load this playlist’s videos: {videos.error}</p>
{:else if videos.total > 0}
	<ScrollPositionChip {virt} total={videos.total} />
	<VirtualVideoGrid list={videos} {virt} {roving} label="Playlist videos" {onRemove} bind:root={listEl} />
{/if}
