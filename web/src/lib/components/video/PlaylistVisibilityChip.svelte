<script lang="ts">
	import type { Playlist } from '$lib/types';

	// The owner-only visibility chip on a playlist row (F69, handoff §1): outlined,
	// muted for Private and accent for Public. Visitors never see it — every row a
	// visitor is shown is public by construction.
	let { visibility }: { visibility: Playlist['visibility'] } = $props();
</script>

<span
	class="shrink-0 rounded-full border px-2 py-0.5 text-xs {visibility === 'public'
		? 'border-accent text-accent'
		: 'border-muted text-muted'}"
>
	{visibility === 'public' ? 'Public' : 'Private'}
</span>
