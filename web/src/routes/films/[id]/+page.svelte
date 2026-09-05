<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage, resolutionBucket, releaseYear } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { providerOf } from '$lib/f36';
	import type {
		DecisionSource,
		EnrichSource,
		Film,
		FilmBilledCredit,
		FilmYearCollision,
		FilmDetailResponse,
		FilmVideo,
		Person,
		ResolvedField,
		Studio,
		Tag,
		Video
	} from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import SourceBadge from '$lib/components/curation/SourceBadge.svelte';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';
	import WritebackFormDialog from '$lib/components/writeback/WritebackFormDialog.svelte';
	import WritebackBatchDialog from '$lib/components/writeback/WritebackBatchDialog.svelte';
	import FilmBulkAttachDialog from '$lib/components/film/FilmBulkAttachDialog.svelte';
	import FilmStudioCascadeDialog from '$lib/components/film/FilmStudioCascadeDialog.svelte';
	import EntityImageSlot from '$lib/components/entity/EntityImageSlot.svelte';
	import StudioLinkCard from '$lib/components/entity/StudioLinkCard.svelte';
	import PeopleGrid from '$lib/components/entity/PeopleGrid.svelte';
	import TagLinkChip from '$lib/components/entity/TagLinkChip.svelte';
	import EnrichPicker from '$lib/components/enrichment/EnrichPicker.svelte';
	import EnrichProviderChips from '$lib/components/enrichment/EnrichProviderChips.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';

	// Film detail (F56, design handoff §2): two hard-separated regions below the header —
	// full-film file(s) (§2b, the only place a film-page writeback button appears) and the
	// scenes list (§2c) — never merged (RD4). Cast/tags/studios are the read-only union of
	// the film's videos (RD2/RD3), not editable chips, so they route through plain links,
	// not SourceSelect. The Details section (description/release_date) mirrors Studio's
	// SourceBadge/`baselineKey='record'` pattern exactly, and since F59/HOLODEX-309 so does
	// its enrichment header row — films still have no rename/aliases (HOLODEX-281 deferred).
	// The poster (HOLODEX-280,
	// ADR-086) is the header's own image now (HOLODEX-307) — EntityImageSlot in its
	// `variant="frame"` hero mode owns upload/replace/remove there, replacing the old
	// dedicated Images section; the `thumb` role had no consumer, so it was dropped.
	let film = $state<Film | null>(null);
	let resolved = $state<ResolvedField[]>([]);
	let scenes = $state<FilmVideo[]>([]);
	let fullFilms = $state<FilmVideo[]>([]);
	let cast = $state<Person[]>([]);
	// The scene union's complement against the provider's billed list (F59/ADR-089 D2),
	// and how many names were billed in total. Computed server-side from the enrichment
	// shadow — nothing is stored, so clearing the provider empties both.
	let billedAbsent = $state<FilmBilledCredit[]>([]);
	let billedTotal = $state(0);
	let tags = $state<Tag[]>([]);
	let studios = $state<Studio[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Enrichment controls (owner-only, F59/HOLODEX-309) — the same five pieces of state
	// the studio page carries. pickerProvider holds the provider whose EnrichPicker is
	// open ('' = closed); busy holds the provider being cleared or refreshed (F47 RD7);
	// refreshingAll is Refresh-all's own flag (F47 RD8). Action errors render inline in
	// the Details section, never via the page-level `error`.
	let sources = $state<EnrichSource[]>([]);
	let pickerProvider = $state('');
	let busy = $state('');
	let refreshingAll = $state(false);
	let actionError = $state('');

	// The owner's year edit (F59/HOLODEX-317). A (name, year) clash surfaces as
	// NameEditControl's inline `verdict` beside the year itself — the control the owner
	// just used — rather than as a message in another section about a field with no
	// visible slot, which is what this replaced.
	async function commitYear(value: string): Promise<{ ok: true } | { conflict: FilmYearCollision }> {
		const year = Number(value.trim());
		if (!Number.isInteger(year) || year <= 0) {
			throw new Error('Enter a year, e.g. 1999.');
		}
		const res = await api.setFilmYear(id, year);
		if (res.conflict) return { conflict: res.conflict };
		await reloadDetail();
		return { ok: true };
	}

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner);

	// A film has no `name` beyond baseline (no rename in v1 — ADR-089 D3 keeps it that
	// way), so only fields beyond `name` gate the Details section — same "hide the whole
	// section, don't show an empty box" rule. The owner also gets the section when a
	// film-capable provider exists but nothing has resolved yet, or there would be no
	// way to reach the Enrich control on an unenriched film.
	const replaceFields = $derived(resolved.filter((f) => f.canonical !== 'name'));
	const hasDetails = $derived(replaceFields.length > 0);

	// Film-capable providers offered as Enrich actions, mirroring studioProviders. No
	// films_enabled check is needed: film enrich routes are unregistered when the flag is
	// off, so getFilm 404s and this whole page renders its error state instead.
	const filmProviders = $derived(
		sources.filter((s) => s.entity_types.includes('film')).map((s) => s.name)
	);

	// A provider is "linked" (Clear offered) when a resolved field carries one of its
	// candidates — the same signal the person and studio pages use.
	function providerLinked(p: string): boolean {
		return resolved.some((f) => (f.candidates ?? []).some((c) => providerOf(c.source) === p));
	}

	// The film's year and its resolved release date are DIFFERENT claims: year is half the
	// (name, year) identity key — which Dune this is — while release_date is provider
	// metadata. They diverge legitimately (a festival year vs a wide release, a re-release,
	// a director's cut dated years later), so nothing reconciles them and nothing should.
	// The page's job is only to stop the divergence being silent, which is what made the
	// first version of this information unreadable.
	//
	// Owner-only, matching the Details section: the provider's date is curation context,
	// not something a visitor needs.
	const releaseDateYear = $derived(
		releaseYear(resolved.find((f) => f.canonical === 'release_date')?.values?.[0])
	);
	const yearDiffersFromRelease = $derived(
		!!film?.year && releaseDateYear > 0 && film.year !== releaseDateYear
	);

	// Unnumbered scenes sort after all numbered ones, in whatever order the API returns
	// them (RD5 — no ordering guarantee among unnumbered scenes, so no secondary sort key).
	const sortedScenes = $derived(
		[...scenes].sort((a, b) => {
			if (a.scene_number == null && b.scene_number == null) return 0;
			if (a.scene_number == null) return 1;
			if (b.scene_number == null) return -1;
			return a.scene_number - b.scene_number;
		})
	);
	const sceneNumberByVideoId = $derived(new Map(scenes.map((s) => [s.video.id, s.scene_number])));
	const sceneNumberOf = (v: Video): number | null | undefined => sceneNumberByVideoId.get(v.id);

	// FilmStudioCascadeDialog's Collision/Error rows only get a video_id back from the
	// cascade response — this names them without a second fetch.
	const videoTitles = $derived(
		new Map([...scenes.map((s) => [s.video.id, s.video.title] as const), ...fullFilms.map((fv) => [fv.video.id, fv.video.title] as const)])
	);
	const attachedVideoCount = $derived(videoTitles.size);

	function applyDetail(res: FilmDetailResponse) {
		film = res.film;
		resolved = res.resolved ?? [];
		scenes = res.scenes ?? [];
		fullFilms = res.full_films ?? [];
		cast = res.cast ?? [];
		tags = res.tags ?? [];
		studios = res.studios ?? [];
		billedAbsent = res.billed_absent ?? [];
		billedTotal = res.billed_total ?? 0;
	}

	function load(current: number) {
		loading = true;
		error = '';
		api
			.getFilm(current)
			.then(applyDetail)
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	}

	$effect(() => {
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

	async function reloadDetail() {
		try {
			applyDetail(await api.getFilm(id));
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
	}

	async function clearProvider(p: string) {
		busy = p;
		actionError = '';
		try {
			await api.enrichFilmClear(id, p);
			await reloadDetail();
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			busy = '';
		}
	}

	// "Refresh" (F47 RD7/P0-5) and "Refresh all" (RD8/P1-2) — shared with the
	// video/person/studio detail pages via $lib/enrichRefresh, which is already generic
	// over EnrichEntityKind; only the busy/error state and reload differ. Films needed no
	// change there at all once the kind union widened (ADR-089 D5).
	async function refreshProvider(p: string) {
		await runEnrichRefresh(
			'film',
			id,
			p,
			(v) => (busy = v),
			(v) => (actionError = v),
			reloadDetail
		);
	}

	async function refreshAll() {
		await runEnrichRefreshAll(
			'film',
			id,
			(v) => (refreshingAll = v),
			(v) => (actionError = v),
			reloadDetail,
			(p) => (pickerProvider = p)
		);
	}

	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		await api.setFilmFieldDecision(id, canonical, {
			source,
			...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
		});
		await reloadDetail();
	}

	// Full-film writeback (§2b, P0-11) — lazy-loads that video's own resolved fields on
	// open, since FilmDetailResponse carries only the film's fields, not each video's.
	let writebackVideoId = $state<number | null>(null);
	let writebackFields = $state<ResolvedField[]>([]);
	let writebackFilePath = $state('');
	let writebackLoadError = $state('');

	async function openWriteback(video: Video) {
		writebackLoadError = '';
		try {
			const res = await api.getMedia(video.id);
			writebackFields = res.resolved ?? [];
			writebackFilePath = video.file_path;
			writebackVideoId = video.id;
		} catch (e) {
			writebackLoadError = toMessage(e);
		}
	}

	let attachOpen = $state(false);

	// Film-studio cascade (F57, HOLODEX-285, design handoff §3/§4): the dialog itself
	// commits via api.cascadeFilmStudio; this page only holds what to do after — mount
	// WritebackBatchDialog once the owner asks to view progress, and refresh the union
	// once it settles.
	let cascadeOpen = $state(false);
	let cascadePending = $state<{ batch_id: string; enqueued: number } | null>(null);

	function openCascade() {
		cascadeOpen = true;
	}
</script>

<AsyncState {loading} {error}>
	{#if film}
		<section class="mx-auto max-w-4xl space-y-6">
			<!-- No "All films" backlink: the nav's Films link already goes there, and the person
			     page dropped its equivalent for the same reason (#286). -->

			<!-- Banner (F59/ADR-089 D4, design handoff §2). The landscape provider image sits
			     BEHIND an otherwise-unchanged header: PR #292 made the portrait poster this
			     page's identity image, and a film's poster carries more recognition weight
			     than the backdrop. The band is atmosphere; the poster stays the subject.
			     Visitor with no banner sees no band at all (F25.30's rule for Person) — the
			     header then renders exactly as it did before this feature. -->
			{#if film.banner_url}
				<div class="relative -mb-14 overflow-hidden rounded-theme">
					<EntityImageSlot
						entityId={id}
						entityName={film.name}
						role="banner"
						label="Banner"
						url={film.banner_url}
						{isOwner}
						variant="frame"
						frameClass="aspect-[8/3] w-full"
						fit="cover"
						upload={api.uploadFilmImage}
						remove={api.deleteFilmImage}
						onchanged={reloadDetail}
					/>
					<!-- Legibility scrim: the header row overlaps the band's lower third, so the
					     title/year/studio must stay readable over arbitrary artwork. Presentational
					     only — pointer-events-none so it never swallows the owner's overlay
					     controls sitting above it. -->
					<div
						class="pointer-events-none absolute inset-x-0 bottom-0 h-2/3 bg-gradient-to-t from-bg to-transparent"
						aria-hidden="true"
					></div>
				</div>
			{:else if isOwner}
				<!-- No band when empty: an 8:3 monogram plate would be a large, meaningless
				     block. The compact row variant carries the same upload affordance. -->
				<EntityImageSlot
					entityId={id}
					entityName={film.name}
					role="banner"
					label="Banner"
					url={undefined}
					{isOwner}
					upload={api.uploadFilmImage}
					remove={api.deleteFilmImage}
					onchanged={reloadDetail}
				/>
			{/if}

			<!-- 2a. Header -->
			<div class="relative flex flex-col gap-4 sm:flex-row">
				<div class="w-40 shrink-0">
					<EntityImageSlot
						entityId={id}
						entityName={film.name}
						role="poster"
						label="Poster"
						url={film.poster_url}
						{isOwner}
						variant="frame"
						frameClass="aspect-[2/3] w-40"
						upload={api.uploadFilmImage}
						remove={api.deleteFilmImage}
						onchanged={reloadDetail}
					/>
				</div>
				<div class="flex-1 space-y-2">
					<h1 class="skin-title text-2xl font-semibold text-ink">{film.name}</h1>

					<!-- Year (F59/HOLODEX-317). Reuses the Media page's edit affordance rather than
					     imitating it — this page previously hand-rolled `name-edit-row`/
					     `name-edit-pencil` for the studio row below without the component.
					     `as="p"` because the title above already owns the page's only h1 and a
					     year is a field value, not a heading. The empty state mirrors "No studio
					     set" one row down, so the year finally has a visible slot to be absent
					     from. -->
					<NameEditControl
						name={film.year ? String(film.year) : ''}
						placeholder="No year set"
						as="p"
						headingClass="text-sm text-muted"
						label="film"
						editLabel={film.year ? 'Change the year for this film' : 'Set a year for this film'}
						{isOwner}
						onCommit={commitYear}
						id="field-year"
					>
						{#snippet verdict(c: FilmYearCollision, resolve: () => void)}
							<div class="mt-2 space-y-2 rounded-theme border border-rule bg-surface p-3">
								<p class="text-sm text-ink">
									<a href={`/films/${c.film_id}`} class="underline hover:text-accent"
										>{c.film_name} ({c.year})</a
									> already uses that name and year.
								</p>
								<p class="text-xs text-muted">
									A film is identified by its name and year together, so the two can't match.
								</p>
								<div class="flex flex-wrap gap-2">
									<a href={`/films/${c.film_id}`} class="btn-ghost px-3 py-1.5 text-sm"
										>View that film</a
									>
									<button type="button" onclick={resolve} class="btn-quiet px-3 py-1.5 text-sm">
										Cancel
									</button>
								</div>
							</div>
						{/snippet}
					</NameEditControl>

					<!-- Year vs. release date (F59). Stated, never reconciled — see the derivation
					     above for why they legitimately differ. Muted and factual: no verb, so it
					     does not imply the year is wrong and nag the owner to "fix" a correct
					     value. Only rendered when both exist and actually disagree. -->
					{#if isOwner && yearDiffersFromRelease}
						<p class="text-xs text-muted">Release date says {releaseDateYear}.</p>
					{/if}

				<!-- Studio — gated exactly like the Media page's studio row
				     (`media/[id]/+page.svelte`: `{#if isOwner || studioField?.values?.length}`).
				     A visitor never sees "No studio set": an empty row is an owner affordance,
				     not information, and films were the only page still showing it. -->
				{#if isOwner || studios.length}
					<div class="name-edit-row flex flex-wrap items-center gap-3 pt-1">
						{#if studios.length}
							{#each studios as s (s.id)}
								<StudioLinkCard studio={s} />
							{/each}
						{:else}
							<span class="text-sm text-muted">No studio set</span>
						{/if}
						{#if isOwner}
							<button
								type="button"
								aria-label="Change the studio for every video in this film"
								onclick={openCascade}
								class="name-edit-pencil rounded-theme border border-rule p-1.5 text-muted hover:border-accent hover:text-ink"
							>
								<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z"
									/>
								</svg>
							</button>
						{/if}
					</div>
				{/if}

					<!-- Tags — read-only union per RD2/RD3 above, but rendered through the same
					     section markup (heading + TagLinkChip wrap) as the Media detail page's
					     Tags section for visual parity; only the owner add/remove controls differ. -->
					{#if tags.length}
						<section class="space-y-1.5">
							<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
							<div class="flex flex-wrap items-center gap-2">
								{#each tags as t (t.id)}
									<TagLinkChip tag={t} />
								{/each}
							</div>
						</section>
					{/if}

					{#each replaceFields.filter((f) => f.canonical === 'description') as f (f.canonical)}
						{#if f.values[0]?.trim()}
							<p class="leading-relaxed text-ink">{f.values[0]}</p>
						{/if}
					{/each}
				</div>
			</div>

			<!-- Cast (design handoff §2a): read-only union of the film's scenes' people, shared
			     PeopleGrid component with the Media detail page's People section (not inline
			     pills) — no attach/detach passed, since this is derived display, not an
			     editable/attachable relationship. -->
			<PeopleGrid title="Cast" people={cast} />

			<!-- Scene coverage (F59/ADR-089 D2, closing ADR-085's deferred P1-3).
			     **Owner-only**, for the same reason the Details section is: it is curation
			     data about the library's completeness, and the copy is written in the
			     owner's voice — telling a visitor "Your scenes cover 0 of 20" names scenes
			     that are not theirs. Also hidden when no provider has billed a cast, so an
			     unenriched film's Cast section renders exactly as it always did.
			     The chips are the scene union's COMPLEMENT, never the whole billed list:
			     rendering both in full would be roughly half duplicates at realistic scale,
			     and the difference is the only version that says something Cast does not.
			     Chips rather than a second PeopleGrid because most of these have no Person
			     row at all — that is precisely what they mean — so there is no portrait to
			     tile. Dashed reads as provisional/absent; deliberately NOT `text-warn`, since
			     incomplete coverage is information, not an error. -->
			{#if isOwner && billedTotal}
				<section class="space-y-1.5">
					<p class="text-xs text-muted">
						{billedAbsent.length === 0
							? `Your scenes cover all ${billedTotal} billed cast.`
							: `Your scenes cover ${billedTotal - billedAbsent.length} of ${billedTotal} billed cast.`}
					</p>
					{#if billedAbsent.length}
						<h2 class="text-xs uppercase tracking-wide text-muted">
							Billed on the release — in no scene you own
						</h2>
						<div class="flex flex-wrap items-center gap-2">
							{#each billedAbsent as c (c.name)}
								{#if c.person_id}
									<a
										href={`/people/${c.person_id}`}
										class="rounded-full border border-dashed border-accent px-2.5 py-0.5 text-sm text-accent hover:border-solid"
										title="In your library, but not in this film's scenes">{c.name}</a
									>
								{:else}
									<span
										class="rounded-full border border-dashed border-rule px-2.5 py-0.5 text-sm text-muted"
										title="Not in your library">{c.name}</span
									>
								{/if}
							{/each}
						</div>
					{/if}
				</section>
			{/if}

			<!-- Details (description/release_date) — mirrors Studio's SourceBadge pattern, and
			     since F59/HOLODEX-309 its enrichment header row too.
			     **Owner-only.** This section is provenance and curation machinery, not reader
			     content: the description a visitor wants already renders in the header above,
			     and everything else here (source badges, provider chips, Released) exists to
			     serve editing decisions. Gating the whole section also retired the visitor's
			     section-level "Enriched from X" note, which could no longer render. The owner
			     still gets it when a provider exists but nothing has resolved yet, or an
			     unenriched film would offer no way to reach Enrich. -->
			{#if isOwner && (hasDetails || filmProviders.length)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted" id="enrich-providers">Details</h2>
						{#if isOwner && filmProviders.length}
							<!-- One compact chip per film-capable provider (icon + name + Enrich),
							     Clear in a ⋯ overflow once linked (HOLODEX-136). Reused verbatim —
							     it takes no entity id and no entity kind (ADR-089 D5). -->
							<EnrichProviderChips
								providers={filmProviders}
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

					{#if actionError}
						<p class="text-sm text-warn">{actionError}</p>
					{/if}


					<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
						{#each replaceFields as f (f.canonical)}
							<div id={`field-${f.canonical}`}>
								<dt class="mb-1 text-muted">{f.label}:</dt>
								<dd>
									{#if isOwner}
										<SourceBadge field={f} baselineKey="record" decide={(s, mv) => decideField(f.canonical, s, mv)} />
									{:else}
										<span class="text-ink">{f.values.join(', ')}</span>
									{/if}
								</dd>
							</div>
						{/each}
					</dl>
				</section>
			{/if}

			<!-- 2b. Full-film file section — a list, only place a film-page writeback button lives. -->
			{#if fullFilms.length}
				<section class="space-y-2">
					<h2 class="text-xs uppercase tracking-wide text-muted">Full film</h2>
					<ul class="space-y-2">
						{#each fullFilms as fv (fv.video.id)}
							<li
								class="flex flex-wrap items-center justify-between gap-2 rounded-theme border border-rule bg-surface px-3 py-2"
							>
								<a href={`/media/${fv.video.id}`} class="min-w-0 flex-1 truncate text-sm text-ink hover:text-accent">
									{fv.video.title}
								</a>
								{#if fv.video.width > 0}
									<span class="rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink"
										>{resolutionBucket(fv.video.width)}</span
									>
								{/if}
								{#if isOwner}
									<button onclick={() => openWriteback(fv.video)} class="btn-ghost px-2.5 py-1 text-xs">
										Write to file…
									</button>
								{/if}
							</li>
						{/each}
					</ul>
					{#if writebackLoadError}
						<p class="text-sm text-warn">{writebackLoadError}</p>
					{/if}
				</section>
			{/if}

			<!-- 2c. Scenes list -->
			<section class="space-y-2">
				<div class="flex items-center justify-between">
					<h2 class="text-xs uppercase tracking-wide text-muted">Scenes</h2>
					{#if isOwner}
						<button onclick={() => (attachOpen = true)} class="btn-accent px-3 py-1.5 text-sm">
							Attach videos…
						</button>
					{/if}
				</div>
				{#if scenes.length === 0}
					<p class="py-8 text-center text-sm text-muted">No scenes attached yet.</p>
				{:else}
					<VideoGrid videos={sortedScenes.map((s) => s.video)} sceneNumbers={sceneNumberOf} />
				{/if}
			</section>
		</section>
	{/if}
</AsyncState>

{#if writebackVideoId && film}
	<WritebackFormDialog
		fields={writebackFields}
		videoId={writebackVideoId}
		filePath={writebackFilePath}
		writeback={api.writebackMedia}
		jobStatus={api.writebackJobStatus}
		decide={async (canonical, source, manualValue) => {
			const res = await api.setFieldDecision(writebackVideoId as number, canonical, {
				source,
				...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
			});
			if (res.conflict) throw new Error(`"${manualValue}" already matches another video: ${res.conflict.title}`);
		}}
		onclose={() => (writebackVideoId = null)}
		onapplied={() => {
			writebackVideoId = null;
			reloadDetail();
		}}
	/>
{/if}

{#if attachOpen && film}
	<FilmBulkAttachDialog
		filmId={id}
		filmName={film.name}
		filmStudios={studios}
		filmCast={cast}
		onclose={() => (attachOpen = false)}
		onattached={reloadDetail}
	/>
{/if}

{#if cascadeOpen && film}
	<FilmStudioCascadeDialog
		filmId={id}
		filmName={film.name}
		{attachedVideoCount}
		currentStudios={studios}
		{videoTitles}
		onclose={() => (cascadeOpen = false)}
		onviewprogress={(batch_id, enqueued) => {
			cascadeOpen = false;
			cascadePending = { batch_id, enqueued };
		}}
	/>
{/if}

{#if cascadePending && film}
	<WritebackBatchDialog
		scopeLabel={film.name}
		videoCountHint={cascadePending.enqueued}
		initialBatch={cascadePending}
		batchStatus={api.writebackBatchStatus}
		onclose={() => (cascadePending = null)}
		onapplied={reloadDetail}
	/>
{/if}

<!-- Provider candidate picker (F59/HOLODEX-309). Reused unchanged from person/studio/
     media — it takes no entity id and no entity kind; the three closures below are the
     entire film-specific surface (ADR-089 D5). -->
{#if pickerProvider}
	<EnrichPicker
		entityName={film?.name ?? ''}
		provider={pickerProvider}
		resolve={(prov, q) => api.enrichFilmResolve(id, prov, q)}
		apply={(prov, extId) => api.enrichFilmApply(id, prov, extId)}
		dismiss={(prov) => api.enrichDismiss('film', id, prov)}
		onclose={() => (pickerProvider = '')}
		onapplied={reloadDetail}
	/>
{/if}
