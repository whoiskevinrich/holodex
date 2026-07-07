<script lang="ts">
	// Duplicates review queue (F43 S5, ADR-061 — Option A dense rows, ratified). The
	// owner works the near-miss queue here: pairs grouped by entity (tags first, they
	// dominate), each row offering Merge (pick the surviving name) or Keep separate
	// (records keep-separate; the pair never returns). A ?type= deep-link (from the
	// entity-list banners) filters to one entity. Tokens only; QA 3 skins.
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { DuplicatePair, EntityKind } from '$lib/types';
	import DuplicatePairRow from '$lib/components/DuplicatePairRow.svelte';

	let pairs = $state<DuplicatePair[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Optional ?type= filter (person|studio|tag) from a deep-link; invalid/absent shows all.
	const typeFilter = $derived.by(() => {
		const t = page.url.searchParams.get('type');
		return t === 'person' || t === 'studio' || t === 'tag' ? (t as EntityKind) : null;
	});

	// Group headings, tags first (the API already orders rows this way).
	const groupLabel: Record<EntityKind, string> = { tag: 'Tags', studio: 'Studios', person: 'People' };
	const groupOrder: EntityKind[] = ['tag', 'studio', 'person'];

	const shown = $derived(typeFilter ? pairs.filter((p) => p.entity_type === typeFilter) : pairs);
	const groups = $derived(
		groupOrder
			.map((et) => ({ type: et, items: shown.filter((p) => p.entity_type === et) }))
			.filter((g) => g.items.length > 0)
	);

	async function load() {
		loading = true;
		error = '';
		try {
			pairs = (await api.duplicates()).pairs ?? [];
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
	});

	// Resolve one pair (merged or dismissed): drop it from the list without a refetch.
	function resolve(pair: DuplicatePair) {
		pairs = pairs.filter((p) => p !== pair);
	}

	function mergePair(pair: DuplicatePair, survivorId: number, fromId: number): Promise<unknown> {
		// One merge endpoint for all three entities (the person route is unified into it).
		return api.mergeEntities(pair.entity_type, survivorId, fromId);
	}
</script>

<div class="space-y-5">
	<p class="text-sm text-muted">
		Possible duplicate names within an entity — case and spacing are already merged
		automatically; these are the judgement calls. Merge folds one into the other (the
		merged name stays as a searchable alias); Keep separate remembers your choice.
	</p>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else if groups.length === 0}
		<p class="py-16 text-center text-sm text-muted">No possible duplicates.</p>
	{:else}
		{#each groups as g (g.type)}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<h2 class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted">
					{groupLabel[g.type]} · {g.items.length}
				</h2>
				{#each g.items as pair (pair.entity_type + pair.a.id + '-' + pair.b.id)}
					<DuplicatePairRow
						{pair}
						merge={(survivorId, fromId) => mergePair(pair, survivorId, fromId)}
						dismiss={() => api.dismissDuplicate(pair.entity_type, pair.a.id, pair.b.id)}
						onresolved={() => resolve(pair)}
					/>
				{/each}
			</section>
		{/each}
	{/if}
</div>
