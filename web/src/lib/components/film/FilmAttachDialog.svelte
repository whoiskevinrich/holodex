<script lang="ts">
	// Video → film attach (design handoff §3b, RD9's lighter picker) — dialog chrome
	// (backdrop, focus trap, Esc-to-close, trigger-focus restore, animation) comes
	// from PickerShell; this component owns only the two-step body: result rows
	// carry a poster thumb + year (not a plain name row), and confirming a film
	// advances to a second in-dialog step (scene number / full-film) instead of
	// attaching immediately. Tokens only; QA 3 skins.
	import { api } from '$lib/api';
	import { toMessage, monogram } from '$lib/format';
	import type { Film, FilmSceneCollision } from '$lib/types';
	import PickerShell, { focusOptionIn } from '$lib/components/entity/PickerShell.svelte';

	let {
		videoId,
		onclose,
		onattached
	}: {
		videoId: number;
		onclose: () => void;
		onattached: () => void;
	} = $props();

	let step = $state<'search' | 'attach'>('search');
	let query = $state('');
	let candidates = $state<Film[]>([]);
	let active = $state(0);
	let loading = $state(false);
	let loadError = $state('');
	let chosen = $state<Film | null>(null);
	let sceneNumber = $state('');
	let isFullFilm = $state(false);
	let attaching = $state(false);
	let attachError = $state('');
	let creating = $state(false);
	let createError = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let searchInput = $state<HTMLInputElement | null>(null);

	$effect(() => {
		searchInput?.focus();
	});

	let timer: ReturnType<typeof setTimeout> | undefined;
	function onInput() {
		clearTimeout(timer);
		const q = query.trim();
		if (q.length < 2) {
			candidates = [];
			return;
		}
		timer = setTimeout(() => void search(q), 300);
	}

	let searchId = 0;
	async function search(q: string) {
		const id = ++searchId;
		loading = true;
		loadError = '';
		try {
			const res = await api.listFilms({ q });
			if (id !== searchId) return;
			candidates = res.items ?? [];
			active = 0;
		} catch (e) {
			if (id !== searchId) return;
			loadError = toMessage(e);
			candidates = [];
		} finally {
			if (id === searchId) loading = false;
		}
	}

	function choose(f: Film) {
		chosen = f;
		attachError = '';
		step = 'attach';
	}

	// createFilm is get-or-create on (name, year) (api.ts) — a duplicate submit
	// (e.g. double-click) resolves to the same existing film rather than erroring,
	// so this needs no separate "already exists" handling.
	async function createNew() {
		const name = query.trim();
		if (name.length < 2) return;
		creating = true;
		createError = '';
		try {
			const { film } = await api.createFilm(name);
			choose(film);
		} catch (e) {
			createError = toMessage(e);
		} finally {
			creating = false;
		}
	}

	function back() {
		step = 'search';
		attachError = '';
	}

	function focusOption(i: number) {
		active = i;
		focusOptionIn(dialogEl, 'film-attach-opt', i);
	}

	function onSearchKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && candidates.length) {
			e.preventDefault();
			focusOption(0);
		}
	}

	function onOptionKey(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			choose(candidates[i]);
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusOption((i + 1) % candidates.length);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) searchInput?.focus();
			else focusOption(i - 1);
		}
	}

	async function confirm() {
		if (!chosen) return;
		// bind:value on type="number" coerces to a Number (or '' when cleared) — sceneNumber
		// is not always a string despite the `$state('')` default, so don't call .trim() on it.
		// isFullFilm always wins: the input is disabled but not cleared when checked, so a
		// scene number typed before toggling "entire film" must not be sent alongside it.
		const n = isFullFilm || sceneNumber === '' ? null : Number(sceneNumber);
		if (n !== null && (!Number.isInteger(n) || n <= 0)) {
			attachError = 'Scene number must be a positive whole number, or left blank.';
			return;
		}
		attaching = true;
		attachError = '';
		try {
			const res = await api.attachFilmVideo(chosen.id, videoId, n, isFullFilm);
			if (res.conflict) {
				const occ: FilmSceneCollision = res.conflict;
				attachError = `Scene ${sceneNumber} is already "${occ.video_title}".`;
				return;
			}
			onattached();
			onclose();
		} catch (e) {
			attachError = toMessage(e);
		} finally {
			attaching = false;
		}
	}
</script>

<PickerShell titleId="film-attach-title" {onclose} bind:dialogEl>
	{#snippet header()}
		<h2 id="film-attach-title" class="skin-title text-lg font-semibold text-ink">
			{step === 'search' ? 'Attach to a film' : chosen?.name}
		</h2>
	{/snippet}

	{#snippet children()}
		{#if step === 'search'}
			<!-- svelte-ignore a11y_role_has_required_aria_props -->
			<input
				bind:this={searchInput}
				bind:value={query}
				oninput={onInput}
				onkeydown={onSearchKey}
				role="combobox"
				aria-expanded={candidates.length > 0}
				aria-controls="film-attach-candidates"
				placeholder="Search films by name…"
				class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
			/>

			<p class="mt-2 text-xs text-muted" aria-live="polite">
				{#if loading}
					Searching…
				{:else if loadError}
					<span class="text-warn">{loadError}</span>
				{:else if query.trim().length < 2}
					Type at least two characters to search.
				{:else if candidates.length}
					{candidates.length} match{candidates.length === 1 ? '' : 'es'}
				{:else}
					No films match "{query.trim()}".
				{/if}
			</p>

			{#if query.trim().length >= 2}
				<button
					onclick={createNew}
					disabled={creating}
					class="btn-ghost mt-2 w-full px-3 py-1.5 text-left text-sm"
				>
					{creating ? 'Creating…' : `+ Create "${query.trim()}" as a new film`}
				</button>
				{#if createError}
					<p class="mt-1 text-sm text-warn">{createError}</p>
				{/if}
			{/if}

			<ul id="film-attach-candidates" role="listbox" aria-label="Films" class="mt-2 flex-1 overflow-y-auto">
				{#each candidates as c, i (c.id)}
					<li
						id="film-attach-opt-{i}"
						role="option"
						tabindex={i === active ? 0 : -1}
						aria-selected={i === active}
						onclick={() => choose(c)}
						onkeydown={(e) => onOptionKey(e, i)}
						onfocus={() => (active = i)}
						onmouseenter={() => (active = i)}
						class="flex cursor-pointer items-center gap-3 rounded-theme border-l-2 px-3 py-2 {i === active
							? 'border-accent bg-surface-2'
							: 'border-transparent'}"
					>
						<span
							class="flex aspect-[2/3] w-8 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate"
						>
							<span class="font-display text-xs font-semibold text-logo-plate-ink" aria-hidden="true"
								>{monogram(c.name)}</span
							>
						</span>
						<span class="min-w-0 flex-1 truncate text-sm text-ink">{c.name}</span>
						{#if c.year}<span class="shrink-0 text-xs text-muted">{c.year}</span>{/if}
					</li>
				{/each}
			</ul>
		{:else if chosen}
			<div class="space-y-3">
				<div class="flex items-center gap-3">
					<span
						class="flex aspect-[2/3] w-10 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate"
					>
						<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true"
							>{monogram(chosen.name)}</span
						>
					</span>
					<span class="text-sm text-ink">{chosen.name}{chosen.year ? ` (${chosen.year})` : ''}</span>
				</div>

				<label class="block text-sm text-ink">
					Scene number
					<input
						type="number"
						min="1"
						bind:value={sceneNumber}
						disabled={isFullFilm}
						placeholder="Unnumbered"
						class="mt-1 w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent disabled:opacity-50"
					/>
				</label>

				<label class="flex items-start gap-2 text-sm text-ink">
					<input type="checkbox" bind:checked={isFullFilm} class="mt-0.5" />
					<span>This file represents the entire film</span>
				</label>
				{#if isFullFilm}
					<p class="text-xs text-muted">
						This file will be marked as the film's full-file source. Once full-film hiding ships,
						it will no longer appear in Browse, search, or entity pages while Films is enabled —
						its own page will stay reachable.
					</p>
				{/if}

				{#if attachError}
					<p class="text-sm text-warn">{attachError}</p>
				{/if}

				<div class="flex items-center justify-between pt-1">
					<button onclick={back} class="btn-ghost px-3 py-1.5 text-sm">← Back</button>
					<button onclick={confirm} disabled={attaching} class="btn-accent px-3 py-1.5 text-sm">
						{attaching ? 'Attaching…' : 'Attach'}
					</button>
				</div>
			</div>
		{/if}
	{/snippet}
</PickerShell>
