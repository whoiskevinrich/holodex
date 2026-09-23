<script lang="ts">
	// Duplicates review queue (F43 S5, ADR-061 — Option A dense rows, ratified). The
	// owner works the near-miss queue here: pairs grouped by entity (tags first, they
	// dominate), each row offering Merge (pick the surviving name) or Keep separate
	// (records keep-separate; the pair never returns). A ?type= deep-link (from the
	// entity-list banners) filters to one entity. A person pair also expands in place into
	// a two-column compare panel (F70, HOLODEX-451); this page owns which one is open so
	// only one ever is, and owns where focus lands when a row is removed under it.
	// Tokens only; QA 3 skins.
	import { tick } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { groupByKind } from '$lib/entityGroups';
	import type { DuplicatePair, EntityKind } from '$lib/types';
	import DuplicatePairRow from '$lib/components/duplicates/DuplicatePairRow.svelte';

	let pairs = $state<DuplicatePair[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Optional ?type= filter (person|studio|tag|film) from a deep-link; invalid/absent shows all.
	const typeFilter = $derived.by(() => {
		const t = page.url.searchParams.get('type');
		return t === 'person' || t === 'studio' || t === 'tag' || t === 'film' ? (t as EntityKind) : null;
	});

	// Group headings, tags first (the API already orders rows this way).
	const groupLabel: Record<EntityKind, string> = { tag: 'Tags', studio: 'Studios', person: 'People', film: 'Films' };
	const groupOrder: EntityKind[] = ['tag', 'studio', 'person', 'film'];

	const shown = $derived(typeFilter ? pairs.filter((p) => p.entity_type === typeFilter) : pairs);
	const groups = $derived(groupByKind(shown, groupOrder, (p) => p.entity_type));

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

	// One key per pair — the `{#each}` key, the compare panel's id and the disclosure's id
	// all derive from it, so focus can be moved by id without threading refs up the tree.
	const pairKey = (p: DuplicatePair) => `${p.entity_type}-${p.a.id}-${p.b.id}`;

	// At most one compare panel open at a time (RD12): opening a second collapses the
	// first, so the page never holds two sets of evidence you aren't reading.
	let openKey = $state<string | null>(null);

	// Resolve one pair (merged or dismissed): drop it from the list without a refetch.
	// The removal is instant and unanimated, so focus would land on <body> if we didn't
	// move it (P0-6). Three landing spots, in order: the next row's disclosure; the
	// group heading; the queue itself. The third is not belt-and-braces — a group with
	// no rows left stops rendering, taking its own heading with it, so the last pair in
	// a group has no heading to land on. The queue always survives, and when the whole
	// board clears it is what holds "No possible duplicates."
	// Non-person groups have no disclosures at all, so they always land on the heading.
	async function resolve(pair: DuplicatePair) {
		const siblings = shown.filter((p) => p.entity_type === pair.entity_type);
		const next = siblings[siblings.indexOf(pair) + 1];
		if (openKey === pairKey(pair)) openKey = null;
		pairs = pairs.filter((p) => p !== pair);
		await tick();
		const byId = (id: string) => document.getElementById(id);
		const landing =
			(next && byId(`dup-disclosure-${pairKey(next)}`)) ||
			byId(`dup-group-${pair.entity_type}`) ||
			byId('dup-queue');
		landing?.focus();
	}

	function mergePair(pair: DuplicatePair, survivorId: number, fromId: number): Promise<unknown> {
		// One merge endpoint for all three entities (the person route is unified into it).
		return api.mergeEntities(pair.entity_type, survivorId, fromId);
	}
</script>

<!-- `tabindex="-1"` so a resolve that empties a group still has somewhere to put focus;
     not a tab stop. -->
<div id="dup-queue" tabindex="-1" class="space-y-5">
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
				<!-- `tabindex="-1"` so focus has somewhere to land when the last row in the
				     group is resolved; it is not a tab stop. -->
				<h2
					id={`dup-group-${g.type}`}
					tabindex="-1"
					class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted"
				>
					{groupLabel[g.type]} · {g.items.length}
				</h2>
				{#each g.items as pair (pairKey(pair))}
					<DuplicatePairRow
						{pair}
						merge={(survivorId, fromId) => mergePair(pair, survivorId, fromId)}
						dismiss={() => api.dismissDuplicate(pair.entity_type, pair.a.id, pair.b.id)}
						onresolved={() => resolve(pair)}
						expanded={openKey === pairKey(pair)}
						onexpand={(v) => (openKey = v ? pairKey(pair) : null)}
					/>
				{/each}
			</section>
		{/each}
	{/if}
</div>
