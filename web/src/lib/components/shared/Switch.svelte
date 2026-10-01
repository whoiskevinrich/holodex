<script lang="ts">
	// A small on/off switch (`role="switch"`): a pill track with a sliding dot and a text
	// label, accent when on. Extracted from EntityImageSlot's Halo knob at its second use
	// (F75's Always shuffle), so the two can't drift.
	let {
		checked,
		label,
		ariaLabel,
		title,
		disabled = false,
		onclick
	}: {
		checked: boolean;
		label: string;
		ariaLabel?: string;
		title?: string;
		disabled?: boolean;
		onclick: () => void;
	} = $props();
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	{onclick}
	{disabled}
	aria-label={ariaLabel}
	{title}
	class="flex items-center gap-1.5 py-0.5 text-xs {checked ? 'text-accent' : 'text-muted hover:text-ink'}"
>
	<span class="relative h-3 w-5 rounded-full border {checked ? 'border-accent' : 'border-muted'}" aria-hidden="true">
		<span
			class="absolute top-px h-2 w-2 rounded-full transition-[left] {checked ? 'left-2.5 bg-accent' : 'left-px bg-muted'}"
		></span>
	</span>
	{label}
</button>
