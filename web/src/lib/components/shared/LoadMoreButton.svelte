<script lang="ts">
	// The paged-list "Load more (N left)" control (F80 handoff): a centred ghost button,
	// "Loading…" and disabled while a page is in flight, gone once nothing is left. The
	// polite live region stays mounted after the button goes, so the last page's
	// announcement is still read.
	let {
		left,
		loading,
		announce = '',
		onclick
	}: { left: number; loading: boolean; announce?: string; onclick: () => void } = $props();
</script>

{#if left > 0}
	<div class="flex justify-center pt-4">
		<button {onclick} disabled={loading} class="btn-ghost px-4 py-2 text-sm">
			{loading ? 'Loading…' : `Load more (${left.toLocaleString()} left)`}
		</button>
	</div>
{/if}
<p class="sr-only" aria-live="polite">{announce}</p>
