<script lang="ts">
	// Video↔tag attach/detach popover (HOLODEX-287, ADR-088) — the multi-select sibling of
	// PersonPicker (Tags is a SET like People, not a single value like Studio): shows what's
	// already attached (each independently removable) alongside search/create, and stays open
	// across commits. No role machinery (a tag has no per-row role, unlike Actor/Director) and
	// no `verdict` prop (addVideoTag/removeVideoTag have no conflict-body union type, unlike
	// curateMedia's 409 collision). Absorbs the one thing neither sibling has: the page's
	// former inline near-miss advisory (F43 P1-5) — the near-miss check is a read, so this
	// component calls api.nearMiss directly rather than taking it as a prop, mirroring how
	// AliasPanel.svelte owns its own flagNearMiss read. See docs/design/tag-picker-handoff.md.
	import { api, ApiError } from '$lib/api';
	import { toMessage, videoCount } from '$lib/format';
	import type { EntityRef } from '$lib/types';
	import PickerShell, { focusOptionIn } from './PickerShell.svelte';

	let {
		tags,
		isOwner,
		attach,
		detach,
		busyKey = $bindable(null)
	}: {
		tags: EntityRef[];
		isOwner: boolean;
		attach: (name: string) => Promise<{ tag: EntityRef }>;
		detach: (tagId: number) => Promise<void>;
		// Shared with the video page's own tag chip row (mirrors PersonPicker's busyKey) —
		// both surfaces detach the same video's tags, so they must share one busy gate or a
		// page-row remove and a picker commit can race on the same tag.
		busyKey?: number | 'create' | null;
	} = $props();

	let open = $state(false);
	let commitError = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let input = $state<HTMLInputElement | null>(null);

	let query = $state('');
	let candidates = $state<EntityRef[]>([]);
	let active = $state(0);
	let searchLoading = $state(false);
	let searchError = $state('');
	let searchId = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;

	// Post-commit near-miss advisory (F43 P1-5), migrated from the page's former
	// tagNearMiss/tagJustAdded — justAdded remembers which tag the attach resolved to, so
	// "Use existing" knows which link to drop when swapping onto the look-alike.
	let nearMiss = $state<EntityRef | null>(null);
	let justAdded = $state<EntityRef | null>(null);

	const trimmedQuery = $derived(query.trim());
	const attachedIds = $derived(new Set(tags.map((t) => t.id)));
	const showCreateRow = $derived(
		trimmedQuery.length >= 2 && !candidates.some((c) => c.name.toLowerCase() === trimmedQuery.toLowerCase())
	);
	const optionCount = $derived(candidates.length + (showCreateRow ? 1 : 0));

	function openPicker() {
		query = '';
		candidates = [];
		active = 0;
		searchError = '';
		commitError = '';
		nearMiss = null;
		justAdded = null;
		open = true;
		Promise.resolve().then(() => input?.focus());
	}

	function closePicker() {
		open = false;
		clearTimeout(timer);
	}

	// Back to a clean, focused search body — the tail shared by a commit that drew no
	// near-miss and by both near-miss exits.
	function resetSearch() {
		query = '';
		candidates = [];
		Promise.resolve().then(() => input?.focus());
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
			candidates = (res.tags ?? []).filter((t) => !attachedIds.has(t.id));
			active = 0;
		} catch (e) {
			if (id !== searchId) return;
			searchError = toMessage(e);
			candidates = [];
		} finally {
			if (id === searchId) searchLoading = false;
		}
	}

	// Advisory-only fuzzy check (F43 P1-5) — a failed probe must never block the already-
	// completed attach, so swallow errors and treat them as "no suggestion."
	async function lookupNearMiss(tagId: number, name: string): Promise<EntityRef | null> {
		try {
			return (await api.nearMiss('tag', tagId, name)).near_miss;
		} catch {
			return null;
		}
	}

	async function commitAttach(busyValue: number | 'create', name: string) {
		if (busyKey !== null) return;
		busyKey = busyValue;
		commitError = '';
		try {
			const { tag } = await attach(name);
			justAdded = tag;
			nearMiss = await lookupNearMiss(tag.id, name);
			if (!nearMiss) resetSearch();
		} catch (e) {
			commitError = e instanceof ApiError && e.status === 422 ? `'${name}' is on the deny-list.` : toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	async function commitDetach(t: EntityRef) {
		if (busyKey !== null) return;
		busyKey = t.id;
		commitError = '';
		try {
			await detach(t.id);
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	// "Use existing": swap the just-added tag for the near-miss it looks like — attach-by-
	// name resolves to the near-miss's existing id (no new row), then detach the tag the
	// original add created. Sequenced, not concurrent: a failed swap-attach leaves the
	// original tag alone; only a successful one is followed by the detach (never neither,
	// never a gap where both are gone) — unchanged from the page's former inline logic.
	async function useNearMiss() {
		if (!nearMiss || !justAdded || busyKey !== null) return;
		const nearMissName = nearMiss.name;
		const justAddedId = justAdded.id;
		busyKey = justAddedId;
		commitError = '';
		try {
			await attach(nearMissName);
			await detach(justAddedId);
			nearMiss = null;
			justAdded = null;
			resetSearch();
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	function dismissNearMiss() {
		nearMiss = null;
		justAdded = null;
		resetSearch();
	}

	function pickAt(i: number) {
		if (i < candidates.length) {
			const c = candidates[i];
			void commitAttach(c.id, c.name);
		} else if (showCreateRow) {
			void commitAttach('create', trimmedQuery);
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
		focusOptionIn(dialogEl, 'tag-search-opt', i);
	}
</script>

{#if isOwner}
	<button type="button" aria-haspopup="dialog" onclick={openPicker} class="btn-quiet px-3 py-1.5 text-sm">
		+ Add tag
	</button>
{/if}

{#if open}
	<PickerShell titleId="tag-picker-title" onclose={closePicker} bind:dialogEl>
		{#snippet header()}
			<h2 id="tag-picker-title" class="skin-title text-lg font-semibold text-ink">Add tags</h2>
		{/snippet}

		{#if tags.length}
			<ul class="mb-3 flex flex-wrap gap-1.5">
				{#each tags as t (t.id)}
					<li
						class="inline-flex items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-ink"
					>
						<span class="max-w-[10rem] truncate">{t.name}</span>
						<button
							type="button"
							aria-label={`Remove tag ${t.name}`}
							disabled={busyKey === t.id}
							onclick={() => commitDetach(t)}
							class="text-muted hover:text-accent disabled:cursor-default"
						>
							{busyKey === t.id ? '…' : '×'}
						</button>
					</li>
				{/each}
			</ul>
		{/if}

		{#if nearMiss && justAdded}
			<div
				class="flex flex-wrap items-center gap-2 rounded-theme border border-rule bg-surface-2 px-3 py-2"
				aria-live="polite"
			>
				<p class="text-sm text-ink">
					Looks a lot like <span class="font-semibold">{nearMiss.name}</span>
					({videoCount(nearMiss.video_count ?? 0)}) — use that instead?
				</p>
				<button
					type="button"
					onclick={useNearMiss}
					disabled={busyKey !== null}
					class="btn-accent px-3 py-1.5 text-sm"
				>
					Use existing
				</button>
				<button
					type="button"
					onclick={dismissNearMiss}
					disabled={busyKey !== null}
					class="btn-ghost px-3 py-1.5 text-sm"
				>
					Add as new anyway
				</button>
			</div>
		{:else}
			<!-- svelte-ignore a11y_role_has_required_aria_props -->
			<input
				bind:this={input}
				bind:value={query}
				oninput={onInput}
				onkeydown={onSearchKey}
				role="combobox"
				aria-expanded={optionCount > 0}
				aria-controls="tag-search-options"
				placeholder="Search tags by name…"
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
					{candidates.length} match{candidates.length === 1 ? '' : 'es'} — Tab or ↑/↓ to choose, then click or press
					Enter
				{:else}
					No matches for "{trimmedQuery}".
				{/if}
			</p>

			<ul id="tag-search-options" role="listbox" aria-label="Tags" class="mt-2 flex-1 overflow-y-auto">
				{#each candidates as c, i (c.id)}
					<li
						id="tag-search-opt-{i}"
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
						<span class="truncate text-sm text-ink">{c.name}{busyKey === c.id ? '…' : ''}</span>
					</li>
				{/each}
				{#if showCreateRow}
					{@const i = candidates.length}
					<li
						id="tag-search-opt-{i}"
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
						<span class="text-xs text-accent">Create tag "{trimmedQuery}"{busyKey === 'create' ? '…' : ''}</span>
					</li>
				{/if}
			</ul>
		{/if}

		{#if commitError}
			<p class="mt-2 text-sm text-warn" aria-live="polite">{commitError}</p>
		{/if}
	</PickerShell>
{/if}
