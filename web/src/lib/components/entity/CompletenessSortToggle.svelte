<script lang="ts">
	import { segmentedToggleClass, segmentedToggleWrapperClass } from '$lib/components/sort/segmentedToggle';

	// Owner-only "sort by completeness" toggle for the People/Studios indexes (F55.5).
	// A separate control from SortToggle rather than widening it — SortToggle's
	// PeopleTagSort is a shared type with the (out-of-scope) Tags page, so a fourth
	// value there would leak an owner-only sort into a page that has no completeness
	// scoring. dir='' means "off" (defer to the page's normal name/count/random sort);
	// clicking the active direction again turns it back off.
	let { dir = $bindable() }: { dir: '' | 'asc' | 'desc' } = $props();

	function toggle(next: 'asc' | 'desc') {
		dir = dir === next ? '' : next;
	}
</script>

<div class={segmentedToggleWrapperClass}>
	<button onclick={() => toggle('desc')} class={segmentedToggleClass(dir === 'desc')} title="Most complete first">
		Completeness ↓
	</button>
	<button onclick={() => toggle('asc')} class={segmentedToggleClass(dir === 'asc')} title="Least complete first">
		Completeness ↑
	</button>
</div>
