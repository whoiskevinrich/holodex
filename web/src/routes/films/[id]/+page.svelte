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
	import PersonPoster from '$lib/components/person/PersonPoster.svelte';
	import VideoGrid from '$lib/components/video/VideoGrid.svelte';
	import WritebackFormDialog from '$lib/components/writeback/WritebackFormDialog.svelte';
	import FilmBulkAttachDialog from '$lib/components/film/FilmBulkAttachDialog.svelte';

	// Film detail (F56, design handoff §2): two hard-separated regions below the header —
	// full-film file(s) (§2b, the only place a film-page writeback button appears) and the
	// scenes list (§2c) — never merged (RD4). Cast/tags/studios are the read-only union of
	// the film's videos (RD2/RD3), not editable chips, so they route through plain links,
	// not SourceSelect. The Details section (description/release_date) mirrors Studio's
	// SourceBadge/`baselineKey='record'` pattern exactly — films have no rename/aliases/
	// enrichment providers wired yet (HOLODEX-280/281 deferred).
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
</script>

<AsyncState {loading} {error}>
	{#if film}
		<section class="space-y-6">
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

					{#if studios.length}
						<div class="flex flex-wrap items-center gap-1.5 pt-1">
							<span class="text-xs uppercase tracking-wide text-muted">Studios</span>
							{#each studios as s (s.id)}
								<a
									href={`/studios/${s.id}`}
									class="rounded-full border border-rule px-2.5 py-0.5 text-sm text-ink hover:text-accent"
									>{s.name}</a
								>
							{/each}
						</div>
					{/if}
					{#if tags.length}
						<div class="flex flex-wrap items-center gap-1.5">
							<span class="text-xs uppercase tracking-wide text-muted">Tags</span>
							{#each tags as t (t.id)}
								<a
									href={`/tags/${t.id}`}
									class="rounded-full border border-rule px-2.5 py-0.5 text-sm text-ink hover:text-accent"
									>{t.name}</a
								>
							{/each}
						</div>
					{/if}

					{#each replaceFields.filter((f) => f.canonical === 'description') as f (f.canonical)}
						{#if f.values[0]?.trim()}
							<p class="leading-relaxed text-ink">{f.values[0]}</p>
						{/if}
					{/each}
				</div>
			</div>

			<!-- Cast (design handoff §2a): read-only union of the film's scenes' people, rendered as
			     the same PersonPoster grid as the Media detail page's People section (not inline
			     pills) — derived display, not an editable/attachable relationship. -->
			{#if cast.length}
				<section class="space-y-1.5">
					<h2 class="text-xs uppercase tracking-wide text-muted">Cast</h2>
					<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-6">
						{#each cast as p (p.id)}
							<li>
								<a href={`/people/${p.id}`} class="block space-y-1.5 text-ink" title={p.name}>
									<div class="rounded-theme transition hover:opacity-90">
										<PersonPoster personId={p.id} name={p.name} />
									</div>
									<span class="line-clamp-2 text-xs text-muted hover:text-accent">{p.name}</span>
								</a>
							</li>
						{/each}
					</ul>
				</section>
			{/if}

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
