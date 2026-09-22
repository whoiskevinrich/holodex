<script lang="ts">
	import { tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage } from '$lib/format';
	import type { Playlist } from '$lib/types';
	import PlaylistVisibilityChip from '$lib/components/video/PlaylistVisibilityChip.svelte';

	// /playlists (F69 P0-5, design handoff §1). The /people list page's shell: title
	// row, `li > a` row cards, the three states. The server already filters by
	// visibility — a visitor's rows are all public, so the chip is owner-only.
	let playlists = $state<Playlist[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	$effect(() => {
		api
			.listPlaylists()
			.then((res) => (playlists = res.items ?? []))
			.catch((err) => {
				loadError = toMessage(err);
				playlists = [];
			})
			.finally(() => (loading = false));
	});

	// + New playlist expands in place (the tag-add form): Enter submits via the form,
	// Cancel closes. An empty playlist is a valid destination (spec story 11).
	let addOpen = $state(false);
	let addValue = $state('');
	let addInput = $state<HTMLInputElement | null>(null);
	let addBusy = $state(false);
	let addError = $state('');
	// An owner with nothing yet lands on the form already open.
	$effect(() => {
		if (!loading && isOwner && playlists.length === 0 && !loadError) void openAdd();
	});

	async function openAdd() {
		addValue = '';
		addError = '';
		addOpen = true;
		await tick();
		addInput?.focus();
	}
	function closeAdd() {
		addOpen = false;
		addValue = '';
		addError = '';
	}
	async function submitAdd(e: SubmitEvent) {
		e.preventDefault();
		const name = addValue.trim();
		if (!name || addBusy) return;
		addBusy = true;
		addError = '';
		try {
			const res = await api.createPlaylist({ name });
			await goto(`/playlists/${res.playlist.id}`);
		} catch (err) {
			addError = toMessage(err);
		} finally {
			addBusy = false;
		}
	}

	const count = $derived(
		playlists.length === 1 ? '1 playlist' : `${playlists.length} playlists`
	);
</script>

<section class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="flex items-baseline gap-3">
			<h1 class="skin-title text-2xl font-semibold text-ink">Playlists</h1>
			{#if !loading && !loadError}
				<span class="text-sm text-muted">{count}</span>
			{/if}
		</div>
		{#if isOwner && !addOpen}
			<button type="button" onclick={openAdd} class="btn-accent px-3 py-1.5 text-sm">+ New playlist</button>
		{/if}
	</div>

	{#if isOwner && addOpen}
		<form onsubmit={submitAdd} class="flex flex-wrap items-center gap-2">
			<input
				bind:this={addInput}
				bind:value={addValue}
				type="text"
				placeholder="Playlist name"
				aria-label="Playlist name"
				maxlength="200"
				class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
			/>
			<button type="submit" disabled={addBusy} class="btn-accent px-3 py-1.5 text-sm">Create</button>
			<button type="button" onclick={closeAdd} disabled={addBusy} class="btn-quiet px-3 py-1.5 text-sm">Cancel</button>
			{#if addError}
				<p class="basis-full text-sm text-warn">{addError}</p>
			{/if}
		</form>
	{/if}

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if loadError}
		<p class="py-16 text-center text-sm text-warn">Couldn’t load playlists: {loadError}</p>
	{:else if playlists.length === 0}
		<p class="py-16 text-center text-sm text-muted">
			{isOwner ? 'No playlists yet.' : 'No playlists shared yet.'}
		</p>
	{:else}
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each playlists as p (p.id)}
				<li>
					<a
						href={`/playlists/${p.id}`}
						class="flex items-center gap-3 rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent"
					>
						<!-- Stacked-frames glyph: the one playlist-specific mark, standing in for
						     the cover P2-3 may add. -->
						<svg class="h-5 w-5 shrink-0 text-muted" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
							<rect x="2.75" y="5.75" width="11.5" height="11.5" rx="1.5" />
							<path d="M6.5 2.75h9.25a1.5 1.5 0 0 1 1.5 1.5V13.5" />
						</svg>
						<span class="flex-1 truncate">{p.name}</span>
						{#if isOwner}
							<PlaylistVisibilityChip visibility={p.visibility} />
						{/if}
						<span class="text-xs text-muted">{p.item_count}</span>
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</section>
