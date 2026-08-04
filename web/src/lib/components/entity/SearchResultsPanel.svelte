<script lang="ts">
	// Shared grouped/tabbed search results (NS1, HOLODEX-249): the nav box's live-typing
	// dropdown and the /search page body render the exact same component — "reuse, don't
	// fork" per the spec. `variant="dropdown"` caps each group at 3 rows (8 on a single
	// tab) behind a "View all" row and positions as an absolute/fixed overlay;
	// `variant="page"` renders every match, uncapped, as static page content.
	//
	// Roving tabindex for both the tab row and result rows (NS5), matching the pattern
	// established by EnrichPicker.svelte — not aria-activedescendant.
	import type { Snippet } from 'svelte';
	import type { Person, SearchResponse, Studio, Tag, Video } from '$lib/types';
	import { SEARCH_TABS, type SearchTab } from '$lib/navSearch.svelte';

	let {
		results,
		loading,
		error,
		query,
		activeTab = $bindable('all'),
		variant = 'dropdown',
		onnavigate
	}: {
		results: SearchResponse | null;
		loading: boolean;
		error: string;
		query: string;
		activeTab?: SearchTab;
		variant?: 'dropdown' | 'page';
		/** Called when a result row or "View all" link is activated — dropdown callers
		 *  use this to close the panel before the navigation completes. */
		onnavigate?: () => void;
	} = $props();

	type GroupKey = 'people' | 'videos' | 'studios' | 'tags';
	type RowItem = { id: string; label: string; sub: string; href: string };
	type Group = { key: GroupKey; label: string; rows: RowItem[]; total: number; hasMore: boolean };

	const GROUP_LABELS: Record<GroupKey, string> = {
		people: 'People',
		videos: 'Videos',
		studios: 'Studios',
		tags: 'Tags'
	};
	const VIEW_ALL_PATH: Record<GroupKey, string> = {
		people: '/people',
		videos: '/',
		studios: '/studios',
		tags: '/tags'
	};

	function personRow(p: Person): RowItem {
		return { id: `p${p.id}`, label: p.name, sub: `${p.video_count ?? 0}`, href: `/people/${p.id}` };
	}
	function studioRow(s: Studio): RowItem {
		return { id: `s${s.id}`, label: s.name, sub: `${s.video_count ?? 0}`, href: `/studios/${s.id}` };
	}
	function tagRow(t: Tag): RowItem {
		return { id: `t${t.id}`, label: t.name, sub: `${t.video_count ?? 0}`, href: `/tags/${t.id}` };
	}
	function videoRow(v: Video): RowItem {
		return { id: `v${v.id}`, label: v.title, sub: '', href: `/media/${v.id}` };
	}

	// Per-tab cap: tight (3) when "All" is sharing screen space across up to 4 groups,
	// looser (8) when a single type has the whole panel to itself. Uncapped on the page.
	const CAP_ALL = 3;
	const CAP_SINGLE = 8;

	// viewAllHref is deliberately NOT part of Group/buildGroup: it depends only on
	// `query`, which changes on every keystroke well before the debounced `results`
	// does — folding it into `groups` would rebuild every row list on each keystroke
	// instead of only when the actual data changes.
	function viewAllHref(key: GroupKey): string {
		return `${VIEW_ALL_PATH[key]}?q=${encodeURIComponent(query)}`;
	}

	function buildGroup<T>(key: GroupKey, items: T[] | null, toRow: (item: T) => RowItem): Group {
		const all = items ?? [];
		const cap = variant === 'page' ? Infinity : activeTab === 'all' ? CAP_ALL : CAP_SINGLE;
		const rows = all.slice(0, cap).map(toRow);
		return { key, label: GROUP_LABELS[key], rows, total: all.length, hasMore: all.length > rows.length };
	}

	const groups = $derived.by((): Group[] => {
		const wantedKeys: GroupKey[] = activeTab === 'all' ? ['people', 'videos', 'studios', 'tags'] : [activeTab];
		const built: Group[] = wantedKeys.map((key) => {
			if (key === 'people') return buildGroup(key, results?.people ?? null, personRow);
			if (key === 'studios') return buildGroup(key, results?.studios ?? null, studioRow);
			if (key === 'tags') return buildGroup(key, results?.tags ?? null, tagRow);
			return buildGroup(key, results?.videos ?? null, videoRow);
		});
		return built.filter((g) => g.total > 0);
	});

	const totalMatches = $derived(groups.reduce((n, g) => n + g.total, 0));
	const zeroMatch = $derived(!loading && !error && query.trim() !== '' && totalMatches === 0);

	// Cumulative start index per group, so a flat roving-tabindex index can be assigned
	// to every row + "View all" link across all rendered groups.
	const groupOffsets = $derived.by(() => {
		const offsets: number[] = [];
		let n = 0;
		for (const g of groups) {
			offsets.push(n);
			n += g.rows.length + (g.hasMore ? 1 : 0);
		}
		return offsets;
	});
	const flatCount = $derived(groups.reduce((n, g) => n + g.rows.length + (g.hasMore ? 1 : 0), 0));

	let activeRowIndex = $state(0);
	// A single root ref + data-attributes, rather than an array of per-row bind:this
	// targets — Svelte 5 flags `bind:this={arr[i]}` as a non-reactive binding, and in
	// practice it can resolve to a stale element after the flat row list reshuffles.
	// querySelector against a stable container sidesteps that entirely.
	let panelRootEl = $state<HTMLDivElement | null>(null);

	// A query/tab change reshuffles the flat row list — reset the roving index rather
	// than leaving it pointing at a row that may no longer exist.
	$effect(() => {
		groups;
		activeRowIndex = 0;
	});

	function rowAt(i: number) {
		return panelRootEl?.querySelector<HTMLAnchorElement>(`[data-row-index="${i}"]`) ?? null;
	}
	function tabAt(i: number) {
		return panelRootEl?.querySelector<HTMLButtonElement>(`[data-tab-index="${i}"]`) ?? null;
	}

	function focusRow(i: number) {
		if (flatCount === 0) return;
		activeRowIndex = Math.max(0, Math.min(i, flatCount - 1));
		rowAt(activeRowIndex)?.focus();
	}

	const activeTabIndex = $derived(SEARCH_TABS.findIndex((t) => t.key === activeTab));

	function selectTab(tab: SearchTab) {
		activeTab = tab;
	}

	function onTabKey(e: KeyboardEvent, i: number) {
		if (e.key === 'ArrowRight') {
			e.preventDefault();
			const next = (i + 1) % SEARCH_TABS.length;
			selectTab(SEARCH_TABS[next].key);
			tabAt(next)?.focus();
		} else if (e.key === 'ArrowLeft') {
			e.preventDefault();
			const prev = (i - 1 + SEARCH_TABS.length) % SEARCH_TABS.length;
			selectTab(SEARCH_TABS[prev].key);
			tabAt(prev)?.focus();
		} else if (e.key === 'ArrowDown' && flatCount > 0) {
			e.preventDefault();
			focusRow(0);
		}
	}

	function onRowKey(e: KeyboardEvent, i: number) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusRow(i + 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) tabAt(activeTabIndex)?.focus();
			else focusRow(i - 1);
		}
	}

	// Screen-reader summary of the result set, announced whenever it changes (NS1's
	// panel replaces the visual history dropdown with no other signal for AT users).
	const announcement = $derived(
		loading
			? 'Searching…'
			: error
				? `Search failed: ${error}`
				: zeroMatch
					? `No matches for "${query.trim()}"`
					: totalMatches > 0
						? `${totalMatches} result${totalMatches === 1 ? '' : 's'} across ${groups.length} categor${groups.length === 1 ? 'y' : 'ies'}`
						: ''
	);
</script>

<div bind:this={panelRootEl} class="search-panel {variant === 'dropdown' ? 'search-panel-dropdown' : ''}">
	<div
		role="tablist"
		aria-label="Search scope"
		class="flex items-center gap-1 overflow-x-auto {variant === 'dropdown'
			? 'border-t border-rule px-1 pb-1.5 pt-1.5'
			: 'pb-2'}"
	>
		{#each SEARCH_TABS as t, i (t.key)}
			<button
				data-tab-index={i}
				type="button"
				role="tab"
				id={`sr-tab-${t.key}`}
				aria-selected={activeTab === t.key}
				aria-controls={`sr-tabpanel-${t.key}`}
				tabindex={activeTab === t.key ? 0 : -1}
				onclick={() => selectTab(t.key)}
				onkeydown={(e) => onTabKey(e, i)}
				class="shrink-0 rounded-theme px-2 py-1 text-xs transition {activeTab === t.key
					? 'bg-surface-2 text-ink'
					: 'text-muted hover:text-ink'}"
			>
				{t.label}
			</button>
		{/each}
	</div>

	<div id={`sr-tabpanel-${activeTab}`} role="tabpanel" aria-labelledby={`sr-tab-${activeTab}`}>
		{#if loading}
			{#each groups.length ? groups : [{ key: 'skeleton', label: '', rows: [], total: 0, hasMore: false }] as g, gi (gi)}
				<div class="px-3 pt-2 pb-1.5" aria-hidden="true">
					{#if g.label}
						<p class="pb-1 text-xs font-medium uppercase tracking-wide text-muted">{g.label}</p>
					{/if}
					<div class="space-y-1.5">
						<div class="h-4 w-3/4 animate-pulse rounded-theme bg-surface-2"></div>
						<div class="h-4 w-1/2 animate-pulse rounded-theme bg-surface-2"></div>
					</div>
				</div>
			{/each}
		{:else if error}
			<p class="px-3 py-4 text-sm text-warn">Couldn't search: {error}</p>
		{:else if zeroMatch}
			<p class="py-6 text-center text-sm text-muted">No matches for "{query.trim()}"</p>
		{:else}
			<!-- Shared shell for a result row and a "View all" row — same roving-tabindex
			     wiring, differing only in href/styling/content. -->
			{#snippet resultRow(flatIndex: number, href: string, kind: 'row' | 'viewall', children: Snippet)}
				<li role="presentation">
					<a
						data-row-index={flatIndex}
						{href}
						role="option"
						aria-selected={flatIndex === activeRowIndex}
						tabindex={flatIndex === activeRowIndex ? 0 : -1}
						onfocus={() => (activeRowIndex = flatIndex)}
						onmouseenter={() => (activeRowIndex = flatIndex)}
						onkeydown={(e) => onRowKey(e, flatIndex)}
						onclick={() => onnavigate?.()}
						class={kind === 'viewall'
							? `block border-t border-rule px-3 py-1.5 text-xs text-muted hover:text-ink ${flatIndex === activeRowIndex ? 'bg-surface-2' : ''}`
							: `flex items-center justify-between gap-2 px-3 py-1.5 text-sm text-ink hover:bg-surface-2 ${flatIndex === activeRowIndex ? 'bg-surface-2' : ''}`}
					>
						{@render children()}
					</a>
				</li>
			{/snippet}
			{#each groups as g, gi (g.key)}
				<div role="group" aria-labelledby={activeTab === 'all' ? `sr-grp-${g.key}` : undefined}>
					{#if activeTab === 'all'}
						<p id={`sr-grp-${g.key}`} class="px-3 pt-2 pb-1 text-xs font-medium uppercase tracking-wide text-muted">
							{g.label}
						</p>
					{/if}
					<ul role="listbox" aria-label={g.label}>
						{#each g.rows as row, ri (row.id)}
							{@const flatIndex = groupOffsets[gi] + ri}
							{#snippet rowBody()}
								<span class="truncate">{row.label}</span>
								{#if row.sub}<span class="shrink-0 text-xs text-muted">{row.sub}</span>{/if}
							{/snippet}
							{@render resultRow(flatIndex, row.href, 'row', rowBody)}
						{/each}
						{#if g.hasMore}
							{@const flatIndex = groupOffsets[gi] + g.rows.length}
							{#snippet viewAllBody()}
								View all {g.total} in {g.label} →
							{/snippet}
							{@render resultRow(flatIndex, viewAllHref(g.key), 'viewall', viewAllBody)}
						{/if}
					</ul>
				</div>
			{/each}
		{/if}
	</div>
</div>

<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

<style>
	/* ≥640px: absolute dropdown matching the box's width (today's behavior). <640px:
	   a fixed full-width sheet so the panel never clips or needs horizontal scroll
	   on a phone viewport (NS1's mobile requirement). Page variant is always static. */
	.search-panel-dropdown {
		position: absolute;
		left: 0;
		right: 0;
		top: 100%;
		z-index: 50;
		margin-top: 0.25rem;
		max-height: 70vh;
		overflow-y: auto;
		border-radius: var(--radius);
		border: 1px solid var(--rule);
		background: var(--surface);
		box-shadow: 0 10px 25px -5px rgb(0 0 0 / 0.2);
	}
	@media (max-width: 639px) {
		.search-panel-dropdown {
			position: fixed;
			left: 0;
			right: 0;
			top: var(--header-height, 3.5rem);
			bottom: 0;
			max-height: none;
			margin-top: 0;
			border-radius: 0;
			border-left: none;
			border-right: none;
		}
	}
</style>
