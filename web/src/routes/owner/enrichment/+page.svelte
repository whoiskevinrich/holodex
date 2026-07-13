<script lang="ts">
	// Enrichment review queue (F47 S2, ADR-065). The entity-generic sibling of the
	// Duplicates tab: rows grouped People → Studios → Media (nav order, spec Q3),
	// actionable rows (an outstanding unreviewed provider) sorting first within a
	// group. Structurally identical to owner/duplicates/+page.svelte — $state rows,
	// $derived groups, $effect load-once — except rows update chips in place on
	// resolve instead of dropping out (the handoff's Animation/Motion table). Tokens
	// only; QA 3 skins.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type {
		EnrichCandidate,
		EnrichedField,
		EnrichEntityKind,
		EnrichQueueProviderState,
		EnrichQueueRow as QueueRow
	} from '$lib/types';
	import EnrichQueueRow from '$lib/components/EnrichQueueRow.svelte';

	let rows = $state<QueueRow[]>([]);
	let loading = $state(true);
	let error = $state('');

	const groupLabel: Record<EnrichEntityKind, string> = { person: 'People', studio: 'Studios', video: 'Media' };
	const groupOrder: EnrichEntityKind[] = ['person', 'studio', 'video'];

	// Per-kind REST dispatch (resolve/apply already exist as one client trio per
	// entity kind; the queue just routes to the right one) plus the detail-page href.
	const ENRICH_OPS: Record<
		EnrichEntityKind,
		{
			resolve: (id: number, provider: string, query: string) => Promise<{ candidates: EnrichCandidate[] }>;
			apply: (id: number, provider: string, externalId: string) => Promise<{ enriched: EnrichedField[] }>;
			href: (id: number) => string;
		}
	> = {
		person: { resolve: api.enrichResolve, apply: api.enrichApply, href: (id) => `/people/${id}` },
		studio: { resolve: api.enrichStudioResolve, apply: api.enrichStudioApply, href: (id) => `/studios/${id}` },
		video: { resolve: api.enrichVideoResolve, apply: api.enrichVideoApply, href: (id) => `/media/${id}` }
	};

	// A row is actionable when it still has an unreviewed provider — a row whose
	// outstanding providers are all `not_matched` sorts with the "nothing to do right
	// now" tail even though it still shows "Try again" (spec Q3's resolved ordering).
	function sortKey(row: QueueRow): number {
		return row.providers.some((p) => p.state === 'unreviewed') ? 0 : 1;
	}

	const groups = $derived(
		groupOrder
			.map((et) => ({
				type: et,
				items: rows
					.filter((r) => r.entity_type === et)
					.map((row) => ({ row, key: sortKey(row) }))
					.sort((a, b) => a.key - b.key || a.row.name.localeCompare(b.row.name))
					.map((x) => x.row)
			}))
			.filter((g) => g.items.length > 0)
	);

	async function load() {
		loading = true;
		error = '';
		try {
			rows = (await api.enrichQueue()).rows ?? [];
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
	});

	// Update one row's provider states in place (never removes the row — enrichment
	// rows resolve without disappearing; only a full reload drops a fully-handled one).
	function updateRow(row: QueueRow, providers: EnrichQueueProviderState[]) {
		rows = rows.map((r) =>
			r.entity_type === row.entity_type && r.entity_id === row.entity_id ? { ...r, providers } : r
		);
	}
</script>

<div class="space-y-5">
	<p class="text-sm text-muted">
		Entries missing metadata from at least one source. Opening a row resolves it — an
		obvious match applies right away; anything ambiguous still asks you.
	</p>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else if groups.length === 0}
		<p class="py-16 text-center text-sm text-muted">Nothing left to review.</p>
	{:else}
		{#each groups as g (g.type)}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<h2 class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted">
					{groupLabel[g.type]} · {g.items.length}
				</h2>
				{#each g.items as row (row.entity_type + row.entity_id)}
					<EnrichQueueRow
						{row}
						href={ENRICH_OPS[row.entity_type].href(row.entity_id)}
						resolve={(p, q) => ENRICH_OPS[row.entity_type].resolve(row.entity_id, p, q)}
						apply={(p, id) => ENRICH_OPS[row.entity_type].apply(row.entity_id, p, id)}
						undismiss={(p) => api.enrichUndismiss(row.entity_type, row.entity_id, p)}
						onchange={(providers) => updateRow(row, providers)}
					/>
				{/each}
			</section>
		{/each}
	{/if}
</div>
