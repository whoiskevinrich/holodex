<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage, aliasHint } from '$lib/format';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { activity } from '$lib/activity.svelte';
	import type {
		Completeness,
		DecisionSource,
		EnrichSource,
		EntityRef,
		ExternalLink,
		Person,
		PersonAlias,
		PersonDetailResponse,
		PersonImageRole,
		PersonImageSet,
		ResolvedField,
		Video
	} from '$lib/types';
	import { providerOf } from '$lib/f36';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import CurationFieldRow from '$lib/components/curation/CurationFieldRow.svelte';
	import EntityVideos from '$lib/components/entity/EntityVideos.svelte';
	import ProvenanceBadge from '$lib/components/enrichment/ProvenanceBadge.svelte';
	import EntityVideoMeta from '$lib/components/entity/EntityVideoMeta.svelte';
	import EnrichPicker from '$lib/components/enrichment/EnrichPicker.svelte';
	import EnrichProviderChips from '$lib/components/enrichment/EnrichProviderChips.svelte';
	import AliasPanel from '$lib/components/person/AliasPanel.svelte';
	import PersonBanner from '$lib/components/person/PersonBanner.svelte';
	import PersonImageFrame from '$lib/components/person/PersonImageFrame.svelte';
	import NationalityFlags from '$lib/components/person/NationalityFlags.svelte';
	import PersonGallery from '$lib/components/person/PersonGallery.svelte';
	import SourceSelect from '$lib/components/curation/SourceSelect.svelte';
	import UrlValueList from '$lib/components/curation/UrlValueList.svelte';
	import AutoFieldRows from '$lib/components/curation/AutoFieldRows.svelte';
	import PromotedFieldEdit from '$lib/components/curation/PromotedFieldEdit.svelte';
	import CompletenessPanel from '$lib/components/completeness/CompletenessPanel.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';
	import MergeOfferCard from '$lib/components/entity/MergeOfferCard.svelte';
	import { providerFromWinningSource, calculatedFrom } from '$lib/format';

	let person = $state<Person | null>(null);
	let videos = $state<Video[]>([]);
	// F37: the unified resolved view (baseline `record`, no in_sync) — same shape the media
	// page consumes; the raw enriched[] block is retired.
	let resolved = $state<ResolvedField[]>([]);
	let images = $state<PersonImageSet>({ roles: {}, gallery: [] });
	let completeness = $state<Completeness | null>(null); // F55.13, owner-gated
	let externalLinks = $state<ExternalLink[]>([]); // HOLODEX-266, ADR-083 D1
	let loading = $state(true);
	let error = $state('');

	// Owner core-slot upload (F25): one hidden file input, retargeted per role.
	let coreInput = $state<HTMLInputElement | null>(null);
	let uploadRole = $state<PersonImageRole>('headshot');
	let uploadBusy = $state('');
	let imageError = $state('');

	// Owner-curated routing aliases (F23, ADR-036) — search & scan routing, deliberately a
	// separate system from the display-only "Also known as" merge row in Details (F37 RD2).
	// Read from the person payload and bound into AliasPanel (F43), which owns add/delete/
	// merge. `conflict` is the collision offer AliasPanel renders — set either by an added
	// alias (inside the panel) or by a rename collision (F37 RD1, routed in from below).
	let aliases = $state<PersonAlias[]>([]);
	let conflict = $state<EntityRef | null>(null);

	// Enrichment controls (owner-only, F22). sources is loaded once when the client
	// is confirmed owner; the picker drives a provider resolve→apply. pickerProvider
	// holds the provider whose EnrichPicker is open ('' = closed); busy holds the
	// provider name currently being cleared or refreshed (HOLODEX-119, F47 RD7);
	// refreshingAll is Refresh-all's own busy flag (F47 RD8 — it isn't one provider).
	let sources = $state<EnrichSource[]>([]);
	let pickerProvider = $state('');
	let busy = $state('');
	let refreshingAll = $state(false);
	// Action errors render inline in the panel — never via the page-level `error`,
	// which AsyncState uses to replace the whole page.
	let actionError = $state('');

	// Rename flow (HOLODEX-269, docked-pencil NameEditControl on the hero name — replaces
	// the F37 RD1 SourceSelect/onadopt intercept). A 409 shows the merge offer inline; these
	// two only cover the inline verdict's own busy/error (NameEditControl owns the rest).
	let renameMergeBusy = $state(false);
	let renameMergeError = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	// Banner renders only when a real one is set — for everyone, including the owner
	// (F25.30). Not every person has a banner-sized image; an empty 8:3 placeholder band
	// would dominate the page with generic art. Mirrors the poster rule (F25.27).
	const hasBanner = $derived(images.roles.banner?.present ?? false);
	// F25.31: when a poster exists it leads the hero (see the hero snippet below) — hoisted
	// so the margin class and the branch pick share one source of truth instead of two
	// textually-identical `images.roles.poster?.present` reads that could drift.
	const posterLed = $derived(images.roles.poster?.present ?? false);
	const heroMarginClass = $derived(
		!hasBanner ? (isOwner ? 'mt-3' : '') : posterLed ? '-mt-12 sm:-mt-14' : '-mt-10 sm:-mt-12'
	);
	// HOLODEX-119: every person-capable provider gets its own match/enrich/clear
	// affordance (the backend is already per-provider). Was collapsed to the first.
	const personProviders = $derived(
		sources.filter((s) => s.entity_types.includes('person')).map((s) => s.name)
	);
	// A provider is "linked" (Clear offered) when a resolved field carries one of its
	// candidates or merge values — the F37 replacement for scanning the retired enriched[].
	function providerLinked(p: string): boolean {
		return resolved.some(
			(f) =>
				(f.candidates ?? []).some((c) => providerOf(c.source) === p) ||
				(f.items ?? []).some((it) => it.sources.includes(p))
		);
	}

	// Field partitions (F37 handoff): the replace fields, then the merge fields ("Also
	// known as"). Name is excluded (rendered via NameEditControl in the hero, HOLODEX-269),
	// as is a field with no value and no candidates; visitors additionally see only fields
	// that resolved to a value.
	// HOLODEX-139: the resolved `nationality` values feed the hero flag beside the name.
	// Free text (TMDB place of birth, or a plain nationality word) → country → flag,
	// derived client-side; visitors see it too since resolved carries surviving values.
	const nationalityValues = $derived(
		resolved.find((f) => f.canonical === 'nationality')?.values ?? []
	);
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
	// Split the compact single-line vitals from the long-text prose (bio): the vitals
	// tile the two-column grid up top, and the full-width prose reads last, so a long
	// bio no longer buries the scannable facts (design-critique 2026-07-01).
	const compactFields = $derived(replaceFields.filter((f) => f.display !== 'long_text'));
	const longFields = $derived(replaceFields.filter((f) => f.display === 'long_text'));
	const mergeFields = $derived(
		resolved.filter((f) => !!f.multi && !f.auto_registered && (isOwner || f.values.length > 0))
	);
	// F39 (ADR-056): display-only auto-registered non-canonical fields — the provider's
	// extra attributes, rendered read-only after the curatable fields under an
	// "Additional details" divider. Same for owner and visitor (no controls).
	const extraFields = $derived(resolved.filter((f) => f.auto_registered && f.values.length > 0));

	// The provider name behind a visitor row's ProvenanceBadge — the winning namespace
	// unless it is a baseline source (record/file/manual). Shared with AutoFieldRows.
	const winnerProvider = (f: ResolvedField): string => providerFromWinningSource(f.winning_source);

	function applyPersonDetail(res: PersonDetailResponse) {
		person = res.person;
		videos = res.items ?? [];
		resolved = res.resolved ?? [];
		images = res.images ?? { roles: {}, gallery: [] };
		aliases = res.person.aliases ?? [];
		completeness = res.completeness ?? null;
		externalLinks = res.external_links ?? [];
	}

	function load(current: number) {
		loading = true;
		error = '';
		api
			.getPerson(current)
			.then((res) => {
				applyPersonDetail(res);
			})
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

	// Refetch-after-mutate (cf. the media page's applyMediaDetail idiom): re-read the
	// person so resolved[] reflects a new decision/curation/enrichment without flashing
	// the whole page through the AsyncState loading view. Non-fatal on error.
	async function reloadDetail() {
		try {
			applyPersonDetail(await api.getPerson(id));
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
	}

	async function clearProvider(p: string) {
		busy = p;
		actionError = '';
		try {
			await api.enrichClear(id, p);
			await reloadDetail();
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			busy = '';
		}
	}

	// "Refresh" (RD7/P0-5) and "Refresh all" (RD8/P1-2) — shared with the video/studio
	// detail pages via $lib/enrichRefresh; only the busy/error state and reload differ.
	async function refreshProvider(p: string) {
		await runEnrichRefresh(
			'person',
			id,
			p,
			(v) => (busy = v),
			(v) => (actionError = v),
			reloadDetail
		);
	}

	async function refreshAll() {
		await runEnrichRefreshAll(
			'person',
			id,
			(v) => (refreshingAll = v),
			(v) => (actionError = v),
			reloadDetail,
			(p) => (pickerProvider = p)
		);
	}

	// F37: persist a per-field source decision then refetch so resolved[] reflects it.
	// DB-only — a person has no file, so there is no writeback and no sync state. Unlike
	// the media page, selecting the record chip is a STANDING blank-pin decision (RD3:
	// `{source:"record"}` suppresses a provider value until re-decided), so every chip
	// maps to PUT. `name` never reaches here (the SourceSelect intercept owns it, RD1).
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		await api.setPersonFieldDecision(id, canonical, {
			source,
			...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
		});
		await reloadDetail();
	}

	// Rename flow (HOLODEX-269). NameEditControl performs the rename call itself via
	// onCommit; a 409 resolves to {conflict} and NameEditControl renders the `verdict`
	// snippet below inline (no navigation into the Aliases card).
	async function commitPersonRename(value: string): Promise<{ ok: true } | { conflict: EntityRef }> {
		const res = await api.renameEntity('person', id, value);
		if (res.conflict) return { conflict: res.conflict };
		await reloadDetail();
		return { ok: true };
	}

	// The owner confirmed the colliding entity is the same person — fold it in (never
	// auto-merge, F23 invariant). `resolve` is NameEditControl's own dismiss callback.
	async function mergeRenameConflict(mergeConflict: EntityRef, resolve: () => void) {
		renameMergeBusy = true;
		renameMergeError = '';
		try {
			await api.mergeEntities('person', id, mergeConflict.id);
			resolve();
			await load(id);
		} catch (e) {
			renameMergeError = toMessage(e);
		} finally {
			renameMergeBusy = false;
		}
	}

	// After any merge (via AliasPanel or the rename verdict), reload so the (now larger)
	// video list + new alias show.
	function onMerged() {
		load(id);
	}

	// Version stamp for a core role's serving URL, or undefined (empty slot → the
	// backend serves the themed placeholder; no ?v= needed).
	function roleVersion(role: PersonImageRole): number | undefined {
		return images.roles[role]?.version || undefined;
	}

	// Owner: open the file picker targeting a specific core role (F25.7).
	function pickCore(role: PersonImageRole) {
		uploadRole = role;
		coreInput?.click();
	}

	async function onCorePick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file || uploadBusy) return;
		uploadBusy = uploadRole;
		imageError = '';
		try {
			await api.uploadPersonImage(id, file, uploadRole);
			// Refresh just the image set — the new ?v= stamp busts the cache for the
			// uploaded role — without flashing the whole page through the AsyncState
			// loading view (a full load() would). Awaited so the busy label holds until
			// the new image is in hand, giving a clean swap (e.g. add-banner → banner).
			await reloadImages();
		} catch (err) {
			imageError = toMessage(err);
		} finally {
			uploadBusy = '';
		}
	}

	// Re-read the image set after a gallery change without refetching 500 videos.
	async function reloadImages() {
		try {
			images = await api.getPersonImages(id);
		} catch (err) {
			// Non-fatal (the mutation already succeeded; a full reload reconciles), but log
			// it so a persistently failing refresh is debuggable rather than silent.
			console.error('reloadImages: image set refresh failed', err);
		}
	}
</script>

<!-- Uniform owner "Edit" overlay for a core image slot (banner/headshot/poster) — one
     shape, one label, one scrim across all three, positioned per call site. `compact`
     swaps the text pill for an icon-only square — the headshot identity badge (64-80px)
     is too small for the full "Edit" pill without the button covering a big chunk of the
     image itself. -->
{#snippet editBtn(role: PersonImageRole, position: string, compact = false)}
	{#if isOwner}
		<button
			onclick={() => pickCore(role)}
			disabled={uploadBusy === role}
			aria-label={`Replace ${role}`}
			title={`Replace ${role}`}
			class="absolute z-10 {position} flex items-center justify-center rounded-theme bg-bg/80 text-ink shadow-sm backdrop-blur-sm hover:text-accent disabled:opacity-60 {compact
				? 'h-6 w-6'
				: 'px-2.5 py-1.5 text-xs font-semibold'}"
		>
			{#if compact}
				<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
					/>
				</svg>
			{:else}
				{uploadBusy === role ? '…' : 'Edit'}
			{/if}
		</button>
	{/if}
{/snippet}

<AsyncState {loading} error={error || (!person ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/people"
		backLabel="All people"
		name={person?.name ?? ''}
		{videos}
		empty="No videos for this person."
		scrollKey={`person:${id}`}
	>
		{#snippet hero()}
			<!-- F25 hero, ratio + hierarchy corrected by the 2026-07-12 design-critique pass: an
			     8:3 (1600×600) parallax banner with the overlap row pulled up into its lower-left
			     so the name reads as one unit with the face (not stranded above the banner). When a
			     poster exists it leads as the primary avatar (poster art fits this app's
			     film-archive framing better than a same-height headshot sliver), with the 1:1
			     headshot as a small identity badge on the poster's lower-left corner; with no
			     poster, the headshot alone is the primary avatar as before. The poster shows only
			     when a real one exists: an empty poster slot would just duplicate the headshot's
			     placeholder. Owners set a missing poster from the gallery below (promote-with-crop).
			     F25.30's decision to show no banner placeholder (owner-only "Add banner" affordance,
			     nothing for visitors) was reviewed and kept — its reasoning (an empty band would
			     dominate the page with generic art) applies just as much at 8:3 as it did at 5:2. -->
			<div class="relative">
				{#if hasBanner}
					<PersonBanner personId={id} name={person?.name ?? ''} version={roleVersion('banner')} eager />
					{@render editBtn('banner', 'right-2 top-2')}
				{:else if isOwner}
					<!-- No banner set: the owner gets an explicit add path here, since the band was the
					     only entry point for setting one. Visitors see nothing — the row below sits flush
					     at the top with no overhang. Reuses the core-slot upload (F25.30). -->
					<button
						onclick={() => pickCore('banner')}
						disabled={uploadBusy === 'banner'}
						title="Add banner"
						class="flex w-full items-center justify-center gap-2 rounded-theme border border-dashed border-rule bg-surface px-3 py-4 text-sm font-semibold text-muted hover:border-accent hover:text-accent"
					>
						{uploadBusy === 'banner' ? 'Adding…' : '+ Add banner'}
					</button>
				{/if}
				<!-- The headshot+name row overhangs the banner only when there is one; with no band it
				     sits flush (a small gap below the owner's add-banner control). (F25.30) -->
				<div class="flex items-end gap-4 pl-3 {heroMarginClass}">
					{#if posterLed}
						<!-- Poster-led: the poster is the primary avatar; the headshot rides as a small
						     identity badge on its lower-left corner (the bg-bg padding stands in for a
						     separating ring so the badge reads as its own layer over the poster art). -->
						<div class="relative shrink-0">
							<PersonImageFrame
								personId={id}
								role="poster"
								name={person?.name ?? ''}
								alt={`${person?.name ?? ''}'s poster`}
								version={roleVersion('poster')}
								frameClass="portrait-frame--2x3 h-36 w-auto sm:h-44"
								eager
							/>
							{@render editBtn('poster', 'right-1 top-1')}
							<div class="absolute -bottom-2 -left-2 rounded-theme bg-bg p-0.5">
								<div class="relative" id="field-photo-upload">
									<PersonImageFrame
										personId={id}
										role="headshot"
										name={person?.name ?? ''}
										version={roleVersion('headshot')}
										frameClass="portrait-frame--1x1 h-16 w-16 sm:h-20 sm:w-20"
										eager
									/>
									{@render editBtn('headshot', '-bottom-1 -right-1', true)}
								</div>
							</div>
						</div>
					{:else}
						<div class="relative shrink-0" id="field-photo-upload">
							<PersonImageFrame
								personId={id}
								role="headshot"
								name={person?.name ?? ''}
								version={roleVersion('headshot')}
								frameClass="portrait-frame--1x1 h-28 w-auto sm:h-36"
								eager
							/>
							{@render editBtn('headshot', 'bottom-1 right-1')}
						</div>
					{/if}
					<div class="min-w-0 flex-1 pb-1">
						<NameEditControl
							name={person?.name ?? ''}
							{isOwner}
							onCommit={commitPersonRename}
							label="person"
							headingClass="skin-title min-w-0 truncate text-3xl font-semibold text-ink sm:text-4xl"
							hint={person ? aliasHint(person.name) : undefined}
						>
							{#snippet trailing()}
								<NationalityFlags values={nationalityValues} />
							{/snippet}
							{#snippet verdict(c, resolve)}
								<MergeOfferCard
									noun="person"
									entityName={person?.name ?? ''}
									conflict={c}
									busy={renameMergeBusy}
									error={renameMergeError}
									onmerge={() => mergeRenameConflict(c, resolve)}
									onkeepseparate={resolve}
								/>
							{/snippet}
						</NameEditControl>
						<EntityVideoMeta
							count={videos.length}
							links={externalLinks}
							entityName={person?.name ?? ''}
						/>
					</div>
				</div>
				{#if isOwner}
					<input
						bind:this={coreInput}
						onchange={onCorePick}
						type="file"
						accept="image/*"
						class="hidden"
						aria-hidden="true"
						tabindex="-1"
					/>
				{/if}
				{#if imageError}
					<p class="mt-2 text-sm text-warn">{imageError}</p>
				{/if}
			</div>
		{/snippet}

		{#snippet detail()}
			<!-- Details (F37): every replace field on the F36 source-chip radiogroup with the
			     `record` baseline; `aliases` as the display-only "Also known as" merge row.
			     Deliberate absences vs. the media page: no Write button, no out-of-sync pill
			     (a person has no file), and the Name row RENAMES (RD1) instead of pinning. -->
			{#if resolved.length || (isOwner && personProviders.length)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted" id="enrich-providers">Details</h2>
						{#if isOwner && personProviders.length}
							<!-- HOLODEX-136: one compact chip per person-capable provider (icon +
							     name + Enrich), Clear in a ⋯ overflow once linked. Each opens its
							     own EnrichPicker and clears independently. -->
							<EnrichProviderChips
								providers={personProviders}
								linked={providerLinked}
								{busy}
								{refreshingAll}
								onenrich={(p) => (pickerProvider = p)}
								onrefresh={refreshProvider}
								onclear={clearProvider}
								onrefreshall={refreshAll}
							/>
						{/if}
					</div>

					{#if resolved.length}
						<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
							{#snippet promotedEdit(f: ResolvedField)}
								<PromotedFieldEdit {isOwner} field={f} entityType="person" entityNoun="people" onchanged={reloadDetail} />
							{/snippet}

							{#each compactFields as f (f.canonical)}
								{#if f.computed}
									<!-- F45 (ADR-063): derived read-only row (Age / Age at death) — identical
									     for owner and visitor, no controls. Branches ahead of the owner/visitor
									     split so a computed field never reaches SourceSelect or promotedEdit. The
									     "calculated from …" provenance is a hover tooltip on the value itself —
									     no separate badge/icon. aria-label restates value + provenance for SR. -->
									{@const provenance = calculatedFrom(f.derived_from ?? [])}
									<div>
										<dt class="inline text-muted">{f.label}:</dt>
										<dd
											class="inline text-ink"
											title={provenance}
											aria-label={`${f.values[0]}, ${provenance}`}
										>
											{f.values[0]}
										</dd>
									</div>
								{:else if isOwner}
									<!-- Replace field, owner: the selected chip IS the value (media idiom). -->
									<div id={`field-${f.canonical}`}>
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
									<!-- Visitor: read-only resolved value, exactly like the old rows. -->
									<div id={`field-${f.canonical}`}>
										<dt class="inline text-muted">{f.label}:</dt>
										{#if f.display === 'url'}
											<!-- HOLODEX-137: the link leads with the provider icon + host,
											     folding provenance in — so no separate badge on url rows. -->
											<dd class="inline"><UrlValueList values={f.values} provider={winnerProvider(f)} /></dd>
										{:else}
											<dd class="inline text-ink">{f.values.join(', ')}</dd>
											{#if winnerProvider(f)}
												<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
											{/if}
										{/if}
									</div>
								{/if}
								{#if !f.computed}{@render promotedEdit(f)}{/if}
							{/each}

							{#each mergeFields as f (f.canonical)}
								<!-- "Also known as" (RD2): provider aliases as an F30 merge row —
								     display-only curation (✕ suppress / + Add); kept chips never route
								     scans or search (that is the separate Aliases card below). No
								     nowrite toggle: persons have no writeback. -->
								<div class="sm:col-span-2" id={`field-${f.canonical}`}>
									<dt class="mb-1 text-muted">
										{f.canonical === 'aliases' ? 'Also known as' : f.label}:
									</dt>
									<dd>
										<CurationFieldRow
											field={f}
											{isOwner}
											showWriteToggle={false}
											curate={(req) => api.curatePerson(id, req)}
											clearCuration={(req) => api.clearPersonCuration(id, req)}
											onchanged={reloadDetail}
										/>
									</dd>
								</div>
								{@render promotedEdit(f)}
							{/each}

							{#each longFields as f (f.canonical)}
								<!-- Long-text (bio) reads last as a full-width prose block, so it
								     doesn't bury the compact vitals above (design-critique 2026-07-01).
								     Long-text fit (P1-1): the resolved value is the reading surface;
								     the chip row beneath is the source selector (chips stay clamped). -->
								<div class="sm:col-span-2" id={`field-${f.canonical}`}>
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
									{:else if winnerProvider(f)}
										<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
									{/if}
								</div>
								{@render promotedEdit(f)}
							{/each}

							<!-- F39 (ADR-056): display-only auto-registered non-canonical fields —
							     read-only rows under an "Additional details" divider (shared component). -->
							<AutoFieldRows
								fields={extraFields}
								{isOwner}
								entityType="person"
								entityNoun="people"
								onchanged={reloadDetail}
							/>
						</dl>
					{:else}
						<p class="text-sm text-muted">No details yet.</p>
					{/if}

					{#if actionError}
						<p class="text-sm text-warn">{actionError}</p>
					{/if}
				</section>
			{/if}

			{#if isOwner}
				<CompletenessPanel {completeness} onchanged={reloadDetail} />
			{/if}

			<!-- The F23 routing-alias card, now the shared AliasPanel (F43) — deliberately its
			     own system below Details (RD2): these names drive search + scan routing, unlike
			     the display-only "Also known as" chips above. Rename stays on the Name chip row
			     (RD1), so the panel's own rename is off; the rename collision is routed in via
			     `conflict`. -->
			{#if person}
				<AliasPanel
					entityType="person"
					entityId={id}
					entityName={person.name}
					bind:aliases
					{isOwner}
					bind:conflict
					onmerged={onMerged}
				/>
			{/if}

			{#if images.gallery.length || isOwner}
				<section class="rounded-theme border border-rule bg-surface p-4">
					<PersonGallery
						personId={id}
						name={person?.name ?? ''}
						items={images.gallery}
						owner={isOwner}
						onchanged={reloadImages}
					/>
				</section>
			{/if}
		{/snippet}
	</EntityVideos>
</AsyncState>

{#if pickerProvider}
	<EnrichPicker
		entityName={person?.name ?? ''}
		provider={pickerProvider}
		resolve={(prov, q) => api.enrichResolve(id, prov, q)}
		apply={(prov, extId) => api.enrichApply(id, prov, extId)}
		dismiss={(prov) => api.enrichDismiss('person', id, prov)}
		onclose={() => (pickerProvider = '')}
		onapplied={reloadDetail}
	/>
{/if}

