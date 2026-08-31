<script lang="ts">
	// Film → video attach (film-side, bulk) — design handoff §4/RD9's heavier picker.
	// Not a reuse of FilmAttachDialog: the search space is the whole video library
	// (tens of thousands of files) vs. a few hundred films, so this needs default-
	// unattached scope, studio/cast filter chips, free-text search, and multi-select
	// bulk attach — genuinely new scope per the spec's own framing. Dialog chrome
	// (backdrop, focus trap, Esc-to-close, trigger-focus restore, animation) comes
	// from PickerShell, widened via widthClass/paddingClass for this dialog's larger
	// candidate list — result shape and multi-select commit still differ from
	// FilmAttachDialog, which is why this stays its own component. Tokens only; QA 3
	// skins.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage, resolutionBucket } from '$lib/format';
	import type { FilmSceneCollision, FilmVideoCandidate, Person, Studio } from '$lib/types';
	import PickerShell, { focusOptionIn } from '$lib/components/entity/PickerShell.svelte';

	let {
		filmId,
		filmName,
		filmStudios,
		filmCast,
		onclose,
		onattached
	}: {
		filmId: number;
		filmName: string;
		filmStudios: Studio[];
		filmCast: Person[];
		onclose: () => void;
		onattached: () => void;
	} = $props();

	let query = $state(filmName);
	let studioFilter = $state<number | null>(null);
	let castFilter = $state<number | null>(null);
	let allVideos = $state(false); // default scope: unattached-only
	let results = $state<FilmVideoCandidate[]>([]);
	let selected = $state<Set<number>>(new Set());
	let active = $state(0);
	let loading = $state(false);
	let loadError = $state('');
	let startingSceneNumber = $state('');
	let committing = $state(false);
	let commitError = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let searchInput = $state<HTMLInputElement | null>(null);

	$effect(() => {
		searchInput?.focus();
	});

	let started = false;
	$effect(() => {
		if (started) return;
		started = true;
		void search();
	});

	let searchId = 0;
	async function search() {
		const id = ++searchId;
		loading = true;
		loadError = '';
		try {
			const res = await api.filmVideoCandidates(filmId, {
				q: query.trim() || undefined,
				studioId: studioFilter ?? undefined,
				personId: castFilter ?? undefined,
				unattached: !allVideos
			});
			if (id !== searchId) return;
			results = res.items ?? [];
			// commit() only sends selected ids that are present in `results` (it numbers
			// scenes by their order there), so a selection from a prior search/filter that
			// fell out of the new results would otherwise silently be dropped from the
			// batch while the "N selected" footer count kept counting it. Prune to match.
			selected = new Set([...selected].filter((id) => results.some((c) => c.video.id === id)));
			active = 0;
		} catch (e) {
			if (id !== searchId) return;
			loadError = toMessage(e);
			results = [];
		} finally {
			if (id === searchId) loading = false;
		}
	}

	let timer: ReturnType<typeof setTimeout> | undefined;
	function onQueryInput() {
		clearTimeout(timer);
		timer = setTimeout(() => void search(), 300);
	}

	function toggleStudio(id: number) {
		studioFilter = studioFilter === id ? null : id;
		void search();
	}
	function toggleCast(id: number) {
		castFilter = castFilter === id ? null : id;
		void search();
	}
	function toggleAllVideos() {
		allVideos = !allVideos;
		void search();
	}

	function toggle(videoId: number) {
		const next = new Set(selected);
		if (next.has(videoId)) next.delete(videoId);
		else next.add(videoId);
		selected = next;
	}

	function selectAllVisible() {
		const next = new Set(selected);
		for (const c of results) next.add(c.video.id);
		selected = next;
	}

	function focusOption(i: number) {
		active = i;
		focusOptionIn(dialogEl, 'film-bulk-opt', i);
	}

	function onSearchKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && results.length) {
			e.preventDefault();
			focusOption(0);
		}
	}

	function onOptionKey(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			toggle(results[i].video.id);
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusOption(Math.min(i + 1, results.length - 1));
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) searchInput?.focus();
			else focusOption(i - 1);
		}
	}

	// All-or-nothing (P0-9/RD9): a scene-number collision anywhere in the batch
	// rejects the whole commit; naming the first occupant, selection stays intact.
	// An empty starting scene number attaches the whole selection unnumbered
	// (design handoff §4c) rather than blocking the commit.
	async function commit() {
		// bind:value on type="number" coerces to a Number (or '' when cleared) --
		// startingSceneNumber is not always a string despite the $state('') default,
		// so don't call .trim() on it (see FilmAttachDialog.svelte's confirm()).
		let n: number | null = null;
		if (startingSceneNumber !== '') {
			n = Number(startingSceneNumber);
			if (!Number.isInteger(n) || n <= 0) {
				commitError = 'Starting scene number must be positive, or left blank for unnumbered.';
				return;
			}
		}
		// Numbered sequentially in the order shown below, not selection order.
		const orderedIds = results.filter((c) => selected.has(c.video.id)).map((c) => c.video.id);
		if (orderedIds.length === 0) return;
		committing = true;
		commitError = '';
		try {
			const res = await api.bulkAttachFilmVideos(filmId, orderedIds, n);
			if (res.conflict) {
				const occ: FilmSceneCollision = res.conflict;
				commitError = `One of the scene numbers in this batch is already taken by "${occ.video_title}".`;
				return;
			}
			onattached();
			onclose();
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			committing = false;
		}
	}
</script>

<PickerShell titleId="film-bulk-title" {onclose} bind:dialogEl widthClass="max-w-3xl" paddingClass="py-[6vh]">
	{#snippet header()}
		<h2 id="film-bulk-title" class="skin-title text-lg font-semibold text-ink">
			Attach videos to "{filmName}"
		</h2>
	{/snippet}

	{#snippet children()}
		<div class="mb-2 flex flex-wrap items-center gap-1.5">
			{#each filmStudios as s (s.id)}
				<button
					onclick={() => toggleStudio(s.id)}
					class="rounded-full border px-2.5 py-0.5 text-xs {studioFilter === s.id
						? 'border-accent bg-accent text-accent-ink'
						: 'border-rule text-muted hover:text-ink'}"
				>
					{s.name}
				</button>
			{/each}
			{#each filmCast as p (p.id)}
				<button
					onclick={() => toggleCast(p.id)}
					class="rounded-full border px-2.5 py-0.5 text-xs {castFilter === p.id
						? 'border-accent bg-accent text-accent-ink'
						: 'border-rule text-muted hover:text-ink'}"
				>
					{p.name}
				</button>
			{/each}
			<label class="ml-auto flex items-center gap-1.5 text-xs text-muted">
				<input type="checkbox" checked={allVideos} onchange={toggleAllVideos} />
				Include already-attached videos
			</label>
		</div>

		<input
			bind:this={searchInput}
			bind:value={query}
			oninput={onQueryInput}
			onkeydown={onSearchKey}
			placeholder="Search by filename…"
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<div class="mt-2 flex items-center justify-between text-xs text-muted">
			<p aria-live="polite">
				{#if loading}
					Searching…
				{:else if loadError}
					<span class="text-warn">{loadError}</span>
				{:else}
					{results.length} candidate{results.length === 1 ? '' : 's'}
				{/if}
			</p>
			{#if results.length}
				<button onclick={selectAllVisible} class="btn-quiet px-2 py-1">Select all visible</button>
			{/if}
		</div>

		<ul role="listbox" aria-label="Candidate videos" aria-multiselectable="true" class="mt-2 flex-1 overflow-y-auto">
			{#each results as c, i (c.video.id)}
				<li
					id="film-bulk-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={selected.has(c.video.id)}
					onclick={() => toggle(c.video.id)}
					onkeydown={(e) => onOptionKey(e, i)}
					onfocus={() => (active = i)}
					class="flex cursor-pointer items-center gap-2 rounded-theme border-l-2 px-3 py-2 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<input type="checkbox" checked={selected.has(c.video.id)} tabindex="-1" class="pointer-events-none" />
					<img
						src={c.video.thumbnail_url || api.thumbnailURL(c.video.id)}
						alt=""
						class="h-10 w-16 shrink-0 rounded-theme border border-rule object-cover"
					/>
					<span class="min-w-0 flex-1 truncate text-sm text-ink">{c.video.title}</span>
					{#if c.video.width > 0}
						<span class="shrink-0 rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink"
							>{resolutionBucket(c.video.width)}</span
						>
					{/if}
					{#each c.already_attached.filter((a) => a.film_id !== filmId) as a (a.film_id)}
						<span class="shrink-0 text-xs text-muted">Also in: {a.film_name}</span>
					{/each}
				</li>
			{/each}
		</ul>

		<div class="mt-3 flex flex-wrap items-center gap-2 border-t border-rule pt-3">
			<span class="text-sm text-ink">{selected.size} selected</span>
			<label class="flex items-center gap-1.5 text-sm text-muted">
				Starting scene #
				<input
					type="number"
					min="1"
					bind:value={startingSceneNumber}
					class="w-20 rounded-theme border border-rule bg-surface px-2 py-1 text-sm text-ink outline-none focus:border-accent"
				/>
			</label>
			<span class="text-xs text-muted"
				>Numbered sequentially from that value in the order shown above, or attached unnumbered if left
				blank.</span
			>
			<button
				onclick={commit}
				disabled={committing || selected.size === 0}
				class="btn-accent ml-auto px-3 py-1.5 text-sm"
			>
				{committing ? 'Attaching…' : `Attach ${selected.size || ''}`}
			</button>
		</div>
		{#if commitError}
			<p class="mt-2 text-sm text-warn">{commitError}</p>
		{/if}
	{/snippet}
</PickerShell>
