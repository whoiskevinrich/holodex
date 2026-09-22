<script lang="ts">
	import { tick } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { forgetPlaylist } from '$lib/playlistContext';
	import type { Playlist } from '$lib/types';

	// The "Add to playlist" picker under the media page's PLAYLISTS row (F69 P0-7,
	// design handoff §3). Every playlist the owner has, updated_at DESC, as toggle
	// rows (✓ = member; a click PUTs or DELETEs membership) plus "+ New playlist…",
	// which creates and adds in one interaction. The parent owns the chip row: it
	// hands in the current memberships and is told about every change.
	let {
		videoId,
		members,
		onchange
	}: {
		videoId: number;
		members: Playlist[];
		onchange: (members: Playlist[]) => void;
	} = $props();

	let playlists = $state<Playlist[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busyId = $state<number | null>(null);
	const memberIds = $derived(new Set(members.map((p) => p.id)));

	$effect(() => {
		api
			.listPlaylists()
			.then((res) => (playlists = res.items ?? []))
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});

	async function toggle(p: Playlist) {
		if (busyId != null) return;
		busyId = p.id;
		error = '';
		const wasMember = memberIds.has(p.id); // before onchange flips the prop
		try {
			if (wasMember) {
				await api.removePlaylistVideo(p.id, videoId);
				onchange(members.filter((m) => m.id !== p.id));
			} else {
				const res = await api.addPlaylistVideo(p.id, videoId);
				onchange([res.playlist, ...members]);
			}
			forgetPlaylist(p.id); // the media page's cached order is now stale
			// Keep the row's count current without a second list fetch.
			playlists = playlists.map((q) =>
				q.id === p.id ? { ...q, item_count: q.item_count + (wasMember ? -1 : 1) } : q
			);
		} catch (e) {
			error = toMessage(e);
		} finally {
			busyId = null;
		}
	}

	let addOpen = $state(false);
	let addValue = $state('');
	let addInput = $state<HTMLInputElement | null>(null);
	let addBusy = $state(false);
	async function openAdd() {
		addValue = '';
		addOpen = true;
		await tick();
		addInput?.focus();
	}
	async function submitAdd(e: SubmitEvent) {
		e.preventDefault();
		const name = addValue.trim();
		if (!name || addBusy) return;
		addBusy = true;
		error = '';
		try {
			const created = await api.createPlaylist({ name });
			const res = await api.addPlaylistVideo(created.playlist.id, videoId);
			playlists = [res.playlist, ...playlists];
			onchange([res.playlist, ...members]);
			addOpen = false;
			addValue = '';
		} catch (e) {
			error = toMessage(e);
		} finally {
			addBusy = false;
		}
	}
</script>

<!-- Outside-click / Escape dismissal is the parent's (`use:dismissable` on the row that
     holds both the trigger and this panel, the HOLODEX-164 idiom). -->
<div class="w-full max-w-sm rounded-theme border border-rule bg-surface text-sm shadow-lg" role="dialog" aria-label="Add to playlist">
	{#if loading}
		<p class="px-3 py-2 text-muted">Loading…</p>
	{:else if playlists.length === 0 && !addOpen}
		<p class="px-3 py-2 text-muted">No playlists yet.</p>
	{:else}
		<ul role="list" class="max-h-64 overflow-y-auto py-1">
			{#each playlists as p (p.id)}
				<li>
					<button
						type="button"
						aria-pressed={memberIds.has(p.id)}
						disabled={busyId != null}
						onclick={() => toggle(p)}
						class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-ink hover:bg-surface-2"
					>
						<span class="w-4 shrink-0 text-accent" aria-hidden="true">{memberIds.has(p.id) ? '✓' : ''}</span>
						<span class="min-w-0 flex-1 truncate">{p.name}</span>
						<span class="shrink-0 text-xs text-muted">{p.item_count}</span>
					</button>
				</li>
			{/each}
		</ul>
	{/if}
	<div class="border-t border-rule px-3 py-2">
		{#if addOpen}
			<form onsubmit={submitAdd} class="flex items-center gap-2">
				<input
					bind:this={addInput}
					bind:value={addValue}
					type="text"
					placeholder="Name"
					aria-label="New playlist name"
					maxlength="200"
					class="min-w-0 flex-1 rounded-theme border border-rule bg-surface-2 px-2 py-1 text-ink focus:border-accent focus:outline-none"
				/>
				<button type="submit" disabled={addBusy} class="btn-accent px-2 py-1">Create &amp; add</button>
			</form>
		{:else}
			<button type="button" onclick={openAdd} class="text-accent hover:underline">+ New playlist…</button>
		{/if}
		{#if error}
			<p class="mt-1 text-warn">{error}</p>
		{/if}
	</div>
</div>
