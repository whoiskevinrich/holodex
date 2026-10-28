<script lang="ts">
	// Duplicates review queue (F43 S5, ADR-061 — Option A dense rows, ratified), as one
	// lane per type (F70 P0-8, HOLODEX-598): a tab row picks Videos, People, Studios, Tags
	// or Films, and only the open lane's pairs are read. Every tab carries its count from
	// whichever read landed last, 0 included, so the tabs never shift. The open lane lives
	// in `?type=`, the entity lists' Duplicates banner deep link.
	//
	// Person and studio pairs expand in place into a compare panel (F70); this page owns
	// which one is open so only one ever is, and owns where focus lands when a row is
	// removed under it. Tokens only; QA Cinémathèque.
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';
	import { onMount, tick, untrack } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { api } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import type {
		DuplicateLane,
		DuplicateLaneCounts,
		DuplicatePair,
		EntityKind,
		VideoDuplicatePair
	} from '#lib/types.ts';
	import DuplicatePairRow from '#lib/components/duplicates/DuplicatePairRow.svelte';
	import VideoPairRow from '#lib/components/duplicates/VideoPairRow.svelte';
	import {
		defaultLane,
		focusLandingIds,
		LANE_LABEL,
		lanePanelId,
		LANES,
		laneTabId,
		pairKey,
		parseLane,
		QUEUE_ID
	} from '#lib/components/duplicates/queue.ts';
	import { videoFocusLandingIds, videoPairKey } from '#lib/components/duplicates/videoPairs.ts';
	import { appendUnique, nextPage, QUEUE_PAGE, windowLimit } from '#lib/queuePaging.ts';

	// The lane named in the URL wins, even at 0 (the banner link must land where it says).
	// Unnamed, it is settled by the first read's counts.
	let lane = $state<DuplicateLane | null>(parseLane(page.url.searchParams.get('type')));
	let counts = $state<DuplicateLaneCounts | null>(null);
	let pairs = $state<DuplicatePair[]>([]);
	let videoPairs = $state<VideoDuplicatePair[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let loadingMore = $state(false);
	let moreError = $state(''); // a failed Load more keeps the loaded rows
	let error = $state('');

	// Bumped by every read that replaces the lane, and by every resolve; a read that
	// started before a newer one is dropped, so a slow re-read can't bring back a pair
	// resolved after it was sent, or paint one lane's rows into another.
	let gen = 0;

	const COMPARE_INTRO = 'Keep separate remembers your choice; merging keeps the other name as an alias.';
	const INLINE_INTRO =
		'Possible duplicate names within an entity — case and spacing are already merged automatically; these are the judgement calls. Merge folds one into the other (the merged name stays as a searchable alias); Keep separate remembers your choice.';
	const LANE_INTRO: Record<DuplicateLane, string> = {
		video: 'Files matched to the same provider item. Keep one, keep both, or label them as editions or parts.',
		person: COMPARE_INTRO,
		studio: COMPARE_INTRO,
		tag: INLINE_INTRO,
		film: INLINE_INTRO
	};

	/** One page of `l`, with every lane's count. */
	async function read(l: DuplicateLane, offset: number, limit: number) {
		if (l === 'video') {
			const res = await api.videoDuplicates({ offset, limit });
			return { videos: res.items ?? [], items: [] as DuplicatePair[], total: res.total, counts: res.counts };
		}
		const res = await api.duplicates({ type: l as EntityKind, offset, limit });
		return { videos: [] as VideoDuplicatePair[], items: res.items ?? [], total: res.total, counts: res.counts };
	}

	function accept(res: Awaited<ReturnType<typeof read>>) {
		pairs = res.items;
		videoPairs = res.videos;
		total = res.total;
		counts = res.counts;
	}

	/** Read the open lane from the top. With no lane named yet, Videos is read first: it
	 *  is the first tab, so when it has pairs that one request is the whole page; otherwise
	 *  its counts name the lane to read next. */
	async function load() {
		loading = true;
		error = '';
		openKey = null;
		const mine = ++gen;
		try {
			let res = await read(lane ?? 'video', 0, QUEUE_PAGE);
			if (mine !== gen) return;
			if (!lane) {
				lane = defaultLane(res.counts);
				if (lane !== 'video') {
					counts = res.counts;
					res = await read(lane, 0, QUEUE_PAGE);
					if (mine !== gen) return;
				}
			}
			accept(res);
		} catch (e) {
			if (mine === gen) error = toMessage(e);
		} finally {
			if (mine === gen) loading = false;
		}
	}
	onMount(load);

	// A same-route navigation to another `?type=` reuses this component, so follow the URL
	// when it names a lane other than the open one. Our own `replaceState` names the open
	// lane, so it is a no-op here.
	$effect(() => {
		const named = parseLane(page.url.searchParams.get('type'));
		untrack(() => {
			if (named && named !== lane) selectLane(named);
		});
	});

	function selectLane(l: DuplicateLane) {
		if (l === lane && !error) return; // already open or opening; a failed lane retries
		lane = l;
		pairs = [];
		videoPairs = [];
		moreError = '';
		const url = new URL(page.url.href);
		url.searchParams.set('type', l);
		replaceState(url, page.state);
		load();
	}

	// The ARIA tabs pattern, copied from SearchResultsPanel: roving tabindex, and the
	// arrows (plus Home/End) move between lanes and open them.
	function onTabKey(e: KeyboardEvent, i: number) {
		const to =
			e.key === 'ArrowRight'
				? (i + 1) % LANES.length
				: e.key === 'ArrowLeft'
					? (i - 1 + LANES.length) % LANES.length
					: e.key === 'Home'
						? 0
						: e.key === 'End'
							? LANES.length - 1
							: -1;
		if (to === -1) return;
		e.preventDefault();
		selectLane(LANES[to]);
		document.getElementById(laneTabId(LANES[to]))?.focus();
	}

	// Resolving drops its own row, so the loaded length is already the next offset.
	async function loadMore() {
		if (!lane) return;
		loadingMore = true;
		moreError = '';
		const mine = gen;
		const l = lane;
		try {
			const p = nextPage(l === 'video' ? videoPairs.length : pairs.length);
			const res = await read(l, p.offset, p.limit);
			if (mine !== gen) return;
			pairs = appendUnique(pairs, res.items, pairKey);
			videoPairs = appendUnique(videoPairs, res.videos, videoPairKey);
			total = res.total;
			counts = res.counts;
		} catch (e) {
			moreError = toMessage(e);
		} finally {
			loadingMore = false;
		}
	}

	// At most one compare panel open at a time (RD12): opening a second collapses the
	// first, so the page never holds two sets of evidence you aren't reading.
	let openKey = $state<string | null>(null);

	// Resolve one pair (merged, dismissed or kept): drop it at once, then re-read only this
	// lane and the counts (P0-8). The removal is instant and unanimated, so focus would land
	// on <body> if we didn't move it (P0-6); the ladder is `focusLandingIds`, and this takes
	// the first of its ids that actually rendered.
	async function resolved(l: DuplicateLane, ids: string[], key: string, drop: () => void) {
		if (counts) counts = { ...counts, [l]: Math.max(counts[l] - 1, 0) };
		// The owner switched lanes while the verdict was in flight: its rows are gone, and
		// the open lane's own read is already on its way (bumping `gen` would strand it).
		if (l !== lane) return;
		if (openKey === key) openKey = null;
		const mine = ++gen;
		drop();
		total = Math.max(total - 1, 0);
		await tick();
		for (const id of ids) {
			const el = document.getElementById(id);
			if (el) {
				el.focus();
				break;
			}
		}
		// A merge also ends every other pair the merged-away entity was in, and keeping one
		// video trashes a file that may sit in other pairs; re-read the loaded window rather
		// than guess, so the next Load more's offset stays true (HOLODEX-587).
		try {
			const res = await read(l, 0, windowLimit(l === 'video' ? videoPairs.length : pairs.length));
			if (mine === gen) accept(res);
		} catch {
			// The optimistic removal already stands; the next load reconciles.
		}
	}

	const resolve = (pair: DuplicatePair) =>
		resolved(pair.entity_type, focusLandingIds(pair, pairs), pairKey(pair), () => (pairs = pairs.filter((p) => p !== pair)));

	const resolveVideo = (pair: VideoDuplicatePair) =>
		resolved(
			'video',
			videoFocusLandingIds(pair, videoPairs),
			videoPairKey(pair),
			() => (videoPairs = videoPairs.filter((p) => p !== pair))
		);

	function mergePair(pair: DuplicatePair, survivorId: number, fromId: number, showAs?: 'survivor' | 'merged') {
		// One merge endpoint for all four entities (the person route is unified into it).
		return api.mergeEntities(pair.entity_type, survivorId, fromId, showAs);
	}

	const shownCount = $derived(lane === 'video' ? videoPairs.length : pairs.length);
</script>

<!-- `tabindex="-1"` so a resolve that lands nowhere else still has somewhere to put focus;
     not a tab stop. -->
<div id={QUEUE_ID} tabindex="-1" class="space-y-3">
	<div role="tablist" aria-label="Duplicate type" class="flex items-center gap-1 overflow-x-auto">
		{#each LANES as l, i (l)}
			<button
				type="button"
				role="tab"
				id={laneTabId(l)}
				aria-selected={lane === l}
				aria-controls={lanePanelId(l)}
				aria-label={counts ? `${LANE_LABEL[l]}, ${counts[l]}` : LANE_LABEL[l]}
				tabindex={(lane ?? 'video') === l ? 0 : -1}
				onclick={() => selectLane(l)}
				onkeydown={(e) => onTabKey(e, i)}
				class="flex shrink-0 items-center gap-1.5 rounded-theme px-2 py-1 text-xs transition {lane === l
					? 'bg-surface-2 text-ink'
					: 'text-muted hover:text-ink'}"
			>
				{LANE_LABEL[l]}
				<!-- No number until the first read returns them: no spinner per tab. -->
				{#if counts}<span class="tabular-nums text-muted">{counts[l]}</span>{/if}
			</button>
		{/each}
	</div>

	{#if lane}
		<!-- The focus ladder's lane rung (`lanePanelId`), hence `tabindex="-1"`. -->
		<div id={lanePanelId(lane)} role="tabpanel" aria-labelledby={laneTabId(lane)} tabindex="-1" class="space-y-3">
			<p class="text-sm text-muted">{LANE_INTRO[lane]}</p>
			{#if loading}
				<p class="py-16 text-center text-sm text-muted">Loading…</p>
			{:else if error}
				<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
			{:else}
				<!-- `overflow-hidden` + `-mt-px` tuck the first row's top rule under the shell's
				     own border, now that no heading sits above it. -->
				<section class="overflow-hidden rounded-theme border border-rule bg-surface">
					<div class="-mt-px">
						{#if shownCount === 0}
							<p class="border-t border-rule py-16 text-center text-sm text-muted">
								No possible duplicate {LANE_LABEL[lane].toLowerCase()}.
							</p>
						{:else if lane === 'video'}
							{#each videoPairs as vp (videoPairKey(vp))}
								<VideoPairRow
									pair={vp}
									onresolved={() => resolveVideo(vp)}
									expanded={openKey === videoPairKey(vp)}
									onexpand={(v) => (openKey = v ? videoPairKey(vp) : null)}
								/>
							{/each}
						{:else}
							{#each pairs as pair (pairKey(pair))}
								<DuplicatePairRow
									{pair}
									merge={(survivorId, fromId, showAs) => mergePair(pair, survivorId, fromId, showAs)}
									dismiss={() => api.dismissDuplicate(pair.entity_type, pair.a.id, pair.b.id)}
									onresolved={() => resolve(pair)}
									expanded={openKey === pairKey(pair)}
									onexpand={(v) => (openKey = v ? pairKey(pair) : null)}
								/>
							{/each}
						{/if}
					</div>
				</section>
				{#if moreError}<p class="text-sm text-warn" role="alert">{moreError}</p>{/if}
				<LoadMoreButton left={total - shownCount} loading={loadingMore} onclick={loadMore} class="" />
			{/if}
		</div>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{/if}
</div>
