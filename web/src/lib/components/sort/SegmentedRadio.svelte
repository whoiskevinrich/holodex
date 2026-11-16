<script lang="ts" generics="T extends string">
	// A segmented single choice as a radiogroup (F90 handoff, Accessibility): the Resolution
	// look, one tab stop (roving tabindex), arrow keys move and select. Provider link and each
	// Fields row's Has / Missing use it.
	import { segmentedToggleClass, segmentedToggleWrapperClass } from './segmentedToggle';

	let {
		options,
		value,
		label,
		small = false,
		onchange
	}: {
		options: readonly { value: T; label: string }[];
		value: T;
		/** The group's accessible name. */
		label: string;
		/** text-xs segments, for a Fields row. */
		small?: boolean;
		onchange: (v: T) => void;
	} = $props();

	let buttons = $state<HTMLButtonElement[]>([]);

	function onkeydown(e: KeyboardEvent, i: number) {
		const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (!step) return;
		e.preventDefault();
		const next = (i + step + options.length) % options.length;
		onchange(options[next].value);
		buttons[next]?.focus();
	}
</script>

<div class="{segmentedToggleWrapperClass} {small ? 'text-xs' : ''} w-fit" role="radiogroup" aria-label={label}>
	{#each options as o, i (o.value)}
		<button
			bind:this={buttons[i]}
			type="button"
			role="radio"
			aria-checked={o.value === value}
			tabindex={o.value === value ? 0 : -1}
			onclick={() => onchange(o.value)}
			onkeydown={(e) => onkeydown(e, i)}
			class="{segmentedToggleClass(o.value === value)} {small ? 'px-2 py-0.5' : ''}"
		>
			{o.label}
		</button>
	{/each}
</div>
