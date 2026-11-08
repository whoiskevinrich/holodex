<script lang="ts">
	// The rename plan (handoff "Plan"): the reason filter row (in the URL as ?reason=), select-all
	// over the server's totals, one RenamePlanRow per video, LoadMoreButton paging, and the sticky
	// bar with its inline confirm and the run's per-row job states.
	import { onDestroy, onMount, tick, untrack } from 'svelte';
	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import { page } from '$app/state';
	import { beforeNavigate, replaceState } from '$app/navigation';
	import { api, ApiError } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import { listScroll } from '#lib/listScroll.svelte.ts';
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';
	import {
		FILTERS,
		atLeastManual,
		nf,
		parseFilter,
		renameableCount,
		renames,
		type RenameMode,
		type RenamePlan,
		type RenamePlanRow,
		type RenamePreview,
		type RenameReason
	} from '#lib/rename.ts';
	import RenamePlanRowView, { type RowJob } from './RenamePlanRow.svelte';

	type Filter = RenameReason | 'all';

	let {
		mode,
		onloaded,
		onsavetemplates,
		saveLabel = 'Save templates'
	}: {
		mode: RenameMode;
		// Every load's response, so the card can show per-template counts and the provider chips.
		onloaded?: (plan: RenamePlan) => void;
		// The bar's Save button while a preview is showing.
		onsavetemplates?: () => void;
		// The card's Save label: Save templates, Save characters or Save changes (HOLODEX-633).
		saveLabel?: string;
	} = $props();

	const PAGE = 100;
	const FETCH_ALL = 500;
	const POLL_MS = 1500;
	const POLL_BATCH = 20;

	let filter = $state<Filter | null>(parseFilter(page.url.searchParams.get('reason')));
	let plan = $state<RenamePlan | null>(null);
	let rows = $state<RenamePlanRow[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');
	// An unsaved draft the plan was last computed from; null shows the saved templates.
	let preview = $state<RenamePreview | null>(null);
	const unsaved = $derived(
		saveLabel === 'Save characters' ? 'characters' : saveLabel === 'Save changes' ? 'changes' : 'templates'
	);

	const selected = new SvelteSet<number>();
	let selectedAllOf = $state<Filter | null>(null);
	let selecting = $state(false);
	let confirming = $state(false);
	let applying = $state(false);
	let barMessage = $state('');
	let renameBtn = $state<HTMLButtonElement>();
	let yesBtn = $state<HTMLButtonElement>();

	const jobs = new SvelteMap<number, RowJob>();
	let batchId = $state('');
	let appliedAt = 0;
	let pollTimer: ReturnType<typeof setTimeout> | undefined;
	let firstLoad = true;

	const active = $derived<Filter>(plan?.reason ?? filter ?? 'all');
	const canRename = $derived(atLeastManual(mode) && !preview);
	const renameable = $derived(plan ? renameableCount(plan.counts, active) : 0);
	const jobCounts = $derived.by(() => {
		let renamed = 0,
			renaming = 0,
			failed = 0;
		for (const j of jobs.values()) {
			if (j.state === 'renamed') renamed++;
			else if (j.state === 'renaming') renaming++;
			else failed++;
		}
		return { renamed, renaming, failed };
	});

	// reload recomputes the plan, from `list` when given (a preview), else from the saved settings.
	// A refused preview rejects with ApiError 422 and leaves the plan as it was.
	export async function reload(list: RenamePreview | null = preview) {
		await load(filter, list);
	}

	async function load(want: Filter | null, list: RenamePreview | null) {
		loading = true;
		error = '';
		try {
			const tpl = list ?? undefined;
			let res = await api.renamePlan({ reason: want ?? 'out-of-date', limit: PAGE }, tpl);
			// No filter in the URL: the tab opens on Out of date while it isn't empty.
			if (!want && res.filtered === 0) res = await api.renamePlan({ reason: 'all', limit: PAGE }, tpl);
			preview = list;
			plan = res;
			rows = res.rows;
			onloaded?.(res);
			// Back from a video lands where the plan was, once its rows have painted.
			if (firstLoad) {
				firstLoad = false;
				const snap = listScroll.take('file-names', res.reason);
				if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
			}
		} catch (e) {
			if (list && e instanceof ApiError && e.status === 422) throw e;
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => load(filter, null));

	// Follow ?reason= when the URL changes under the page (Back to a different filter).
	$effect(() => {
		const named = parseFilter(page.url.searchParams.get('reason'));
		untrack(() => {
			if (named && named !== filter) choose(named, false);
		});
	});

	function choose(f: Filter, write = true) {
		filter = f;
		if (write) {
			const url = new URL(page.url.href);
			url.searchParams.set('reason', f);
			replaceState(url, page.state);
		}
		load(f, preview);
	}

	async function loadMore() {
		if (!plan || loadingMore) return;
		loadingMore = true;
		try {
			const res = await api.renamePlan({ reason: active, offset: rows.length, limit: PAGE }, preview ?? undefined);
			rows = [...rows, ...res.rows];
		} catch (e) {
			error = toMessage(e);
		} finally {
			loadingMore = false;
		}
	}

	function toggle(id: number) {
		if (selected.has(id)) selected.delete(id);
		else selected.add(id);
		selectedAllOf = null;
	}

	// Select all reaches past the loaded page: it pages through every renameable id in the filter.
	async function selectAll() {
		if (selectedAllOf === active) {
			clearSelection();
			return;
		}
		selecting = true;
		try {
			const reasons: Filter[] = active === 'all' ? ['will-rename', 'out-of-date'] : [active];
			for (const reason of reasons) {
				for (let offset = 0; ; offset += FETCH_ALL) {
					const res = await api.renamePlan({ reason, offset, limit: FETCH_ALL });
					for (const r of res.rows) if (renames(r.reason)) selected.add(r.video_id);
					if (offset + FETCH_ALL >= res.filtered) break;
				}
			}
			selectedAllOf = active;
		} catch (e) {
			barMessage = toMessage(e);
		} finally {
			selecting = false;
		}
	}

	function clearSelection() {
		selected.clear();
		selectedAllOf = null;
	}

	async function askConfirm() {
		if (selected.size === 0) return;
		barMessage = '';
		confirming = true;
		await tick();
		yesBtn?.focus();
	}

	async function cancelConfirm() {
		confirming = false;
		await tick();
		renameBtn?.focus();
	}

	async function apply() {
		const ids = [...selected];
		confirming = false;
		applying = true;
		barMessage = '';
		try {
			const res = await api.renameApply(ids);
			batchId = res.batch_id;
			appliedAt = Date.now();
			for (const id of ids) jobs.set(id, { state: 'renaming' });
			clearSelection();
			schedulePoll(0);
		} catch (e) {
			barMessage = toMessage(e);
		} finally {
			applying = false;
		}
	}

	function schedulePoll(ms = POLL_MS) {
		clearTimeout(pollTimer);
		if (destroyed) return;
		pollTimer = setTimeout(poll, ms);
	}

	// poll settles the run's rows a few at a time, loaded rows first. A row the queue no longer
	// holds is renamed when its history has a rename since the apply; otherwise the apply refused
	// it because the plan moved, so it asks for a refresh.
	async function poll() {
		const loaded = new Set(rows.map((r) => r.video_id));
		const open = [...jobs.entries()].filter(([, j]) => j.state === 'renaming').map(([id]) => id);
		if (open.length === 0) return;
		open.sort((a, b) => Number(loaded.has(b)) - Number(loaded.has(a)));
		await Promise.all(
			open.slice(0, POLL_BATCH).map(async (id) => {
				try {
					const st = await api.renameStatus(id);
					if (st.queued) return;
					if (st.failed) {
						jobs.set(id, { state: 'failed', error: st.error });
						return;
					}
					const hist = await api.renameHistory(id);
					const last = hist.at(-1);
					if (last && Date.parse(last.created_at) >= appliedAt - 60_000) {
						jobs.set(id, { state: 'renamed' });
						const i = rows.findIndex((r) => r.video_id === id);
						if (i >= 0) rows[i] = { ...rows[i], name: last.new_name, current: last.new_name };
					} else {
						jobs.set(id, { state: 'failed', error: 'the name changed since the plan; refresh the plan' });
					}
				} catch {
					// a dropped poll is retried on the next tick
				}
			})
		);
		schedulePoll();
	}

	// A poll in flight at unmount must not re-arm the timer.
	let destroyed = false;
	onDestroy(() => {
		destroyed = true;
		clearTimeout(pollTimer);
	});

	beforeNavigate(() => {
		if (plan) listScroll.save('file-names', { key: plan.reason, scrollY: window.scrollY });
	});

	async function retry(id: number) {
		try {
			await api.renameRetry(id);
			jobs.set(id, { state: 'renaming' });
			schedulePoll(0);
		} catch (e) {
			barMessage = toMessage(e);
		}
	}

	async function dismiss(id: number) {
		try {
			await api.renameDismiss(id);
		} catch {
			// a failure the queue no longer holds dismisses locally
		}
		jobs.delete(id);
	}

	async function refresh() {
		for (const [id, j] of jobs) if (j.state === 'failed') jobs.delete(id);
		await load(filter, preview);
	}

	// A lock change refetches only that row, which stays in place until the next reload so it
	// doesn't jump under the cursor; the counts come from the same response.
	async function relock(id: number) {
		try {
			const res = await api.renamePlan({ videoId: id, reason: 'all', limit: 1 }, preview ?? undefined);
			const fresh = res.rows[0];
			if (plan) plan.counts = res.counts;
			if (!fresh) return;
			const i = rows.findIndex((r) => r.video_id === id);
			if (i >= 0) rows[i] = fresh;
			if (!renames(fresh.reason)) selected.delete(id);
		} catch (e) {
			error = toMessage(e);
		}
	}

	function confirmKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			cancelConfirm();
		}
	}

	const emptyLine = $derived(
		active === 'out-of-date' ? 'No names are out of date.' : active === 'all' ? 'Every file has its name.' : 'No videos here.'
	);
	const filesWord = (n: number) => (n === 1 ? 'file' : 'files');
</script>

<section class="space-y-3" aria-labelledby="plan-heading">
	<div class="space-y-1">
		<h2 id="plan-heading" class="text-xs uppercase tracking-wide text-muted">Plan</h2>
		{#if plan}
			<p class="text-sm text-muted">
				{nf.format(plan.total)} videos · names from your {preview ? `unsaved ${unsaved}` : 'saved templates'}
			</p>
		{/if}
	</div>

	{#if plan}
		<div class="flex gap-1 max-sm:-mx-4 max-sm:overflow-x-auto max-sm:px-4 sm:flex-wrap" role="group" aria-label="Filter by reason">
			{#each FILTERS as f (f.value)}
				{@const on = f.value === active}
				{@const n = f.value === 'all' ? plan.total : (plan.counts[f.value] ?? 0)}
				<button
					type="button"
					aria-pressed={on}
					class="shrink-0 whitespace-nowrap rounded-theme px-2 py-1 text-xs {on ? 'bg-surface-2 text-ink' : 'text-ink hover:bg-surface-2'}"
					onclick={() => choose(f.value)}
				>
					{f.label} <span class="text-muted">{nf.format(n)}</span>
				</button>
			{/each}
		</div>
	{/if}

	{#if loading && !plan}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error && !plan}
		<p class="py-16 text-center text-sm text-warn" role="alert">
			Couldn't load the plan. <button type="button" class="text-accent hover:underline" onclick={() => load(filter, preview)}>Retry</button>
		</p>
	{:else if plan}
		{#if canRename && renameable > 0}
			<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
				<span>
					{nf.format(selected.size)} selected of {active === 'out-of-date'
						? `${nf.format(renameable)} out of date`
						: `the ${nf.format(renameable)} you can rename`}
				</span>
				<button type="button" class="text-accent hover:underline" aria-busy={selecting} onclick={selectAll}>
					{selectedAllOf === active ? 'Clear selection' : `Select all ${nf.format(renameable)}`}
				</button>
			</div>
		{/if}

		{#if rows.length === 0}
			<p class="py-16 text-center text-sm text-muted">{loading ? 'Loading…' : emptyLine}</p>
		{:else}
			<ul class="rounded-theme border border-rule bg-surface" aria-busy={loading}>
				{#each rows as row (row.video_id)}
					<RenamePlanRowView
						{row}
						templates={plan.templates}
						selectable={canRename && renames(row.reason)}
						selected={selected.has(row.video_id)}
						job={jobs.get(row.video_id)}
						ontoggle={() => toggle(row.video_id)}
						onlock={() => relock(row.video_id)}
						onretry={() => retry(row.video_id)}
						ondismiss={() => dismiss(row.video_id)}
						onrefresh={refresh}
					/>
				{/each}
			</ul>
			<LoadMoreButton left={plan.filtered - rows.length} loading={loadingMore} noun="video" onclick={loadMore} />
		{/if}
		{#if error}
			<p class="text-sm text-warn" role="alert">{error}</p>
		{/if}

		<div class="sticky bottom-0 flex flex-wrap items-center justify-between gap-3 border-t border-rule bg-surface px-3 py-2.5">
			{#if preview}
				<p class="text-sm text-muted">Previewing unsaved {unsaved}. Save them to rename.</p>
				<button type="button" class="btn-accent rounded-theme px-3 py-1.5 text-sm" onclick={() => onsavetemplates?.()}>{saveLabel}</button>
			{:else if !atLeastManual(mode)}
				<p class="text-sm text-muted">Plan mode: nothing on disk changes. Choose Manual above to rename.</p>
			{:else if confirming}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="flex w-full flex-wrap items-center justify-between gap-3" onkeydown={confirmKey}>
					<p class="text-sm text-ink">
						Rename {nf.format(selected.size)} {filesWord(selected.size)}? You can undo each one from its video page.
					</p>
					<div class="flex items-center gap-2">
						<button
							type="button"
							class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2"
							onclick={cancelConfirm}>Cancel</button
						>
						<button
							bind:this={yesBtn}
							type="button"
							class="rounded-theme bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90"
							onclick={apply}>Yes, rename {nf.format(selected.size)}</button
						>
					</div>
				</div>
			{:else if jobs.size > 0 && selected.size === 0}
				<p class="text-sm text-muted" role="status" aria-live="polite">
					{jobCounts.renamed} renamed · {jobCounts.renaming} renaming · {jobCounts.failed} failed.
				</p>
				<a class="text-sm text-accent hover:underline" href="/owner/status{batchId ? `?batch=${encodeURIComponent(batchId)}` : ''}"
					>Open System activity</a
				>
			{:else}
				<p class="text-sm text-muted" role="status" aria-live="polite">{nf.format(selected.size)} selected</p>
				<div class="flex items-center gap-2">
					<button
						type="button"
						class="rounded-theme border border-rule px-2.5 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:border-transparent disabled:text-muted max-sm:hidden"
						disabled={selected.size === 0}
						onclick={clearSelection}>Clear</button
					>
					<button
						bind:this={renameBtn}
						type="button"
						class="rounded-theme bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90 disabled:bg-surface-2 disabled:text-muted"
						disabled={selected.size === 0 || applying}
						onclick={askConfirm}>Rename {nf.format(selected.size)}</button
					>
				</div>
			{/if}
			{#if barMessage}
				<p class="w-full text-xs text-warn" role="alert">{barMessage}</p>
			{/if}
		</div>
	{/if}
</section>
