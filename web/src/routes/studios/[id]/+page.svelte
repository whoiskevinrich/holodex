<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage, providerFromWinningSource, aliasHint, videoCount } from '$lib/format';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { activity } from '$lib/activity.svelte';
	import { providerOf } from '$lib/f36';
	import { expandedField } from '$lib/expandedField.svelte';
	import type {
		Completeness,
		DecisionSource,
		EnrichSource,
		EntityRef,
		ExternalLink,
		PersonAlias,
		SkippedAlias,
		ResolvedField,
		Studio,
		StudioDetailResponse,
		Video
	} from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import AliasPanel from '$lib/components/person/AliasPanel.svelte';
	import EntityImageSlot from '$lib/components/entity/EntityImageSlot.svelte';
	import EntityVideos from '$lib/components/entity/EntityVideos.svelte';
	import FilmsRow from '$lib/components/entity/FilmsRow.svelte';
	import { filmsRow } from '$lib/filmsRow.svelte';
	import EntityVideoMeta from '$lib/components/entity/EntityVideoMeta.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';
	import MergeOfferCard from '$lib/components/entity/MergeOfferCard.svelte';
	import CompletenessPanel from '$lib/components/completeness/CompletenessPanel.svelte';
	import EnrichPicker from '$lib/components/enrichment/EnrichPicker.svelte';
	import EnrichProviderChips from '$lib/components/enrichment/EnrichProviderChips.svelte';
	import ProvenanceBadge from '$lib/components/enrichment/ProvenanceBadge.svelte';
	import SourceBadge from '$lib/components/curation/SourceBadge.svelte';
	import UrlValueList from '$lib/components/curation/UrlValueList.svelte';
	import AutoFieldRows from '$lib/components/curation/AutoFieldRows.svelte';
	import CurationFieldRow from '$lib/components/curation/CurationFieldRow.svelte';
	import PromotedFieldEdit from '$lib/components/curation/PromotedFieldEdit.svelte';

	// Studio detail (F38, ADR-053): name header + video grid + a Details section that
	// reuses the F36 source-chip radiogroup with the `record` baseline (RD5), plus an
	// Images section (F51, ADR-079: icon/logo/poster, owner upload/replace/remove).
	// Unlike the person page there is still no writeback — a studio's name is derived
	// identity, corrected by editing the studio field on its videos (rename/aliases
	// exist since F43). The Details section is hidden until enrichment or a decision
	// gives it something beyond `name` to curate — OR the owner has a studio-capable
	// provider to enrich from (S3).
	let studio = $state<Studio | null>(null);
	let videos = $state<Video[]>([]);
	let resolved = $state<ResolvedField[]>([]);
	// Owner-curated routing aliases (F43, ADR-061), bound into AliasPanel. A studio's name
	// is derived identity, renamed via the hero's NameEditControl (HOLODEX-269) — the
	// merge/rename register the loser/old name as an alias so RelinkVideoStudios won't
	// resurrect it (RD6).
	let aliases = $state<PersonAlias[]>([]);
	// Provider names a collision kept off this studio (F58, ADR-088 D5) — the panel's
	// review line. Owner-gated server-side: absent from a visitor's payload.
	let skippedAliases = $state<SkippedAlias[]>([]);
	// Rename-collision verdict state (HOLODEX-269) — mirrors the person page's inline
	// merge offer; NameEditControl owns the rest of the rename flow itself.
	let renameMergeBusy = $state(false);
	let renameMergeError = $state('');
	// Non-blocking near-miss advisory (F43 P1-5, mirrors AliasPanel's flagNearMiss) — a
	// fuzzy look-alike surfaced after a successful rename, distinct from the exact-name
	// `conflict` above. Studio only (`api.nearMiss` excludes person).
	let nearMiss = $state<EntityRef | null>(null);
	let nearMissBusy = $state(false);
	let nearMissError = $state('');
	let completeness = $state<Completeness | null>(null); // F55.13, owner-gated
	let externalLinks = $state<ExternalLink[]>([]); // HOLODEX-266, ADR-083 D1
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
	const compactFields = $derived(replaceFields.filter((f) => f.display !== 'long_text'));
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
		skippedAliases = res.skipped_aliases ?? [];
		completeness = res.completeness ?? null;
		externalLinks = res.external_links ?? [];
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

	$effect(() => {
		expandedField.reset(); // no per-entity scope of its own (F56.9) — clear on nav between studios
		load(id);
	});

	// Load providers once the client is confirmed owner (the layout polls caps).
	$effect(() => {
		if (isOwner && sources.length === 0) {
			api
				.enrichSources()
				.then((res) => (sources = res.sources ?? []))
				.catch(() => {});
		}
	});

	// Films row (F56, design handoff §5) — see filmsRow.svelte.ts.
	const filmsState = filmsRow(() => id, 'studioId');

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

	// Rename flow (HOLODEX-269) — same shared mechanism as Person: NameEditControl performs
	// the rename call and, on a 409, renders the `verdict` snippet below inline.
	async function commitStudioRename(value: string): Promise<{ ok: true } | { conflict: EntityRef }> {
		const res = await api.renameEntity('studio', id, value);
		if (res.conflict) return { conflict: res.conflict };
		await reloadDetail();
		// Advisory-only fuzzy look-alike check, same as AliasPanel's post-add flagNearMiss —
		// must never block the rename that already succeeded.
		try {
			nearMiss = (await api.nearMiss('studio', id, value)).near_miss;
		} catch {
			nearMiss = null;
		}
		return { ok: true };
	}

	async function mergeRenameConflict(mergeConflict: EntityRef, resolve: () => void) {
		renameMergeBusy = true;
		renameMergeError = '';
		try {
			await api.mergeEntities('studio', id, mergeConflict.id);
			resolve();
			await reloadDetail();
		} catch (e) {
			renameMergeError = toMessage(e);
		} finally {
			renameMergeBusy = false;
		}
	}

	async function mergeNearMiss() {
		if (!nearMiss) return;
		nearMissBusy = true;
		nearMissError = '';
		try {
			await api.mergeEntities('studio', id, nearMiss.id);
			nearMiss = null;
			await reloadDetail();
		} catch (e) {
			nearMissError = toMessage(e);
		} finally {
			nearMissBusy = false;
		}
	}

	async function keepNearMissSeparate() {
		if (!nearMiss) return;
		nearMissBusy = true;
		nearMissError = '';
		try {
			await api.dismissDuplicate('studio', id, nearMiss.id);
			nearMiss = null;
		} catch (e) {
			nearMissError = toMessage(e);
		} finally {
			nearMissBusy = false;
		}
	}
</script>

<AsyncState {loading} {error}>
	<EntityVideos
		backHref="/studios"
		backLabel="All studios"
		{videos}
		empty="No videos for this studio."
		scrollKey={`studio:${id}`}
	>
		{#snippet hero()}
			<!-- Docked-pencil rename (HOLODEX-269), replacing AliasPanel's old allowRename
			     trigger — same shared mechanism as Person/Tag. A studio's name is derived
			     identity; the old name is kept as an alias so re-derivation (RelinkVideoStudios)
			     survives (RD6). -->
			<NameEditControl
				name={studio?.name ?? ''}
				{isOwner}
				onCommit={commitStudioRename}
				label="studio"
				headingClass="skin-title text-2xl font-semibold text-ink"
				hint={studio ? aliasHint(studio.name) : undefined}
			>
				{#snippet verdict(c, resolve)}
					<MergeOfferCard
						noun="studio"
						entityName={studio?.name ?? ''}
						conflict={c}
						busy={renameMergeBusy}
						error={renameMergeError}
						onmerge={() => mergeRenameConflict(c, resolve)}
						onkeepseparate={() => {
							renameMergeError = '';
							resolve();
						}}
					/>
				{/snippet}
			</NameEditControl>
			{#if nearMiss}
				<!-- Non-blocking near-miss (P1-5): the rename already saved; this is an advisory
				     nudge, distinct from the blocking exact-name conflict above (mirrors AliasPanel). -->
				<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
					<p class="text-sm text-ink">
						Saved. Looks a lot like <span class="font-semibold">{nearMiss.name}</span>
						({videoCount(nearMiss.video_count ?? 0)}) — merge them?
					</p>
					<div class="flex flex-wrap items-center gap-2">
						<button onclick={mergeNearMiss} disabled={nearMissBusy} class="btn-accent px-3 py-1.5 text-sm">
							Yes, merge them in
						</button>
						<button
							onclick={keepNearMissSeparate}
							disabled={nearMissBusy}
							class="btn-ghost px-3 py-1.5 text-sm"
						>
							No, keep separate
						</button>
					</div>
					{#if nearMissError}
						<p class="text-sm text-warn">{nearMissError}</p>
					{/if}
				</div>
			{/if}
			<EntityVideoMeta count={videos.length} links={externalLinks} entityName={studio?.name ?? ''} />
		{/snippet}

		{#snippet detail()}
			<!-- Aliases are core identity, so the panel reads above the Details/enrichment
			     shadow (F43 handoff §1). Rename lives on the hero's NameEditControl now
			     (HOLODEX-269); this panel keeps only its Add-alias/merge functionality. -->
			{#if studio}
				<AliasPanel
					entityType="studio"
					entityId={id}
					entityName={studio.name}
					bind:aliases
					{isOwner}
					{skippedAliases}
					onmerged={() => load(id)}
				/>
			{/if}

			<!-- Images (F51, ADR-079): logo (existing usage) first, then icon (studios
			     list), then poster (no consumer yet). Always shown — owners can seed an
			     image even before any enrichment; visitors see filled slots read-only. -->
			{#if studio}
				<section class="space-y-2">
					<h2 class="text-xs uppercase tracking-wide text-muted">Images</h2>
					<div class="grid grid-cols-1 gap-2 sm:grid-cols-3" id="field-branding_image-upload">
						<EntityImageSlot
							entityId={id}
							entityName={studio.name}
							role="logo"
							label="Logo"
							url={studio.logo_url}
							{isOwner}
							upload={api.uploadStudioImage}
							remove={api.deleteStudioImage}
							onchanged={reloadDetail}
						/>
						<EntityImageSlot
							entityId={id}
							entityName={studio.name}
							role="icon"
							label="Icon"
							url={studio.icon_url}
							{isOwner}
							upload={api.uploadStudioImage}
							remove={api.deleteStudioImage}
							onchanged={reloadDetail}
						/>
						<EntityImageSlot
							entityId={id}
							entityName={studio.name}
							role="poster"
							label="Poster"
							url={studio.poster_url}
							{isOwner}
							upload={api.uploadStudioImage}
							remove={api.deleteStudioImage}
							onchanged={reloadDetail}
						/>
					</div>
				</section>
			{/if}

			{#if hasDetails || (isOwner && studioProviders.length)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted" id="enrich-providers">Details</h2>
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
							{#snippet promotedEdit(f: ResolvedField)}
								<PromotedFieldEdit {isOwner} field={f} entityType="studio" entityNoun="studios" onchanged={reloadDetail} />
							{/snippet}

							{#each compactFields as f (f.canonical)}
								{#if isOwner}
									<div class={f.display === 'url' ? 'sm:col-span-2' : ''} id={`field-${f.canonical}`}>
										<dt class="mb-1 text-muted">{f.label}:</dt>
										<dd>
											<SourceBadge
												field={f}
												baselineKey="record"
												decide={(s, mv) => decideField(f.canonical, s, mv)}
											/>
										</dd>
									</div>
								{:else}
									<div class={f.display === 'url' ? 'sm:col-span-2' : ''} id={`field-${f.canonical}`}>
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
								<div class="sm:col-span-2" id={`field-${f.canonical}`}>
									<dt class="inline text-muted">{f.label}:</dt>
									{#if !isOwner && f.values[0]?.trim()}
										<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
									{/if}
									{#if isOwner}
										<dd class="block">
											<SourceBadge
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
								<div class="sm:col-span-2" id={`field-${f.canonical}`}>
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

			{#if isOwner}
				<CompletenessPanel {completeness} onchanged={reloadDetail} />
			{/if}
		{/snippet}
		{#snippet footer()}
			<FilmsRow films={filmsState.films} />
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
