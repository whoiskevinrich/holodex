<script lang="ts">
	// One (entity, facet) row in the remediation queue (F55.7/F55.8, design handoff
	// docs/design/entity-completeness-handoff.md §1 DD2/DD3). Visual vocabulary only
	// borrowed from ExtractionQueueRow (tier-badge idiom, individual-action idiom) —
	// not the component itself, since this row's shape (apply/search/upload) is
	// uniform across every facet where extraction's is heterogeneous per field type.
	//
	// Candidate-ready (row.provider set) shows a single Apply button that mutates in
	// place; Needs-research shows a single Search link that navigates to the entity's
	// detail page anchored at that facet, plus — for the three image-typed facets —
	// a lighter "or upload an image directly" link to the page's upload control.
	import ProvenanceBadge from '$lib/components/enrichment/ProvenanceBadge.svelte';
	import PersonAvatar from '$lib/components/person/PersonAvatar.svelte';
	import { monogram } from '$lib/format';
	import { toMessage } from '$lib/format';
	import { api, ENRICH_ENTITY_BASE } from '$lib/api';
	import type { CompletenessQueueRow } from '$lib/types';

	let {
		row,
		facetCanonical,
		facetLabel,
		apply,
		onhandled
	}: {
		row: CompletenessQueueRow;
		facetCanonical: string;
		facetLabel: string;
		/** Applies the cached candidate — only called for candidate-ready rows. */
		apply: () => Promise<unknown>;
		/** The row is fully handled (applied); parent drops it from local state. */
		onhandled: () => void;
	} = $props();

	// Per-facet anchor overrides, keyed by canonical. Facets with no dedicated
	// resolved-field row to anchor Search at (photo, branding_image are driven
	// entirely by the image-upload system, not the generic field list) fall back
	// to the entity page's enrich-providers section; the three image-typed facets
	// additionally get a secondary "upload directly" link (DD3).
	const FACET_ANCHOR: Record<string, { search?: string; upload?: string }> = {
		poster_url: { upload: 'field-poster_url-upload' },
		photo: { search: 'enrich-providers', upload: 'field-photo-upload' },
		branding_image: { search: 'enrich-providers', upload: 'field-branding_image-upload' }
	};

	const detailHref = $derived(`/${ENRICH_ENTITY_BASE[row.entity_type]}/${row.entity_id}`);
	const searchHref = $derived(
		`${detailHref}#${FACET_ANCHOR[facetCanonical]?.search ?? `field-${facetCanonical}`}`
	);
	const uploadHref = $derived(
		FACET_ANCHOR[facetCanonical]?.upload ? `${detailHref}#${FACET_ANCHOR[facetCanonical].upload}` : ''
	);
	const candidateReady = $derived(!!row.provider);

	let busy = $state(false);
	let error = $state('');

	// `.btn-row`/`.btn-pill` (app.css) carry the shape and size shared with the
	// other owner queue rows (ExtractionQueueRow, EnrichQueueRow, DuplicatePairRow).
	const APPLY = 'btn-row btn-pill btn-accent';
	const SEARCH = 'btn-row btn-pill btn-accent gap-1';
	const UPLOAD = 'btn-row btn-quiet gap-1';

	async function doApply() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await apply();
			onhandled();
		} catch (e) {
			error = toMessage(e);
			busy = false;
		}
	}
</script>

{#snippet externalGlyph()}
	<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
		<path stroke-linecap="round" stroke-linejoin="round" d="M7 17L17 7M8 7h9v9" />
	</svg>
{/snippet}

<div class="flex flex-wrap items-center gap-3 border-t border-rule px-3 py-2.5 text-sm">
	<!-- Thumbnail: decorative, the entity link carries the accessible label. -->
	{#if row.entity_type === 'video'}
		<img
			src={row.thumbnail_url || api.thumbnailURL(row.entity_id)}
			alt=""
			class="h-10 w-16 shrink-0 rounded-theme border border-rule object-cover"
		/>
	{:else if row.entity_type === 'person'}
		<PersonAvatar personId={row.entity_id} name={row.name} version={row.headshot_version} size="sm" />
	{:else}
		<span
			class="flex h-[26px] w-10 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate"
		>
			{#if row.icon_url}
				<img src={row.icon_url} alt="" class="h-full w-full object-contain p-0.5" />
			{:else}
				<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true"
					>{monogram(row.name)}</span
				>
			{/if}
		</span>
	{/if}

	<div class="min-w-0 flex-1">
		<a href={detailHref} class="truncate text-ink hover:underline" title={row.name}>{row.name}</a>
		{#if candidateReady}
			<ProvenanceBadge provider={row.provider} label={row.provider} />
		{/if}
		{#if error}
			<p class="text-xs text-warn" role="alert">{error}</p>
		{/if}
	</div>

	<div class="flex shrink-0 flex-col items-end gap-1">
		{#if candidateReady}
			<button
				onclick={doApply}
				disabled={busy}
				aria-label={`Apply candidate ${facetLabel} to ${row.name}`}
				class={APPLY}
			>
				{busy ? 'Applying…' : 'Apply'}
			</button>
		{:else}
			<a href={searchHref} aria-label={`Search for ${facetLabel} — ${row.name}`} class={SEARCH}>
				Search {@render externalGlyph()}
			</a>
			{#if uploadHref}
				<a
					href={uploadHref}
					aria-label={`Upload ${facetLabel} directly — ${row.name}`}
					class={UPLOAD}
				>
					or upload an image directly {@render externalGlyph()}
				</a>
			{/if}
		{/if}
	</div>
</div>
