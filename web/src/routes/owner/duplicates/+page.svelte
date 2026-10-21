<script lang="ts">
	// Duplicates review queue (F43 S5, ADR-061 — Option A dense rows, ratified). The
	// owner works the near-miss queue here: pairs grouped by entity (tags first, they
	// dominate), each row offering Merge (pick the surviving name) or Keep separate
	// (records keep-separate; the pair never returns). A ?type= deep-link (from the
	// entity-list banners) filters to one entity. A person pair also expands in place into
	// a two-column compare panel (F70, HOLODEX-451); this page owns which one is open so
	// only one ever is, and owns where focus lands when a row is removed under it.
	// Tokens only; QA Cinémathèque.
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';
	import { tick } from 'svelte';
	import { page } from '$app/state';
	import { api } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import { groupByKind } from '#lib/entityGroups.ts';
	import type { DuplicatePair, EntityKind, VideoDuplicatePair } from '#lib/types.ts';
	import DuplicatePairRow from '#lib/components/duplicates/DuplicatePairRow.svelte';
	import VideoPairRow from '#lib/components/duplicates/VideoPairRow.svelte';
	import { focusLandingIds, groupId, pairKey, QUEUE_ID } from '#lib/components/duplicates/queue.ts';
	import { videoFocusLandingIds, videoPairKey } from '#lib/components/duplicates/videoPairs.ts';
	import { appendUnique, nextPage, QUEUE_PAGE, windowLimit } from '#lib/queuePaging.ts';

	let pairs = $state<DuplicatePair[]>([]);
	// Duplicate videos (F76): two files matched to the same provider item. Their own group,
	// last, with file verbs instead of Merge / Keep separate.
	let videoPairs = $state<VideoDuplicatePair[]>([]);
	// Paged (HOLODEX-587): the totals across every page, and every type's pair count for
	// the group headings (a heading counts its whole group, not the loaded rows).
	let pairTotal = $state(0);
	let videoTotal = $state(0);
	let counts = $state<Partial<Record<EntityKind, number>>>({});
	let loading = $state(true);
	let loadingMore = $state(false);
	let moreError = $state(''); // a failed Load more keeps the loaded rows
	let error = $state('');

	// Optional ?type= filter (person|studio|tag|film|video) from a deep-link; invalid/absent shows all.
	const typeFilter = $derived.by(() => {
		const t = page.url.searchParams.get('type');
		return t === 'person' || t === 'studio' || t === 'tag' || t === 'film' || t === 'video'
			? (t as EntityKind | 'video')
			: null;
	});

	// Group headings, tags first (the API already orders rows this way).
	const groupLabel: Record<EntityKind, string> = { tag: 'Tags', studio: 'Studios', person: 'People', film: 'Films' };
	const groupOrder: EntityKind[] = ['tag', 'studio', 'person', 'film'];

	const shown = $derived(typeFilter ? pairs.filter((p) => p.entity_type === typeFilter) : pairs);
	const groups = $derived(groupByKind(shown, groupOrder, (p) => p.entity_type));
	const shownVideos = $derived(typeFilter && typeFilter !== 'video' ? [] : videoPairs);

	// The server filters by type, so a deep-linked filter pages its own pairs.
	const entityType = $derived(typeFilter && typeFilter !== 'video' ? typeFilter : undefined);
	const wantsEntities = $derived(typeFilter !== 'video');
	const wantsVideos = $derived(!typeFilter || typeFilter === 'video');
	const noVideos = { items: [] as VideoDuplicatePair[], total: 0 };

	const noPairs = { items: [] as DuplicatePair[], total: 0, counts: {} };
	// Bumped by every load and resolve; a read that started before a newer change is
	// dropped, so a slow re-read can't bring back a pair resolved after it was sent.
	let pairGen = 0;
	let videoGen = 0;

	function readEntities(offset: number, limit: number) {
		return api.duplicates({ type: entityType, offset, limit });
	}

	function acceptEntities(res: { items: DuplicatePair[]; total: number; counts?: typeof counts }) {
		pairTotal = res.total;
		counts = res.counts ?? {};
		return res.items ?? [];
	}

	async function load() {
		loading = true;
		error = '';
		const mine = [++pairGen, ++videoGen];
		try {
			// A failing video read must not hide the entity pairs, so it degrades to none.
			const [entity, video] = await Promise.all([
				wantsEntities ? readEntities(0, QUEUE_PAGE) : noPairs,
				wantsVideos ? api.videoDuplicates({ limit: QUEUE_PAGE }).catch(() => noVideos) : noVideos
			]);
			if (mine[0] !== pairGen || mine[1] !== videoGen) return;
			pairs = acceptEntities(entity);
			videoPairs = video.items ?? [];
			videoTotal = video.total;
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
	});

	// Resolving drops its own row, so pairs.length is already the next offset.
	async function loadMorePairs() {
		loadingMore = true;
		moreError = '';
		const mine = pairGen;
		try {
			const p = nextPage(pairs.length);
			const res = await readEntities(p.offset, p.limit);
			if (mine === pairGen) pairs = appendUnique(pairs, acceptEntities(res), pairKey);
		} catch (e) {
			moreError = toMessage(e);
		} finally {
			loadingMore = false;
		}
	}

	async function loadMoreVideos() {
		loadingMore = true;
		moreError = '';
		const mine = videoGen;
		try {
			const res = await api.videoDuplicates(nextPage(videoPairs.length));
			if (mine !== videoGen) return;
			videoPairs = appendUnique(videoPairs, res.items ?? [], videoPairKey);
			videoTotal = res.total;
		} catch (e) {
			moreError = toMessage(e);
		} finally {
			loadingMore = false;
		}
	}

	// At most one compare panel open at a time (RD12): opening a second collapses the
	// first, so the page never holds two sets of evidence you aren't reading.
	let openKey = $state<string | null>(null);

	// Resolve one pair (merged or dismissed): drop it from the list at once, then re-read.
	// The removal is instant and unanimated, so focus would land on <body> if we didn't
	// move it (P0-6). `focusLandingIds` owns the ladder (and the reasons for each rung);
	// this takes the first of them that actually rendered.
	async function resolve(pair: DuplicatePair) {
		const ids = focusLandingIds(
			pair,
			shown.filter((p) => p.entity_type === pair.entity_type)
		);
		if (openKey === pairKey(pair)) openKey = null;
		const mine = ++pairGen;
		pairs = pairs.filter((p) => p !== pair);
		pairTotal = Math.max(pairTotal - 1, pairs.length);
		counts = { ...counts, [pair.entity_type]: Math.max((counts[pair.entity_type] ?? 1) - 1, 0) };
		await tick();
		for (const id of ids) {
			const el = document.getElementById(id);
			if (el) {
				el.focus();
				break;
			}
		}
		// A merge also ends every other pair the merged-away entity was in, which may be
		// loaded or not; re-read the loaded window rather than guess, so the next Load
		// more's offset stays true (HOLODEX-587).
		try {
			const res = await readEntities(0, windowLimit(pairs.length));
			if (mine === pairGen) pairs = acceptEntities(res);
		} catch {
			// The optimistic removal already stands; the next load reconciles.
		}
	}

	async function resolveVideo(pair: VideoDuplicatePair) {
		const ids = videoFocusLandingIds(pair, shownVideos);
		if (openKey === videoPairKey(pair)) openKey = null;
		// Keeping one copy trashes a file that may sit in other pairs too; refetch rather than
		// guess which rows it took with it.
		const mine = ++videoGen;
		videoPairs = videoPairs.filter((p) => p !== pair);
		videoTotal = Math.max(videoTotal - 1, videoPairs.length);
		await tick();
		for (const id of ids) {
			const el = document.getElementById(id);
			if (el) {
				el.focus();
				break;
			}
		}
		try {
			const res = await api.videoDuplicates({ limit: windowLimit(videoPairs.length) });
			if (mine !== videoGen) return;
			videoPairs = res.items ?? [];
			videoTotal = res.total;
		} catch {
			// The optimistic removal already stands; the next load reconciles.
		}
	}

	function mergePair(pair: DuplicatePair, survivorId: number, fromId: number): Promise<unknown> {
		// One merge endpoint for all three entities (the person route is unified into it).
		return api.mergeEntities(pair.entity_type, survivorId, fromId);
	}
</script>

<!-- `tabindex="-1"` so a resolve that empties a group still has somewhere to put focus;
     not a tab stop. -->
<div id={QUEUE_ID} tabindex="-1" class="space-y-5">
	<p class="text-sm text-muted">
		Possible duplicate names within an entity — case and spacing are already merged
		automatically; these are the judgement calls. Merge folds one into the other (the
		merged name stays as a searchable alias); Keep separate remembers your choice.
	</p>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else if pairTotal + videoTotal === 0}
		<p class="py-16 text-center text-sm text-muted">No possible duplicates.</p>
	{:else}
		{#each groups as g (g.type)}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<!-- `tabindex="-1"` so focus has somewhere to land when the last row in the
				     group is resolved; it is not a tab stop. -->
				<h2
					id={groupId(g.type)}
					tabindex="-1"
					class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted"
				>
					{groupLabel[g.type]} · {counts[g.type] ?? g.items.length}
				</h2>
				{#each g.items as pair (pairKey(pair))}
					<DuplicatePairRow
						{pair}
						merge={(survivorId, fromId) => mergePair(pair, survivorId, fromId)}
						dismiss={() => api.dismissDuplicate(pair.entity_type, pair.a.id, pair.b.id)}
						onresolved={() => resolve(pair)}
						expanded={openKey === pairKey(pair)}
						onexpand={(v) => (openKey = v ? pairKey(pair) : null)}
					/>
				{/each}
			</section>
		{/each}
		<!-- Entity pairs page as one stream across the groups (tags first), so their Load
		     more sits after the last group, ahead of Videos (HOLODEX-587). -->
		{#if moreError}<p class="text-sm text-warn" role="alert">{moreError}</p>{/if}
		<LoadMoreButton left={pairTotal - pairs.length} loading={loadingMore} onclick={loadMorePairs} class="" />
		{#if shownVideos.length}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<h2
					id={groupId('video')}
					tabindex="-1"
					class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted"
				>
					Videos · {videoTotal}
				</h2>
				<!-- The page intro describes name pairs; files need their own line. -->
				<p class="px-3 pb-2 text-xs text-muted">
					Files matched to the same provider item. Keep one, keep both, or label them as editions or parts.
				</p>
				{#each shownVideos as vp (videoPairKey(vp))}
					<VideoPairRow
						pair={vp}
						onresolved={() => resolveVideo(vp)}
						expanded={openKey === videoPairKey(vp)}
						onexpand={(v) => (openKey = v ? videoPairKey(vp) : null)}
					/>
				{/each}
				<LoadMoreButton left={videoTotal - videoPairs.length} loading={loadingMore} onclick={loadMoreVideos} class="py-3" />
			</section>
		{/if}
	{/if}
</div>
