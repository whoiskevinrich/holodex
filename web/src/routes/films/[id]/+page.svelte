<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage, monogram, resolutionBucket } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import type {
		DecisionSource,
		Film,
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

	// Film detail (F56, design handoff §2): two hard-separated regions below the header —
	// full-film file(s) (§2b, the only place a film-page writeback button appears) and the
	// scenes list (§2c) — never merged (RD4). Cast/tags/studios are the read-only union of
	// the film's videos (RD2/RD3), not editable chips, so they route through plain links,
	// not SourceSelect. The Details section (description/release_date) mirrors Studio's
	// SourceBadge/`baselineKey='record'` pattern exactly — films have no rename/aliases/
	// enrichment providers wired yet (HOLODEX-281 deferred). Poster/thumb images
	// (HOLODEX-280, ADR-086) mirror Studio's dedicated Images section below the
	// header, not a hero-image swap — see EntityImageSlot.
	let film = $state<Film | null>(null);
	let resolved = $state<ResolvedField[]>([]);
	let scenes = $state<FilmVideo[]>([]);
	let fullFilms = $state<FilmVideo[]>([]);
	let cast = $state<Person[]>([]);
	let tags = $state<Tag[]>([]);
	let studios = $state<Studio[]>([]);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner);

	// A studio has no `name` beyond baseline, so only fields beyond `name` gate the
	// Details section — same "hide the whole section, don't show an empty box" rule.
	const replaceFields = $derived(resolved.filter((f) => f.canonical !== 'name'));
	const hasDetails = $derived(replaceFields.length > 0);

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

	async function reloadDetail() {
		try {
			applyDetail(await api.getFilm(id));
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
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
				<div
					class="flex aspect-[2/3] w-40 shrink-0 items-center justify-center overflow-hidden rounded-theme border border-rule bg-logo-plate"
				>
					<span class="font-display text-4xl font-semibold text-logo-plate-ink" aria-hidden="true"
						>{monogram(film.name)}</span
					>
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

			<!-- Images (F56/HOLODEX-280, ADR-086): poster (search results, future header
			     use), thumb (no consumer yet) — mirrors Studio's dedicated Images section
			     exactly (F51, ADR-079). Always shown — owners can seed a poster even
			     before any enrichment; visitors see filled slots read-only. -->
			<section class="space-y-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">Images</h2>
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
					<EntityImageSlot
						entityId={id}
						entityName={film.name}
						role="poster"
						label="Poster"
						url={film.poster_url}
						{isOwner}
						upload={api.uploadFilmImage}
						remove={api.deleteFilmImage}
						onchanged={reloadDetail}
					/>
					<EntityImageSlot
						entityId={id}
						entityName={film.name}
						role="thumb"
						label="Thumb"
						url={film.thumb_url}
						{isOwner}
						upload={api.uploadFilmImage}
						remove={api.deleteFilmImage}
						onchanged={reloadDetail}
					/>
				</div>
			</section>

			<!-- Cast (design handoff §2a): read-only union of the film's scenes' people, shared
			     PeopleGrid component with the Media detail page's People section (not inline
			     pills) — no attach/detach passed, since this is derived display, not an
			     editable/attachable relationship. -->
			<PeopleGrid title="Cast" people={cast} />

			<!-- Details (description/release_date) — mirrors Studio's SourceBadge pattern. -->
			{#if hasDetails}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
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
