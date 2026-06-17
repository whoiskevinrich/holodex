<script lang="ts">
	// 16:9 banner hero (F25, ADR-038) — a thin wrapper over PersonImageFrame fixing the
	// banner role at full width.
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

<PersonImageFrame {personId} role="banner" {name} {version} {eager} frameClass="portrait-frame--16x9 w-full" />
