<script lang="ts">
	// Studio relationship-edit popover (HOLODEX-271) — replaces SourceSelect's Studio
	// radiogroup with a picker that also searches the full studio library and creates
	// one inline. Composes: NameEditControl's docked-pencil/busy/error/conflict state
	// machine (:42-101, here the "editing" surface is a popover instead of an inline
	// text field), EntityPickerDialog's debounced search + create-fallback body
	// (:67-96,:216-225 — inlined rather than nested, since this uses PickerShell's
	// frame directly instead of stacking two dialogs), and CollisionOfferCard via the
	// same verdict-snippet mechanism NameEditControl uses for Video Title (HOLODEX-270's
	// collision check, generalized to Studio). Known candidates (today's SourceSelect
	// chip set) stay one click away via `sourceChips` — no regression vs today's speed.
	import type { Snippet } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { sourceChips } from '$lib/f36';
	import type { DecisionSource, ResolvedField, Studio, VideoCollisionRef } from '$lib/types';
	import PickerShell, { focusOptionIn } from './PickerShell.svelte';

	let {
		// The resolver drops a replace field with no value from any source and no standing
		// decision (internal/resolver/resolver.go) — so a video with no studio candidate at
		// all has no 'studio' entry in `resolved`. Callers pass that lookup straight through
		// (possibly undefined); the default below keeps the "add studio" affordance available
		// regardless.
		field = { canonical: 'studio', label: 'Studio', values: [] },
		hasStudio,
		isOwner,
		decide,
		verdict
	}: {
		field?: ResolvedField;
		// Whether a studio is actually linked (the caller's own entity data, e.g. `studios.length`
		// on the video detail page) — distinct from `field`, which is the resolved candidate value
		// and can be present even with nothing linked yet. Swaps the trigger between "change" (a
		// value exists to replace) and "add" (nothing does, matching the page's other empty-state
		// affordances like "+ Add tag").
		hasStudio: boolean;
		isOwner: boolean;
		// Persist a studio decision, mirroring SourceSelect's `decide` shape — a known
		// candidate passes its source with no value; search/create pass 'manual' + the
		// picked/typed name. Resolves {conflict} on a composite-key collision (HOLODEX-270),
		// the same shape commitTitle already returns for Title.
		decide: (
			source: DecisionSource,
			manualValue?: string
		) => Promise<{ ok: true } | { conflict: VideoCollisionRef }>;
		verdict?: Snippet<[VideoCollisionRef, () => void]>;
	} = $props();

	// Known-candidate chips (today's SourceSelect set): the baseline + one per distinct
	// provider value, minus the trailing Custom chip — search/create replace Custom here.
	const candidateChips = $derived(sourceChips(field, 'file').filter((c) => !c.manual));

	let open = $state(false);
	let busyKey = $state<string | null>(null);
	let commitError = $state('');
	let conflict = $state<VideoCollisionRef | null>(null);
	let pencil = $state<HTMLButtonElement | null>(null);
	let dialogEl = $state<HTMLElement | null>(null);
	let input = $state<HTMLInputElement | null>(null);

	let query = $state('');
	let candidates = $state<Studio[]>([]);
	let active = $state(0);
	let searchLoading = $state(false);
	let searchError = $state('');
	let searchId = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;

	const trimmedQuery = $derived(query.trim());
	const showCreateRow = $derived(
		trimmedQuery.length >= 2 && !candidates.some((c) => c.name.toLowerCase() === trimmedQuery.toLowerCase())
	);
	const optionCount = $derived(candidates.length + (showCreateRow ? 1 : 0));

	function focusPencil() {
		Promise.resolve().then(() => pencil?.focus());
	}

	function openPicker() {
		query = '';
		candidates = [];
		active = 0;
		searchError = '';
		commitError = '';
		open = true;
		Promise.resolve().then(() => input?.focus());
	}

	function closePicker() {
		open = false;
		clearTimeout(timer);
	}

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
		if (busyKey) return;
		busyKey = key;
		commitError = '';
		try {
			const res = await decide(source, manualValue);
			if ('conflict' in res) {
				conflict = res.conflict;
				closePicker();
				return;
			}
			closePicker();
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	function resolveConflict() {
		conflict = null;
		focusPencil();
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
			if (i === 0) input?.focus();
			else focusOption(i - 1);
		}
	}

	function focusOption(i: number) {
		active = i;
		focusOptionIn(dialogEl, 'studio-search-opt', i);
	}
</script>

{#if isOwner}
	{#if hasStudio}
		<div class="name-edit-row inline-flex items-center">
			<!-- Always visible, not hover-revealed (unlike Person/Studio/Tag name headers, which use
			     the default name-edit-pencil hover reveal): the sibling "+ Add studio" branch below is
			     already always-visible, so hiding this one until hover would make "change" harder to
			     discover than "add" for no reason. -->
			<button
				bind:this={pencil}
				type="button"
				aria-label="Change this video's studio"
				onclick={openPicker}
				class="name-edit-pencil name-edit-pencil--visible rounded-theme border border-rule p-1.5 text-muted hover:border-accent hover:text-ink"
			>
				<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z"
					/>
				</svg>
			</button>
		</div>
	{:else}
		<button bind:this={pencil} type="button" onclick={openPicker} class="btn-quiet px-3 py-1.5 text-sm">
			+ Add studio
		</button>
	{/if}
{/if}

{#if open}
	<PickerShell titleId="studio-picker-title" onclose={closePicker} bind:dialogEl>
		{#snippet header()}
			<h2 id="studio-picker-title" class="skin-title text-lg font-semibold text-ink">
				{hasStudio ? 'Change studio' : 'Add studio'}
			</h2>
		{/snippet}

		{#if candidateChips.length > 1}
			<div class="mb-3 flex flex-wrap items-center gap-1.5">
				{#each candidateChips as chip (chip.key)}
					<button
						type="button"
						disabled={busyKey !== null}
						onclick={() => commit(chip.key, chip.decisionSource)}
						class="curation-chip inline-flex max-w-full items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-muted hover:border-accent hover:text-ink disabled:cursor-default"
					>
						<span class="max-w-[10rem] truncate">{chip.value || '—'}</span>
						<span class="shrink-0 text-[0.65rem] text-muted"
							>·{chip.sources.join(' + ')}{busyKey === chip.key ? '…' : ''}</span
						>
					</button>
				{/each}
			</div>
		{/if}

		<!-- svelte-ignore a11y_role_has_required_aria_props -->
		<input
			bind:this={input}
			bind:value={query}
			oninput={onInput}
			onkeydown={onSearchKey}
			role="combobox"
			aria-expanded={optionCount > 0}
			aria-controls="studio-search-options"
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

		<ul id="studio-search-options" role="listbox" aria-label="Studios" class="mt-2 flex-1 overflow-y-auto">
			{#each candidates as c, i (c.id)}
				<li
					id="studio-search-opt-{i}"
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
					id="studio-search-opt-{i}"
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
	</PickerShell>
{/if}

{#if conflict && verdict}
	{@render verdict(conflict, resolveConflict)}
{/if}
