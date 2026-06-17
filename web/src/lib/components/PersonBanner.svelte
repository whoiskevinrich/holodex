<script lang="ts">
	// Banner hero (F25, ADR-038) — a thin wrapper over PersonImageFrame fixing the banner
	// role to a wide 5:1 band capped at 270px tall (cover-cropped), so the hero doesn't
	// dominate the page on wide viewports. The 5:1 ratio is shared with the crop editor's
	// banner frame (.crop-frame--banner in app.css) so what you crop matches what's shown.
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

<PersonImageFrame {personId} role="banner" {name} {version} {eager} frameClass="aspect-[5/1] max-h-[270px] w-full" />
