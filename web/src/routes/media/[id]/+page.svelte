<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/stores';
	import { afterNavigate, goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import type { Completeness, DecisionSource, EnrichedField, EnrichSource, ExtraMetadata, EntityRef, FilmAttachment, MappedField, MediaDetailResponse, Person, RefreshReport, RelatedResponse, ResolvedField, Studio, Video, VideoCollisionRef, VideoWritebackStatus } from '$lib/types';
	import {
		formatBitrate,
		formatBytes,
		formatDuration,
		formatYear,
		monogram,
		personKey,
		resolutionBucket,
		toMessage,
		videoCount
	} from '$lib/format';
	import { runEnrichRefresh, runEnrichRefreshAll } from '$lib/enrichRefresh';
	import { isReplaceField, outOfSyncCount } from '$lib/f36';
	import { expandedField } from '$lib/expandedField.svelte';
	import RelatedShelf from '$lib/components/video/RelatedShelf.svelte';
	import UrlValueList from '$lib/components/curation/UrlValueList.svelte';
	import AutoFieldRows from '$lib/components/curation/AutoFieldRows.svelte';
	import PromotedFieldEdit from '$lib/components/curation/PromotedFieldEdit.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import ExpandableText from '$lib/components/shared/ExpandableText.svelte';
	import EnrichPicker from '$lib/components/enrichment/EnrichPicker.svelte';
	import EnrichProviderChips from '$lib/components/enrichment/EnrichProviderChips.svelte';
	import ProvenanceBadge from '$lib/components/enrichment/ProvenanceBadge.svelte';
	import WritebackFormDialog from '$lib/components/writeback/WritebackFormDialog.svelte';
	import CurationFieldRow from '$lib/components/curation/CurationFieldRow.svelte';
	import SourceSelect from '$lib/components/curation/SourceSelect.svelte';
	import SourceBadge from '$lib/components/curation/SourceBadge.svelte';
	import CompletenessPanel from '$lib/components/completeness/CompletenessPanel.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';
	import CollisionOfferCard from '$lib/components/entity/CollisionOfferCard.svelte';
	import StudioPicker from '$lib/components/entity/StudioPicker.svelte';
	import StudioLinkCard from '$lib/components/entity/StudioLinkCard.svelte';
	import PeopleGrid from '$lib/components/entity/PeopleGrid.svelte';
	import TagLinkChip from '$lib/components/entity/TagLinkChip.svelte';
	import FilmAttachDialog from '$lib/components/film/FilmAttachDialog.svelte';
	import EditSceneNumberDialog from '$lib/components/film/EditSceneNumberDialog.svelte';
	import ExtractionQueueRow from '$lib/components/extraction/ExtractionQueueRow.svelte';
	import ExtractionPreviewDialog from '$lib/components/extraction/ExtractionPreviewDialog.svelte';
	import {
		buildPreviewItems,
		isEntityField,
		makeFieldLabel,
		sortRows,
		stagePick,
		unstagePick,
		type StagedPicks
	} from '$lib/extraction';
	import type { ExtractionQueueRow as QueueRow, ExtractionResolveAction } from '$lib/types';
	import { waitForWritebackJob, waitForVideoWriteback } from '$lib/writebackJob';

	let video = $state<Video | null>(null);
	let extra = $state<ExtraMetadata[]>([]);
	let fields = $state<MappedField[]>([]);
	let resolved = $state<ResolvedField[]>([]);
	let enriched = $state<EnrichedField[]>([]);
	// Studio entities linked to this video (F38): the resolved studio value links to its
	// /studios/{id} page; the link always matches the displayed value (RD1).
	let studios = $state<Studio[]>([]);
	// Films this video is attached to (F56, design handoff §3a) — read-only badge (scene
	// number or "Full film") + owner-only detach; asserted links, so no relink/prune ever
	// touches these regardless of films_enabled state (ADR-085).
	let films = $state<FilmAttachment[]>([]);
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
		expandedField.reset(); // no per-entity scope of its own (F56.9) — clear on nav between videos
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

	// Metadata writeback (F28, ADR-041; ADR-091/HOLODEX-323 fire-and-forget revision).
	// writebackOpen drives the confirm dialog. writebackStatus is the page-level
	// pending/failed signal near Metadata — sourced from the server (writeback_queue
	// by video_id, spec R2.1) rather than a job id this component holds, so it
	// survives reload, another tab, and a restart. The poll effect below shares
	// pageGeneration (declared further down) with the extraction feature's own
	// poll/wait — a poll started for one video must never resolve into a different
	// one's badge after in-place /media/A -> /media/B navigation, and that
	// invalidation moment is identical for every async feature on this page.
	let writebackOpen = $state(false);
	let writebackStatus = $state<VideoWritebackStatus>({ pending: false, failed: false });
	// retrying/dismissing are mutually exclusive by construction (each guards against
	// re-entry and both disable the same two buttons), so one variable suffices.
	let writebackAction = $state<'retry' | 'dismiss' | null>(null);
	let writebackActionError = $state('');

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

	// Title composite-key collision verdict (HOLODEX-270). pendingTitleValue remembers the
	// value that collided so "Save anyway" can resubmit it with override — NameEditControl
	// clears its own input state as soon as the conflict comes back, so the page must hold it.
	let pendingTitleValue = $state('');
	let titleCollisionBusy = $state(false);
	let titleCollisionError = $state('');

	// Studio composite-key collision verdict (HOLODEX-271, reusing HOLODEX-270's mechanism).
	// Unlike Title, a Studio pick isn't manual-only, so the pending source is remembered too
	// (not just the value) so "Save anyway" resubmits the exact same decision with override.
	let pendingStudioSource = $state<DecisionSource | null>(null);
	let pendingStudioValue = $state<string | undefined>(undefined);
	let studioCollisionBusy = $state(false);
	let studioCollisionError = $state('');

	// People composite-key collision verdict (HOLODEX-272, reusing HOLODEX-270/271's
	// mechanism). Attach/detach both flow through the curation model (F30/ADR-048;
	// actors/director are multi/merge fields the field-decision model structurally
	// rejects, worklog HOLODEX-272) rather than SetDecision, so the pending value is a
	// curation field+value+action triple. Shared by both call sites — the grid's own
	// remove control and PersonPicker's search/attached-list — since a detach from
	// either produces the identical curation call.
	let pendingPersonField = $state<'actors' | 'director' | null>(null);
	let pendingPersonValue = $state('');
	let pendingPersonAction = $state<'add' | 'suppress'>('add');
	let personConflict = $state<VideoCollisionRef | null>(null);
	let personCollisionBusy = $state(false);
	let personCollisionError = $state('');
	let personBusyKey = $state<string | null>(null);
	let personRemoveError = $state('');

	// Films (F56): detach busy-keyed by film_id (the other half of the film_videos PK
	// alongside this video's id), same shape as personBusyKey.
	let filmBusyKey = $state<number | null>(null);
	let filmRemoveError = $state('');
	let filmAttachOpen = $state(false);

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	// Prefer the resolved title (may come from an enrichment provider) over the
	// filename-derived video.title. Falls back gracefully when no mapping is configured.
	const displayTitle = $derived(
		resolved.find((f) => f.canonical === 'title')?.values[0] ?? video?.title ?? ''
	);
	// F39 (ADR-056): split the curatable canonical/mapped fields from the display-only
	// auto-registered non-canonical fields, which render read-only after them.
	// Canonical fields the Metadata list must NOT render, because the page already
	// surfaces each one somewhere richer. Kept as one list rather than a condition per
	// render site so "every field is shown exactly once" stays auditable in one place:
	//   studio      → StudioLinkCard/StudioPicker under the header (F52)
	//   title       → edited in place on the header <h1> (HOLODEX-269)
	//   overview    → the synopsis under the header meta line (see overviewField)
	//   poster_url  → the player's own poster + upload/remove/regenerate controls
	//   genres      → materialized into real Tag rows, curated in the Tags section
	//   actors      → the People grid (video_people), attach/detach per person
	const METADATA_ELSEWHERE = ['studio', 'title', 'overview', 'poster_url', 'genres', 'actors'];
	const canonicalResolved = $derived(
		resolved.filter((f) => !f.auto_registered && !METADATA_ELSEWHERE.includes(f.canonical))
	);
	const studioField = $derived(resolved.find((f) => f.canonical === 'studio'));
	const overviewField = $derived(resolved.find((f) => f.canonical === 'overview'));
	const extraFields = $derived(resolved.filter((f) => f.auto_registered && f.values.length > 0));
	// Metadata fold (media-detail-entity-ux): the field list collapses the same way the
	// Completeness panel does. Collapsed is the resting state, and — as there the score
	// stays visible — the field count is the summary that keeps the fold honest.
	let metadataExpanded = $state(false);
	const metadataFieldCount = $derived(
		canonicalResolved.length + extraFields.length || fields.length
	);
	// Canonicals whose `#field-<canonical>` anchor is rendered elsewhere on an owner's page,
	// so the hidden completeness fallback below must not emit a second element with the same
	// id. studio/title/genres/actors are unconditional for an owner (their containers gate on
	// `isOwner || …`). overview is not: its header block keys off `overviewField`, and a field
	// existing is NOT the same as it having a value — a decided-or-film-candidate field with
	// zero values is deliberately retained so its pin stays changeable
	// (internal/resolver/resolver.go), and that field reports tier `missing`. So it is tested,
	// not listed.
	function hasPageAnchor(canonical: string): boolean {
		if (canonical === 'overview') return !!overviewField;
		return canonical === 'studio' || canonical === 'title' || canonical === 'genres' || canonical === 'actors';
	}
	// Films + People are co-located in one row (design handoff, media-detail-reorder) —
	// each keeps its own pre-existing gate, but the row itself must contribute nothing
	// to the page when both sides are hidden.
	const filmsVisible = $derived(!!activity.caps?.films_enabled && (isOwner || films.length > 0));
	const peopleVisible = $derived(isOwner || (video?.people?.length ?? 0) > 0);
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

	// Filename extraction on this video (F48.5a/F48.6i, ADR-090 layer 1 at entity
	// scope). The trigger alone would look inert — extraction auto-apply defaults
	// off, so nearly every candidate lands as logged_only/queued and its whole
	// outcome would otherwise be on /owner/extraction. The panel below is what makes
	// the button honest.
	//
	// Deliberately NOT fetched on mount: an always-on panel for any video with
	// pending rows is the deferred option D (see the handoff's Non-goals), so the
	// panel appears only once the owner runs extraction here.
	let extracting = $state(false);
	let extractRows = $state<QueueRow[]>([]);
	// null until a run completes here; `.matched` then distinguishes "no pattern
	// matched" from "matched, nothing to review" (F48.6l). One nullable beats two
	// booleans, whose fourth state (not run, but matched) is unrepresentable anyway.
	let extractRun = $state<{ matched: boolean } | null>(null);
	let extractError = $state('');
	let extractLabels = $state<Record<string, string>>({});
	let extractStaged = $state<StagedPicks>({});
	let extractPreviewOpen = $state(false);
	let extractApplying = $state(false); // waiting on the queued write + its re-extract
	// Bumped by resetExtraction on every video change, and on unmount. Shared by every
	// feature on this page that runs an async poll or wait (extraction below, writeback
	// above) — each captures it and bails if it no longer matches, so work started on
	// one video never writes state back onto another (the component is reused across
	// /media/A -> /media/B, so unmount alone is not the boundary that matters). One
	// counter for the whole page rather than one per feature, since every feature needs
	// to invalidate at exactly the same two moments (video change, unmount).
	let pageGeneration = 0;
	let unmounted = false;
	$effect(() => () => {
		unmounted = true;
		pageGeneration += 1;
	});

	// Labels, ordering, staging and preview-item construction all come from
	// $lib/extraction — the same module the owner tab uses, so the two surfaces
	// cannot drift (ADR-090 D2).
	const extractLabel = $derived(makeFieldLabel(extractLabels));
	const extractSorted = $derived(sortRows(extractRows));
	// Counted off the preview items, not the raw staged map: a pick whose row has left
	// the queue (resolved or dismissed elsewhere, then Re-extract) is dropped by
	// buildPreviewItems, and a commit bar promising more writes than the dialog performs
	// is a lie about what the button will do.
	// Indexed once per rows change, not per staged click.
	const extractRowsById = $derived(new Map(extractRows.map((r) => [r.id, r])));
	const extractPreviewItems = $derived(buildPreviewItems(extractStaged, extractRowsById, extractLabel));
	const extractStagedCount = $derived(extractPreviewItems.length);
	// Rows are only ever assigned by a run, so a run having happened is the whole
	// condition — `extractRows.length > 0` would be an unreachable second disjunct.
	const extractPanelVisible = $derived(isOwner && extractRun !== null);

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
		films = res.films ?? [];
		enrichQueries = res.enrich_queries ?? {};
		completeness = res.completeness ?? null;
		writebackStatus = res.writeback_status ?? { pending: false, failed: false };
	}

	// Poll while a write is pending (ADR-091, HOLODEX-323, spec R2.5): reacts to
	// writebackStatus.pending going true — set by applyMediaDetail on initial load (a
	// page opened mid-write) or after the dialog's onenqueued reload (a write just
	// submitted) — and keeps checking until it clears, so the badge and the per-field
	// "file out of sync" pills clear without a manual refresh. Reuses
	// waitForVideoWriteback's backoff loop rather than a bespoke interval; extracts
	// just writeback_status from each tick's getMedia response rather than calling
	// applyMediaDetail on every poll (which would repeatedly reassign resolved/video
	// mid-poll), applying the full detail only once, on settlement.
	//
	// pollingWriteback guards against a real re-entrancy bug: applyMediaDetail
	// reassigns writebackStatus to a NEW object on every reloadDetail() call, and this
	// page has ~20 unrelated call sites for reloadDetail() (decisions, tags, enrich,
	// poster upload, completeness, ...). Since this $effect depends on writebackStatus
	// by reference, any one of those unrelated reloads re-runs the effect while a write
	// is still pending — without this guard, each re-run would start a brand-new,
	// fully independent poll loop with nothing to cancel the ones already in flight.
	let pollingWriteback = false;
	$effect(() => {
		if (!writebackStatus.pending || pollingWriteback) return;
		pollingWriteback = true;
		const gen = pageGeneration;
		const videoId = id;
		waitForVideoWriteback(
			async () => (await api.getMedia(videoId)).writeback_status ?? { pending: false, failed: false },
			{ cancelled: () => unmounted || gen !== pageGeneration }
		).then((settled) => {
			pollingWriteback = false;
			if (unmounted || gen !== pageGeneration || settled.pending) return;
			// The job landed or failed — re-resolve the full detail so in_sync
			// recomputes against the post-write baseline (ADR-073 D1) and the
			// out-of-sync pills clear alongside the badge.
			void reloadDetail();
		});
	});

	// Shared by retryWriteback/dismissWriteback below — both are guard/set-action/
	// clear-error/call/reload/catch/finally with only the action name and the API call
	// differing. optimisticStatus applies the known outcome to writebackStatus BEFORE
	// reloadDetail() runs: reloadDetail() swallows its own fetch errors by design
	// ("non-fatal, caller's optimistic state stands" — see its own comment), so without
	// this, a successful retry/dismiss whose follow-up reload hits a transient network
	// blip would leave the stale failed badge and buttons rendering as if the action
	// never happened, even though the server-side state already changed.
	async function runWritebackAction(
		action: 'retry' | 'dismiss',
		call: () => Promise<unknown>,
		optimisticStatus: VideoWritebackStatus
	) {
		if (writebackAction) return;
		writebackAction = action;
		writebackActionError = '';
		try {
			await call();
			writebackStatus = optimisticStatus;
			await reloadDetail();
		} catch (e) {
			writebackActionError = toMessage(e);
		} finally {
			writebackAction = null;
		}
	}

	// Retry (spec R3.3): resets the failed job back to pending and lets the poll
	// effect above pick it up. A safe no-op server-side when nothing is failed.
	function retryWriteback() {
		return runWritebackAction('retry', () => api.retryWriteback(id), { pending: true, failed: false });
	}

	// Dismiss (spec R3.4/RD2): deletes the failed row without retrying. job_runs
	// keeps the permanent audit record — this only clears the page-level badge.
	// Preserves `pending` rather than assuming false: a dismiss only ever touches the
	// failed row, and an unrelated write could already be in flight for this video.
	function dismissWriteback() {
		return runWritebackAction('dismiss', () => api.dismissWriteback(id), {
			pending: writebackStatus.pending,
			failed: false
		});
	}

	// F36: persist a per-field source decision then refetch so resolved[] reflects it. DB-only
	// (RD5) — no file write here; the file changes only via "Write decisions to file". Selecting
	// Keep file clears the decision (reverts to the file default), so it maps to DELETE.
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		if (source === 'file') {
			await api.clearFieldDecision(id, canonical);
		} else {
			const res = await api.setFieldDecision(id, canonical, {
				source,
				...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
			});
			// decideField has no verdict UI to hand a collision to — SourceSelect/SourceBadge and
			// WritebackFormDialog only expect ok-or-throw, so surface it as a thrown error instead
			// of silently proceeding to reloadDetail() (HOLODEX-270 review fix).
			if (res.conflict) {
				throw new Error(`"${manualValue}" already matches another video: ${res.conflict.title}`);
			}
		}
		await reloadDetail();
	}

	// Title rename (HOLODEX-269, docked-pencil NameEditControl on the header <h1>). Video
	// isn't on the identity spine (no alias/merge concept), but a manual title edit can still
	// collide on the composite key {title, people, date, studio} (HOLODEX-270) — that 409
	// resolves to {conflict} the same way a Person/Studio/Tag rename does, rendering
	// CollisionOfferCard via NameEditControl's verdict slot instead of MergeOfferCard.
	async function commitTitle(value: string): Promise<{ ok: true } | { conflict: VideoCollisionRef }> {
		const res = await api.setFieldDecision(id, 'title', { source: 'manual', manual_value: value });
		if (res.conflict) {
			pendingTitleValue = value;
			return { conflict: res.conflict };
		}
		await reloadDetail();
		return { ok: true };
	}

	// "Save anyway, keep both" — resubmits the same pending value with override, bypassing
	// the collision gate. `resolve` is NameEditControl's own dismiss callback.
	async function saveTitleAnyway(resolve: () => void) {
		titleCollisionBusy = true;
		titleCollisionError = '';
		try {
			await api.setFieldDecision(id, 'title', {
				source: 'manual',
				manual_value: pendingTitleValue,
				override: true
			});
			resolve();
			await reloadDetail();
		} catch (e) {
			titleCollisionError = toMessage(e);
		} finally {
			titleCollisionBusy = false;
		}
	}

	// Studio reassignment (HOLODEX-271, StudioPicker). Unlike Title, every source pick
	// (known-candidate chip, searched, or created) runs through this same collision-checked
	// path — a chip pick changes the composite key exactly as much as a manual one does.
	async function decideStudio(
		source: DecisionSource,
		manualValue?: string
	): Promise<{ ok: true } | { conflict: VideoCollisionRef }> {
		const res = await api.setFieldDecision(id, 'studio', {
			source,
			...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
		});
		if (res.conflict) {
			pendingStudioSource = source;
			pendingStudioValue = manualValue;
			return { conflict: res.conflict };
		}
		await reloadDetail();
		return { ok: true };
	}

	// "Save anyway, keep both" — resubmits the exact same pending decision with override.
	async function saveStudioAnyway(resolve: () => void) {
		if (!pendingStudioSource) return;
		studioCollisionBusy = true;
		studioCollisionError = '';
		try {
			await api.setFieldDecision(id, 'studio', {
				source: pendingStudioSource,
				...(pendingStudioSource === 'manual' ? { manual_value: pendingStudioValue ?? '' } : {}),
				override: true
			});
			resolve();
			await reloadDetail();
		} catch (e) {
			studioCollisionError = toMessage(e);
		} finally {
			studioCollisionBusy = false;
		}
	}

	function roleField(role: 'actor' | 'director'): 'actors' | 'director' {
		return role === 'actor' ? 'actors' : 'director';
	}

	// People attach/detach (HOLODEX-272, PersonPicker + grid remove control). Both commit
	// through the curation model (F30/ADR-048) — actors/director are multi/merge fields
	// SetDecision structurally rejects (worklog HOLODEX-272) — rather than the field-decision
	// model Title/Studio use. A person-typed add/suppress may 409 on the People composite-key
	// collision gate; conflict handling is shared across both call sites since detaching from
	// either the grid or the picker produces the identical curation call.
	async function curatePerson(
		field: 'actors' | 'director',
		value: string,
		action: 'add' | 'suppress'
	): Promise<{ ok: true } | { conflict: VideoCollisionRef }> {
		const res = await api.curateMedia(id, { field, value, action });
		if (res.conflict) {
			pendingPersonField = field;
			pendingPersonValue = value;
			pendingPersonAction = action;
			personConflict = res.conflict;
			return { conflict: res.conflict };
		}
		await reloadDetail();
		return { ok: true };
	}

	const attachPerson = (name: string, role: 'actor' | 'director') => curatePerson(roleField(role), name, 'add');
	const detachPerson = (name: string, role: 'actor' | 'director') => curatePerson(roleField(role), name, 'suppress');

	// "Save anyway, keep both" — resubmits the exact same pending curation with override.
	async function savePersonAnyway(resolve: () => void) {
		if (!pendingPersonField) return;
		personCollisionBusy = true;
		personCollisionError = '';
		try {
			await api.curateMedia(id, {
				field: pendingPersonField,
				value: pendingPersonValue,
				action: pendingPersonAction,
				override: true
			});
			resolve();
			await reloadDetail();
		} catch (e) {
			personCollisionError = toMessage(e);
		} finally {
			personCollisionBusy = false;
		}
	}

	function resolvePersonConflict() {
		personConflict = null;
		pendingPersonField = null;
		personCollisionError = '';
	}

	async function removeGridPerson(p: Person) {
		if (personBusyKey) return;
		// A legacy pre-migration-0037 link can still carry the unset-role sentinel
		// ('') — roleField('') would silently fall through to 'director', suppressing
		// the wrong field's link (HOLODEX-272 review fix).
		if (p.role !== 'actor' && p.role !== 'director') {
			personRemoveError = `${p.name} has no role set on this video — can't determine which field to remove.`;
			return;
		}
		personBusyKey = personKey(p);
		personRemoveError = '';
		try {
			await detachPerson(p.name, p.role);
		} catch (e) {
			personRemoveError = toMessage(e);
		} finally {
			personBusyKey = null;
		}
	}

	async function removeFilm(f: FilmAttachment) {
		if (filmBusyKey || !video) return;
		filmBusyKey = f.film_id;
		filmRemoveError = '';
		try {
			await api.detachFilmVideo(f.film_id, video.id);
			films = films.filter((fa) => fa.film_id !== f.film_id);
		} catch (e) {
			filmRemoveError = toMessage(e);
		} finally {
			filmBusyKey = null;
		}
	}

	// Edit scene number (HOLODEX-326): the Films chip row's badge opens this dialog
	// instead of detach+reattach. editingSceneFilm holds the attachment being
	// renumbered (null = none open).
	let editingSceneFilm = $state<FilmAttachment | null>(null);

	async function saveFilmSceneNumber(value: number | null) {
		if (!editingSceneFilm || !video) throw new Error('no film selected');
		const filmId = editingSceneFilm.film_id;
		const res = await api.updateFilmVideoScene(filmId, video.id, value);
		if (res.conflict) return { conflict: res.conflict };
		films = films.map((fa) => (fa.film_id === filmId ? { ...fa, scene_number: value } : fa));
		return { ok: true as const };
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

	// Run extraction for this video only (F48.5a). Synchronous: the endpoint applies
	// or queues per field and returns the outcome, so refetching the scoped queue
	// straight after gives the panel its rows.
	async function runExtract() {
		if (!video || extracting) return;
		const gen = pageGeneration;
		extracting = true;
		extractError = '';
		try {
			const res = await api.extractVideo(id);
			const rows = await fetchExtractionRows();
			if (gen !== pageGeneration) return; // navigated away mid-run
			extractRows = rows;
			extractRun = { matched: res.matched };
		} catch (e) {
			if (gen === pageGeneration) extractError = toMessage(e);
		} finally {
			if (gen === pageGeneration) extracting = false;
		}
	}

	// Facets are fetched once, and only when the panel is actually used — the media
	// page has no other need for the registry, so an unused video never pays for it.
	async function fetchExtractionRows(): Promise<QueueRow[]> {
		const needLabels = Object.keys(extractLabels).length === 0;
		const [queue, facets] = await Promise.all([
			api.extractionQueue(id),
			needLabels ? api.facets() : Promise.resolve(null)
		]);
		// Labels are video-independent, so they survive navigation and are still worth
		// caching across it.
		if (facets) extractLabels = Object.fromEntries(facets.facets.map((f) => [f.canonical, f.label]));
		return queue.rows ?? [];
	}

	function dropExtractRow(reviewId: number) {
		extractRows = extractRows.filter((r) => r.id !== reviewId);
		extractStaged = unstagePick(extractStaged, reviewId);
	}

	// Clears everything scoped to one video. Bumping the generation also strands any
	// in-flight run or job wait: those check it before writing state back, so a wait
	// started on the previous video can neither resurrect its panel nor overwrite the
	// new one's detail.
	function resetExtraction() {
		pageGeneration += 1;
		extracting = false;
		extractApplying = false;
		extractRows = [];
		extractRun = null;
		extractStaged = {};
		extractPreviewOpen = false;
		extractError = '';
	}

	// ADR-090 D3: an adopted value has to visibly land in the field list below,
	// otherwise the panel just empties and the owner is left to infer that something
	// happened somewhere.
	//
	// Getting there is indirect. Resolving a row enqueues a *file* write, and only the
	// queue's post-write re-extract (ADR-073) puts the new value back into the `file`
	// baseline the resolver reads. Both steps are off-request, so refetching straight
	// away shows the old value. But the queue already exposes exactly this: it runs the
	// re-extract *before* marking the job done (`writequeue.go:275`), so waiting on the
	// job is waiting on the value being readable. `resolveExtractionReview` now returns
	// that job id, and `waitForWritebackJob` is the same tested waiter
	// `WritebackFormDialog` uses — backoff, unmount cancellation, and a real error
	// channel, so a *failed* write surfaces as an error instead of reading "written".
	//
	// The value lands as `file`, not `filename`: adoption writes the file tag, and
	// `filename` stays the candidate namespace.
	async function onExtractSubmitted(resolvedIds: number[], jobIds: number[]) {
		const gen = pageGeneration;
		resolvedIds.forEach(dropExtractRow);
		extractApplying = true;
		try {
			await Promise.all(
				jobIds.map((jobId) =>
					waitForWritebackJob(jobId, api.writebackJobStatus, {
						cancelled: () => unmounted || gen !== pageGeneration
					})
				)
			);
			if (gen !== pageGeneration) return; // this video is no longer on screen
			applyMediaDetail(await api.getMedia(id));
		} catch (e) {
			if (gen === pageGeneration) extractError = toMessage(e);
		} finally {
			if (gen === pageGeneration) extractApplying = false;
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
		// A pending title-collision verdict is scoped to the video that produced it — carrying
		// it across navigation would let "Save anyway" commit the old value onto the new id.
		pendingTitleValue = '';
		titleCollisionBusy = false;
		titleCollisionError = '';
		// Same reasoning as the collision verdict above, and the same consequence if
		// skipped: SvelteKit reuses this component across /media/A -> /media/B, so an
		// extraction panel left open would keep A's review rows and staged picks while
		// the header renders B's file path — and "Review & write" would resolve A's
		// review ids, writing to A's file, from B's page. Extraction is per-video state.
		resetExtraction();
		// resetExtraction already bumped the shared pageGeneration above, which is what
		// stops a poll from the previous video resolving into this one's badge.
		// writebackOpen/Action are per-action transient UI state, not per-video data,
		// so they reset like the extraction panel does rather than needing to survive
		// navigation.
		writebackOpen = false;
		writebackAction = null;
		writebackActionError = '';
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
		// Any other successful commit invalidates a still-open People collision
		// verdict — resubmitting its forgotten field/value/action with override:true
		// via "Save anyway" would silently clobber unrelated state (HOLODEX-272
		// review fix). curatePerson never calls reloadDetail on the branch that sets
		// personConflict, so this only ever clears an already-stale card.
		if (personConflict) resolvePersonConflict();
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
			{#key id}
				<NameEditControl
					id="field-title"
					name={displayTitle}
					{isOwner}
					onCommit={commitTitle}
					label="video"
					headingClass="skin-title text-2xl font-semibold text-ink"
					pencilAlwaysVisible
				>
					{#snippet verdict(c, resolve)}
						<CollisionOfferCard
							video={c}
							proposedTitle={pendingTitleValue}
							busy={titleCollisionBusy}
							error={titleCollisionError}
							onviewexisting={() => goto(`/media/${c.id}`)}
							onsaveanyway={() => saveTitleAnyway(resolve)}
							oncancel={resolve}
						/>
					{/snippet}
				</NameEditControl>
			{/key}
			<div class="flex flex-wrap items-center gap-2 text-sm text-muted">
				<span class="rounded-theme bg-accent px-2 py-0.5 text-accent-ink">{resolutionBucket(video.width)}</span>
				<span>{video.width}×{video.height}</span>
				<span>·</span>
				<span>{formatDuration(video.duration_sec)}</span>
				{#if formatYear(video.recorded_at)}
					<span>·</span><span>{formatYear(video.recorded_at)}</span>
				{/if}
			</div>

			<!-- Overview (media-detail-entity-ux): the synopsis reads as page content, not
			     as a data-management row, so it sits under the header meta line instead of
			     in the Metadata list. Owners keep exactly the control the Metadata
			     long_text branch gave it — the ADR-051 SourceBadge precedence chip row. -->
			{#if overviewField && (isOwner || overviewField.values[0]?.trim())}
				<div id="field-overview">
					{#if isReplaceField(overviewField) && isOwner}
						<SourceBadge field={overviewField} decide={(src, mv) => decideField('overview', src, mv)} />
					{:else if overviewField.values[0]?.trim()}
						<ExpandableText text={overviewField.values[0]} tone="muted" chevronLabel="overview" />
					{/if}
				</div>
			{/if}
		</header>

		{#if isOwner || studioField?.values?.length}
			<div class="flex flex-wrap items-center gap-3" id="field-studio">
				{#each studios as s (s.id)}
					<StudioLinkCard studio={s} />
				{/each}
				{#if isOwner}
					<StudioPicker field={studioField} hasStudio={studios.length > 0} {isOwner} decide={decideStudio}>
						{#snippet verdict(c, resolve)}
							<CollisionOfferCard
								video={c}
								busy={studioCollisionBusy}
								error={studioCollisionError}
								onviewexisting={() => goto(`/media/${c.id}`)}
								onsaveanyway={() => saveStudioAnyway(resolve)}
								oncancel={resolve}
							/>
						{/snippet}
					</StudioPicker>
				{:else if !studios.length && studioField?.values?.length}
					<span class="text-ink">{studioField.values[0]}</span>
				{/if}
			</div>
		{/if}

		{#if isOwner || video.tags?.length}
			<!-- id="field-genres": resolved genres materialize into Tag rows, so the
			     completeness queue's #field-genres deep link lands here now that the
			     Metadata list no longer carries a Genres row. -->
			<section id="field-genres" class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
				<div class="flex flex-wrap items-center gap-2">
					{#each video.tags ?? [] as t (t.id)}
						<TagLinkChip tag={t} busy={tagBusy} onremove={isOwner ? removeTag : undefined} />
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

		<!-- Films + People (media-detail-reorder): co-located in one row so Films can
		     shrink-wrap beside People instead of stacking as its own full-width section.
		     Each side keeps its own pre-existing gate; the row contributes nothing when
		     both are hidden (filmsVisible/peopleVisible above). -->
		{#if filmsVisible || peopleVisible}
			<div class="flex items-start gap-6">
				{#if filmsVisible}
					<!-- Films (F56, design handoff §3a): poster-tile chips mirroring the People grid,
					     not Studio's read-only pills — film_videos is many-to-many like video_people. -->
					<section class="max-w-[50%] flex-none space-y-1.5">
						<h2 class="text-xs uppercase tracking-wide text-muted">Films</h2>
						<ul class="flex flex-wrap gap-3">
							{#each films as f (f.film_id)}
								<!-- Edit in place (HOLODEX-326): owner-only, and only for a scene attachment --
								     a full-film row has no scene number to edit. -->
								{@const editable = isOwner && !f.is_full_film}
								<li class="curation-chip group relative w-20 shrink-0">
									<a href={`/films/${f.film_id}`} class="block space-y-1.5 text-ink" title={f.film_name}>
										<div
											class="flex aspect-[2/3] items-center justify-center overflow-hidden rounded-theme bg-logo-plate transition group-hover:opacity-90"
										>
											<span class="font-display text-lg font-semibold text-logo-plate-ink" aria-hidden="true"
												>{monogram(f.film_name)}</span
											>
										</div>
										<span class="line-clamp-2 text-xs text-muted group-hover:text-accent">{f.film_name}</span>
									</a>
									<svelte:element
										this={editable ? 'button' : 'span'}
										type={editable ? 'button' : undefined}
										role={editable ? 'button' : undefined}
										onclick={editable ? () => (editingSceneFilm = f) : undefined}
										aria-label={editable ? `Edit scene number in ${f.film_name}` : undefined}
										class="mt-1.5 block w-full rounded-theme bg-accent px-1.5 py-0.5 text-center text-[10px] font-semibold text-accent-ink {editable
											? 'hover:ring-1 hover:ring-inset hover:ring-accent-ink/50'
											: ''}"
									>
										{f.is_full_film ? 'Full film' : f.scene_number !== null ? `#${f.scene_number}` : 'Unnumbered'}
									</svelte:element>
									{#if isOwner}
										<button
											type="button"
											onclick={() => removeFilm(f)}
											disabled={filmBusyKey === f.film_id}
											aria-label={`Remove ${f.film_name}`}
											class="curation-actions absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full border border-rule bg-surface-2/90 text-sm text-muted hover:border-accent hover:text-accent focus-visible:border-accent focus-visible:text-accent disabled:cursor-default"
										>
											{filmBusyKey === f.film_id ? '…' : '×'}
										</button>
									{/if}
								</li>
							{/each}
							{#if isOwner}
								<li class="w-20 shrink-0">
									<button
										type="button"
										onclick={() => (filmAttachOpen = true)}
										class="flex aspect-[2/3] w-full flex-col items-center justify-center gap-1 rounded-theme border border-dashed border-rule text-muted hover:border-accent hover:text-accent"
									>
										<span class="text-2xl leading-none">+</span>
										<span class="text-xs">Attach film</span>
									</button>
								</li>
							{/if}
						</ul>
						{#if filmRemoveError}
							<p class="text-sm text-warn" aria-live="polite">{filmRemoveError}</p>
						{/if}
					</section>
				{/if}

				<!-- id="field-actors": the actors facet's deep link, for the same reason. -->
				<div id="field-actors" class="min-w-0 flex-1">
					<PeopleGrid
						title="People"
						people={video.people ?? []}
						{isOwner}
						attach={attachPerson}
						detach={detachPerson}
						bind:busyKey={personBusyKey}
						onRemove={removeGridPerson}
						removeError={personRemoveError}
					/>
				</div>
			</div>
		{/if}

		{#if personConflict}
			{@const conflict = personConflict}
			<CollisionOfferCard
				video={conflict}
				busy={personCollisionBusy}
				error={personCollisionError}
				onviewexisting={() => goto(`/media/${conflict.id}`)}
				onsaveanyway={() => savePersonAnyway(resolvePersonConflict)}
				oncancel={resolvePersonConflict}
			/>
		{/if}

		<!-- Metadata section (F27): resolved fields (merged file + enrichment) with
		     enrichment controls and writeback inline in the header. Falls back to
		     file-only fields when no resolver output is present. Owner-only
		     (media-detail-reorder) — visitors previously saw a filtered subset. -->
		{#if isOwner}
			<section class="space-y-1.5">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-baseline gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Metadata</h2>
						<span class="text-xs text-muted">{metadataFieldCount} field{metadataFieldCount === 1 ? '' : 's'}</span>
					</div>
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
							<!-- F48.5a: the single-video extraction trigger. Ghost text, no border, no chip —
							     the identical treatment Refresh uses beside it. It is a one-shot action, not a
							     stateful provider link, so it must never grow chip chrome that would read as a
							     sibling of the provider chips (ADR-090 D2). -->
							<button
								onclick={runExtract}
								disabled={extracting}
								title="Read this file's name for metadata (title, studio, people, year)"
								aria-label="Extract metadata from the filename"
								class="flex items-center gap-1 rounded-theme px-2 py-0.5 text-xs text-muted hover:text-accent focus-visible:text-accent"
							>
								<svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M14 3v5h5M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-5z"/></svg>
								{extracting ? 'Extracting…' : 'Extract from filename'}
							</button>
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
							<!-- Writeback badges (ADR-091, HOLODEX-323, spec R2.3): sit beside the write
							     action they're all about, not the section label — see the design
							     handoff. One pill geometry, two weights: "out of sync" is a steady
							     state (outline only, no fill — reuses SourceBadge's own "file out of
							     sync" pill treatment so the two read as one family) and is never
							     hidden by pending/failed (R2.4/RD6) — the file genuinely still
							     differs until a queued write lands, and a write can sit behind a
							     large batch. Pending/failed are events (filled). RD5 clears a
							     video's failed row only when a NEW write is submitted through this
							     dialog — merge propagation, tag sync, and film-studio cascade
							     enqueue via Queue.Enqueue/EnqueueMany directly and don't clear a
							     prior failure first, so pending and failed CAN coexist for one video
							     (TestGetVideoWritebackStatus asserts exactly this). The badge favors
							     pending here since a write is actively in flight; the failed-detail
							     line below is gated on `!pending` too, so a stale failure's Retry/
							     Dismiss never renders next to an unrelated write that's already
							     running. No counts anywhere here — the write is atomic per job
							     (RD1/RD4). -->
							{#if writebackStatus.pending}
								<span
									class="inline-flex items-center gap-1 rounded-full bg-accent px-2 py-0.5 text-[0.65rem] text-accent-ink"
									aria-live="polite"
								>
									<svg class="h-3 w-3 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
									</svg>
									writing to file
								</span>
							{:else if writebackStatus.failed}
								<span class="inline-flex items-center gap-1 rounded-full bg-warn px-2 py-0.5 text-[0.65rem] text-warn-ink">
									<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
										<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a1 1 0 00.86 1.5h18.64a1 1 0 00.86-1.5L13.71 3.86a1 1 0 00-1.72 0z" />
									</svg>
									couldn't write
								</span>
							{/if}
							{#if outOfSyncN > 0}
								<span class="inline-block rounded-full border border-warn px-2 py-0.5 text-[0.65rem] text-warn">
									out of sync
								</span>
							{/if}
							<button
								onclick={() => (writebackOpen = true)}
								class="flex items-center gap-1 rounded-theme px-2 py-0.5 text-xs text-muted hover:text-accent focus-visible:text-accent"
								title="Write decided field values to the file tags"
							>
								<svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v13m0 0l-4-4m4 4l4-4M5 20h14"/></svg>
								Write decisions to file
							</button>
						{/if}
						<button
							type="button"
							onclick={() => (metadataExpanded = !metadataExpanded)}
							aria-expanded={metadataExpanded}
							aria-controls="metadata-fields"
							aria-label={metadataExpanded ? 'Hide metadata fields' : 'Show metadata fields'}
							title={metadataExpanded ? 'Hide fields' : 'Show fields'}
							class="btn-quiet flex h-7 w-7 shrink-0 items-center justify-center rounded-theme hover:bg-surface-2"
						>
							<svg
								class="h-4 w-4 transition-transform duration-200 motion-reduce:transition-none"
								class:rotate-180={metadataExpanded}
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								aria-hidden="true"
							>
								<path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6" />
							</svg>
						</button>
					</div>
				</div>
				{#if canWriteback && writebackStatus.failed && !writebackStatus.pending}
					<!-- Failed-writeback detail line (spec R3.2): job-level, not per-field — the
					     write is one exiftool/mkvpropedit invocation, so it lands whole or not at
					     all (RD4). Persists until retried or dismissed (R3.1) — a failure that
					     cleared itself would break "absence means nothing to report" everywhere
					     else on this page. The extra `!pending` guard matters because pending and
					     failed CAN coexist for one video (see the badge block's comment above) —
					     without it, a stale failure's Retry/Dismiss controls would render right
					     next to an unrelated write that's already in flight, inviting the owner to
					     "retry" a write that isn't the one failing. -->

					<p class="flex flex-wrap items-center gap-2 text-xs text-warn" aria-live="polite">
						<span
							>{writebackStatus.error ||
								"Couldn't write to the file — it may be locked or read-only."}</span
						>
						<button
							onclick={retryWriteback}
							disabled={writebackAction !== null}
							class="text-accent underline hover:no-underline disabled:cursor-not-allowed"
						>
							{writebackAction === 'retry' ? 'Retrying…' : 'Retry'}
						</button>
						<button
							onclick={dismissWriteback}
							disabled={writebackAction !== null}
							class="text-muted underline hover:no-underline disabled:cursor-not-allowed"
						>
							{writebackAction === 'dismiss' ? 'Dismissing…' : 'Dismiss'}
						</button>
					</p>
					{#if writebackActionError}
						<p class="text-xs text-warn" aria-live="polite">{writebackActionError}</p>
					{/if}
				{/if}
				{#if extractPanelVisible}
					<!-- ADR-090 layer 1 at entity scope: adoption only — the filename value against the
					     file's own tag. A provider's competing value never appears here; that is layer
					     2's question and the SourceBadge chip row below already owns it. -->
					<section class="rounded-theme border border-rule bg-surface" aria-labelledby="extract-panel-heading">
						<div class="flex flex-wrap items-baseline justify-between gap-2 px-3 pb-1.5 pt-2.5">
							<h3 id="extract-panel-heading" class="text-xs font-medium text-ink">
								From filename{#if extractSorted.length}<span class="text-muted"> · {extractSorted.length} to review</span>{/if}
							</h3>
							<button onclick={runExtract} disabled={extracting} class="btn-quiet px-2 py-0.5 text-xs">
								{extracting ? 'Extracting…' : 'Re-extract'}
							</button>
						</div>
						{#if video}
							<p class="truncate px-3 pb-2 text-xs text-muted" title={video.file_path}>{video.file_path}</p>
						{/if}

						<!-- A failed write must not take the rest of the panel with it: the remaining rows
						     are still pending and actionable, so the error sits above them rather than
						     replacing the chain (which stranded them until a full Re-extract). -->
						{#if extractError}
							<p class="border-t border-rule px-3 py-2 text-xs text-warn" role="alert">{extractError}</p>
						{/if}
						{#if extractApplying}
							<p class="border-t border-rule px-3 py-2 text-xs text-muted" aria-live="polite">
								Written to the file — waiting for it to be read back…
							</p>
						{:else if extractSorted.length === 0}
							<!-- F48.6l: "no pattern matched" and "matched, nothing to review" are different
							     outcomes and must read differently. Never an empty panel, never a zero count. -->
							<p class="border-t border-rule px-3 py-2 text-xs text-muted" aria-live="polite">
								{#if !extractRun?.matched}
									No filename pattern matched this file.
								{:else}
									Nothing needs review — matched values are in the list below.
								{/if}
							</p>
						{:else}
							{#each extractSorted as row (row.id)}
								<ExtractionQueueRow
									{row}
									fieldLabel={extractLabel(row.field_key)}
									isEntityField={isEntityField(row.field_key)}
									staged={extractStaged[row.id]}
									onstage={(action, value) => (extractStaged = stagePick(extractStaged, row.id, action, value))}
									onunstage={() => (extractStaged = unstagePick(extractStaged, row.id))}
									resolveTag={() => api.resolveExtractionReview(row.id, 'tag')}
									dismiss={() => api.dismissExtractionReview(row.id)}
									onhandled={() => dropExtractRow(row.id)}
								/>
							{/each}
							<div class="flex flex-wrap items-center justify-between gap-2 border-t border-rule px-3 py-2">
								<p class="text-xs text-muted" aria-live="polite">{extractStagedCount} staged · nothing written yet</p>
								<div class="flex items-center gap-2">
									<button onclick={() => (extractStaged = {})} disabled={extractStagedCount === 0} class="btn-quiet px-2 py-0.5 text-xs">
										Clear
									</button>
									<button onclick={() => (extractPreviewOpen = true)} disabled={extractStagedCount === 0} class="btn-accent px-2.5 py-1 text-xs">
										Review &amp; write {extractStagedCount}
									</button>
								</div>
							</div>
						{/if}
					</section>
				{/if}
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
				<!-- The fold covers the field list only: Refresh / Extract / provider chips /
				     writeback stay reachable while collapsed, and the extraction review panel
				     above stays visible because it is transient work waiting on the owner. -->
				<div
					id="metadata-fields"
					class="overflow-hidden transition-[max-height] duration-200 ease-out motion-reduce:transition-none"
					style="max-height: {metadataExpanded ? '6000px' : '0px'}"
					inert={!metadataExpanded}
				>
				{#if canonicalResolved.length || extraFields.length}
				<dl class="grid grid-cols-1 gap-3 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
					{#each canonicalResolved as f (f.canonical)}
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
								{#if isReplaceField(f) && isOwner}
									<dd class="mt-1 block leading-relaxed">
										<SourceBadge field={f} decide={(s, mv) => decideField(f.canonical, s, mv)} />
									</dd>
								{:else if f.values[0]?.trim()}
									<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
									{#if winnerProvider}<ProvenanceBadge provider={winnerProvider} label={winnerProvider} />{/if}
								{/if}
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

		{#if isOwner && completeness}
			{#each completeness.facets as cf (cf.canonical)}
				{#if cf.tier === 'missing' && !canonicalResolved.some((f) => f.canonical === cf.canonical) && !hasPageAnchor(cf.canonical)}
					<div id={`field-${cf.canonical}`} class="hidden" aria-hidden="true"></div>
				{/if}
			{/each}
		{/if}

		{#if isOwner}
			<CompletenessPanel {completeness} videoId={id} onchanged={reloadDetail} />
		{/if}

		<!-- Admin-only metadata sources (F29): the raw file-extracted payload and the raw
		     provider enrichment payload, kept as audit/debug disclosures at the bottom of
		     the page. Owner + Admin mode only (effectiveOwner); each self-omits when
		     empty. Headings aligned to "Enrichment data: {source}". -->
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

	{#if extractPreviewOpen}
		<ExtractionPreviewDialog
			items={extractPreviewItems}
			onclose={() => (extractPreviewOpen = false)}
			onsubmitted={onExtractSubmitted}
			resolve={(reviewId, action, value) => api.resolveExtractionReview(reviewId, action, value)}
		/>
	{/if}
	{#if writebackOpen && video}
		<WritebackFormDialog
			fields={resolved}
			videoId={id}
			filePath={video.file_path}
			writeback={api.writebackMedia}
			decide={async (canonical, source, manualValue) => {
				const res = await api.setFieldDecision(id, canonical, {
					source,
					...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
				});
				// ensureDecision doesn't inspect this return value — a collision must throw so
				// submit() aborts before writeback() commits the colliding value to the file
				// (HOLODEX-270 review fix).
				if (res.conflict) {
					throw new Error(`"${manualValue}" already matches another video: ${res.conflict.title}`);
				}
			}}
			onclose={() => (writebackOpen = false)}
			onenqueued={async () => {
				// Fire-and-forget (ADR-091): the dialog has already closed by the time this
				// fires. One reload picks up the fresh pending writeback_status row (the
				// enqueue commits before the 202 is sent, so it's already visible) plus any
				// decisions ensureDecision() created, and the $effect above takes it from
				// there — polling until the job settles, then reloading again so resolved[]
				// recomputes in_sync against the post-write baseline (ADR-073 D1). Same
				// reason decideField/clearProvider/onApplied reload.
				thumbVersion += 1;
				await reloadDetail();
			}}
		/>
	{/if}

	{#if filmAttachOpen && video}
		<FilmAttachDialog
			videoId={video.id}
			onclose={() => (filmAttachOpen = false)}
			onattached={reloadDetail}
		/>
	{/if}

	{#if editingSceneFilm}
		<EditSceneNumberDialog
			contextLabel={editingSceneFilm.film_name}
			currentScene={editingSceneFilm.scene_number}
			onsave={saveFilmSceneNumber}
			onclose={() => (editingSceneFilm = null)}
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
