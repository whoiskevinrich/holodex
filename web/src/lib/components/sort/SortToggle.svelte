<script lang="ts">
	import type { PeopleTagSort } from '$lib/types';
	import { segmentedToggleClass, segmentedToggleWrapperClass } from './segmentedToggle';

	// Shared A–Z / Most-videos / Random segmented control for the people & tags
	// indexes. "Random" is a seeded client-side shuffle (ADR-045).
	let { sort = $bindable() }: { sort: PeopleTagSort } = $props();
</script>

<div class={segmentedToggleWrapperClass}>
	<button onclick={() => (sort = 'name')} class={segmentedToggleClass(sort === 'name')}>A–Z</button>
	<button onclick={() => (sort = 'count')} class={segmentedToggleClass(sort === 'count')}>Most videos</button>
	<button onclick={() => (sort = 'random')} class={segmentedToggleClass(sort === 'random')}>Random</button>
</div>
