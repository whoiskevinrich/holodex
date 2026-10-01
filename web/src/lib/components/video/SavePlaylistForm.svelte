<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { segmentedToggleClass, segmentedToggleWrapperClass } from '../sort/segmentedToggle';

	// *Save as playlist…* (F75 handoff "Save form", P0-4): one form for every query-backed
	// grid, under its count line. **Smart** (the default) stores the grid's query and
	// re-runs it on every read (ADR-121); **Snapshot** is F69's `from_query`, which fixes
	// today's result. `query` is the grid's shareable filter string with its sort and no
	// paging; a random sort passes `seed` so a snapshot keeps the shuffle on screen (F69
	// P0-3) — a smart playlist ignores it and re-shuffles per play-through (RD4). On
	// success the page goes to the new playlist (there's no toast system).
	let {
		query,
		seed,
		defaultName = '',
		onclose
	}: { query: string; seed?: number; defaultName?: string; onclose: () => void } = $props();

	let name = $state('');
	let smart = $state(true);
	let busy = $state(false);
	let error = $state('');
	let input = $state<HTMLInputElement | null>(null);

	// Prefill once on open, then the name is the owner's: a later defaultName never
	// overwrites what they typed.
	name = untrack(() => defaultName);
	$effect(() => input?.focus());

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const n = name.trim();
		if (!n || busy) return;
		busy = true;
		error = '';
		try {
			let body: { name: string; query?: string; from_query?: string };
			if (smart) {
				body = { name: n, query };
			} else {
				const qs = new URLSearchParams(query);
				if (seed != null) qs.set('seed', String(seed));
				body = { name: n, from_query: qs.toString() };
			}
			const res = await api.createPlaylist(body);
			await goto(`/playlists/${res.playlist.id}`);
		} catch (err) {
			error = toMessage(err);
		} finally {
			busy = false;
		}
	}
</script>

<form onsubmit={submit} class="flex flex-wrap items-center gap-2">
	<input
		bind:this={input}
		bind:value={name}
		type="text"
		placeholder="Playlist name"
		aria-label="Playlist name"
		maxlength="200"
		class="rounded-theme border border-rule bg-surface px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none"
	/>
	<div class={segmentedToggleWrapperClass} role="group" aria-label="Playlist kind">
		<button type="button" aria-pressed={smart} onclick={() => (smart = true)} class={segmentedToggleClass(smart)}>
			Smart
		</button>
		<button type="button" aria-pressed={!smart} onclick={() => (smart = false)} class={segmentedToggleClass(!smart)}>
			Snapshot
		</button>
	</div>
	<button type="submit" disabled={busy} class="btn-accent px-3 py-2 text-sm">Save</button>
	<button type="button" onclick={onclose} disabled={busy} class="btn-quiet px-3 py-2 text-sm">Cancel</button>
	<p class="basis-full text-xs text-muted">
		Smart: updates as videos are added or tagged · Snapshot: stays as it is now
	</p>
	{#if error}
		<p class="basis-full text-sm text-warn">{error}</p>
	{/if}
</form>
