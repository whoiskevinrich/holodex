<script lang="ts">
	// 1:1 headshot frame (F24, ADR-037) — a thin wrapper over PersonImageFrame fixing
	// the headshot role and mapping `size` to a width.
	import PersonImageFrame from './PersonImageFrame.svelte';

	let {
		personId,
		name,
		version,
		size = 'md',
		eager = false
	}: {
		personId: number;
		name: string;
		version?: number;
		size?: 'sm' | 'md' | 'lg';
		eager?: boolean;
	} = $props();

	const widths: Record<typeof size, string> = { sm: 'w-12', md: 'w-20', lg: 'w-32' };
</script>

<PersonImageFrame
	{personId}
	role="headshot"
	{name}
	{version}
	{eager}
	frameClass={`portrait-frame--1x1 ${widths[size]}`}
/>
