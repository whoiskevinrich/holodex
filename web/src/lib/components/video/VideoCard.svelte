<script lang="ts">
	import type { Video } from '#lib/types.ts';
	import { api } from '#lib/api.ts';
	import { formatDuration, resolutionBucket } from '#lib/format.ts';
	import { sceneBadgeLabel } from '#lib/components/film/sceneNumber.ts';
	import { partBadgeLabel } from '#lib/components/video/partBadge.ts';
	import CompletenessRing from '#lib/components/completeness/CompletenessRing.svelte';
	import { ThumbnailLoad } from './thumbnailLoad.svelte.ts';

	let {
		video,
		sceneNumber,
		onEditScene
	}: { video: Video; sceneNumber?: number | null; onEditScene?: (video: Video) => void } = $props();

	const bucket = $derived(resolutionBucket(video.width));

	// Thumbnails are generated in the background (ADR-009): the card attempts the image
	// at once and retries with backoff while it's still generating (thumbnailLoad.svelte.ts).
	// Prefer the server-provided URL — it carries a ?v={mtime} cache-bust token so a
	// writeback that rewrites the cover art isn't masked by a stale browser cache.
	const thumb = new ThumbnailLoad(() => video.thumbnail_url || api.thumbnailURL(video.id));
</script>

<div class="group relative block">
	<a href={`/media/${video.id}`} class="block">
		<!-- Cover image with background-generation fallback (ADR-009). `.video-frame`
		     carries the flourishes (letterbox bars) from
		     app.css; the <img> sits beneath them (their z-index is 1). -->
		<div
			class="video-frame flex items-center justify-center transition group-hover:border-accent {!thumb.loaded &&
			!thumb.gaveUp
				? 'thumb-shimmer'
				: ''}"
		>
			{#if !thumb.gaveUp}
				<img
					src={thumb.src}
					alt=""
					class="absolute inset-0 h-full w-full object-cover transition-opacity duration-300 {thumb.loaded
						? 'opacity-100'
						: 'opacity-0'}"
					onload={thumb.onload}
					onerror={thumb.onerror}
				/>
			{/if}
			{#if !thumb.loaded}
				<svg class="h-10 w-10 text-rule" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
					<path d="M8 5v14l11-7z" />
				</svg>
			{/if}
			{#if thumb.loaded}
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
	{#if video.completeness || video.part}
		<!-- Bottom-left group: the completeness ring chip (F65.4, HOLODEX-412) then the
		     part badge (HOLODEX-389), both in the duration badge's neutral treatment so
		     they read as facts along the bottom edge. The ring is owner-only by payload
		     — it mounts iff the list item carries `completeness` (the API strips it for
		     visitors), so a visitor's part badge sits exactly where it did before the
		     ring existed: the wrapper's inset equals the badge's old inset. Since F65.8
		     the ring is a <button>, so the group lives in a sibling of the <a> — an
		     overlay that mirrors .video-frame's box (app.css) — rather than inside it;
		     only the ring takes pointer events, the part badge falls through to the link
		     (the media page row is still its only edit mount, RD8). -->
		<div class="video-frame-overlay pointer-events-none absolute inset-x-0 top-0 z-[2]">
			<span class="absolute bottom-1.5 left-1.5 flex items-center gap-1">
				{#if video.completeness}
					<span class="completeness-chip pointer-events-auto inline-flex items-center rounded-theme bg-black/70 px-1 py-[3px]">
						<CompletenessRing
							required={video.completeness.required}
							extras={video.completeness.extras}
							size="card"
							entity={{ kind: 'video', id: video.id }}
						/>
					</span>
				{/if}
				{#if video.part}
					<span class="part-badge rounded-theme bg-black/70 px-1.5 py-0.5 text-xs text-ink">
						{partBadgeLabel(video.part)}
					</span>
				{/if}
			</span>
		</div>
	{/if}
	{#if sceneNumber !== undefined}
		<!-- Film scenes list (F56, design handoff §2c): numbered scenes get their number;
		     unnumbered scenes get a muted em-dash rather than no badge, so the position in
		     the (unnumbered-last) sort reads as intentional. A sibling of the anchor, not
		     nested inside it, so the owner-only edit affordance (HOLODEX-326) renders as a
		     real <button> (via svelte:element) without nesting interactive elements. When
		     there's no onEditScene (visitor, or a non-owner viewing the scenes grid), the
		     badge stays a plain, non-interactive label -- `pointer-events-none` lets a click
		     on it fall through to the <a> beneath, preserving the pre-HOLODEX-326 behavior
		     where the badge was part of the anchor and clicking it navigated like the rest
		     of the card. -->
		<svelte:element
			this={onEditScene ? 'button' : 'span'}
			type={onEditScene ? 'button' : undefined}
			role={onEditScene ? 'button' : undefined}
			onclick={onEditScene ? () => onEditScene(video) : undefined}
			aria-label={onEditScene ? `Edit scene number for ${video.title}` : undefined}
			class="absolute right-1.5 top-1.5 z-[2] rounded-theme px-1.5 py-0.5 text-[10px] font-semibold shadow-xs ring-1 ring-black/20 {onEditScene
				? 'hover:ring-accent focus-visible:ring-accent'
				: 'pointer-events-none'} {sceneNumber === null ? 'bg-bg text-muted' : 'bg-accent text-accent-ink'}"
		>
			{sceneBadgeLabel(sceneNumber)}
		</svelte:element>
	{/if}
</div>
