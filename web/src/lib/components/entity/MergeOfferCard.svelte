<script lang="ts">
	// Homonym-collision verdict card (F23/F43, ADR-061 — extracted from AliasPanel, HOLODEX-269).
	// Shown on an exact-name collision from either an alias add or a rename: never a silent merge,
	// the owner picks "same entity" (merge in) or "different entity" (keep separate). Presentational
	// only — the caller owns busy/error state and the merge/keep-separate calls themselves, so this
	// stays reusable from both AliasPanel's alias-add flow and NameEditControl's rename flow.
	import { videoCount } from '$lib/format';
	import type { EntityRef } from '$lib/types';

	let {
		noun,
		entityName,
		conflict,
		busy = false,
		error = '',
		onmerge,
		onkeepseparate
	}: {
		noun: string;
		entityName: string;
		conflict: EntityRef;
		busy?: boolean;
		error?: string;
		onmerge: () => void;
		onkeepseparate: () => void;
	} = $props();
</script>

<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
	<p class="text-sm text-ink">
		<span class="font-semibold">{conflict.name}</span> ({videoCount(conflict.video_count ?? 0)})
		is already a separate {noun}. Are they the same as {entityName}?
	</p>
	<div class="flex flex-wrap items-center gap-2">
		<button onclick={onmerge} disabled={busy} class="btn-accent px-3 py-1.5 text-sm">
			Yes, merge them in
		</button>
		<button onclick={onkeepseparate} disabled={busy} class="btn-ghost px-3 py-1.5 text-sm">
			No, keep separate
		</button>
	</div>
	{#if error}
		<p class="text-sm text-warn">{error}</p>
	{/if}
</div>
