<script lang="ts">
	// Banner hero (F25, ADR-038) — a thin wrapper over PersonImageFrame fixing the banner
	// role to a full-width 270px-tall band (cover-cropped), so the hero doesn't dominate
	// the page on wide viewports the way a full 16:9 box would.
	import PersonImageFrame from './PersonImageFrame.svelte';

	let {
		personId,
		name,
		version,
		eager = false
	}: {
		personId: number;
		name: string;
		version?: number;
		eager?: boolean;
	} = $props();
</script>

<PersonImageFrame {personId} role="banner" {name} {version} {eager} frameClass="h-[270px] w-full" />
