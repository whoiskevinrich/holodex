<script lang="ts">
	// Category assign/remove picker (HOLODEX-240, ADR-077 §4): a new sibling of
	// EntityPicker, not a fork of it — assign/remove are single-step and
	// reversible (unlike merge's two-step, irreversible fold-and-delete), so
	// there's no informed-confirm step here. Copies EntityPicker's search/
	// listbox/keyboard shell verbatim (role=combobox + role=listbox with roving
	// tabindex; Tab and ↑/↓ move through results, Enter/Space/click pick, Esc
	// closes, focus trapped + returned). Two callers: the /tags Manage-bar bulk
	// actions and a tag pill's own ⋯ menu "Add to category…" (single tag).
	// Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage, tagCount, filterByName } from '$lib/format';
	import type { Category } from '$lib/types';

	let {
		tagIds,
		mode,
		categories,
		onclose,
		onapplied
	}: {
		tagIds: number[];
		mode: 'add' | 'remove';
		// The candidate list to search/pick from. 'add' passes every category
		// (plus an inline create option); 'remove' passes only the categories
		// that intersect the selected tags' current memberships (the caller
		// skips opening this picker at all when that set is empty).
		categories: Category[];
		onclose: () => void;
		onapplied: () => void;
	} = $props();

	let query = $state('');
	let active = $state(0);
	let applying = $state(false);
	let error = $state('');
	let input = $state<HTMLInputElement | null>(null);
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	const listId = 'category-picker-options';
	const title = $derived(mode === 'add' ? 'Add to category' : 'Remove from category');

	const results = $derived(filterByName(categories, query).slice(0, 50));
	// 'add' offers an inline create row once the query has no exact-name match
	// among the candidates already shown.
	const exactMatch = $derived(categories.some((c) => c.name.toLowerCase() === query.trim().toLowerCase()));
	const showCreate = $derived(mode === 'add' && query.trim() !== '' && !exactMatch);
	// Roving-tabindex index space spans results then the trailing create row.
	const optionCount = $derived(results.length + (showCreate ? 1 : 0));

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		input?.focus();
		return () => trigger?.focus?.();
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('input, button, [tabindex="0"]')].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
		);
		if (f.length === 0) return;
		const first = f[0];
		const last = f[f.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	async function pickExisting(c: Category) {
		if (applying) return;
		applying = true;
		error = '';
		try {
			if (mode === 'add') await api.assignCategoryTags(c.id, tagIds);
			else await api.unassignCategoryTags(c.id, tagIds);
			onapplied();
			onclose();
		} catch (e) {
			error = toMessage(e);
			applying = false;
		}
	}

	async function createAndAssign() {
		const name = query.trim();
		if (!name || applying) return;
		applying = true;
		error = '';
		try {
			const { category } = await api.createCategory(name);
			await api.assignCategoryTags(category.id, tagIds);
			onapplied();
			onclose();
		} catch (e) {
			error = toMessage(e);
			applying = false;
		}
	}

	function pickAt(i: number) {
		if (i < results.length) pickExisting(results[i]);
		else if (showCreate) createAndAssign();
	}

	function onKey(e: KeyboardEvent) {
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
		dialogEl?.querySelector<HTMLElement>(`#category-opt-${i}`)?.focus();
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="merge-pop flex max-h-[80vh] w-full max-w-lg flex-col rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="category-picker-title"
	>
		<div class="mb-3 flex items-start justify-between gap-3">
			<div>
				<h2 id="category-picker-title" class="skin-title text-lg font-semibold text-ink">{title}</h2>
				<p class="text-xs text-muted">{tagCount(tagIds.length)} selected</p>
			</div>
			<button onclick={onclose} aria-label="Close" class="rounded-theme px-2 py-0.5 text-muted hover:text-ink">✕</button>
		</div>

		<!-- svelte-ignore a11y_role_has_required_aria_props -->
		<input
			bind:this={input}
			bind:value={query}
			onkeydown={onKey}
			role="combobox"
			aria-expanded={optionCount > 0}
			aria-controls={listId}
			placeholder={mode === 'add' ? 'Find or create a category…' : 'Find a category…'}
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<p class="mt-2 text-xs text-muted" aria-live="polite">
			{#if error}
				<span class="text-warn">{error}</span>
			{:else if optionCount}
				{results.length}
				{results.length === 1 ? 'category' : 'categories'} — pick one to
				{mode === 'add' ? 'add every selected tag to' : 'remove every selected tag from'}
			{:else}
				No categories{query.trim() ? ` match “${query.trim()}”` : ''}.
			{/if}
		</p>

		<ul id={listId} role="listbox" aria-label="Categories" class="mt-2 flex-1 overflow-y-auto">
			{#each results as c, i (c.id)}
				<li
					id="category-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={i === active}
					onclick={() => pickExisting(c)}
					onkeydown={(ev) => onOptionKey(ev, i)}
					onfocus={() => (active = i)}
					onmouseenter={() => (active = i)}
					class="flex cursor-pointer items-center justify-between gap-2 rounded-theme border-l-2 px-3 py-2 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<span class="truncate text-sm text-ink">{c.name}</span>
					<span class="shrink-0 text-xs text-muted">{tagCount(c.tag_count)}</span>
				</li>
			{/each}
			{#if showCreate}
				{@const i = results.length}
				<li
					id="category-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={i === active}
					onclick={createAndAssign}
					onkeydown={(ev) => onOptionKey(ev, i)}
					onfocus={() => (active = i)}
					onmouseenter={() => (active = i)}
					class="flex cursor-pointer items-center gap-1 rounded-theme border-l-2 px-3 py-2 text-sm text-accent {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					+ Create “{query.trim()}”
				</li>
			{/if}
		</ul>

		{#if applying}
			<p class="mt-2 text-xs text-muted">Working…</p>
		{/if}
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		.merge-pop {
			animation: merge-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes merge-rise {
		from {
			opacity: 0;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
</style>
