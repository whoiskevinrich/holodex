<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/stores';
	import { afterNavigate, goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import type { Completeness, DecisionSource, EnrichedField, EnrichSource, ExtraMetadata, EntityRef, MappedField, MediaDetailResponse, RefreshReport, RelatedResponse, ResolvedField, Studio, Video } from '$lib/types';
	import {
		formatBitrate,
		formatBytes,
		formatDuration,
		formatYear,
		providerFromWinningSource,
		resolutionBucket,
		toMessage,
		videoCount
	} from '$lib/format';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { isReplaceField, outOfSyncCount } from '$lib/f36';
	import RelatedShelf from '$lib/components/video/RelatedShelf.svelte';
	import UrlValueList from '$lib/components/curation/UrlValueList.svelte';
	import AutoFieldRows from '$lib/components/curation/AutoFieldRows.svelte';
	import PromotedFieldEdit from '$lib/components/curation/PromotedFieldEdit.svelte';
	import PersonPoster from '$lib/components/person/PersonPoster.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import EnrichPicker from '$lib/components/enrichment/EnrichPicker.svelte';
	import EnrichProviderChips from '$lib/components/enrichment/EnrichProviderChips.svelte';
	import ProvenanceBadge from '$lib/components/enrichment/ProvenanceBadge.svelte';
	import WritebackFormDialog from '$lib/components/writeback/WritebackFormDialog.svelte';
	import CurationFieldRow from '$lib/components/curation/CurationFieldRow.svelte';
	import SourceSelect from '$lib/components/curation/SourceSelect.svelte';
	import SourceBadge from '$lib/components/curation/SourceBadge.svelte';
	import CompletenessPanel from '$lib/components/completeness/CompletenessPanel.svelte';

	let video = $state<Video | null>(null);
	let extra = $state<ExtraMetadata[]>([]);
	let fields = $state<MappedField[]>([]);
	let resolved = $state<ResolvedField[]>([]);
	let enriched = $state<EnrichedField[]>([]);
	// Studio entities linked to this video (F38): the resolved studio value links to its
	// /studios/{id} page; the link always matches the displayed value (RD1).
	let studios = $state<Studio[]>([]);
	let completeness = $state<Completeness | null>(null); // F55.13, owner-gated
	let related = $state<RelatedResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let playFailed = $state(false);
	let showRaw = $state(false);
	// Per-provider raw-enrichment disclosures (HOLODEX-119): one open/closed flag per
	// provider name, since a video can now carry enrichment from several providers.
	let openEnriched = $state<Record<string, boolean>>({});
	let regenerating = $state(false);
	let thumbVersion = $state(0); // cache-bust the preview after a regenerate
	// Poster upload (F52, HOLODEX-252): a new highest-precedence tier on the same
	// thumbnail pipeline regenerate/thumbVersion already drive.
	let posterUploading = $state(false);
	let posterError = $state('');
	let posterInput = $state<HTMLInputElement | null>(null);

	// Owner-only delete (F24, ADR-037). confirmMode drives which confirm dialog shows.
	let confirmMode = $state<'soft' | 'purge' | null>(null);
	let deleteBusy = $state(false);
	let deleteError = $state('');
	// Whether this page was reached via in-app navigation vs. a fresh/direct load — decides
	// whether Delete can pop browser history back to the referring list (HOLODEX-41).
	let cameFromInApp = $state(false);
	afterNavigate(({ type }) => {
		cameFromInApp = type !== 'enter';
	});

	// Film enrichment (F26). sources loaded once; picker drives resolve→apply.
	// pickerProvider holds the provider whose EnrichPicker is open ('' = closed);
	// enrichBusy holds the provider name currently being cleared or refreshed
	// (HOLODEX-119, F47 RD7). enrichRefreshingAll is Refresh-all's own busy flag (F47
	// RD8, distinct from the unrelated file-refresh `refreshing` below).
	let sources = $state<EnrichSource[]>([]);
	let pickerProvider = $state('');
	let enrichBusy = $state('');
	let enrichRefreshingAll = $state(false);
	let enrichError = $state('');
	// enrichQueries holds the server-rendered per-provider search query (F54,
	// ADR-080 D5) — seeds EnrichPicker's search box in place of the raw title.
	let enrichQueries = $state<Record<string, string>>({});

	// Metadata writeback (F28, ADR-041). writebackOpen drives the batch form dialog.
	let writebackOpen = $state(false);

	// Per-item metadata refresh (F31, ADR-047). refreshing drives the spinner/disable;
	// refreshStatus is the inline aria-live outcome line (no toast system).
	let refreshing = $state(false);
	let refreshStatus = $state<{ tone: 'muted' | 'warn'; text: string } | null>(null);

	// Video↔tag attach/detach (F50, ADR-075 P0-8) — the owner-only add/remove chips.
	// tagAddOpen reveals the add-tag input (a UI-only toggle, not a mutation).
	// tagJustAdded remembers the tag the last successful add resolved to, so "Use
	// existing" (below) knows which link to drop when swapping onto the near-miss.
	let tagAddOpen = $state(false);
	let tagAddValue = $state('');
	let tagInput = $state<HTMLInputElement | null>(null);
	let tagBusy = $state(false);
	let tagError = $state('');
	let tagNearMiss = $state<EntityRef | null>(null);
	let tagJustAdded = $state<EntityRef | null>(null);

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	// Prefer the resolved title (may come from an enrichment provider) over the
	// filename-derived video.title. Falls back gracefully when no mapping is configured.
	const displayTitle = $derived(
		resolved.find((f) => f.canonical === 'title')?.values[0] ?? video?.title ?? ''
	);
	// F39 (ADR-056): split the curatable canonical/mapped fields from the display-only
	// auto-registered non-canonical fields, which render read-only after them.
	// Studio (F52) and Commentary render in their own header-adjacent spots instead of
	// the generic metadata dl — excluded here so they don't also render there.
	const canonicalResolved = $derived(
		resolved.filter((f) => !f.auto_registered && f.canonical !== 'studio' && f.canonical !== 'commentary')
	);
	// Visitor view only: a field whose winner is the file/tag baseline just restates
	// what's already visible elsewhere on the page (title in the header, genres — a
	// tag-materialized field, ADR-013 — in the Tags section) — hide it there and keep
	// the Metadata section for genuine enrichment/manual content. providerFromWinningSource
	// already excludes file/record/manual/computed; "tag" is the video's own Tag rows,
	// excluded here rather than there since it still needs a provenance badge elsewhere.
	// Owner view keeps everything (it's what they curate).
	const visibleResolved = $derived(
		isOwner
			? canonicalResolved
			: canonicalResolved.filter(
					(f) => (f.winning_source ?? '').split(':')[0] !== 'tag' && providerFromWinningSource(f.winning_source)
				)
	);
	const studioField = $derived(resolved.find((f) => f.canonical === 'studio'));
	const commentaryField = $derived(resolved.find((f) => f.canonical === 'commentary'));
	const extraFields = $derived(resolved.filter((f) => f.auto_registered && f.values.length > 0));
	// Gates the whole Metadata section for visitors: nothing to show there once
	// title/genres-style baseline duplicates are filtered out of visibleResolved.
	const hasVisibleMetadata = $derived(isOwner || visibleResolved.length > 0 || extraFields.length > 0);
	// HOLODEX-119: every video-capable provider gets its own match/enrich/clear
	// affordance (the backend is already per-provider — entity_enrichment keyed by
	// provider). Was collapsed to the first capable provider, so a second matched
	// provider could never be enriched or cleared from the UI.
	const videoProviders = $derived(
		sources.filter((s) => s.entity_types.includes('video')).map((s) => s.name)
	);
	// Stored enrichment grouped by provider — drives the per-provider Clear button
	// (has(p)) and the per-provider raw disclosures at the foot of the page.
	const enrichedByProvider = $derived.by(() => {
		const m = new Map<string, EnrichedField[]>();
		for (const f of enriched) {
			const arr = m.get(f.provider);
			if (arr) arr.push(f);
			else m.set(f.provider, [f]);
		}
		return m;
	});
	// F36: "Write decisions to file" is available whenever the owner has resolved fields — a
	// decided file value is just as writable as an adopted provider value (RD5/P0-4). The count
	// of out-of-sync fields rides alongside it (RD2).
	const canWriteback = $derived(isOwner && resolved.length > 0);
	const outOfSyncN = $derived(outOfSyncCount(resolved));

	const graceDays = $derived(
		activity.caps?.delete_grace_period_seconds
			? Math.round(activity.caps.delete_grace_period_seconds / 86400)
			: 0
	);

	function openConfirm(mode: 'soft' | 'purge') {
		deleteError = '';
		confirmMode = mode;
	}

	async function confirmDelete() {
		if (!video || deleteBusy) return;
		deleteBusy = true;
		deleteError = '';
		try {
			await api.deleteMedia(video.id, { purge: confirmMode === 'purge' });
			// The item is gone — return to wherever it was opened from (a filtered list, a
			// person's filmography, a studio page) rather than always resetting to the
			// unfiltered browse root (HOLODEX-41). Falls back to '/' when there's no in-app
			// history to pop (direct link / new tab).
			if (cameFromInApp) {
				history.back();
			} else {
				goto('/');
			}
		} catch (e) {
			deleteError = toMessage(e);
			deleteBusy = false; // keep the dialog open so the message is visible
		}
	}

	async function regenerateThumbnail() {
		if (!video) return;
		regenerating = true;
		try {
			const status = await api.regenerateThumbnail(video.id);
			if (status === 200) {
				thumbVersion += 1; // embedded art extracted synchronously; bust now
			} else {
				setTimeout(() => (thumbVersion += 1), 4000); // queued; give the worker a moment
			}
		} catch (e) {
			// Generation may be disabled (503) or the request may fail; non-fatal.
			console.warn('thumbnail regenerate failed', e);
		} finally {
			regenerating = false;
		}
	}

	// Poster upload (F52, HOLODEX-252): a new highest-precedence tier on the same
	// thumbnail pipeline regenerateThumbnail already drives, so it shares its
	// cache-bust mechanism (thumbVersion).
	function triggerPosterUpload() {
		posterInput?.click();
	}
	async function runPosterAction(action: () => Promise<unknown>) {
		if (!video || posterUploading) return;
		posterUploading = true;
		posterError = '';
		try {
			await action();
			thumbVersion += 1;
			await reloadDetail();
		} catch (err) {
			posterError = toMessage(err);
		} finally {
			posterUploading = false;
		}
	}
	function onPosterFileChosen(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		(e.target as HTMLInputElement).value = ''; // allow re-choosing the same file
		if (!file) return;
		const vid = video;
		if (!vid) return;
		runPosterAction(() => api.uploadVideoPoster(vid.id, file));
	}
	function removePoster() {
		const vid = video;
		if (!vid) return;
		runPosterAction(() => api.deleteVideoPoster(vid.id));
	}

	// Apply a freshly-fetched media detail to the page state (shared by the initial
	// load, post-enrich, and post-refresh refetches).
	function applyMediaDetail(res: MediaDetailResponse) {
		video = res.video;
		extra = res.metadata ?? [];
		fields = res.fields ?? [];
		resolved = res.resolved ?? [];
		enriched = res.enriched ?? [];
		studios = res.studios ?? [];
		enrichQueries = res.enrich_queries ?? {};
		completeness = res.completeness ?? null;
	}

	// F36: persist a per-field source decision then refetch so resolved[] reflects it. DB-only
	// (RD5) — no file write here; the file changes only via "Write decisions to file". Selecting
	// Keep file clears the decision (reverts to the file default), so it maps to DELETE.
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		if (source === 'file') {
			await api.clearFieldDecision(id, canonical);
		} else {
			await api.setFieldDecision(id, canonical, {
				source,
				...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
			});
		}
		await reloadDetail();
	}

	// Refresh metadata (F31): force re-extract the file + re-enrich linked providers,
	// then refetch the detail so resolved[] reflects the new data and bust the cover.
	async function refreshMetadata() {
		if (!video || refreshing) return;
		refreshing = true;
		refreshStatus = null;
		try {
			const report = await api.refreshMedia(id);
			applyMediaDetail(await api.getMedia(id));
			thumbVersion += 1; // a re-extract may surface new embedded cover art
			refreshStatus = summarizeRefresh(report);
		} catch (e) {
			refreshStatus = { tone: 'warn', text: toMessage(e) };
		} finally {
			refreshing = false;
		}
	}

	// Build the inline status line from the report. A failed source reads as a warn
	// line (the file still updated); otherwise muted "synced" / "already in sync".
	function summarizeRefresh(r: RefreshReport): { tone: 'muted' | 'warn'; text: string } {
		const failed = r.sources.filter((s) => !s.ok).map((s) => s.source);
		if (failed.length) {
			return { tone: 'warn', text: `${failed.join(', ')} lookup failed — file metadata still updated` };
		}
		if (!r.changed) {
			return { tone: 'muted', text: 'Already in sync — nothing changed' };
		}
		const providers = r.sources.filter((s) => s.source !== 'file' && s.ok).map((s) => s.source);
		return {
			tone: 'muted',
			text: providers.length ? `Synced from file and ${providers.join(', ')}` : 'Synced from file'
		};
	}

	// Hide the full-viewport atmosphere overlay (.app-atmosphere::after, z-40) while a
	// video plays so the scan/vignette flourishes don't sit on top of the picture —
	// worst in Broadcast. Pure-CSS-gated: we only toggle the class; app.css owns the rule.
	function setPlaying(on: boolean) {
		document.body?.classList.toggle('is-playing', on);
	}
	// Restore the atmosphere on teardown (navigate away mid-play). The data-load effect
	// below also resets it per id — this route's component is REUSED across /media/[id]
	// param changes, so a play state from the previous item would otherwise linger.
	$effect(() => () => setPlaying(false));

	$effect(() => {
		const current = id;
		let cancelled = false; // ignore a stale response if id changes before it resolves
		loading = true;
		error = '';
		playFailed = false;
		refreshStatus = null; // a freshly-opened item starts with no refresh outcome
		setPlaying(false); // a freshly-opened item starts with the atmosphere visible
		api
			.getMedia(current)
			.then((res) => {
				if (cancelled) return;
				applyMediaDetail(res);
			})
			.catch((e) => {
				if (!cancelled) error = toMessage(e);
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => (cancelled = true);
	});

	// Load providers once the client is confirmed owner.
	$effect(() => {
		if (isOwner && sources.length === 0) {
			api
				.enrichSources()
				.then((res) => (sources = res.sources ?? []))
				.catch(() => {});
		}
	});

	// reloadDetail re-fetches the detail so resolved[] reflects new enrichment or
	// curation (the resolver re-runs server-side on each GET). Non-fatal on error.
	async function reloadDetail() {
		try {
			applyMediaDetail(await api.getMedia(id));
		} catch {
			// Non-fatal — caller's optimistic state stands.
		}
	}

	// Video↔tag attach/detach (F50, ADR-075 P0-8/P0-7).

	function resetTagForm() {
		tagAddValue = '';
		tagError = '';
		tagNearMiss = null;
		tagJustAdded = null;
	}

	async function openTagAdd() {
		resetTagForm();
		tagAddOpen = true;
		await tick();
		tagInput?.focus();
	}

	function closeTagAdd() {
		resetTagForm();
		tagAddOpen = false;
	}

	// Shared busy/error/finally scaffolding for the three tag mutations below.
	// formatError lets a caller (submitTagAdd) turn a specific status into its own
	// copy; everything else falls back to the page's usual toMessage(err).
	async function runTagAction(fn: () => Promise<void>, formatError?: (err: unknown) => string) {
		if (tagBusy) return;
		tagBusy = true;
		tagError = '';
		try {
			await fn();
		} catch (err) {
			tagError = formatError ? formatError(err) : toMessage(err);
		} finally {
			tagBusy = false;
		}
	}

	function submitTagAdd(e: SubmitEvent) {
		e.preventDefault();
		const name = tagAddValue.trim();
		if (!name) return;
		runTagAction(
			async () => {
				const { tag } = await api.addVideoTag(id, name);
				tagJustAdded = { id: tag.id, name: tag.name };
				// Fire-and-forget: the detail refetch and the near-miss check are
				// independent, so don't serialize them (mirrors /tags' reload()-then-
				// nearMiss() concurrency).
				void reloadDetail();
				// Non-blocking near-miss (mirrors /tags' actionNearMiss, F43 P1-5): advisory,
				// shown after the attach already succeeded.
				const nm = await api.nearMiss('tag', tag.id, name).then((r) => r.near_miss);
				if (nm) {
					tagNearMiss = nm;
				} else {
					closeTagAdd();
				}
			},
			(err) =>
				err instanceof ApiError && err.status === 422
					? `'${name}' is on the deny-list.`
					: toMessage(err)
		);
	}

	// "Use existing": swap this video's just-added tag for the near-miss it looks
	// like — attach-by-name resolves the near-miss's exact name to its existing id
	// (no new row), then detach the tag the add just created/resolved. Sequenced,
	// not concurrent: if the add fails, the just-added tag is left alone (no
	// data loss); only once the add has actually succeeded does the swap remove
	// it, so a failure here leaves both tags attached instead of neither.
	async function useTagNearMiss() {
		if (!tagNearMiss || !tagJustAdded) return;
		const nearMissName = tagNearMiss.name;
		const justAddedId = tagJustAdded.id;
		await runTagAction(async () => {
			await api.addVideoTag(id, nearMissName);
			await api.removeVideoTag(id, justAddedId);
			await reloadDetail();
			closeTagAdd();
		});
	}

	function removeTag(tagId: number) {
		runTagAction(async () => {
			await api.removeVideoTag(id, tagId);
			await reloadDetail();
		});
	}

	async function onApplied(f: EnrichedField[]) {
		enriched = f;
		await reloadDetail();
	}

	async function clearProvider(p: string) {
		enrichBusy = p;
		enrichError = '';
		try {
			await api.enrichVideoClear(id, p);
			// Refetch so the resolved chips drop this provider's candidates too, not just
			// the raw disclosure — clearing removes it as an adoptable source (F36).
			await reloadDetail();
		} catch (e) {
			enrichError = toMessage(e);
		} finally {
			enrichBusy = '';
		}
	}

	// "Refresh" (RD7/P0-5) and "Refresh all" (RD8/P1-2) — shared with the person/studio
	// detail pages via $lib/enrichRefresh; only the busy/error state and reload differ.
	async function refreshProvider(p: string) {
		await runEnrichRefresh(
			'video',
			id,
			p,
			(v) => (enrichBusy = v),
			(v) => (enrichError = v),
			reloadDetail
		);
	}

	async function refreshAllProviders() {
		await runEnrichRefreshAll(
			'video',
			id,
			(v) => (enrichRefreshingAll = v),
			(v) => (enrichError = v),
			reloadDetail,
			(p) => (pickerProvider = p)
		);
	}

	// Related "More with …" shelves (QW2/QW3). Non-blocking and tracks ONLY `id`, so it
	// fetches once per page view and the shelves don't reshuffle on incidental re-renders
	// (skin switch, thumbnail regenerate) — "stable per page view" (ADR-031). A fresh
	// item id draws anew; an error just omits the shelves. The cancel guard prevents a
	// slow response for a previous item from landing on the current one.
	$effect(() => {
		const current = id;
		let cancelled = false;
		related = null;
		api
			.related(current)
			.then((res) => {
				if (!cancelled) related = res;
			})
			.catch(() => {
				if (!cancelled) related = null;
			});
		return () => (cancelled = true);
	});
</script>

{#if loading}
	<p class="py-16 text-center text-sm text-muted">Loading…</p>
{:else if error || !video}
	<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">
		{error || 'Not found.'}
	</p>
{:else}
	<article class="mx-auto max-w-4xl space-y-6">
		<a href="/" class="text-sm text-muted hover:text-ink">← Back to library</a>

		<div
			class="group relative overflow-hidden rounded-theme border border-rule bg-black"
			id="field-poster_url-upload"
		>
			{#if playFailed}
				<div class="flex aspect-video flex-col items-center justify-center gap-3 bg-surface text-center">
					<p class="text-sm text-muted">This browser can't decode this file's codec.</p>
					<a href={api.streamURL(video.id)} download class="rounded-theme bg-accent px-4 py-2 text-sm font-medium text-accent-ink">
						Download / open file
					</a>
				</div>
			{:else}
				<!-- svelte-ignore a11y_media_has_caption -->
				<!-- The larger poster tier (F53/HOLODEX-253) is the player's poster, so
				     it shows a sharp cover instead of a black box until play — the small
				     list thumbnail (VideoCard) is a separate, unaffected derivative. -->
				<video
					src={api.streamURL(video.id)}
					poster={video.poster_url
						? api.thumbnailReload(video.poster_url, thumbVersion)
						: undefined}
					controls
					preload="metadata"
					class="aspect-video w-full bg-black"
					onplay={() => setPlaying(true)}
					onpause={() => setPlaying(false)}
					onended={() => setPlaying(false)}
					onerror={() => (playFailed = true)}
				></video>
				{#if isOwner}
					<input
						bind:this={posterInput}
						type="file"
						accept="image/*"
						class="sr-only"
						onchange={onPosterFileChosen}
					/>
					<button
						onclick={triggerPosterUpload}
						disabled={posterUploading}
						title="Upload poster"
						aria-label="Upload poster"
						class="absolute right-16 top-2 z-10 rounded-theme bg-black/60 p-1.5 text-muted opacity-0 transition hover:text-ink focus-visible:opacity-100 group-hover:opacity-100"
					>
						<svg
							class="h-4 w-4 {posterUploading ? 'animate-spin' : ''}"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							viewBox="0 0 24 24"
							aria-hidden="true"
						>
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 16V4m0 0L7 9m5-5l5 5M5 20h14" />
						</svg>
					</button>
					{#if video.poster_uploaded}
						<button
							onclick={removePoster}
							disabled={posterUploading}
							title="Remove uploaded poster"
							aria-label="Remove uploaded poster"
							class="absolute right-9 top-2 z-10 rounded-theme bg-black/60 p-1.5 text-muted opacity-0 transition hover:text-ink focus-visible:opacity-100 group-hover:opacity-100"
						>
							<svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" aria-hidden="true">
								<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
							</svg>
						</button>
					{/if}
				{/if}
				<button
					onclick={regenerateThumbnail}
					disabled={regenerating}
					title="Regenerate thumbnail from file"
					aria-label="Regenerate thumbnail from file"
					class="absolute right-2 top-2 z-10 rounded-theme bg-black/60 p-1.5 text-muted opacity-0 transition hover:text-ink focus-visible:opacity-100 group-hover:opacity-100"
				>
					<svg
						class="h-4 w-4 {regenerating ? 'animate-spin' : ''}"
						fill="currentColor"
						viewBox="0 0 24 24"
						aria-hidden="true"
					>
						<path
							d="M17.65 6.35A7.96 7.96 0 0 0 12 4a8 8 0 1 0 7.74 10h-2.08A6 6 0 1 1 12 6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"
						/>
					</svg>
				</button>
			{/if}
		</div>
		{#if posterError}
			<p class="text-xs text-warn" aria-live="polite">{posterError}</p>
		{/if}

		<header class="space-y-2">
			<h1 class="skin-title text-2xl font-semibold text-ink">{displayTitle}</h1>
			{#if isOwner || studioField?.values?.length}
				<div class="flex flex-wrap items-center gap-2 text-sm" id="field-studio">
					{#if isOwner && studioField}
						<SourceSelect field={studioField} decide={(s, mv) => decideField('studio', s, mv)} />
						{#each studios as s (s.id)}
							<a href={`/studios/${s.id}`} class="text-muted hover:text-accent">→ {s.name}</a>
						{/each}
					{:else if studios.length}
						<!-- Visitor view: the resolved studio value always matches its linked
						     entity (RD1), so show the link alone instead of the text + arrow-link
						     duplicate (owner view keeps both — the arrow-link there is a shortcut
						     to the studio page distinct from the editable SourceSelect value). -->
						{#each studios as s, i (s.id)}
							{#if i > 0}<span class="text-muted">,</span>{/if}
							<a href={`/studios/${s.id}`} class="text-ink hover:text-accent">{s.name}</a>
						{/each}
					{:else if studioField?.values?.length}
						<span class="text-ink">{studioField.values[0]}</span>
					{/if}
				</div>
			{/if}
			<div class="flex flex-wrap items-center gap-2 text-sm text-muted">
				<span class="rounded-theme bg-accent px-2 py-0.5 text-accent-ink">{resolutionBucket(video.width)}</span>
				<span>{video.width}×{video.height}</span>
				<span>·</span>
				<span>{formatDuration(video.duration_sec)}</span>
				{#if formatYear(video.recorded_at)}
					<span>·</span><span>{formatYear(video.recorded_at)}</span>
				{/if}
			</div>
		</header>

		{#if isOwner || commentaryField?.values?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Commentary</h2>
				{#if isOwner && commentaryField}
					<!-- Tier-2 replace field (F56): SourceBadge, not SourceSelect — Commentary
					     has its own section but isn't in the Video Tier-1 set (Title/People/
					     Studio/Tags, per spec §Non-Goals / design handoff Overview). -->
					<SourceBadge field={commentaryField} decide={(s, mv) => decideField('commentary', s, mv)} />
				{:else if commentaryField?.values?.length}
					<p class="leading-relaxed text-ink">{commentaryField.values[0]}</p>
				{/if}
			</section>
		{/if}

		{#if isOwner || video.tags?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
				<div class="flex flex-wrap items-center gap-2">
					{#each video.tags ?? [] as t (t.id)}
						{#if isOwner}
							<!-- Editable chip (P0-8): reuses CurationChip's pill + hover-reveal
							     remove + ·provenance suffix idiom (curation-chip/curation-actions
							     from app.css), adapted for a Tag rather than a ResolvedValue. -->
							<span
								class="curation-chip group relative inline-flex items-center gap-1 rounded-full border border-rule bg-surface-2 px-2.5 py-1 text-sm text-ink"
							>
								<a href={`/tags/${t.id}`} class="hover:text-accent focus-visible:text-accent">{t.name}</a>
								{#if t.source && t.source !== 'manual'}
									<span class="{t.source.startsWith('provider:') ? 'text-accent' : 'text-muted'} text-[0.65rem]">
										·{t.source.startsWith('provider:') ? t.source.slice('provider:'.length) : t.source}
									</span>
								{/if}
								<span class="curation-actions ml-0.5 inline-flex items-center">
									<button
										type="button"
										onclick={() => removeTag(t.id)}
										disabled={tagBusy}
										aria-label={`Remove tag ${t.name}`}
										title={t.source === 'file'
											? 'Removing a file-sourced tag may reappear on the next rescan'
											: undefined}
										class="rounded p-0.5 -m-0.5 text-muted hover:text-accent focus-visible:text-accent"
									>
										×
									</button>
								</span>
							</span>
						{:else}
							<a href={`/tags/${t.id}`} class="rounded-theme bg-surface-2 px-2.5 py-1 text-sm text-ink hover:text-accent">
								{t.name}
							</a>
						{/if}
					{/each}

					{#if isOwner}
						{#if tagAddOpen}
							<form onsubmit={submitTagAdd} class="inline-flex items-center gap-2">
								<input
									bind:this={tagInput}
									bind:value={tagAddValue}
									type="text"
									placeholder="Add a tag"
									aria-label="Add a tag"
									class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
								/>
								<button type="submit" disabled={tagBusy} class="btn-accent px-3 py-1.5 text-sm">Add</button>
								<button type="button" onclick={closeTagAdd} disabled={tagBusy} class="btn-quiet px-3 py-1.5 text-sm">
									Cancel
								</button>
							</form>
						{:else}
							<button type="button" onclick={openTagAdd} class="btn-quiet px-3 py-1.5 text-sm">+ Add tag</button>
						{/if}
					{/if}
				</div>

				{#if tagNearMiss}
					<!-- Non-blocking near-miss nudge (F43 P1-5, verbatim copy from /tags'
					     actionNearMiss card) — the attach already succeeded; this only offers
					     to consolidate onto the look-alike instead. -->
					<div class="flex flex-wrap items-center gap-2 rounded-theme border border-rule bg-surface-2 px-3 py-2">
						<p class="text-sm text-ink">
							Looks a lot like <span class="font-semibold">{tagNearMiss.name}</span>
							({videoCount(tagNearMiss.video_count ?? 0)}) — use that instead?
						</p>
						<button
							type="button"
							onclick={useTagNearMiss}
							disabled={tagBusy}
							class="btn-accent px-3 py-1.5 text-sm"
						>
							Use existing
						</button>
						<button type="button" onclick={closeTagAdd} disabled={tagBusy} class="btn-ghost px-3 py-1.5 text-sm">
							Add as new anyway
						</button>
					</div>
				{/if}
				{#if tagError}
					<p class="text-sm text-warn">{tagError}</p>
				{/if}
			</section>
		{/if}

		{#if video.people?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">People</h2>
				<!-- F25: 2:3 poster cards (placeholder when a person has no poster). -->
				<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-6">
					{#each video.people as p (p.id)}
						<li>
							<a
								href={`/people/${p.id}`}
								class="group block space-y-1.5 text-ink"
								title={p.name}
							>
								<div class="rounded-theme transition group-hover:opacity-90">
									<PersonPoster personId={p.id} name={p.name} />
								</div>
								<span class="line-clamp-2 text-xs text-muted group-hover:text-accent">{p.name}</span>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<!-- Metadata section (F27): resolved fields (merged file + enrichment) with
		     enrichment controls and writeback inline in the header. Falls back to
		     file-only fields when no resolver output is present. -->
		{#if hasVisibleMetadata}
			<section class="space-y-1.5">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<h2 class="text-xs uppercase tracking-wide text-muted">Metadata</h2>
					<div class="flex flex-wrap items-center gap-2">
						{#if isOwner}
							<button
								onclick={refreshMetadata}
								disabled={refreshing}
								title="Refresh metadata from the file and providers"
								aria-label="Refresh metadata from the file and providers"
								class="flex items-center gap-1 rounded-theme px-2 py-0.5 text-xs text-muted hover:text-accent focus-visible:text-accent"
							>
								<svg
									class="h-3.5 w-3.5 {refreshing ? 'animate-spin' : ''}"
									fill="currentColor"
									viewBox="0 0 24 24"
									aria-hidden="true"
								>
									<path
										d="M17.65 6.35A7.96 7.96 0 0 0 12 4a8 8 0 1 0 7.74 10h-2.08A6 6 0 1 1 12 6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"
									/>
								</svg>
								{refreshing ? 'Refreshing…' : 'Refresh'}
							</button>
						{/if}
						{#if isOwner}
							<!-- HOLODEX-136: compact per-provider enrich chips (icon + name +
							     Enrich), Clear in a ⋯ overflow once matched. -->
							<EnrichProviderChips
								providers={videoProviders}
								linked={(p) => enrichedByProvider.has(p)}
								busy={enrichBusy}
								refreshingAll={enrichRefreshingAll}
								size="xs"
								onenrich={(p) => (pickerProvider = p)}
								onrefresh={refreshProvider}
								onclear={clearProvider}
								onrefreshall={refreshAllProviders}
							/>
						{/if}
						{#if canWriteback}
							<button
								onclick={() => (writebackOpen = true)}
								class="flex items-center gap-1 rounded-theme px-2 py-0.5 text-xs text-muted hover:text-accent focus-visible:text-accent"
								title="Write decided field values to the file tags"
							>
								<svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v13m0 0l-4-4m4 4l4-4M5 20h14"/></svg>
								Write decisions to file{#if outOfSyncN > 0}<span class="text-warn"> · {outOfSyncN} out of sync</span>{/if}
							</button>
						{/if}
					</div>
				</div>
				{#if enrichError}
					<p class="text-xs text-warn">{enrichError}</p>
				{/if}
				{#if refreshStatus}
					<p
						class="text-xs {refreshStatus.tone === 'warn' ? 'text-warn' : 'text-muted'}"
						aria-live="polite"
					>
						{refreshStatus.text}
					</p>
				{/if}
				{#if visibleResolved.length || extraFields.length}
				<dl class="grid grid-cols-1 gap-3 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
					{#each visibleResolved as f (f.canonical)}
						{@const winnerProvider = f.winning_source && !f.winning_source.startsWith('file:') ? f.winning_source.split(':')[0] : ''}
						{#if f.display === 'image_url'}
							<div class="sm:col-span-2" id={`field-${f.canonical}`}>
								<dt class="mb-1 text-muted">{f.label}:</dt>
								<dd>
									<img
										src={f.values[0]}
										alt={f.label}
										class="max-h-64 rounded-theme border border-rule object-contain"
									/>
								</dd>
								{#if winnerProvider}<ProvenanceBadge provider={winnerProvider} label={winnerProvider} />{/if}
							</div>
						{:else if f.display === 'long_text'}
							<div class="sm:col-span-2" id={`field-${f.canonical}`}>
								<dt class="inline text-muted">{f.label}:</dt>
								<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
								{#if winnerProvider}<ProvenanceBadge provider={winnerProvider} label={winnerProvider} />{/if}
							</div>
						{:else if f.display === 'url'}
							<div id={`field-${f.canonical}`}>
								<dt class="inline text-muted">{f.label}:</dt>
								<!-- HOLODEX-137: provider icon + host in the link folds in provenance. -->
								<dd class="inline"><UrlValueList values={f.values} provider={winnerProvider} /></dd>
							</div>
						{:else}
							<!-- Curatable text/set field (F30): per-value chips with provenance,
							     edit/remove/no-write, and an add affordance for set fields. -->
							<div id={`field-${f.canonical}`}>
								<dt class="mb-1 text-muted">{f.label}:</dt>
								<dd>
									{#if isReplaceField(f) && isOwner}
										<!-- Tier-2 replace field (F56): SourceBadge — collapsed
										     ProvenanceBadge at rest, click-to-expand chip row + Confirm/
										     Cancel. Merge fields and the visitor view keep the F30
										     CurationFieldRow read-only render. -->
										<SourceBadge field={f} decide={(s, mv) => decideField(f.canonical, s, mv)} />
									{:else}
										<CurationFieldRow
											field={f}
											videoId={id}
											{isOwner}
											people={video.people ?? []}
											personStyle={f.canonical === 'actors' || f.canonical === 'director'}
											onchanged={reloadDetail}
										/>
									{/if}
								</dd>
							</div>
						{/if}
						<PromotedFieldEdit {isOwner} field={f} entityType="video" entityNoun="videos" onchanged={reloadDetail} />
					{/each}

					<!-- F39 (ADR-056): display-only auto-registered non-canonical fields. -->
					<AutoFieldRows
						fields={extraFields}
						{isOwner}
						entityType="video"
						entityNoun="videos"
						onchanged={reloadDetail}
					/>
				</dl>
				{:else if fields.length}
				<dl class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
					{#each fields as f (f.canonical)}
						<div>
							<dt class="inline text-muted">{f.label}:</dt>
							<dd class="inline">{f.values.join(', ')}</dd>
						</div>
					{/each}
				</dl>
				{:else}
				<p class="rounded-theme border border-rule bg-surface px-4 py-3 text-sm text-muted">
					No metadata extracted yet.
				</p>
				{/if}
			</section>
		{/if}

		{#if isOwner && completeness}
			{#each completeness.facets as cf (cf.canonical)}
				{#if cf.tier === 'missing' && !visibleResolved.some((f) => f.canonical === cf.canonical) && cf.canonical !== 'studio'}
					<div id={`field-${cf.canonical}`} class="hidden" aria-hidden="true"></div>
				{/if}
			{/each}
		{/if}

		{#if isOwner}
			<CompletenessPanel {completeness} videoId={id} onchanged={reloadDetail} />
		{/if}

		{#if isOwner}
		<section class="space-y-1.5">
			<h2 class="text-xs uppercase tracking-wide text-muted">File</h2>
			<div class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
			<div><span class="text-muted">File size:</span> {formatBytes(video.file_size)}</div>
			{#if video.container}<div><span class="text-muted">Container:</span> {video.container}</div>{/if}
			{#if video.video_codec}<div><span class="text-muted">Video codec:</span> {video.video_codec}</div>{/if}
			{#if video.audio_codec}<div><span class="text-muted">Audio codec:</span> {video.audio_codec}</div>{/if}
			{#if video.bitrate_kbps}
				<div><span class="text-muted">Bitrate:</span> {formatBitrate(video.bitrate_kbps)}</div>
			{/if}
			<div class="truncate sm:col-span-2" title={video.file_path}>
				<span class="text-muted">Path:</span> {video.file_path}
			</div>
		</div>
		</section>
		{/if}

		<!-- "More with …" shelves (QW3): person first, then tag. Each self-omits when
		     its block is null or empty, so an item with no siblings shows no rail. -->
		{#if related?.person}
			<RelatedShelf
				title={related.person.name}
				href={`/people/${related.person.id}`}
				items={related.person.items}
			/>
		{/if}
		{#if related?.tag}
			<RelatedShelf title={related.tag.name} href={`/tags/${related.tag.id}`} items={related.tag.items} />
		{/if}

		<!-- Owner-only Manage block (F24): destructive actions, kept apart from the
		     content and the Back link so a delete is never adjacent to navigation.
		     Effective gate (F29) so visitor view hides it. -->
		{#if isOwner}
			<section class="space-y-2 border-t border-rule pt-4">
				<h2 class="text-xs uppercase tracking-wide text-muted">Manage</h2>
				<div class="flex flex-wrap gap-2">
					<button
						onclick={() => openConfirm('soft')}
						class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
					>
						Move to Trash
					</button>
					<button
						onclick={() => openConfirm('purge')}
						class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
					>
						Delete permanently
					</button>
				</div>
			</section>
		{/if}

		<!-- Admin-only metadata sources (F29): the raw file-extracted payload and the raw
		     provider enrichment payload, kept as audit/debug disclosures at the bottom of
		     the page (below Manage). Owner + Admin mode only (effectiveOwner); each
		     self-omits when empty. Headings aligned to "Enrichment data: {source}". -->
		{#if isOwner && extra.length}
			<section>
				<button onclick={() => (showRaw = !showRaw)} class="text-sm text-muted hover:text-ink">
					{showRaw ? '▾' : '▸'} Enrichment data: File Extraction ({extra.length})
				</button>
				{#if showRaw}
					<table class="mt-2 w-full text-left text-xs">
						<tbody>
							{#each extra as m (m.source_key)}
								<tr class="border-b border-rule">
									<td class="py-1 pr-4 font-mono text-muted">{m.source_key}</td>
									<td class="py-1 text-ink">{m.value}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</section>
		{/if}

		{#if isOwner && enriched.length}
			<!-- One raw-payload disclosure per provider (HOLODEX-119): a video may carry
			     enrichment from several providers, each its own audit/debug block. -->
			{#each [...enrichedByProvider] as [p, fields] (p)}
				<section>
					<button onclick={() => (openEnriched[p] = !openEnriched[p])} class="text-sm text-muted hover:text-ink">
						{openEnriched[p] ? '▾' : '▸'} Enrichment data: {p} ({fields.length})
					</button>
					{#if openEnriched[p]}
						<dl class="mt-2 grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
							{#each fields as f (f.canonical + f.provider)}
								{#if f.display === 'image_url'}
									<div class="sm:col-span-2">
										<dt class="mb-1 text-muted">{f.label}:</dt>
										<dd>
											<img
												src={f.values[0]}
												alt={f.label}
												class="max-h-64 rounded-theme border border-rule object-contain"
											/>
										</dd>
									</div>
								{:else}
									<div>
										<dt class="inline text-muted">{f.label}:</dt>
										<dd class="inline text-ink">{f.display === 'long_text' ? f.values[0].slice(0, 120) + '…' : f.values.join(', ')}</dd>
									</div>
								{/if}
							{/each}
						</dl>
					{/if}
				</section>
			{/each}
		{/if}
	</article>

	{#if writebackOpen && video}
		<WritebackFormDialog
			fields={resolved}
			videoId={id}
			filePath={video.file_path}
			writeback={api.writebackMedia}
			jobStatus={api.writebackJobStatus}
			onclose={() => (writebackOpen = false)}
			onapplied={async () => {
				// The dialog reports applied only once the queued write has landed and the
				// server has re-read the file, so resolved[] now recomputes in_sync against
				// the post-write baseline — the header count and the per-field warn pills
				// clear here rather than persisting until a rescan (ADR-073). Same reason
				// decideField/clearProvider/onApplied reload.
				thumbVersion += 1;
				await reloadDetail();
			}}
		/>
	{/if}

	{#if pickerProvider && video}
		<EnrichPicker
			entityName={enrichQueries[pickerProvider] ?? displayTitle}
			provider={pickerProvider}
			resolve={(prov, q) => api.enrichVideoResolve(id, prov, q)}
			apply={(prov, extId) => api.enrichVideoApply(id, prov, extId)}
			dismiss={(prov) => api.enrichDismiss('video', id, prov)}
			onclose={() => (pickerProvider = '')}
			onapplied={onApplied}
		/>
	{/if}

	{#if confirmMode === 'soft'}
		<ConfirmDialog
			title="Move to Trash?"
			confirmLabel="Move to Trash"
			busy={deleteBusy}
			error={deleteError}
			onconfirm={confirmDelete}
			oncancel={() => (confirmMode = null)}
		>
			{#snippet body()}
				<p>
					<span class="font-semibold">{video?.title}</span> will be hidden from your library{graceDays
						? ` and permanently deleted in ${graceDays} ${graceDays === 1 ? 'day' : 'days'}`
						: ''}. You can restore it from Trash{graceDays ? ' until then' : ''}.
				</p>
			{/snippet}
		</ConfirmDialog>
	{:else if confirmMode === 'purge'}
		<ConfirmDialog
			title="Delete permanently?"
			confirmLabel="Delete permanently"
			busy={deleteBusy}
			error={deleteError}
			onconfirm={confirmDelete}
			oncancel={() => (confirmMode = null)}
		>
			{#snippet body()}
				<p>
					<span class="font-semibold">{video?.title}</span> and its file will be
					<span class="font-semibold">permanently removed</span> from disk. This cannot be undone.
				</p>
				<p class="truncate font-mono text-xs text-muted" title={video?.file_path}>{video?.file_path}</p>
			{/snippet}
		</ConfirmDialog>
	{/if}
{/if}
