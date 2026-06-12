<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import type { ExtraMetadata, Video } from '$lib/types';
	import { formatBytes, formatDuration, formatYear, resolutionBucket, toMessage } from '$lib/format';

	let video = $state<Video | null>(null);
	let extra = $state<ExtraMetadata[]>([]);
	let loading = $state(true);
	let error = $state('');
	let playFailed = $state(false);
	let showRaw = $state(false);
	let regenerating = $state(false);
	let thumbVersion = $state(0); // cache-bust the preview after a regenerate

	const id = $derived(Number($page.params.id));

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

	$effect(() => {
		const current = id;
		loading = true;
		error = '';
		playFailed = false;
		api
			.getMedia(current)
			.then((res) => {
				video = res.video;
				extra = res.metadata ?? [];
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
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

		{#if video.people?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">People</h2>
				<div class="flex flex-wrap gap-2">
					{#each video.people as p (p.id)}
						<a href={`/people/${p.id}`} class="rounded-full border border-rule px-3 py-1 text-sm text-ink hover:border-accent">
							{p.name}
						</a>
					{/each}
				</div>
			</section>
		{/if}

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

		<section class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
			<div><span class="text-muted">File size:</span> {formatBytes(video.file_size)}</div>
			<div class="truncate" title={video.file_path}><span class="text-muted">Path:</span> {video.file_path}</div>
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
	</article>
{/if}
