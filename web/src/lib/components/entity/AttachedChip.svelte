<script lang="ts">
	// The attached chip (HOLODEX-493, ui-vocabulary "attached chip"): one entity already
	// attached, shown at the top of a relationship picker, whose × IS the detach. The one
	// markup PersonPicker, StudioPicker and FilmStudioCascadeDialog render, so a change to its
	// size, target or glyph lands in all three at once (entity/CLAUDE.md "Relationship
	// pickers"). Renders an <li>; the caller owns the <ul>, its label and the rule-off.
	let {
		label,
		detail = '',
		removeLabel,
		busy = false,
		disabled = false,
		onremove
	}: {
		label: string;
		// Muted qualifier after the name (PersonPicker's role); none for a studio.
		detail?: string;
		// Accessible name of the ×, naming what is removed from what.
		removeLabel: string;
		// This chip's own detach is in flight: the × shows … and can't be pressed again.
		busy?: boolean;
		// Another commit in the picker is in flight.
		disabled?: boolean;
		onremove: () => void;
	} = $props();
</script>

<li
	class="inline-flex items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-ink"
>
	<span class="max-w-[10rem] truncate" title={label}>{label}</span>
	{#if detail}
		<span class="text-muted">{detail}</span>
	{/if}
	<button
		type="button"
		aria-label={removeLabel}
		disabled={busy || disabled}
		onclick={onremove}
		class="text-muted hover:text-accent disabled:cursor-default"
	>
		{busy ? '…' : '×'}
	</button>
</li>
