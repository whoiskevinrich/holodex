<script lang="ts">
	// Local entity-search picker for the Extraction tab's "Edit…"/"Pick suggested"
	// action on People/Studio fields (F48.6c). Borrows EnrichPicker's dialog chrome,
	// roving-tabindex candidate list, and focus-trap/Esc/return-focus wiring, but
	// searches the app's own entities (GET /search) instead of a provider — there is
	// no external round trip to resolve/apply, just a name to hand back. Confirming
	// (existing entity or freeform) never writes anything itself; the caller (the
	// extraction row) stages the picked name like any other manual edit. Tokens only;
	// QA 3 skins.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { EntityKind, Person, Studio } from '$lib/types';

	let {
		kind,
		seedQuery,
		onclose,
		onselect
	}: {
		kind: Extract<EntityKind, 'person' | 'studio'>;
		seedQuery: string;
		onclose: () => void;
		onselect: (name: string) => void;
	} = $props();

	// svelte-ignore state_referenced_locally
	let query = $state(seedQuery);
	let candidates = $state<Array<{ id: number; name: string; video_count?: number }>>([]);
	let active = $state(0);
	let loading = $state(false);
	let error = $state('');
	let input = $state<HTMLInputElement | null>(null);
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	const listId = 'extraction-entity-candidates';
	const kindLabel = $derived(kind === 'person' ? 'person' : 'studio');

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		input?.focus();
		input?.select();
		if (query.trim().length >= 2) void search(query.trim());
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
		error = '';
		try {
			const res = await api.search(q);
			if (id !== searchId) return;
			const list: Array<Person | Studio> = (kind === 'person' ? res.people : res.studios) ?? [];
			candidates = list.map((e) => ({ id: e.id, name: e.name, video_count: e.video_count }));
			active = 0;
		} catch (e) {
			if (id !== searchId) return;
			error = toMessage(e);
			candidates = [];
		} finally {
			if (id === searchId) loading = false;
		}
	}

	function confirm(name: string) {
		const trimmed = name.trim();
		if (!trimmed) return;
		onselect(trimmed);
		onclose();
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && candidates.length) {
			e.preventDefault();
			focusOption(0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (candidates[active]) confirm(candidates[active].name);
			else confirm(query);
		}
	}

	function onOptionKey(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			confirm(candidates[i].name);
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusOption((i + 1) % candidates.length);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) input?.focus();
			else focusOption(i - 1);
		}
	}

	function focusOption(i: number) {
		active = i;
		dialogEl?.querySelector<HTMLElement>(`#extraction-entity-opt-${i}`)?.focus();
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
		class="entity-pick-pop flex max-h-[80vh] w-full max-w-lg flex-col rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="extraction-entity-title"
	>
		<div class="mb-3 flex items-start justify-between gap-3">
			<h2 id="extraction-entity-title" class="skin-title text-lg font-semibold text-ink">
				Choose a {kindLabel}
			</h2>
			<button
				onclick={onclose}
				aria-label="Close"
				class="rounded-theme px-2 py-0.5 text-muted hover:text-ink">✕</button
			>
		</div>

		<!-- svelte-ignore a11y_role_has_required_aria_props -->
		<input
			bind:this={input}
			bind:value={query}
			oninput={onInput}
			onkeydown={onKey}
			role="combobox"
			aria-expanded={candidates.length > 0}
			aria-controls={listId}
			placeholder="Search {kindLabel}s by name…"
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<p class="mt-2 text-xs text-muted" aria-live="polite">
			{#if loading}
				Searching…
			{:else if error}
				<span class="text-warn">{error}</span>
			{:else if query.trim().length < 2}
				Type at least two characters to search.
			{:else if candidates.length}
				{candidates.length} match{candidates.length === 1 ? '' : 'es'} — Tab or ↑/↓ to choose, then
				click or press Enter
			{:else}
				No matches for "{query.trim()}".
			{/if}
		</p>

		<ul id={listId} role="listbox" aria-label="Candidates" class="mt-2 flex-1 overflow-y-auto">
			{#each candidates as c, i (c.id)}
				<li
					id="extraction-entity-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={i === active}
					onclick={() => confirm(c.name)}
					onkeydown={(e) => onOptionKey(e, i)}
					onfocus={() => (active = i)}
					onmouseenter={() => (active = i)}
					class="cursor-pointer rounded-theme border-l-2 px-3 py-2 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<div class="flex items-center justify-between gap-2">
						<span class="truncate text-sm text-ink">{c.name}</span>
						{#if c.video_count !== undefined}
							<span class="shrink-0 text-xs text-muted">{c.video_count} video{c.video_count === 1 ? '' : 's'}</span>
						{/if}
					</div>
				</li>
			{/each}
		</ul>

		{#if query.trim().length >= 2 && !candidates.some((c) => c.name.toLowerCase() === query.trim().toLowerCase())}
			<div class="mt-2 border-t border-rule pt-2">
				<button
					onclick={() => confirm(query)}
					class="rounded-theme px-2 py-1 text-xs text-accent hover:underline"
				>
					Use "{query.trim()}" as a new {kindLabel}
				</button>
			</div>
		{/if}
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		.entity-pick-pop {
			animation: entity-pick-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes entity-pick-rise {
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
