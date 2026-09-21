<script lang="ts">
	import type { Playlist, Video } from '$lib/types';
	import { formatDuration } from '$lib/format';
	import { hotkey } from '$lib/actions/hotkey.svelte';
	import { neighbours, playlistHref, setPlayIntent, type PlaylistParam } from '$lib/playlistContext';

	// The next-up strip under the media page's player (F69 P0-9, design handoff §5).
	// A sibling of the player box, never a wrapper — it mounts and unmounts with the
	// `?playlist=` context without touching the persistent <video>. Three states:
	// playing from (Prev / Next), last item (Prev / ↺ Start), and not a member — a
	// stale link or a removed item — (Play from start). Every navigation here is a
	// gesture, so it sets the play-on-load intent; a reload or a shared link shows the
	// strip and waits.
	let {
		playlist,
		items,
		param,
		videoId
	}: { playlist: Playlist; items: Video[]; param: PlaylistParam; videoId: number } = $props();

	const ids = $derived(items.map((v) => v.id));
	const nav = $derived(neighbours(ids, videoId));
	const nextItem = $derived(nav.next == null ? null : (items.find((v) => v.id === nav.next) ?? null));
	const first = $derived(items[0]?.id ?? null);

	const label = $derived(nav.index === -1 ? 'Not in' : nav.next == null ? 'Last in' : 'Playing from');
	const position = $derived(
		nav.index === -1
			? 'this video was removed, or the link is stale'
			: `${nav.index + 1} of ${ids.length}${nav.next == null ? ' — playback stops here' : ''}`
	);

	const go = (id: number) => () => setPlayIntent(id);
</script>

<div class="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-theme border border-rule bg-surface px-3 py-2 text-sm">
	<p class="min-w-0 flex-1 basis-48">
		<span class="text-xs uppercase tracking-wide text-muted">{label}</span>
		<a href="/playlists/{playlist.id}" class="font-medium text-accent hover:underline">{playlist.name}</a>
		<span class="text-muted" aria-live="polite">· {position}</span>
	</p>

	{#if nextItem}
		<a
			href={playlistHref(nextItem.id, param)}
			onclick={go(nextItem.id)}
			class="flex min-w-0 items-center gap-2 text-ink hover:underline"
		>
			{#if nextItem.thumbnail_url}
				<img src={nextItem.thumbnail_url} alt="" loading="lazy" class="h-8 w-14 shrink-0 rounded-theme bg-black object-cover" />
			{/if}
			<span class="shrink-0 text-xs uppercase tracking-wide text-muted">Next</span>
			<span class="truncate">{nextItem.title}</span>
			<span class="shrink-0 text-muted">· {formatDuration(nextItem.duration_sec)}</span>
		</a>
	{/if}

	<span class="ml-auto flex items-center gap-2">
		{#if nav.prev != null}
			<a
				href={playlistHref(nav.prev, param)}
				onclick={go(nav.prev)}
				class="btn-ghost px-3 py-1.5"
				aria-label="Previous in playlist"
				use:hotkey={{ key: 'p', label: 'Previous in playlist' }}
			>
				‹ Prev
			</a>
		{/if}
		{#if nav.next != null}
			<a
				href={playlistHref(nav.next, param)}
				onclick={go(nav.next)}
				class="btn-accent px-3 py-1.5"
				aria-label="Next in playlist"
				use:hotkey={{ key: 'n', label: 'Next in playlist' }}
			>
				Next ›
			</a>
		{:else if first != null}
			<a
				href={playlistHref(first, param)}
				onclick={go(first)}
				class="{nav.index === -1 ? 'btn-accent' : 'btn-ghost'} px-3 py-1.5"
			>
				{nav.index === -1 ? 'Play from start' : '↺ Start'}
			</a>
		{/if}
	</span>
</div>
