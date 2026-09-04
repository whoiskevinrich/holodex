<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage, resolutionBucket, providerFromWinningSource } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { providerOf } from '$lib/f36';
	import type {
		DecisionSource,
		EnrichSource,
		Film,
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
	// A withheld films.year fill (F59/ADR-089 D3). Deliberately separate from
	// actionError: the apply *succeeded*, so this is an advisory about one skipped
	// identity write, not a failed request.
	let yearCollision = $state<FilmYearCollision | null>(null);

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

	// Visitor view: when every shown field resolves from the SAME single provider, hoist
	// one "Enriched from …" note to the section header instead of repeating an identical
	// badge per row. Empty when providers differ (or none). Mirrors the studio page.
	const soleProvider = $derived.by(() => {
		const set = new Set(
			replaceFields.map((f) => providerFromWinningSource(f.winning_source)).filter(Boolean)
		);
		return set.size === 1 ? [...set][0] : '';
	});

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
			<a href="/films" class="text-sm text-muted hover:text-ink">← All films</a>

			<!-- 2a. Header -->
			<div class="flex flex-col gap-4 sm:flex-row">
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
					{#if film.year}<p class="text-sm text-muted">{film.year}</p>{/if}

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

			<!-- Details (description/release_date) — mirrors Studio's SourceBadge pattern, and
			     since F59/HOLODEX-309 its enrichment header row too. The owner keeps the
			     section when a film-capable provider exists but nothing has resolved yet,
			     or an unenriched film would offer no way to reach Enrich. -->
			{#if hasDetails || (isOwner && filmProviders.length)}
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
								onenrich={(p) => {
									yearCollision = null;
									pickerProvider = p;
								}}
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

					<!-- Withheld year fill (F59/ADR-089 D3): the enrich succeeded, so this names
					     the occupying film and links to it rather than reading as a failure. -->
					{#if yearCollision}
						<p class="text-sm text-warn">
							Kept this film's year unset — <a
								href={`/films/${yearCollision.film_id}`}
								class="underline hover:text-ink">{yearCollision.film_name} ({yearCollision.year})</a
							> already uses that name and year. Everything else from the provider was applied.
						</p>
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
		apply={async (prov, extId) => {
			const res = await api.enrichFilmApply(id, prov, extId);
			// Captured here rather than in onapplied, which only receives the fields.
			yearCollision = res.year_collision ?? null;
			return res;
		}}
		dismiss={(prov) => api.enrichDismiss('film', id, prov)}
		onclose={() => (pickerProvider = '')}
		onapplied={reloadDetail}
	/>
{/if}
