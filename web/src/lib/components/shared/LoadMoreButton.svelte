<script lang="ts">
	// The paged-list "Load more (N left)" control (F80 handoff): a centred ghost button,
	// "Loading…" and disabled while a page is in flight, gone once nothing is left. The
	// polite live region stays mounted after the button goes, so the last page's
	// announcement is still read. Every paged list uses it (HOLODEX-589), so its wording
	// and a11y live here once. `noun` counts something other than entries ("3 videos
	// left"); `disabled` holds the button while a sibling list's page is in flight;
	// `class` is the row's spacing in its context.
	let {
		left,
		loading,
		disabled,
		noun = '',
		announce = '',
		class: cls = 'pt-4',
		onclick
	}: {
		left: number;
		loading: boolean;
		disabled?: boolean;
		noun?: string;
		announce?: string;
		class?: string;
		onclick: () => void;
	} = $props();

	const count = $derived(
		noun ? `${left.toLocaleString()} ${noun}${left === 1 ? '' : 's'}` : left.toLocaleString()
	);
</script>

{#if left > 0}
	<div class="flex justify-center {cls}">
		<button {onclick} disabled={disabled ?? loading} class="btn-ghost px-4 py-2 text-sm">
			{loading ? 'Loading…' : `Load more (${count} left)`}
		</button>
	</div>
{/if}
<p class="sr-only" aria-live="polite">{announce}</p>
