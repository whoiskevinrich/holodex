<script lang="ts">
	import type { Playlist } from '$lib/types';

	// The outline chip on a playlist row (F69, handoff §1): muted for Private and accent
	// for Public, owner-only — every row a visitor is shown is public by construction.
	// `smart` renders F75's `smart` marker instead (handoff §2), in the muted variant;
	// one chip shape, so the two never drift.
	let { visibility, smart = false }: { visibility?: Playlist['visibility']; smart?: boolean } = $props();
	const accent = $derived(!smart && visibility === 'public');
</script>

<span
	class="shrink-0 rounded-full border px-2 py-0.5 text-xs {accent
		? 'border-accent text-accent'
		: 'border-muted text-muted'}"
>
	{smart ? 'smart' : visibility === 'public' ? 'Public' : 'Private'}
</span>
