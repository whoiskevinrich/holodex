<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, videoCount } from '$lib/format';
	import { MEDIA_SORTS } from '$lib/filters';
	import { forgetPlaylist } from '$lib/playlistContext';
	import { beginRun } from '$lib/runContext';
	import type { RunMode } from '$lib/run';
	import PlaySplitButton from '$lib/components/video/PlaySplitButton.svelte';
	import PlaylistVisibilityChip from '$lib/components/video/PlaylistVisibilityChip.svelte';
	import type { Playlist, PlaylistResponse, PlaylistStaleRef, Video } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import Switch from '$lib/components/shared/Switch.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';
	import SortDropdown from '$lib/components/sort/SortDropdown.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';

	// /playlists/[id] (F69 P0-6, design handoff §2). A container page, not an entity
	// page: name, count + sort, the owner's controls, the browse tile grid in the
	// playlist's order, Delete at the very end. A visitor on a private playlist gets
	// the API's 404 rendered as the page's error state — the same as an unknown id
	// (ADR-104 D5).
	//
	// A smart playlist (F75 handoff §4, ADR-121) is the same page plus a `smart` chip,
	// Edit filter, Freeze, no Manual order and no per-tile remove; its tiles page like
	// browse (Load more), and a stale query replaces the grid with a notice.
	type PlaylistSort = string; // a browse sort key or 'manual'
	const MANUAL = { value: 'manual', label: 'Manual order' };
	const PAGE_SIZE = 50;

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	let playlist = $state<Playlist | null>(null);
	let items = $state<Video[]>([]);
	let total = $state(0);
	let staleRefs = $state<PlaylistStaleRef[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');
	// The sort the dropdown binds to; written back to the playlist with PATCH.
	let sort = $state<PlaylistSort>('added_desc');
	// A 'random' playlist's seed is the one the server echoed for the order on screen
	// (ADR-045): every read and the Play all link carry that same seed, so the grid and
	// the play-through it starts walk one shuffle. Undefined asks the server to mint
	// one — a first load, a switch to Random, or a reroll.
	let seed = $state<number | undefined>(undefined);

	const smart = $derived(playlist?.query != null);
	const hasMore = $derived(smart && items.length < total);

	// Snapshot playlists ignore the page and return every item; a smart one pages.
	const firstPage = { limit: PAGE_SIZE, offset: 0 };

	// Keyed on the id only: a sort change or a reroll re-reads through reload() below.
	$effect(() => {
		const current = id;
		let cancelled = false;
		loading = true;
		error = '';
		seed = undefined;
		api
			.getPlaylist(current, undefined, firstPage)
			.then((res) => {
				if (cancelled) return;
				apply(res);
				sort = res.playlist.sort;
			})
			.catch((e) => {
				if (!cancelled) error = toMessage(e);
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => (cancelled = true);
	});

	// Every read lands through here; `generation` drops a Load more page that a reload
	// (a sort change, a reroll, Freeze) has since replaced the grid under.
	let generation = 0;
	function apply(res: PlaylistResponse) {
		generation++;
		playlist = res.playlist;
		items = res.items;
		total = res.total;
		staleRefs = res.stale_refs ?? [];
		seed = res.seed;
	}

	async function reload() {
		if (!playlist) return;
		apply(await api.getPlaylist(playlist.id, seed, firstPage));
	}

	async function loadMore() {
		if (!playlist || loadingMore) return;
		loadingMore = true;
		const mine = generation;
		try {
			const res = await api.getPlaylist(playlist.id, seed, { limit: PAGE_SIZE, offset: items.length });
			if (mine !== generation) return;
			items = [...items, ...res.items];
			total = res.total;
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			loadingMore = false;
		}
	}

	// Owner edits. Each one PATCHes and re-reads the items in place (a sort change
	// reorders; a rename and a visibility flip only touch the header).
	let actionError = $state('');
	async function patch(body: Parameters<typeof api.updatePlaylist>[1], refetch = false): Promise<boolean> {
		if (!playlist) return false;
		actionError = '';
		try {
			const res = await api.updatePlaylist(playlist.id, body);
			playlist = res.playlist;
			forgetPlaylist(playlist.id); // a sort change reorders the play-through
			if (refetch) await reload();
			return true;
		} catch (e) {
			actionError = toMessage(e);
			return false;
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

	// Always shuffle (F75 P0-12): optimistic, reverted with a warn line on failure.
	let shuffleError = $state('');
	let shuffleBusy = $state(false);
	async function toggleShuffled() {
		if (!playlist || shuffleBusy) return;
		const next = !playlist.play_shuffled;
		playlist = { ...playlist, play_shuffled: next };
		shuffleError = '';
		shuffleBusy = true;
		try {
			playlist = (await api.updatePlaylist(playlist.id, { play_shuffled: next })).playlist;
		} catch (e) {
			playlist = { ...playlist, play_shuffled: !next };
			shuffleError = toMessage(e);
		} finally {
			shuffleBusy = false;
		}
	}

	async function remove(video: Video) {
		if (!playlist) return;
		actionError = '';
		try {
			await api.removePlaylistVideo(playlist.id, video.id);
			forgetPlaylist(playlist.id);
			items = items.filter((v) => v.id !== video.id);
			total = items.length;
			playlist = { ...playlist, item_count: items.length };
		} catch (e) {
			actionError = toMessage(e);
		}
	}

	// Freeze (F75 P0-7): the current result becomes a fixed membership; the chip and Edit
	// filter go, Manual order arrives.
	let confirmFreeze = $state(false);
	let freezeBusy = $state(false);
	let freezeError = $state('');
	async function freeze() {
		if (!playlist) return;
		freezeBusy = true;
		freezeError = '';
		try {
			await api.freezePlaylist(playlist.id);
			forgetPlaylist(playlist.id);
			await reload();
			confirmFreeze = false;
		} catch (e) {
			freezeError = toMessage(e);
		} finally {
			freezeBusy = false;
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

	// Play all / Shuffle start a run with this playlist as its source (F75, ADR-121 D7).
	// A 'random' playlist already plays in a shuffled order (the seed on screen), so its
	// two actions coincide (spec RD10): both walk that order.
	function playRun(mode: RunMode): Promise<boolean> {
		if (!playlist) return Promise.resolve(false);
		return beginRun(
			{ kind: 'playlist', id: playlist.id, seed, label: { kind: '', name: playlist.name, href: `/playlists/${playlist.id}` } },
			playlist.sort === 'random' ? 'in-order' : mode
		);
	}

	// Edit filter (handoff §5): browse with the stored query and the playlist's sort, in
	// edit mode. The flag is UI state, so it's in the fragment, never the filter string.
	const editHref = $derived.by(() => {
		const p = new URLSearchParams(playlist?.query ?? '');
		if (playlist) p.set('sort', playlist.sort);
		return `/?${p}#edit_playlist=${id}`;
	});

	// Stale-reference notice copy (handoff §4), by the first ref's kind.
	const ENTITY_NOUN: Record<string, string> = { person: 'person', tag: 'tag', studio_id: 'studio', category_id: 'category' };
	const staleCopy = $derived.by(() => {
		const r = staleRefs[0];
		if (!r) return null;
		const fix = 'Nothing is shown until the filter is fixed.';
		switch (r.kind) {
			case 'missing': {
				const noun = ENTITY_NOUN[r.key ?? ''] ?? 'entry';
				return { sentence: `This playlist refers to a ${noun} that was deleted`, detail: `(${noun} #${r.value}). ${fix}` };
			}
			case 'unknown_key':
				return { sentence: 'This playlist filters on a field that no longer exists', detail: `(${r.key}). ${fix}` };
			case 'owner_only':
				return { sentence: 'This public playlist uses an owner-only filter', detail: 'Make it private or edit the filter.' };
			default:
				return { sentence: "This playlist's filter can't be read", detail: fix };
		}
	});

	const sortLabel = $derived(
		sort === 'manual' ? MANUAL.label : (MEDIA_SORTS.find((s) => s.value === sort)?.label ?? sort)
	);
	const count = $derived(videoCount(total));
</script>

<AsyncState {loading} error={error || (!playlist ? 'Not found.' : '')}>
	{#if playlist}
		<section class="space-y-6">
			<div class="space-y-3">
				<div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
					<div class="min-w-0 space-y-1">
						<div class="flex flex-wrap items-center gap-3">
							<NameEditControl
								name={playlist.name}
								{isOwner}
								onCommit={commitName}
								label="playlist"
								headingClass="skin-title text-2xl font-semibold text-ink"
								pencilAlwaysVisible
							/>
							{#if smart}<PlaylistVisibilityChip smart />{/if}
						</div>
						<p class="text-sm text-muted">
							{count} · {sortLabel}{#if playlist.play_shuffled}{' · '}<span class="text-xs text-accent"
									>always shuffled</span
								>{/if}
						</p>
					</div>

					<div class="flex flex-wrap items-end gap-2">
						{#if isOwner}
							{#if smart}
								<a href={editHref} class="btn-ghost px-3 py-2 text-sm">Edit filter</a>
							{/if}
							<div class="flex items-end gap-2">
								<!-- A smart playlist has no position to honour: no Manual order (RD4). -->
								<SortDropdown bind:sort owner extra={smart ? [] : [MANUAL]} />
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
						<!-- Play all ▾ (F75 handoff §4): the page's one solid action, Shuffle in its
						     menu — or first, with Always shuffle on. Withdrawn (ghost look, same
						     label) when there is nothing to play. -->
						<PlaySplitButton
							variant="primary"
							shuffleFirst={playlist.play_shuffled}
							disabled={total === 0}
							onplay={playRun}
						/>
					</div>
				</div>
				{#if isOwner}
					<div class="flex flex-wrap items-center gap-4 text-xs">
						<Switch checked={playlist.play_shuffled} onclick={toggleShuffled} disabled={shuffleBusy} label="Always shuffle" />
						{#if smart && !staleCopy}
							<button type="button" onclick={() => (confirmFreeze = true)} class="btn-quiet text-xs">
								Freeze into a fixed playlist…
							</button>
						{/if}
						{#if shuffleError}<p class="text-warn">{shuffleError}</p>{/if}
					</div>
				{/if}
			</div>
			{#if actionError}
				<p class="text-sm text-warn" aria-live="polite">{actionError}</p>
			{/if}

			{#if staleCopy}
				<!-- Stale-reference notice (handoff §4): replaces the grid; nothing is evaluated
				     until the filter is fixed (ADR-121 D5). -->
				<div role="status" class="flex flex-wrap items-center justify-between gap-3 rounded-theme border border-warn bg-surface px-3 py-2">
					{#if isOwner}
						<p class="text-sm">
							<span class="text-warn">{staleCopy.sentence}</span>
							<span class="text-xs text-muted">{staleCopy.detail}</span>
						</p>
						<div class="flex items-center gap-2">
							<a href={editHref} class="btn-accent px-3 py-1.5 text-sm">Edit filter</a>
							<button type="button" onclick={() => (confirmDelete = true)} class="btn-quiet px-3 py-1.5 text-sm">
								Delete playlist
							</button>
						</div>
					{:else}
						<p class="text-sm text-warn">This playlist can’t be shown right now.</p>
					{/if}
				</div>
			{:else if items.length === 0}
				<p class="py-16 text-center text-sm text-muted">
					{#if smart}
						Nothing matches this filter yet.
					{:else if isOwner}
						Nothing here yet — add videos from any video's page, or
						<a href="/" class="text-accent hover:underline">save a browse view</a> as a playlist.
					{:else}
						Nothing here yet.
					{/if}
				</p>
			{:else}
				<VideoGrid videos={items} onRemove={isOwner && !smart ? remove : undefined} />
				{#if hasMore}
					<div class="flex justify-center pt-2">
						<button onclick={loadMore} disabled={loadingMore} class="btn-ghost px-4 py-2 text-sm">
							{loadingMore ? 'Loading…' : `Load more (${total - items.length} left)`}
						</button>
					</div>
				{/if}
			{/if}

			{#if isOwner}
				<div class="flex justify-end">
					<button type="button" onclick={() => (confirmDelete = true)} class="btn-quiet px-3 py-1.5 text-sm">
						Delete playlist
					</button>
				</div>
			{/if}
		</section>

		{#if confirmFreeze}
			<ConfirmDialog
				title="Freeze playlist?"
				confirmLabel="Freeze"
				variant="accent"
				busy={freezeBusy}
				error={freezeError}
				onconfirm={freeze}
				oncancel={() => (confirmFreeze = false)}
			>
				{#snippet body()}
					<p>
						Freeze <span class="font-semibold">{playlist?.name}</span>? It will keep its current
						{count} and stop updating.
					</p>
				{/snippet}
			</ConfirmDialog>
		{/if}

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
						{total === 1 ? 'stays' : 'stay'} in your library.
					</p>
				{/snippet}
			</ConfirmDialog>
		{/if}
	{/if}
</AsyncState>
