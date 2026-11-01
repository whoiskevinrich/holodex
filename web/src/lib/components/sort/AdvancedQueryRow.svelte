<script lang="ts">
	// An F87 query the chips can't draw (deeper than one OR level): one row in the chips
	// row's inset box with the canonical text, never a simplified chip set
	// (query-language-handoff.md). Edit as text puts the query in the nav box.
	let { text, onedit }: { text: string; onedit: () => void } = $props();
</script>

<span
	class="inline-flex min-w-0 max-w-full shrink-0 items-center gap-2 rounded-theme border border-rule bg-surface-2 px-2 py-1"
	aria-label={`Advanced query: ${text}`}
	role="group"
>
	<span class="shrink-0 text-xs text-ink">Advanced query</span>
	<span class="truncate font-mono text-[13px] text-muted" title={text}>{text}</span>
</span>
<button type="button" onclick={onedit} class="shrink-0 text-xs text-accent underline">Edit as text</button>
