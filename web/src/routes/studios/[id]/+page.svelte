<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage, providerFromWinningSource } from '$lib/format';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { activity } from '$lib/activity.svelte';
	import { providerOf } from '$lib/f36';
	import type {
		DecisionSource,
		EnrichSource,
		PersonAlias,
		ResolvedField,
		Studio,
		StudioDetailResponse,
		Video
	} from '$lib/types';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import AliasPanel from '$lib/components/AliasPanel.svelte';
	import EntityVideos from '$lib/components/EntityVideos.svelte';
	import EnrichPicker from '$lib/components/EnrichPicker.svelte';
	import EnrichProviderChips from '$lib/components/EnrichProviderChips.svelte';
	import ProvenanceBadge from '$lib/components/ProvenanceBadge.svelte';
	import SourceSelect from '$lib/components/SourceSelect.svelte';
	import UrlValueList from '$lib/components/UrlValueList.svelte';
	import AutoFieldRows from '$lib/components/AutoFieldRows.svelte';
	import CurationFieldRow from '$lib/components/CurationFieldRow.svelte';
	import PromotedFieldEdit from '$lib/components/PromotedFieldEdit.svelte';

	// Studio detail (F38, ADR-053): name header + video grid + a Details section that
	// reuses the F36 source-chip radiogroup with the `record` baseline (RD5). Unlike the
	// person page there is no rename, no aliases, no images, no writeback — a studio's
	// name is derived identity, corrected by editing the studio field on its videos. The
	// Details section is hidden until enrichment or a decision gives it something beyond
	// `name` to curate — OR the owner has a studio-capable provider to enrich from (S3).
	let studio = $state<Studio | null>(null);
	let videos = $state<Video[]>([]);
	let resolved = $state<ResolvedField[]>([]);
	// Owner-curated routing aliases (F43, ADR-061), bound into AliasPanel. A studio's name
	// is derived identity, so the panel also offers Rename (allowRename) — the merge/rename
	// register the loser/old name as an alias so RelinkVideoStudios won't resurrect it (RD6).
	let aliases = $state<PersonAlias[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Enrichment controls (owner-only, F38 S3). sources loads once the client is
	// confirmed owner; the picker drives a provider resolve→apply. pickerProvider holds
	// the provider whose EnrichPicker is open ('' = closed); busy holds the provider
	// currently being cleared or refreshed (F47 RD7). Action errors render inline, never
	// via the page `error`. refreshingAll is Refresh-all's own busy flag (F47 RD8).
	let sources = $state<EnrichSource[]>([]);
	let pickerProvider = $state('');
	let busy = $state('');
	let refreshingAll = $state(false);
	let actionError = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner);

	// Studio-capable providers offered as Enrich actions (one per provider, mirroring
	// the person page's per-provider affordance).
	const studioProviders = $derived(
		sources.filter((s) => s.entity_types.includes('studio')).map((s) => s.name)
	);
	// A provider is "linked" (Clear offered) when a resolved field carries one of its
	// candidates — the same signal the person page uses.
	function providerLinked(p: string): boolean {
		return resolved.some((f) => (f.candidates ?? []).some((c) => providerOf(c.source) === p));
	}

	// Replace fields other than `name` that have a value or (for the owner) a candidate.
	// `name` is read-only identity — never a chip row.
	const replaceFields = $derived(
		resolved.filter(
			(f) =>
				f.canonical !== 'name' &&
				!f.multi &&
				!f.auto_registered &&
				(isOwner
					? f.values.length > 0 || (f.candidates ?? []).length > 0
					: f.values.some((v) => v.trim() !== ''))
		)
	);
	const imageFields = $derived(replaceFields.filter((f) => f.display === 'image_url'));
	const compactFields = $derived(
		replaceFields.filter((f) => f.display !== 'long_text' && f.display !== 'image_url')
	);
	const longFields = $derived(replaceFields.filter((f) => f.display === 'long_text'));
	// F44 (ADR-062): a chips-render promotion is a merge field — studio has no native
	// merge field, but a promotion can introduce one, so it needs its own row.
	const mergeFields = $derived(
		resolved.filter((f) => !!f.multi && !f.auto_registered && (isOwner || f.values.length > 0))
	);
	// F39 (ADR-056): display-only auto-registered non-canonical fields, read-only after
	// the curatable ones.
	const extraFields = $derived(resolved.filter((f) => f.auto_registered && f.values.length > 0));
	// Show the section when there's something to curate or display, or (owner) a provider to enrich from.
	const hasDetails = $derived(
		replaceFields.length > 0 || mergeFields.length > 0 || extraFields.length > 0
	);

	// The provider behind a visitor row's ProvenanceBadge — the winning namespace unless
	// it is a baseline source (record/file/manual). Shared with AutoFieldRows.
	const winnerProvider = (f: ResolvedField): string => providerFromWinningSource(f.winning_source);

	// When every shown field resolves from the SAME single provider, we hoist one
	// "Enriched from …" note to the section header (visitor view) instead of repeating an
	// identical badge on every row. Empty when providers differ (or none) — rows keep their
	// per-field badges so the divergence stays visible.
	const soleProvider = $derived.by(() => {
		const set = new Set(replaceFields.map(winnerProvider).filter(Boolean));
		return set.size === 1 ? [...set][0] : '';
	});

	function apply(res: StudioDetailResponse) {
		studio = res.studio;
		videos = res.items ?? [];
		resolved = res.resolved ?? [];
		aliases = res.studio.aliases ?? [];
	}

	function load(current: number) {
		loading = true;
		error = '';
		api
			.getStudio(current)
			.then(apply)
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	}

	$effect(() => load(id));

	// Load providers once the client is confirmed owner (the layout polls caps).
	$effect(() => {
		if (isOwner && sources.length === 0) {
			api
				.enrichSources()
				.then((res) => (sources = res.sources ?? []))
				.catch(() => {});
		}
	});

	async function reloadDetail() {
		try {
			apply(await api.getStudio(id));
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
	}

	async function clearProvider(p: string) {
		busy = p;
		actionError = '';
		try {
			await api.enrichStudioClear(id, p);
			await reloadDetail();
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			busy = '';
		}
	}

	// "Refresh" (RD7/P0-5) and "Refresh all" (RD8/P1-2) — shared with the video/person
	// detail pages via $lib/enrichRefresh; only the busy/error state and reload differ.
	async function refreshProvider(p: string) {
		await runEnrichRefresh(
			'studio',
			id,
			p,
			(v) => (busy = v),
			(v) => (actionError = v),
			reloadDetail
		);
	}

	async function refreshAll() {
		await runEnrichRefreshAll(
			'studio',
			id,
			(v) => (refreshingAll = v),
			(v) => (actionError = v),
			reloadDetail,
			(p) => (pickerProvider = p)
		);
	}

	// Persist a studio field decision then refetch. DB-only — a studio has no file, so
	// selecting the record chip is a standing blank-pin (RD5), so every chip maps to PUT.
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		await api.setStudioFieldDecision(id, canonical, {
			source,
			...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
		});
		await reloadDetail();
	}
</script>

<AsyncState {loading} {error}>
	<EntityVideos
		backHref="/studios"
		backLabel="All studios"
		name={studio?.name ?? ''}
		{videos}
		empty="No videos for this studio."
	>
		{#snippet detail()}
			<!-- Aliases are core identity, so the panel reads above the Details/enrichment
			     shadow (F43 handoff §1). Studio name is derived identity → allowRename lets the
			     owner correct it; the old name is kept as an alias so re-derivation survives. -->
			{#if studio}
				<AliasPanel
					entityType="studio"
					entityId={id}
					entityName={studio.name}
					bind:aliases
					{isOwner}
					allowRename
					onmerged={() => load(id)}
					onrenamed={() => load(id)}
				/>
			{/if}

			{#if hasDetails || (isOwner && studioProviders.length)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
						{#if isOwner && studioProviders.length}
							<!-- HOLODEX-136: one compact chip per studio-capable provider
							     (icon + name + Enrich), Clear in a ⋯ overflow once linked. -->
							<EnrichProviderChips
								providers={studioProviders}
								linked={providerLinked}
								{busy}
								{refreshingAll}
								onenrich={(p) => (pickerProvider = p)}
								onrefresh={refreshProvider}
								onclear={clearProvider}
								onrefreshall={refreshAll}
							/>
						{:else if !isOwner && soleProvider}
							<!-- Visitor: one section-level provenance note when every field shares a
							     single provider, instead of an identical badge per row. -->
							<span class="text-xs text-muted"
								>Enriched from <span class="text-accent">{soleProvider}</span></span
							>
						{/if}
					</div>

					{#if actionError}
						<p class="text-sm text-warn">{actionError}</p>
					{/if}

					{#if hasDetails}
						<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
							{#each imageFields as f (f.canonical)}
								<div class="sm:col-span-2">
									<dt class="mb-1 text-muted">{f.label}:</dt>
									<!-- The logo renders from the self-hosted, normalized copy (studio.logo_url,
									     served from our own origin — HOLODEX-130/ADR-057), not the hotlinked
									     provider URL in the resolved field. Present only when the cache is
									     populated for the resolved logo; the chip below stays authoritative. -->
									{#if f.canonical === 'logo' && studio?.logo_url}
										<dd class="mb-1">
											<img
												src={studio.logo_url}
												alt={studio?.name ? `${studio.name} ${f.label.toLowerCase()}` : f.label}
												class="max-h-32 rounded-theme border border-rule bg-logo-plate object-contain p-2"
											/>
										</dd>
									{/if}
									{#if isOwner}
										<dd>
											<SourceSelect
												field={f}
												baselineKey="record"
												decide={(s, mv) => decideField(f.canonical, s, mv)}
											/>
										</dd>
									{:else if !soleProvider && winnerProvider(f)}
										<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
									{/if}
								</div>
							{/each}

							{#snippet promotedEdit(f: ResolvedField)}
								<PromotedFieldEdit {isOwner} field={f} entityType="studio" entityNoun="studios" onchanged={reloadDetail} />
							{/snippet}

							{#each compactFields as f (f.canonical)}
								{#if isOwner}
									<div class={f.display === 'url' ? 'sm:col-span-2' : ''}>
										<dt class="mb-1 text-muted">{f.label}:</dt>
										<dd>
											<SourceSelect
												field={f}
												baselineKey="record"
												decide={(s, mv) => decideField(f.canonical, s, mv)}
											/>
										</dd>
									</div>
								{:else}
									<div class={f.display === 'url' ? 'sm:col-span-2' : ''}>
										<dt class="inline text-muted">{f.label}:</dt>
										{#if f.display === 'url'}
											<!-- HOLODEX-137: provider icon + host in the link folds in
											     provenance; suppressed when soleProvider shows the section note. -->
											<dd class="inline">
												<UrlValueList
													values={f.values}
													hostname
													provider={soleProvider ? '' : winnerProvider(f)}
												/>
											</dd>
										{:else}
											<dd class="inline text-ink">{f.values.join(', ')}</dd>
											{#if !soleProvider && winnerProvider(f)}
												<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
											{/if}
										{/if}
									</div>
								{/if}
								{@render promotedEdit(f)}
							{/each}

							{#each longFields as f (f.canonical)}
								<div class="sm:col-span-2">
									<dt class="inline text-muted">{f.label}:</dt>
									{#if f.values[0]?.trim()}
										<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
									{:else if isOwner}
										<dd class="mt-1 block text-muted">—</dd>
									{/if}
									{#if isOwner}
										<dd class="block">
											<SourceSelect
												field={f}
												baselineKey="record"
												decide={(s, mv) => decideField(f.canonical, s, mv)}
											/>
										</dd>
									{:else if !soleProvider && winnerProvider(f)}
										<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
									{/if}
								</div>
								{@render promotedEdit(f)}
							{/each}

							{#each mergeFields as f (f.canonical)}
								<div class="sm:col-span-2">
									<dt class="mb-1 text-muted">{f.label}:</dt>
									<dd>
										<CurationFieldRow
											field={f}
											{isOwner}
											showWriteToggle={false}
											curate={(req) => api.curateStudio(id, req)}
											clearCuration={(req) => api.clearStudioCuration(id, req)}
											onchanged={reloadDetail}
										/>
									</dd>
								</div>
								{@render promotedEdit(f)}
							{/each}

							<!-- F39 (ADR-056): display-only auto-registered non-canonical fields. -->
							<AutoFieldRows
								fields={extraFields}
								{isOwner}
								entityType="studio"
								entityNoun="studios"
								onchanged={reloadDetail}
							/>
						</dl>
					{/if}
				</section>
			{/if}
		{/snippet}
	</EntityVideos>
</AsyncState>

{#if pickerProvider}
	<EnrichPicker
		entityName={studio?.name ?? ''}
		provider={pickerProvider}
		resolve={(prov, q) => api.enrichStudioResolve(id, prov, q)}
		apply={(prov, extId) => api.enrichStudioApply(id, prov, extId)}
		dismiss={(prov) => api.enrichDismiss('studio', id, prov)}
		onclose={() => (pickerProvider = '')}
		onapplied={reloadDetail}
	/>
{/if}
