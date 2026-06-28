<script lang="ts">
	// Shuffle-again control shown beside the sort picker while "Random" is active
	// (spec sort-persistence SP4). Tokens only, so it reads in all three skins.
	let { onreroll }: { onreroll: () => void } = $props();
</script>

<button
	onclick={onreroll}
	aria-label="Shuffle again"
	title="Shuffle again"
	class="rounded-theme border border-rule px-2 py-1 text-sm text-muted hover:border-accent hover:text-accent"
>
	<span aria-hidden="true">↻</span>
</button>
