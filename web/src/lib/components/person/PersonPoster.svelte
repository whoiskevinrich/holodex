<script lang="ts">
	// 2:3 poster card for the video credits surface (F25, ADR-038) — a thin wrapper over
	// PersonImageFrame fixing the poster role at full width. The consumer wraps it in a
	// link to /people/{id} with the name beneath.
	import PersonImageFrame from './PersonImageFrame.svelte';

	let {
		personId,
		name,
		version,
		eager = false,
		sizes
	}: {
		personId: number;
		name: string;
		version?: number;
		eager?: boolean;
		// The width the consumer's box gives this full-width poster (see PersonImageFrame).
		sizes: string;
	} = $props();
</script>

<PersonImageFrame {personId} role="poster" {name} {version} {eager} {sizes} frameClass="portrait-frame--2x3 w-full" />
