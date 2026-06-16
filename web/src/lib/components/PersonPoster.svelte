<script lang="ts">
	// 2:3 poster card for the video credits surface (F24, ADR-037) — a thin wrapper over
	// PersonImageFrame fixing the poster role at full width. The consumer wraps it in a
	// link to /people/{id} with the name beneath.
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

<PersonImageFrame {personId} role="poster" {name} {version} {eager} frameClass="portrait-frame--2x3 w-full" />
