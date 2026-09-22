<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage } from '$lib/format';
	import { MEDIA_SORTS } from '$lib/filters';
	import { forgetPlaylist, playlistHref, setPlayIntent } from '$lib/playlistContext';
	import type { Playlist, Video } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';

	// /playlists/[id] (F69 P0-6, design handoff §2). A container page, not an entity
	// page: name, count + sort, the owner's controls, the browse tile grid in the
	// playlist's order, Delete at the very end. A visitor on a private playlist gets
	// the API's 404 rendered as the page's error state — the same as an unknown id
	// (ADR-104 D5).
	type PlaylistSort = string; // a browse sort key or 'manual'
	const MANUAL = { value: 'manual', label: 'Manual order' };

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	let playlist = $state<Playlist | null>(null);
	let items = $state<Video[]>([]);
	let loading = $state(true);
	let error = $state('');
	// The sort the dropdown binds to; written back to the playlist with PATCH.
	let sort = $state<PlaylistSort>('added_desc');
	// A 'random' playlist's seed is the one the server echoed for the order on screen
	// (ADR-045): every read and the Play all link carry that same seed, so the grid and
	// the play-through it starts walk one shuffle. Undefined asks the server to mint
	// one — a first load, a switch to Random, or a reroll.
	let seed = $state<number | undefined>(undefined);

	// Keyed on the id only: a sort change or a reroll re-reads through reload() below.
	$effect(() => {
		const current = id;
		let cancelled = false;
		loading = true;
		error = '';
		seed = undefined;
		api
			.getPlaylist(current)
			.then((res) => {
				if (cancelled) return;
				playlist = res.playlist;
				items = res.items;
				sort = res.playlist.sort;
				seed = res.seed;
			})
			.catch((e) => {
				if (!cancelled) error = toMessage(e);
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => (cancelled = true);
	});

	async function reload() {
		if (!playlist) return;
		const res = await api.getPlaylist(playlist.id, seed);
		playlist = res.playlist;
		items = res.items;
		seed = res.seed;
	}

	// Owner edits. Each one PATCHes and re-reads the items in place (a sort change
	// reorders; a rename and a visibility flip only touch the header).
	let actionError = $state('');
	async function patch(body: Parameters<typeof api.updatePlaylist>[1], refetch = false) {
		if (!playlist) return;
		actionError = '';
		try {
			const res = await api.updatePlaylist(playlist.id, body);
			playlist = res.playlist;
			forgetPlaylist(playlist.id); // a sort change reorders the play-through
			if (refetch) await reload();
		} catch (e) {
			actionError = toMessage(e);
		}
	}
	async function commitName(value: string): Promise<{ ok: true }> {
		await patch({ name: value });
		return { ok: true };
	}
	// Sort: the dropdown writes `sort`; this effect persists a change the owner made
	// (the load effect's own assignment is skipped by comparing with the stored sort).
	$effect(() => {
		const next = sort;
		if (!playlist || !isOwner || next === playlist.sort) return;
		seed = undefined; // a new sort is a new order; Random gets a fresh shuffle
		void patch({ sort: next }, true);
	});
	function reroll() {
		seed = undefined; // the server mints the next shuffle
		void reload();
	}

	async function remove(video: Video) {
		if (!playlist) return;
		actionError = '';
		try {
			await api.removePlaylistVideo(playlist.id, video.id);
			forgetPlaylist(playlist.id);
			items = items.filter((v) => v.id !== video.id);
			playlist = { ...playlist, item_count: items.length };
		} catch (e) {
			actionError = toMessage(e);
		}
	}

	let confirmDelete = $state(false);
	let deleteBusy = $state(false);
	let deleteError = $state('');
	async function deletePlaylist() {
		if (!playlist) return;
		deleteBusy = true;
		deleteError = '';
		try {
			await api.deletePlaylist(playlist.id);
			await goto('/playlists');
		} catch (e) {
			deleteError = toMessage(e);
			deleteBusy = false;
		}
	}

	const first = $derived(items[0] ?? null);
	const playParam = $derived(playlist ? { id: playlist.id, seed } : null);
	const sortLabel = $derived(
		sort === 'manual' ? MANUAL.label : (MEDIA_SORTS.find((s) => s.value === sort)?.label ?? sort)
	);
	const count = $derived(items.length === 1 ? '1 video' : `${items.length} videos`);
</script>

<AsyncState {loading} error={error || (!playlist ? 'Not found.' : '')}>
	{#if playlist}
		<section class="space-y-6">
			<div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
				<div class="min-w-0 space-y-1">
					<NameEditControl
						name={playlist.name}
						{isOwner}
						onCommit={commitName}
						label="playlist"
						headingClass="skin-title text-2xl font-semibold text-ink"
						pencilAlwaysVisible
					/>
					<p class="text-sm text-muted">{count} · {sortLabel}</p>
				</div>

				<div class="flex flex-wrap items-end gap-2">
					{#if isOwner}
						<div class="flex items-end gap-2">
							<SortDropdown bind:sort owner extra={[MANUAL]} />
							{#if sort === 'random'}
								<SortReroll onreroll={reroll} />
							{/if}
						</div>
						<!-- Visibility: a two-state segment; reversible in one click, nothing is
						     sent anywhere, so no confirm. -->
						<div role="radiogroup" aria-label="Visibility" class="flex items-center">
							{#each [['private', 'Private'], ['public', 'Public']] as [value, label] (value)}
								<button
									type="button"
									role="radio"
									aria-checked={playlist.visibility === value}
									onclick={() => patch({ visibility: value as Playlist['visibility'] })}
									class="btn-ghost px-3 py-2 text-sm first:rounded-r-none last:rounded-l-none last:border-l-0 {playlist.visibility === value
										? 'border-accent text-ink'
										: ''}"
								>
									{label}
								</button>
							{/each}
						</div>
					{/if}
					<!-- Play all: the page's one solid action. An <a> because it navigates; the
					     click is the gesture that sets the play-on-load intent. Withdrawn (ghost
					     look, same label) when there is nothing to play. -->
					{#if first && playParam}
						<a
							href={playlistHref(first.id, playParam)}
							onclick={() => setPlayIntent(first.id)}
							class="rounded-theme bg-accent px-4 py-2 text-sm font-medium text-accent-ink"
						>
							▶ Play all
						</a>
					{:else}
						<span class="btn-ghost px-4 py-2 text-sm" aria-disabled="true">▶ Play all</span>
					{/if}
				</div>
			</div>
			{#if actionError}
				<p class="text-sm text-warn" aria-live="polite">{actionError}</p>
			{/if}

			{#if items.length === 0}
				<p class="py-16 text-center text-sm text-muted">
					{#if isOwner}
						Nothing here yet — add videos from any video's page, or
						<a href="/" class="text-accent hover:underline">save a browse view</a> as a playlist.
					{:else}
						Nothing here yet.
					{/if}
				</p>
			{:else}
				<VideoGrid videos={items} onRemove={isOwner ? remove : undefined} />
			{/if}

			{#if isOwner}
				<div class="flex justify-end">
					<button type="button" onclick={() => (confirmDelete = true)} class="btn-quiet px-3 py-1.5 text-sm">
						Delete playlist
					</button>
				</div>
			{/if}
		</section>

		{#if confirmDelete}
			<ConfirmDialog
				title="Delete playlist?"
				confirmLabel="Delete"
				busy={deleteBusy}
				error={deleteError}
				onconfirm={deletePlaylist}
				oncancel={() => (confirmDelete = false)}
			>
				{#snippet body()}
					<p>
						<span class="font-semibold">{playlist?.name}</span> will be deleted. The {count}
						{items.length === 1 ? 'stays' : 'stay'} in your library.
					</p>
				{/snippet}
			</ConfirmDialog>
		{/if}
	{/if}
</AsyncState>
