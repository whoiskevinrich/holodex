<script lang="ts">
	// The Details section of the film and studio pages (HOLODEX-582) — one copy of the markup
	// HOLODEX-95 left in both. Visitor vs. owner is a control gate (routes/CLAUDE.md): a
	// visitor sees the section whenever a row has a value, unboxed, with one "Enriched from X"
	// note when a single provider supplies every row; the owner gets the box, the enrich chips
	// and a SourceBadge per row. SourceBadge renders the value itself, which the routes rule
	// forbids on a visitor-visible block — short scalar rows are its recorded exception.
	//
	// The page passes its replace fields minus the ones it mounts elsewhere (name, and the
	// film's description); this component decides which of them show for whom.
	import type { Snippet } from 'svelte';
	import type { DecisionSource, ResolvedField } from '#lib/types.ts';
	import { soleProvider as soleProviderOf, rowProvenance } from '#lib/format.ts';
	import EnrichProviderChips from '#lib/components/enrichment/EnrichProviderChips.svelte';
	import NoProviderHint from '#lib/components/enrichment/NoProviderHint.svelte';
	import ProvenanceBadge from '#lib/components/enrichment/ProvenanceBadge.svelte';
	import SourceBadge from '#lib/components/curation/SourceBadge.svelte';
	import UrlValueList from '#lib/components/curation/UrlValueList.svelte';

	let {
		fields,
		isOwner,
		sourcesLoaded,
		providers,
		linked,
		busy = '',
		refreshingAll = false,
		onenrich,
		onrefresh,
		onclear,
		onrefreshall,
		decide,
		error = '',
		status = '',
		hasMore = false,
		afterRow,
		children
	}: {
		fields: ResolvedField[];
		isOwner: boolean;
		sourcesLoaded: boolean;
		// Enrich-capable providers for this entity kind, and EnrichProviderChips' handlers.
		providers: string[];
		linked: (p: string) => boolean;
		busy?: string;
		refreshingAll?: boolean;
		onenrich: (p: string, opts?: { rematch: boolean }) => void;
		onrefresh: (p: string) => void;
		onclear: (p: string) => void;
		onrefreshall: () => void;
		decide: (canonical: string, source: DecisionSource, manualValue?: string) => Promise<void>;
		error?: string;
		status?: string;
		// The page renders more rows through `children` (studio: merge and auto-registered
		// fields), so the section shows even when no replace row does.
		hasMore?: boolean;
		// Rendered after each row, inside the list (studio: the promoted-field edit pill).
		afterRow?: Snippet<[ResolvedField]>;
		children?: Snippet;
	} = $props();

	// A row shows a visitor its value. The owner also gets a row with only candidates, or with
	// a standing decision and nothing else — a "keep blank" pin with no candidate would
	// otherwise hide the one control that can undo it.
	const shown = $derived(
		fields.filter((f) =>
			isOwner
				? f.values.length > 0 || (f.candidates ?? []).length > 0 || !!f.decision?.standing
				: f.values.some((v) => v.trim() !== '')
		)
	);
	const compactFields = $derived(shown.filter((f) => f.display !== 'long_text'));
	const longFields = $derived(shown.filter((f) => f.display === 'long_text'));
	const soleProvider = $derived(soleProviderOf(shown));
	const hasRows = $derived(shown.length > 0 || hasMore);
</script>

{#if hasRows || (isOwner && sourcesLoaded)}
	<!-- The owner's box frames controls; a visitor gets the section unboxed, in the rail's own
	     heading-and-rows rhythm (HOLODEX-95, film-details-parity-handoff). The owner still gets
	     it with nothing resolved once providers have loaded: with a provider so Enrich is
	     reachable, with none so the "No metadata provider configured" hint has a home. -->
	<section class={isOwner ? 'space-y-3 rounded-theme border border-rule bg-surface p-4' : 'space-y-2'}>
		<div class="flex flex-wrap items-start justify-between gap-2">
			<h2 class="text-xs uppercase tracking-wide text-muted" id="enrich-providers">Details</h2>
			{#if isOwner && providers.length}
				<!-- HOLODEX-136: one compact chip per provider (icon + name + Enrich), Clear in a
				     ⋯ overflow once linked. -->
				<EnrichProviderChips {providers} {linked} {busy} {refreshingAll} {onenrich} {onrefresh} {onclear} {onrefreshall} />
			{:else if isOwner && sourcesLoaded}
				<NoProviderHint />
			{:else if !isOwner && soleProvider}
				<span class="text-xs text-muted">Enriched from <span class="text-accent">{soleProvider}</span></span>
			{/if}
		</div>

		{#if error}
			<p class="text-sm text-warn">{error}</p>
		{/if}
		{#if status}
			<p class="text-sm text-muted" aria-live="polite">{status}</p>
		{/if}

		{#if hasRows}
			<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
				{#each compactFields as f (f.canonical)}
					<div class={f.display === 'url' ? 'sm:col-span-2' : ''} id={`field-${f.canonical}`}>
						{#if isOwner}
							<dt class="mb-1 text-muted">{f.label}:</dt>
							<dd>
								<SourceBadge field={f} baselineKey="record" decide={(s, mv) => decide(f.canonical, s, mv)} />
							</dd>
						{:else}
							{@const badge = soleProvider ? null : rowProvenance(f.winning_source)}
							<dt class="inline text-muted">{f.label}:</dt>
							{#if f.display === 'url'}
								<!-- HOLODEX-137: the provider icon leads the link, folding in its
								     provenance. A custom value has no provider, so it keeps the pill. -->
								<dd class="inline">
									<UrlValueList values={f.values} hostname provider={badge?.provider ?? ''} />
								</dd>
								{#if badge?.manual}
									<ProvenanceBadge provider="" label="" manual />
								{/if}
							{:else}
								<dd class="inline text-ink">{f.values.join(', ')}</dd>
								{#if badge}
									<ProvenanceBadge provider={badge.provider} label={badge.provider} manual={badge.manual} />
								{/if}
							{/if}
						{/if}
					</div>
					{@render afterRow?.(f)}
				{/each}

				{#each longFields as f (f.canonical)}
					<div class="sm:col-span-2" id={`field-${f.canonical}`}>
						<dt class="inline text-muted">{f.label}:</dt>
						{#if isOwner}
							<dd class="block">
								<SourceBadge field={f} baselineKey="record" decide={(s, mv) => decide(f.canonical, s, mv)} />
							</dd>
						{:else}
							{@const badge = soleProvider ? null : rowProvenance(f.winning_source)}
							{#if f.values[0]?.trim()}
								<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
							{/if}
							{#if badge}
								<ProvenanceBadge provider={badge.provider} label={badge.provider} manual={badge.manual} />
							{/if}
						{/if}
					</div>
					{@render afterRow?.(f)}
				{/each}

				{@render children?.()}
			</dl>
		{/if}
	</section>
{/if}
