<script lang="ts">
	// Video composite-key collision verdict card (HOLODEX-270) — the video-flavored sibling
	// of MergeOfferCard, shown in NameEditControl's same verdict slot on the Video Title mount.
	// Unlike MergeOfferCard there's no merge verb (two video files can't be folded into one
	// record): the choice is "View existing video" / "Save anyway, keep both", both
	// non-destructive. Presentational only — the caller owns busy/error state and the
	// navigate/resubmit calls themselves.
	import { formatYear } from '$lib/format';
	import type { VideoCollisionRef } from '$lib/types';

	let {
		video,
		proposedTitle,
		busy = false,
		error = '',
		onviewexisting,
		onsaveanyway,
		oncancel
	}: {
		video: VideoCollisionRef;
		// The value that triggered this collision (the typed title, or undefined for a
		// Studio pick, which has no single "proposed title" to quote) — shown in the
		// headline when present; falls back to the colliding video's own title otherwise.
		proposedTitle?: string;
		busy?: boolean;
		error?: string;
		onviewexisting: () => void;
		onsaveanyway: () => void;
		oncancel: () => void;
	} = $props();
</script>

<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
	<p class="text-sm text-ink">"{proposedTitle ?? video.title}" already matches another video:</p>
	<p class="text-sm font-semibold text-ink">{video.title}</p>
	<p class="text-sm text-muted">
		{video.people.length ? video.people.join(', ') : '—'} · {formatYear(video.recorded_at) || '—'} · {video
			.studios.length
			? video.studios.join(', ')
			: '—'}
	</p>
	<div class="flex flex-wrap items-center gap-2">
		<button onclick={onviewexisting} disabled={busy} class="btn-accent px-3 py-1.5 text-sm">
			View existing video
		</button>
		<button onclick={onsaveanyway} disabled={busy} class="btn-ghost px-3 py-1.5 text-sm">
			{busy ? 'Saving…' : 'Save anyway, keep both'}
		</button>
		<button onclick={oncancel} disabled={busy} class="btn-quiet px-3 py-1.5 text-sm"> Cancel </button>
	</div>
	{#if error}
		<p class="text-sm text-warn">{error}</p>
	{/if}
</div>
