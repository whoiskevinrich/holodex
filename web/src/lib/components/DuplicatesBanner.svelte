<script lang="ts">
	// "N possible duplicates" banner (F43 S5, ADR-061) — an owner-only notice above an
	// entity list that counts the review queue for that entity and deep-links the Owner
	// hub's Duplicates tab. Self-gating and self-fetching so a list page mounts it with
	// one line. Hidden for visitors and when the queue is empty. Tokens only; 3 skins.
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import type { EntityKind } from '$lib/types';

	let { entityType }: { entityType: EntityKind } = $props();

	const isOwner = $derived(activity.effectiveOwner);
	const plural: Record<EntityKind, string> = { person: 'people', studio: 'studios', tag: 'tags' };
	const singular: Record<EntityKind, string> = { person: 'person', studio: 'studio', tag: 'tag' };

	let count = $state(0);
	const noun = $derived(count === 1 ? singular[entityType] : plural[entityType]);

	// Load the count once the client is a confirmed owner (the queue read is gated).
	$effect(() => {
		if (!isOwner) {
			count = 0;
			return;
		}
		api
			.duplicates()
			.then((res) => (count = (res.pairs ?? []).filter((p) => p.entity_type === entityType).length))
			.catch(() => (count = 0));
	});
</script>

{#if isOwner && count > 0}
	<div
		role="status"
		aria-live="polite"
		class="flex flex-wrap items-center justify-between gap-2 rounded-theme border border-warn bg-surface px-3 py-2 text-sm text-ink"
	>
		<span>
			<span class="font-semibold text-warn">{count} possible duplicate {noun}</span>
			— names that look like the same thing.
		</span>
		<a class="text-accent hover:underline" href={`/owner/duplicates?type=${entityType}`}>Review ↗</a>
	</div>
{/if}
