<script lang="ts">
	// Film-studio cascade dialog (F57, HOLODEX-285, design handoff §3) — visually mirrors
	// StudioPicker step-for-step (same PickerShell chrome, same chip/search/create body,
	// same tokens) but is its own component, not a reuse: StudioPicker's `decide` prop
	// resolves one video's {ok}|{conflict}, while this resolves N videos' mixed outcomes
	// at once via POST /films/{id}/studio/cascade (ADR-086 D2's best-effort posture). Same
	// "deliberately separate" reasoning the Films folder already applied to
	// FilmAttachDialog vs. FilmBulkAttachDialog (film/CLAUDE.md). Commits by calling
	// api.cascadeFilmStudio directly, matching FilmBulkAttachDialog's convention rather
	// than StudioPicker's caller-supplied decide callback.
	import { api } from '$lib/api';
	import { toMessage, formatYear } from '$lib/format';
	import type { DecisionSource, FilmStudioCascadeResult, Studio } from '$lib/types';
	import PickerShell, { focusOptionIn } from '$lib/components/entity/PickerShell.svelte';

	let {
		filmId,
		filmName,
		attachedVideoCount,
		currentStudios,
		videoTitles,
		onclose,
		onviewprogress
	}: {
		filmId: number;
		filmName: string;
		attachedVideoCount: number;
		// The film's already-fetched studio union (design handoff §3a: "same union query
		// the read-only view already renders; no new endpoint") — doubles as the chip set.
		currentStudios: Studio[];
		// Results only name a video by id; the page supplies titles from scenes/full_films
		// so Collision/Error rows can link out without a second fetch.
		videoTitles: Map<number, string>;
		onclose: () => void;
		// Fired when the owner clicks "View writeback progress ->" with the enqueued batch —
		// the page closes this popover and mounts WritebackBatchDialog itself (handoff §4:
		// stacking it inside PickerShell's backdrop would double the dimming).
		onviewprogress: (batchId: string, enqueued: number) => void;
	} = $props();

	let busyKey = $state<string | null>(null);
	let commitError = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let statusLine = $state<HTMLElement | null>(null);

	let query = $state('');
	let candidates = $state<Studio[]>([]);
	let active = $state(0);
	let searchLoading = $state(false);
	let searchError = $state('');
	let searchId = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;

	let results = $state<FilmStudioCascadeResult[] | null>(null);

	const trimmedQuery = $derived(query.trim());
	const showCreateRow = $derived(
		trimmedQuery.length >= 2 && !candidates.some((c) => c.name.toLowerCase() === trimmedQuery.toLowerCase())
	);
	const optionCount = $derived(candidates.length + (showCreateRow ? 1 : 0));

	const enqueued = $derived(results?.filter((r) => r.status === 'enqueued') ?? []);
	const collisions = $derived(results?.filter((r) => r.status === 'collision') ?? []);
	const errors = $derived(results?.filter((r) => r.status === 'error') ?? []);
	let batchId = $state('');

	const statusSummary = $derived(
		[
			`${enqueued.length} queued for writeback`,
			collisions.length ? `${collisions.length} skipped (collision)` : '',
			errors.length ? `${errors.length} failed` : ''
		]
			.filter(Boolean)
			.join(', ') + '.'
	);

	function onInput() {
		clearTimeout(timer);
		const q = query.trim();
		if (q.length < 2) {
			candidates = [];
			return;
		}
		timer = setTimeout(() => void search(q), 300);
	}

	async function search(q: string) {
		const id = ++searchId;
		searchLoading = true;
		searchError = '';
		try {
			const res = await api.search(q);
			if (id !== searchId) return;
			candidates = res.studios ?? [];
			active = 0;
		} catch (e) {
			if (id !== searchId) return;
			searchError = toMessage(e);
			candidates = [];
		} finally {
			if (id === searchId) searchLoading = false;
		}
	}

	async function commit(key: string, source: DecisionSource, manualValue?: string) {
		if (busyKey || attachedVideoCount === 0) return;
		busyKey = key;
		commitError = '';
		try {
			const res = await api.cascadeFilmStudio(filmId, { source, manual_value: manualValue });
			results = res.results;
			batchId = res.batch_id;
			Promise.resolve().then(() => statusLine?.focus());
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	function pickAt(i: number) {
		if (i < candidates.length) {
			const c = candidates[i];
			void commit(`search:${c.id}`, 'manual', c.name);
		} else if (showCreateRow) {
			void commit('create', 'manual', trimmedQuery);
		}
	}

	function onSearchKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && optionCount) {
			e.preventDefault();
			focusOption(0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			pickAt(active);
		}
	}

	function onOptionKey(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			pickAt(i);
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusOption((i + 1) % optionCount);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) document.getElementById('cascade-search-input')?.focus();
			else focusOption(i - 1);
		}
	}

	function focusOption(i: number) {
		active = i;
		focusOptionIn(dialogEl, 'cascade-search-opt', i);
	}

	function titleOf(videoId: number): string {
		return videoTitles.get(videoId) ?? `Video ${videoId}`;
	}
</script>

<PickerShell titleId="cascade-title" {onclose} bind:dialogEl>
	{#snippet header()}
		<h2 id="cascade-title" class="skin-title text-lg font-semibold text-ink">
			{results ? `Studio change for "${filmName}"` : 'Change studio for this film'}
		</h2>
	{/snippet}

	{#if !results}
		{#if attachedVideoCount === 0}
			<p class="text-sm text-muted">This film has no attached videos yet.</p>
		{:else}
			<p class="text-sm text-muted">
				Applies to all {attachedVideoCount} video{attachedVideoCount === 1 ? '' : 's'} attached to this film —
				any existing studio decision on those videos is overwritten.
			</p>

			{#if currentStudios.length}
				<p class="mb-1.5 mt-3 text-xs text-muted">Already used in this film</p>
				<div class="mb-3 flex flex-wrap items-center gap-1.5">
					{#each currentStudios as s (s.id)}
						<button
							type="button"
							disabled={busyKey !== null}
							onclick={() => commit(`current:${s.id}`, 'manual', s.name)}
							class="curation-chip inline-flex max-w-full items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-muted hover:border-accent hover:text-ink disabled:cursor-default"
						>
							<span class="max-w-[10rem] truncate">{s.name}{busyKey === `current:${s.id}` ? '…' : ''}</span>
						</button>
					{/each}
				</div>
			{/if}

			<!-- svelte-ignore a11y_role_has_required_aria_props -->
			<input
				id="cascade-search-input"
				bind:value={query}
				oninput={onInput}
				onkeydown={onSearchKey}
				role="combobox"
				aria-expanded={optionCount > 0}
				aria-controls="cascade-search-options"
				placeholder="Search studios by name…"
				class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
			/>

			<p class="mt-2 text-xs text-muted" aria-live="polite">
				{#if searchLoading}
					Searching…
				{:else if searchError}
					<span class="text-warn">{searchError}</span>
				{:else if trimmedQuery.length < 2}
					Type at least two characters to search.
				{:else if candidates.length}
					{candidates.length} match{candidates.length === 1 ? '' : 'es'} — Tab or ↑/↓ to choose, then click or
					press Enter
				{:else}
					No matches for "{trimmedQuery}".
				{/if}
			</p>

			<ul id="cascade-search-options" role="listbox" aria-label="Studios" class="mt-2 flex-1 overflow-y-auto">
				{#each candidates as c, i (c.id)}
					<li
						id="cascade-search-opt-{i}"
						role="option"
						tabindex={i === active ? 0 : -1}
						aria-selected={i === active}
						aria-disabled={busyKey !== null}
						onclick={() => pickAt(i)}
						onkeydown={(e) => onOptionKey(e, i)}
						onfocus={() => (active = i)}
						onmouseenter={() => (active = i)}
						class="cursor-pointer rounded-theme border-l-2 px-3 py-2 {i === active
							? 'border-accent bg-surface-2'
							: 'border-transparent'}"
					>
						<div class="flex items-center justify-between gap-2">
							<span class="truncate text-sm text-ink">{c.name}{busyKey === `search:${c.id}` ? '…' : ''}</span>
							{#if c.video_count !== undefined}
								<span class="shrink-0 text-xs text-muted">{c.video_count} video{c.video_count === 1 ? '' : 's'}</span>
							{/if}
						</div>
					</li>
				{/each}
				{#if showCreateRow}
					{@const i = candidates.length}
					<li
						id="cascade-search-opt-{i}"
						role="option"
						tabindex={i === active ? 0 : -1}
						aria-selected={i === active}
						aria-disabled={busyKey !== null}
						onclick={() => pickAt(i)}
						onkeydown={(e) => onOptionKey(e, i)}
						onfocus={() => (active = i)}
						onmouseenter={() => (active = i)}
						class="cursor-pointer rounded-theme border-l-2 px-3 py-2 text-xs text-accent {i === active
							? 'border-accent bg-surface-2'
							: 'border-transparent'}"
					>
						Use "{trimmedQuery}" as a new studio{busyKey === 'create' ? '…' : ''}
					</li>
				{/if}
			</ul>

			{#if commitError}
				<p class="mt-2 text-sm text-warn" aria-live="polite">{commitError}</p>
			{/if}
		{/if}
	{:else}
		<p bind:this={statusLine} tabindex="-1" class="text-sm text-ink outline-none" aria-live="polite">
			{statusSummary}
		</p>

		<div class="mt-3 space-y-2">
			<details open={collisions.length === 0 && errors.length === 0}>
				<summary class="cursor-pointer text-sm text-ink">Enqueued ({enqueued.length})</summary>
			</details>

			{#if collisions.length}
				<details open>
					<summary class="cursor-pointer text-sm text-ink">Collision ({collisions.length})</summary>
					<ul class="mt-1.5 space-y-1.5 pl-3">
						{#each collisions as r (r.video_id)}
							<li>
								<a href={`/media/${r.video_id}`} class="text-sm text-ink hover:text-accent">{titleOf(r.video_id)}</a>
								{#if r.conflict}
									<p class="text-xs text-muted">
										Collides with "{r.conflict.title}" — {r.conflict.people.length
											? r.conflict.people.join(', ')
											: '—'} · {formatYear(r.conflict.recorded_at) || '—'} · {r.conflict.studios.length
											? r.conflict.studios.join(', ')
											: '—'}
									</p>
								{/if}
							</li>
						{/each}
					</ul>
				</details>
			{/if}

			{#if errors.length}
				<details open>
					<summary class="cursor-pointer text-sm text-ink">Error ({errors.length})</summary>
					<ul class="mt-1.5 space-y-1.5 pl-3">
						{#each errors as r (r.video_id)}
							<li>
								<a href={`/media/${r.video_id}`} class="text-sm text-ink hover:text-accent">{titleOf(r.video_id)}</a>
								<p class="text-xs text-warn">{r.error}</p>
							</li>
						{/each}
					</ul>
				</details>
			{/if}
		</div>

		<div class="mt-3 flex flex-wrap items-center gap-2 border-t border-rule pt-3">
			{#if enqueued.length > 0}
				<button
					onclick={() => onviewprogress(batchId, enqueued.length)}
					class="btn-accent px-3 py-1.5 text-sm"
				>
					View writeback progress →
				</button>
			{/if}
			<button onclick={onclose} class="btn-ghost px-3 py-1.5 text-sm">Close</button>
			{#if enqueued.length > 0}
				<span class="text-xs text-muted">Already running in the background — closing this won't stop it.</span>
			{/if}
		</div>
	{/if}
</PickerShell>
