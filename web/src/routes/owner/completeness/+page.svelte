<script lang="ts">
	// Facet-first remediation queue (F55.7/F55.8, docs/design/entity-completeness-handoff.md
	// §1). The backend (GET /owner/completeness-queue) already groups by facet, orders
	// critical-first-then-count-desc, and splits candidate-ready/needs-research (DD1) — this
	// page renders that shape directly with zero client-side grouping/sorting.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import CompletenessQueueRow from '$lib/components/completeness/CompletenessQueueRow.svelte';
	import type { CompletenessFacetGroup, CompletenessQueueRow as QueueRow, DecisionRequest } from '$lib/types';

	let groups = $state<CompletenessFacetGroup[]>([]);
	let loading = $state(true);
	let error = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await api.completenessQueue();
			groups = res.groups ?? [];
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
	});

	// Drops one row from its group's candidate_ready list once applied (DD3: the row
	// disappearing is the only confirmation — no toast). A group that empties out on
	// both sides is dropped entirely so its heading doesn't linger with 0 rows.
	function dropRow(canonical: string, row: QueueRow) {
		groups = groups
			.map((g) =>
				g.canonical === canonical
					? { ...g, candidate_ready: g.candidate_ready.filter((r) => r !== row) }
					: g
			)
			.filter((g) => g.candidate_ready.length > 0 || g.needs_research.length > 0);
	}

	// Applying pins the field to the candidate's provider — the same per-field
	// source-decision mechanism the detail pages already use (F36/F37/F38), just
	// invoked from the queue instead of a field's own dropdown.
	function apply(row: QueueRow, canonical: string) {
		const req: DecisionRequest = { source: `provider:${row.provider}` };
		if (row.entity_type === 'video') return api.setFieldDecision(row.entity_id, canonical, req);
		if (row.entity_type === 'person') return api.setPersonFieldDecision(row.entity_id, canonical, req);
		return api.setStudioFieldDecision(row.entity_id, canonical, req);
	}
</script>

<div class="space-y-5">
	<p class="max-w-2xl text-sm text-muted">
		Every scored facet currently missing across your library, grouped by facet — critical facets
		first. Candidate-ready rows have a cached provider match one click away; Needs-research rows
		hand off to the entity page to look one up.
	</p>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else if groups.length === 0}
		<p class="py-16 text-center text-sm text-muted">
			Nothing to remediate — every scored facet across your library is resolved or marked not
			applicable.
		</p>
	{:else}
		{#each groups as g (g.canonical)}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<h3 class="border-b border-rule px-3 pb-2 pt-3 text-sm font-medium text-ink">
					Missing {g.label} · {g.candidate_ready.length + g.needs_research.length}
				</h3>
				{#if g.candidate_ready.length > 0}
					<h4 class="px-3 pb-1 pt-2 text-xs uppercase tracking-wide text-muted">Candidate-ready</h4>
					{#each g.candidate_ready as row (`${row.entity_type}-${row.entity_id}`)}
						<CompletenessQueueRow
							{row}
							facetCanonical={g.canonical}
							facetLabel={g.label}
							apply={() => apply(row, g.canonical)}
							onhandled={() => dropRow(g.canonical, row)}
						/>
					{/each}
				{/if}
				{#if g.needs_research.length > 0}
					<h4 class="px-3 pb-1 pt-2 text-xs uppercase tracking-wide text-muted">Needs research</h4>
					{#each g.needs_research as row (`${row.entity_type}-${row.entity_id}`)}
						<CompletenessQueueRow
							{row}
							facetCanonical={g.canonical}
							facetLabel={g.label}
							apply={() => apply(row, g.canonical)}
							onhandled={() => dropRow(g.canonical, row)}
						/>
					{/each}
				{/if}
			</section>
		{/each}
	{/if}
</div>
