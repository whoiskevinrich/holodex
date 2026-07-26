<script lang="ts">
	// Compact header pill (F21.5): shown only while background work is active,
	// links to the Status tab under the Owner hub (F35). The pulsing dot uses bg-accent so it picks up each skin's
	// accent; the pulse lives in app.css gated by prefers-reduced-motion.
	import { activity } from '$lib/activity.svelte';

	const d = $derived(activity.data);
</script>

{#if activity.active && d}
	<a
		href="/owner/status"
		title="System activity"
		class="flex items-center gap-1.5 rounded-theme border border-rule bg-surface-2 px-2 py-1 text-xs text-muted hover:text-ink"
	>
		<span class="activity-dot h-2 w-2 rounded-full bg-accent" aria-hidden="true"></span>
		<span role="status" aria-live="polite" class="hidden sm:inline">
			{d.scan.state === 'running' ? 'Indexing…' : `${d.thumbnails.depth} thumbnails`}
		</span>
	</a>
{/if}
