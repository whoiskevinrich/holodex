<script lang="ts">
	import type { Video } from '$lib/types';
	import { api } from '$lib/api';
	import { formatDuration, resolutionBucket } from '$lib/format';

	let {
		video,
		sceneNumber,
		onEditScene
	}: { video: Video; sceneNumber?: number | null; onEditScene?: (video: Video) => void } = $props();

	const bucket = $derived(resolutionBucket(video.width));

	// Thumbnails are generated in the background (ADR-009). The card attempts the
	// image immediately; while it is still generating the server returns 404, so
	// we retry with backoff until it lands, then give up to the placeholder.
	const MAX_RETRIES = 5;
	let attempt = $state(0);
	let loaded = $state(false);
	let gaveUp = $state(false);
	// Prefer the server-provided URL — it carries a ?v={mtime} cache-bust token so a
	// writeback that rewrites the cover art isn't masked by a stale browser cache.
	// Fall back to the bare URL while the thumbnail is still generating (no
	// thumbnail_url yet) so the 404-retry loop below still has something to poll.
	const base = $derived(video.thumbnail_url || api.thumbnailURL(video.id));
	const src = $derived(api.thumbnailReload(base, attempt));

	function onError() {
		if (attempt < MAX_RETRIES) {
			// Exponential backoff with jitter so a freshly-scanned grid doesn't
			// retry every card in lockstep (thundering herd) against the API.
			const base = Math.min(2000 * 2 ** attempt, 30000);
			const delay = base * (0.5 + Math.random());
			const next = attempt + 1;
			setTimeout(() => (attempt = next), delay);
		} else {
			gaveUp = true;
		}
	}
</script>

<div class="group relative block">
	<a href={`/media/${video.id}`} class="block">
		<!-- Cover image with background-generation fallback (ADR-009). `.video-frame`
		     carries the per-skin flourishes (letterbox, scanlines, index counter) from
		     app.css; the <img> sits beneath them (their z-index is 1). -->
		<div
			class="video-frame flex items-center justify-center transition group-hover:border-accent {!loaded &&
			!gaveUp
				? 'thumb-shimmer'
				: ''}"
		>
			{#if !gaveUp}
				<img
					{src}
					alt=""
					class="absolute inset-0 h-full w-full object-cover transition-opacity duration-300 {loaded
						? 'opacity-100'
						: 'opacity-0'}"
					onload={() => (loaded = true)}
					onerror={onError}
				/>
			{/if}
			{#if !loaded}
				<svg class="h-10 w-10 text-rule" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
					<path d="M8 5v14l11-7z" />
				</svg>
			{/if}
			{#if loaded}
				<!-- "Click to watch" affordance: a play glyph fades in over the cover on hover. -->
				<div
					class="pointer-events-none absolute inset-0 z-[1] flex items-center justify-center bg-black/25 opacity-0 transition-opacity duration-200 group-hover:opacity-100"
				>
					<svg class="h-11 w-11 text-ink drop-shadow-lg" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
						<path d="M8 5v14l11-7z" />
					</svg>
				</div>
			{/if}
			<span
				class="absolute bottom-1.5 right-1.5 z-[2] rounded-theme bg-black/70 px-1.5 py-0.5 text-xs tabular-nums text-ink"
			>
				{formatDuration(video.duration_sec)}
			</span>
			{#if video.width > 0}
				<span
					class="absolute left-1.5 top-1.5 z-[2] rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink shadow-xs ring-1 ring-black/20"
				>
					{bucket}
				</span>
			{/if}
		</div>

		<div class="space-y-1.5 p-3">
			<h3 class="skin-title line-clamp-2 text-sm font-medium text-ink" title={video.title}>
				{video.title}
			</h3>
		</div>
	</a>
	{#if sceneNumber !== undefined}
		<!-- Film scenes list (F56, design handoff §2c): numbered scenes get their number;
		     unnumbered scenes get a muted em-dash rather than no badge, so the position in
		     the (unnumbered-last) sort reads as intentional. A sibling of the anchor, not
		     nested inside it, so the owner-only edit affordance (HOLODEX-326) renders as a
		     real <button> (via svelte:element) without nesting interactive elements. -->
		<svelte:element
			this={onEditScene ? 'button' : 'span'}
			type={onEditScene ? 'button' : undefined}
			role={onEditScene ? 'button' : undefined}
			onclick={onEditScene ? () => onEditScene(video) : undefined}
			aria-label={onEditScene ? `Edit scene number for ${video.title}` : undefined}
			class="absolute right-1.5 top-1.5 z-[2] rounded-theme px-1.5 py-0.5 text-[10px] font-semibold shadow-xs ring-1 ring-black/20 {onEditScene
				? 'hover:ring-accent focus-visible:ring-accent'
				: ''} {sceneNumber === null ? 'bg-black/70 text-muted' : 'bg-accent text-accent-ink'}"
		>
			{sceneNumber === null ? '—' : `#${sceneNumber}`}
		</svelte:element>
	{/if}
</div>
