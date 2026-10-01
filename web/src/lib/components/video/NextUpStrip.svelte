<script lang="ts">
	import { untrack } from 'svelte';
	import type { Video } from '$lib/types';
	import { formatDuration } from '$lib/format';
	import { hotkey } from '$lib/actions/hotkey.svelte';
	import { setPlayIntent } from '$lib/playlistContext';
	import { neighbours, runHref, type Run } from '$lib/run';
	import { nextTile } from '$lib/runContext';

	// The next-up strip under the media page's player (F69 P0-9; F75 run strip, design
	// handoff §3). A sibling of the player box, never a wrapper — it mounts and unmounts
	// with the run without touching the persistent <video>. Three states: playing from
	// (Prev / Next), last item (Prev / ↺ Start), and not in the run — a stale link or a
	// removed item — (Play from start). Every navigation here is a gesture, so it sets
	// the play-on-load intent; a reload or a shared link shows the strip and waits.
	//
	// One strip for every run source: a playlist shows its name alone (as F69), a grid
	// shows its kind and name ("Person · Ana"). The shuffle and repeat toggles only change
	// the run's client-held state (spec RD11/RD12): the page owns the run and applies them.
	let {
		run,
		videoId,
		onshuffle,
		onrepeat
	}: { run: Run; videoId: number; onshuffle: () => void; onrepeat: () => void } = $props();

	const nav = $derived(neighbours(run.order, videoId));
	const first = $derived(run.order[0] ?? null);
	const shuffled = $derived(run.mode === 'shuffled');
	const label = $derived(run.source.label);

	// Keyed on the next item alone: a toggle replaces `run` but often leaves the next item
	// as it was, and the preview shouldn't blank for that.
	let tile = $state<Video | null>(null);
	$effect(() => {
		const next = nav.next;
		if (next == null) {
			tile = null;
			return;
		}
		if (untrack(() => tile)?.id === next) return;
		let cancelled = false;
		nextTile(untrack(() => run), next).then((v) => {
			if (!cancelled) tile = v;
		});
		return () => (cancelled = true);
	});

	// The live region says which way a toggle went (handoff §3): "Shuffled" or "In order".
	let announce = $state('');
	let lastMode = untrack(() => run.mode);
	$effect(() => {
		const mode = run.mode;
		if (mode !== lastMode) announce = mode === 'shuffled' ? 'Shuffled' : 'In order';
		lastMode = mode;
	});

	const lead = $derived(nav.index === -1 ? 'Not in' : nav.next == null && !run.repeat ? 'Last in' : 'Playing from');
	const position = $derived(
		nav.index === -1
			? 'this video was removed, or the link is stale'
			: `${nav.index + 1} of ${run.order.length}${shuffled ? ' · shuffled' : ''}${
					nav.next == null && !run.repeat ? ' — playback stops here' : ''
				}`
	);

	const go = (id: number) => () => setPlayIntent(id);
</script>

<div class="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-theme border border-rule bg-surface px-3 py-2 text-sm">
	<p class="min-w-0 flex-1 basis-48">
		<span class="text-xs uppercase tracking-wide text-muted">{lead}</span>
		{#if label.name}
			{#if label.kind}<span class="text-muted">{label.kind} ·</span>{/if}
			<a
				href={label.href}
				title={label.name}
				class="inline-block max-w-[24ch] truncate align-bottom font-medium text-accent hover:underline">{label.name}</a
			>
		{:else}
			<!-- No name known (a browse run, or one rebuilt from a URL): the kind is the link. -->
			<a href={label.href} class="font-medium text-accent hover:underline">{label.kind || 'Playlist'}</a>
		{/if}
		<span class="text-muted" aria-live="polite">· {position}</span>
		<span class="sr-only" aria-live="polite">{announce}</span>
	</p>

	{#if nav.next != null && tile?.id === nav.next}
		<a
			href={runHref(tile.id, run)}
			onclick={go(tile.id)}
			class="flex min-w-0 items-center gap-2 text-ink hover:underline"
		>
			{#if tile.thumbnail_url}
				<img src={tile.thumbnail_url} alt="" loading="lazy" class="h-8 w-14 shrink-0 rounded-theme bg-black object-cover" />
			{/if}
			<span class="shrink-0 text-xs uppercase tracking-wide text-muted">Next</span>
			<span class="truncate">{tile.title}</span>
			<span class="shrink-0 text-muted">· {formatDuration(tile.duration_sec)}</span>
		</a>
	{/if}

	<span class="ml-auto flex items-center gap-2">
		<button
			type="button"
			onclick={onshuffle}
			aria-pressed={shuffled}
			aria-label="Shuffle"
			title="Shuffle ({shuffled ? 'on' : 'off'})"
			class="btn-ghost px-2 py-1.5 {shuffled ? 'border-accent text-accent' : ''}"
		>
			<svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				<path d="M2 4h3l6 8h3M2 12h3l6-8h3M12 2l2 2-2 2M12 10l2 2-2 2" />
			</svg>
		</button>
		<button
			type="button"
			onclick={onrepeat}
			aria-pressed={run.repeat}
			aria-label="Repeat"
			title="Repeat ({run.repeat ? 'on' : 'off'})"
			class="btn-ghost px-2 py-1.5 {run.repeat ? 'border-accent text-accent' : ''}"
		>
			<svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				<path d="M3 7V5h9l-2-2M13 9v2H4l2 2" />
			</svg>
		</button>
		{#if nav.prev != null}
			<a
				href={runHref(nav.prev, run)}
				onclick={go(nav.prev)}
				class="btn-ghost px-3 py-1.5"
				aria-label="Previous"
				use:hotkey={{ key: 'p', label: 'Previous in run' }}
			>
				‹ Prev
			</a>
		{/if}
		{#if nav.next != null}
			<a
				href={runHref(nav.next, run)}
				onclick={go(nav.next)}
				class="btn-accent px-3 py-1.5"
				aria-label="Next"
				use:hotkey={{ key: 'n', label: 'Next in run' }}
			>
				Next ›
			</a>
		{:else if first != null}
			<a
				href={runHref(first, run)}
				onclick={go(first)}
				class="{nav.index === -1 ? 'btn-accent' : 'btn-ghost'} px-3 py-1.5"
			>
				{nav.index === -1 ? 'Play from start' : '↺ Start'}
			</a>
		{/if}
	</span>
</div>
