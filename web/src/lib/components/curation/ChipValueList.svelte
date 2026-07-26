<script lang="ts">
	// Renders a `chips`-display field's values (F39, ADR-056) as a read-only pill list.
	// The static, control-free cousin of CurationFieldRow's chips: used by auto-registered
	// non-canonical fields, which are display-only (no ✕/＋, no per-value provenance —
	// the row carries one ProvenanceBadge). Tokens only; `rounded-full` is the intentional
	// pill shape, not a hardcoded radius.
	let { values }: { values: string[] } = $props();
</script>

<span class="inline-flex flex-wrap gap-1.5 align-middle">
	{#each values as value, i (i)}
		<span class="inline-flex items-center rounded-full border border-rule px-2.5 py-0.5 text-sm text-ink"
			>{value}</span
		>
	{/each}
</span>
