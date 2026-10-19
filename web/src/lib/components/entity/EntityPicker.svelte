<script lang="ts">
	// Merge picker (F43, ADR-061 — generalized from F23's PersonPicker): a modal to choose
	// another entity to fold into the current (canonical) one. Two steps — search/pick, then
	// an INFORMED confirm showing both video counts (never a silent merge of possibly-distinct
	// same-named entities). Entity-generic via `entityType` (person | studio | tag); dialog
	// chrome (backdrop/focus-trap/Escape/animation) is shared with CategoryPicker via
	// PickerShell. role=combobox + role=listbox with roving tabindex; Tab and ↑/↓ move through
	// results, Enter/Space/click pick, Esc closes. Tokens only; QA Cinémathèque.
	import { onDestroy, onMount } from 'svelte';
	import { api } from '#lib/api.ts';
	import { toMessage, videoCount, refLabel } from '#lib/format.ts';
	import { ServerSearch } from '#lib/serverSearch.svelte.ts';
	import type { EntityKind, EntityRef, SearchResponse } from '#lib/types.ts';
	import PickerShell, { focusOptionIn } from './PickerShell.svelte';

	let {
		entityType,
		canonicalId,
		canonicalName,
		onclose,
		onmerged
	}: {
		entityType: EntityKind;
		canonicalId: number;
		canonicalName: string;
		onclose: () => void;
		onmerged: () => void;
	} = $props();

	// Per-entity noun for the copy — everything else is identical across the three.
	const NOUNS: Record<EntityKind, { one: string; many: string }> = {
		person: { one: 'person', many: 'people' },
		studio: { one: 'studio', many: 'studios' },
		tag: { one: 'tag', many: 'tags' },
		film: { one: 'film', many: 'films' }
	};
	const noun = $derived(NOUNS[entityType]);

	let query = $state('');
	let active = $state(0);
	let merging = $state(false);
	let error = $state('');
	let selected = $state<EntityRef | null>(null);
	let input = $state<HTMLInputElement | null>(null);
	let dialogEl = $state<HTMLElement | null>(null);

	const listId = 'merge-entities';

	// Candidates come from the server as the owner types (HOLODEX-586): GET /search's
	// group for this kind, capped server-side, exact name first. Never a whole-table read.
	const GROUP: Record<EntityKind, (r: SearchResponse) => EntityRef[] | null> = {
		person: (r) => r.people,
		studio: (r) => r.studios,
		tag: (r) => r.tags,
		film: (r) => r.films
	};
	const search = new ServerSearch<EntityRef>(async (q) => GROUP[entityType](await api.search(q)) ?? []);
	const results = $derived(search.items.filter((e) => e.id !== canonicalId));
	const typed = $derived(query.trim());

	$effect(() => search.set(query));
	// New rows (a server answer, or the clear on an empty query) start at the top.
	$effect(() => {
		void search.items;
		active = 0;
	});
	onDestroy(() => search.cancel());

	onMount(() => input?.focus());

	function pick(e: EntityRef | undefined) {
		if (!e) return;
		selected = e; // move to the confirm step
		error = '';
	}

	async function doMerge() {
		if (!selected || merging) return;
		merging = true;
		error = '';
		try {
			await api.mergeEntities(entityType, canonicalId, selected.id);
			onmerged();
			onclose();
		} catch (e) {
			error = toMessage(e);
		} finally {
			merging = false;
		}
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && results.length) {
			e.preventDefault();
			focusOption(0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			// Never pick from an older query's rows while the typed one is in flight.
			if (search.answered === typed) pick(results[active]);
		}
	}

	function onOptionKey(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			pick(results[i]);
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusOption((i + 1) % results.length);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) input?.focus();
			else focusOption(i - 1);
		}
	}

	function focusOption(i: number) {
		active = i;
		focusOptionIn(dialogEl, 'merge-opt', i);
	}
</script>

<PickerShell titleId="merge-title" {onclose} bind:dialogEl>
	{#snippet header()}
		<h2 id="merge-title" class="skin-title text-lg font-semibold text-ink">
			Merge into {canonicalName}
		</h2>
	{/snippet}

	{#if !selected}
		<!-- Step 1: pick an entity -->
		<!-- svelte-ignore a11y_role_has_required_aria_props -->
		<input
			bind:this={input}
			bind:value={query}
			onkeydown={onKey}
			role="combobox"
			aria-expanded={results.length > 0}
			aria-controls={listId}
			placeholder={`Find the ${noun.one} to merge in…`}
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<p class="mt-2 text-xs text-muted" aria-live="polite">
			{#if !typed}
				Type a name to find the {noun.one} to merge in.
			{:else if search.error}
				<span class="text-warn">{search.error}</span>
			{:else if results.length}
				{results.length}
				{results.length === 1 ? noun.one : noun.many} — choose which to fold into {canonicalName}
			{:else if search.loading}
				Searching {noun.many}…
			{:else}
				No other {noun.many} match “{typed}”.
			{/if}
		</p>

		<ul id={listId} role="listbox" aria-label={noun.many} class="mt-2 flex-1 overflow-y-auto">
			{#each results as e, i (e.id)}
				<li
					id="merge-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={i === active}
					onclick={() => pick(e)}
					onkeydown={(ev) => onOptionKey(ev, i)}
					onfocus={() => (active = i)}
					onmouseenter={() => (active = i)}
					class="flex cursor-pointer items-center justify-between gap-2 rounded-theme border-l-2 px-3 py-2 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<span class="truncate text-sm text-ink">{refLabel(e)}</span>
					<span class="shrink-0 text-xs text-muted">{videoCount(e.video_count ?? 0)}</span>
				</li>
			{/each}
		</ul>
	{:else}
		<!-- Step 2: informed confirm -->
		<p class="text-sm text-ink">
			Merge <span class="font-semibold">{refLabel(selected)}</span> ({videoCount(selected.video_count ?? 0)})
			into <span class="font-semibold">{canonicalName}</span>?
		</p>
		<p class="mt-2 text-xs text-muted">
			Their videos move to {canonicalName}, “{selected.name}” becomes an alias (still searchable),
			and the separate “{selected.name}” entry is removed. This can’t be auto-undone.
		</p>
		{#if error}
			<p class="mt-2 text-sm text-warn">{error}</p>
		{/if}
		<div class="mt-4 flex flex-wrap items-center justify-end gap-2">
			<button
				onclick={() => (selected = null)}
				disabled={merging}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				Back
			</button>
			<button
				onclick={doMerge}
				disabled={merging}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
			>
				{merging ? 'Merging…' : 'Merge'}
			</button>
		</div>
	{/if}
</PickerShell>
