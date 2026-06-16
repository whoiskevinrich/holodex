<script lang="ts">
	// Merge picker (F23, ADR-036): a modal to choose another person to fold into the
	// current (canonical) one. Two steps — search/pick a person, then an INFORMED
	// confirm showing both video counts (never a silent merge of possibly-distinct
	// same-named people). role=combobox + role=listbox with roving tabindex; Tab and
	// ↑/↓ move through results, Enter/Space/click pick, Esc closes, focus trapped +
	// returned. Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { videoCount } from '$lib/format';
	import type { Person } from '$lib/types';

	let {
		canonicalId,
		canonicalName,
		onclose,
		onmerged
	}: {
		canonicalId: number;
		canonicalName: string;
		onclose: () => void;
		onmerged: (person: Person) => void;
	} = $props();

	let query = $state('');
	let all = $state<Person[]>([]);
	let active = $state(0);
	let loading = $state(true);
	let merging = $state(false);
	let error = $state('');
	let selected = $state<Person | null>(null);
	let input = $state<HTMLInputElement | null>(null);
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	const listId = 'merge-people';

	// Candidate list: every other person, name-filtered client-side (personal-library
	// scale — no dedicated search endpoint needed).
	const results = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return all
			.filter((p) => p.id !== canonicalId && (!q || p.name.toLowerCase().includes(q)))
			.slice(0, 50);
	});

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		api
			.listPeople('name')
			.then((res) => (all = res.items ?? []))
			.catch((e) => (error = toMessage(e)))
			.finally(() => {
				loading = false;
				input?.focus();
			});
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

	function pick(p: Person | undefined) {
		if (!p) return;
		selected = p; // move to the confirm step
		error = '';
	}

	async function doMerge() {
		if (!selected || merging) return;
		merging = true;
		error = '';
		try {
			const res = await api.mergePersons(canonicalId, selected.id);
			onmerged(res.person);
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
			pick(results[active]);
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
		dialogEl?.querySelector<HTMLElement>(`#merge-opt-${i}`)?.focus();
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
		aria-labelledby="merge-title"
	>
		<div class="mb-3 flex items-start justify-between gap-3">
			<h2 id="merge-title" class="skin-title text-lg font-semibold text-ink">
				Merge into {canonicalName}
			</h2>
			<button onclick={onclose} aria-label="Close" class="rounded-theme px-2 py-0.5 text-muted hover:text-ink">✕</button>
		</div>

		{#if !selected}
			<!-- Step 1: pick a person -->
			<!-- svelte-ignore a11y_role_has_required_aria_props -->
			<input
				bind:this={input}
				bind:value={query}
				onkeydown={onKey}
				role="combobox"
				aria-expanded={results.length > 0}
				aria-controls={listId}
				placeholder="Find the person to merge in…"
				class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
			/>

			<p class="mt-2 text-xs text-muted" aria-live="polite">
				{#if loading}
					Loading people…
				{:else if error}
					<span class="text-warn">{error}</span>
				{:else if results.length}
					{results.length} {results.length === 1 ? 'person' : 'people'} — choose who to fold into {canonicalName}
				{:else}
					No other people{query.trim() ? ` match “${query.trim()}”` : ''}.
				{/if}
			</p>

			<ul id={listId} role="listbox" aria-label="People" class="mt-2 flex-1 overflow-y-auto">
				{#each results as p, i (p.id)}
					<li
						id="merge-opt-{i}"
						role="option"
						tabindex={i === active ? 0 : -1}
						aria-selected={i === active}
						onclick={() => pick(p)}
						onkeydown={(e) => onOptionKey(e, i)}
						onfocus={() => (active = i)}
						onmouseenter={() => (active = i)}
						class="flex cursor-pointer items-center justify-between gap-2 rounded-theme border-l-2 px-3 py-2 {i === active
							? 'border-accent bg-surface-2'
							: 'border-transparent'}"
					>
						<span class="truncate text-sm text-ink">{p.name}</span>
						<span class="shrink-0 text-xs text-muted">{videoCount(p.video_count ?? 0)}</span>
					</li>
				{/each}
			</ul>
		{:else}
			<!-- Step 2: informed confirm -->
			<p class="text-sm text-ink">
				Merge <span class="font-semibold">{selected.name}</span> ({videoCount(selected.video_count ?? 0)})
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
