<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import type { ExtraMetadata, MappedField, RelatedResponse, Video } from '$lib/types';
	import { formatBitrate, formatBytes, formatDuration, formatYear, resolutionBucket, toMessage } from '$lib/format';
	import RelatedShelf from '$lib/components/RelatedShelf.svelte';
	import PersonPoster from '$lib/components/PersonPoster.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	let video = $state<Video | null>(null);
	let extra = $state<ExtraMetadata[]>([]);
	let fields = $state<MappedField[]>([]);
	let related = $state<RelatedResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let playFailed = $state(false);
	let showRaw = $state(false);
	let regenerating = $state(false);
	let thumbVersion = $state(0); // cache-bust the preview after a regenerate

	// Owner-only delete (F24, ADR-037). confirmMode drives which confirm dialog shows.
	let confirmMode = $state<'soft' | 'purge' | null>(null);
	let deleteBusy = $state(false);
	let deleteError = $state('');

	const id = $derived(Number($page.params.id));

	const graceDays = $derived(
		activity.caps?.delete_grace_period_seconds
			? Math.round(activity.caps.delete_grace_period_seconds / 86400)
			: 0
	);

	function openConfirm(mode: 'soft' | 'purge') {
		deleteError = '';
		confirmMode = mode;
	}

	async function confirmDelete() {
		if (!video || deleteBusy) return;
		deleteBusy = true;
		deleteError = '';
		try {
			await api.deleteMedia(video.id, { purge: confirmMode === 'purge' });
			// The item is gone from the library — returning to the grid (where it no
			// longer appears) is the feedback (no toast system yet, per the handoff).
			goto('/');
		} catch (e) {
			deleteError = toMessage(e);
			deleteBusy = false; // keep the dialog open so the message is visible
		}
	}

	async function regenerateThumbnail() {
		if (!video) return;
		regenerating = true;
		try {
			await api.regenerateThumbnail(video.id);
			// Give the worker a moment, then refresh the poster with a busted URL.
			setTimeout(() => (thumbVersion += 1), 4000);
		} catch (e) {
			// Generation may be disabled (503) or the request may fail; non-fatal.
			console.warn('thumbnail regenerate failed', e);
		} finally {
			regenerating = false;
		}
	}

	// Hide the full-viewport atmosphere overlay (.app-atmosphere::after, z-40) while a
	// video plays so the scan/vignette flourishes don't sit on top of the picture —
	// worst in Broadcast. Pure-CSS-gated: we only toggle the class; app.css owns the rule.
	function setPlaying(on: boolean) {
		document.body?.classList.toggle('is-playing', on);
	}
	// Restore the atmosphere on teardown (navigate away mid-play). The data-load effect
	// below also resets it per id — this route's component is REUSED across /media/[id]
	// param changes, so a play state from the previous item would otherwise linger.
	$effect(() => () => setPlaying(false));

	$effect(() => {
		const current = id;
		let cancelled = false; // ignore a stale response if id changes before it resolves
		loading = true;
		error = '';
		playFailed = false;
		setPlaying(false); // a freshly-opened item starts with the atmosphere visible
		api
			.getMedia(current)
			.then((res) => {
				if (cancelled) return;
				video = res.video;
				extra = res.metadata ?? [];
				fields = res.fields ?? [];
			})
			.catch((e) => {
				if (!cancelled) error = toMessage(e);
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => (cancelled = true);
	});

	// Related "More with …" shelves (QW2/QW3). Non-blocking and tracks ONLY `id`, so it
	// fetches once per page view and the shelves don't reshuffle on incidental re-renders
	// (skin switch, thumbnail regenerate) — "stable per page view" (ADR-031). A fresh
	// item id draws anew; an error just omits the shelves. The cancel guard prevents a
	// slow response for a previous item from landing on the current one.
	$effect(() => {
		const current = id;
		let cancelled = false;
		related = null;
		api
			.related(current)
			.then((res) => {
				if (!cancelled) related = res;
			})
			.catch(() => {
				if (!cancelled) related = null;
			});
		return () => (cancelled = true);
	});
</script>

{#if loading}
	<p class="py-16 text-center text-sm text-muted">Loading…</p>
{:else if error || !video}
	<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">
		{error || 'Not found.'}
	</p>
{:else}
	<article class="mx-auto max-w-4xl space-y-6">
		<a href="/" class="text-sm text-muted hover:text-ink">← Back to library</a>

		<div class="group relative overflow-hidden rounded-theme border border-rule bg-black">
			{#if playFailed}
				<div class="flex aspect-video flex-col items-center justify-center gap-3 bg-surface text-center">
					<p class="text-sm text-muted">This browser can't decode this file's codec.</p>
					<a href={api.streamURL(video.id)} download class="rounded-theme bg-accent px-4 py-2 text-sm font-medium text-accent-ink">
						Download / open file
					</a>
				</div>
			{:else}
				<!-- svelte-ignore a11y_media_has_caption -->
				<!-- The generated cover (ADR-009) is the poster, so the player shows the
				     same frame as the card instead of a black box until play. -->
				<video
					src={api.streamURL(video.id)}
					poster={video.thumbnail_url
						? `${video.thumbnail_url}${thumbVersion ? `?r=${thumbVersion}` : ''}`
						: undefined}
					controls
					preload="metadata"
					class="aspect-video w-full bg-black"
					onplay={() => setPlaying(true)}
					onpause={() => setPlaying(false)}
					onended={() => setPlaying(false)}
					onerror={() => (playFailed = true)}
				></video>
				<button
					onclick={regenerateThumbnail}
					disabled={regenerating}
					title="Regenerate thumbnail"
					aria-label="Regenerate thumbnail"
					class="absolute right-2 top-2 z-10 rounded-theme bg-black/60 p-1.5 text-muted opacity-0 transition hover:text-ink focus-visible:opacity-100 group-hover:opacity-100 disabled:opacity-50"
				>
					<svg
						class="h-4 w-4 {regenerating ? 'animate-spin' : ''}"
						fill="currentColor"
						viewBox="0 0 24 24"
						aria-hidden="true"
					>
						<path
							d="M17.65 6.35A7.96 7.96 0 0 0 12 4a8 8 0 1 0 7.74 10h-2.08A6 6 0 1 1 12 6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"
						/>
					</svg>
				</button>
			{/if}
		</div>

		<header class="space-y-2">
			<h1 class="skin-title text-2xl font-semibold text-ink">{video.title}</h1>
			<div class="flex flex-wrap items-center gap-2 text-sm text-muted">
				<span class="rounded-theme bg-accent px-2 py-0.5 text-accent-ink">{resolutionBucket(video.width)}</span>
				<span>{video.width}×{video.height}</span>
				<span>·</span>
				<span>{formatDuration(video.duration_sec)}</span>
				{#if formatYear(video.recorded_at)}
					<span>·</span><span>{formatYear(video.recorded_at)}</span>
				{/if}
			</div>
		</header>

		{#if video.tags?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
				<div class="flex flex-wrap gap-2">
					{#each video.tags as t (t.id)}
						<a href={`/tags/${t.id}`} class="rounded-theme bg-surface-2 px-2.5 py-1 text-sm text-ink hover:text-accent">
							{t.name}
						</a>
					{/each}
				</div>
			</section>
		{/if}

		{#if video.people?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">People</h2>
				<!-- F25: 2:3 poster cards (placeholder when a person has no poster). -->
				<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-6">
					{#each video.people as p (p.id)}
						<li>
							<a
								href={`/people/${p.id}`}
								class="group block space-y-1.5 text-ink"
								title={p.name}
							>
								<div class="rounded-theme transition group-hover:opacity-90">
									<PersonPoster personId={p.id} name={p.name} />
								</div>
								<span class="line-clamp-2 text-xs text-muted group-hover:text-accent">{p.name}</span>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		{#if fields.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
				<dl class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
					{#each fields as f (f.canonical)}
						<div>
							<dt class="inline text-muted">{f.label}:</dt>
							<dd class="inline">{f.values.join(', ')}</dd>
						</div>
					{/each}
				</dl>
			</section>
		{/if}

		<section class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
			<div><span class="text-muted">File size:</span> {formatBytes(video.file_size)}</div>
			{#if video.container}<div><span class="text-muted">Container:</span> {video.container}</div>{/if}
			{#if video.video_codec}<div><span class="text-muted">Video codec:</span> {video.video_codec}</div>{/if}
			{#if video.audio_codec}<div><span class="text-muted">Audio codec:</span> {video.audio_codec}</div>{/if}
			{#if video.bitrate_kbps}
				<div><span class="text-muted">Bitrate:</span> {formatBitrate(video.bitrate_kbps)}</div>
			{/if}
			<div class="truncate sm:col-span-2" title={video.file_path}>
				<span class="text-muted">Path:</span> {video.file_path}
			</div>
		</section>

		{#if extra.length}
			<section>
				<button onclick={() => (showRaw = !showRaw)} class="text-sm text-muted hover:text-ink">
					{showRaw ? '▾' : '▸'} Raw extracted metadata ({extra.length})
				</button>
				{#if showRaw}
					<table class="mt-2 w-full text-left text-xs">
						<tbody>
							{#each extra as m (m.source_key)}
								<tr class="border-b border-rule">
									<td class="py-1 pr-4 font-mono text-muted">{m.source_key}</td>
									<td class="py-1 text-ink">{m.value}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</section>
		{/if}

		<!-- "More with …" shelves (QW3): person first, then tag. Each self-omits when
		     its block is null or empty, so an item with no siblings shows no rail. -->
		{#if related?.person}
			<RelatedShelf
				title={related.person.name}
				href={`/people/${related.person.id}`}
				items={related.person.items}
			/>
		{/if}
		{#if related?.tag}
			<RelatedShelf title={related.tag.name} href={`/tags/${related.tag.id}`} items={related.tag.items} />
		{/if}

		<!-- Owner-only Manage block (F24): destructive actions, kept apart from the
		     content and the Back link so a delete is never adjacent to navigation. -->
		{#if activity.isOwner}
			<section class="space-y-2 border-t border-rule pt-4">
				<h2 class="text-xs uppercase tracking-wide text-muted">Manage</h2>
				<div class="flex flex-wrap gap-2">
					<button
						onclick={() => openConfirm('soft')}
						class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
					>
						Move to Trash
					</button>
					<button
						onclick={() => openConfirm('purge')}
						class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
					>
						Delete permanently
					</button>
				</div>
			</section>
		{/if}
	</article>

	{#if confirmMode === 'soft'}
		<ConfirmDialog
			title="Move to Trash?"
			confirmLabel="Move to Trash"
			busy={deleteBusy}
			error={deleteError}
			onconfirm={confirmDelete}
			oncancel={() => (confirmMode = null)}
		>
			{#snippet body()}
				<p>
					<span class="font-semibold">{video?.title}</span> will be hidden from your library{graceDays
						? ` and permanently deleted in ${graceDays} ${graceDays === 1 ? 'day' : 'days'}`
						: ''}. You can restore it from Trash{graceDays ? ' until then' : ''}.
				</p>
			{/snippet}
		</ConfirmDialog>
	{:else if confirmMode === 'purge'}
		<ConfirmDialog
			title="Delete permanently?"
			confirmLabel="Delete permanently"
			busy={deleteBusy}
			error={deleteError}
			onconfirm={confirmDelete}
			oncancel={() => (confirmMode = null)}
		>
			{#snippet body()}
				<p>
					<span class="font-semibold">{video?.title}</span> and its file will be
					<span class="font-semibold">permanently removed</span> from disk. This cannot be undone.
				</p>
				<p class="truncate font-mono text-xs text-muted" title={video?.file_path}>{video?.file_path}</p>
			{/snippet}
		</ConfirmDialog>
	{/if}
{/if}
