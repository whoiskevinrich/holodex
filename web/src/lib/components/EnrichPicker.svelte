<script lang="ts">
	// Disambiguation picker (F22.5b): a modal listbox of provider candidates the
	// owner searches and confirms. Mirrors the search-history combobox a11y
	// (role=listbox + aria-activedescendant, ↑/↓/Enter/Esc). Tokens only; QA 3 skins.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { EnrichCandidate, EnrichedField } from '$lib/types';

	let {
		personId,
		personName,
		provider,
		onclose,
		onapplied
	}: {
		personId: number;
		personName: string;
		provider: string;
		onclose: () => void;
		onapplied: (fields: EnrichedField[]) => void;
	} = $props();

	// Seed the search box with the person's name; we want the initial value only
	// (the prop never changes for a given picker instance).
	// svelte-ignore state_referenced_locally
	let query = $state(personName);
	let candidates = $state<EnrichCandidate[]>([]);
	let active = $state(0);
	let loading = $state(false);
	let applying = $state(false);
	let error = $state('');
	let input = $state<HTMLInputElement | null>(null);

	const listId = 'enrich-candidates';

	$effect(() => {
		input?.focus();
		input?.select();
	});

	// Debounced provider search; below 2 chars we don't call.
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

	async function search(q: string) {
		loading = true;
		error = '';
		try {
			const res = await api.enrichResolve(personId, provider, q);
			candidates = res.candidates ?? [];
			active = 0;
		} catch (e) {
			error = toMessage(e);
			candidates = [];
		} finally {
			loading = false;
		}
	}

	async function confirm(c: EnrichCandidate | undefined) {
		if (!c || applying) return;
		applying = true;
		error = '';
		try {
			const res = await api.enrichApply(personId, provider, c.external_id);
			onapplied(res.enriched ?? []);
			onclose();
		} catch (e) {
			error = toMessage(e);
		} finally {
			applying = false;
		}
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onclose();
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (candidates.length) active = (active + 1) % candidates.length;
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (candidates.length) active = (active - 1 + candidates.length) % candidates.length;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			void confirm(candidates[active]);
		}
	}

	function matchLabel(c: number): { text: string; accent: boolean } {
		if (c >= 0.85) return { text: 'Strong match', accent: true };
		if (c >= 0.5) return { text: 'Possible match', accent: false };
		return { text: 'Weak match', accent: false };
	}
</script>

<!-- Backdrop: token color + opacity; click closes. -->
<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		class="enrich-pop flex max-h-[80vh] w-full max-w-lg flex-col rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="enrich-title"
	>
		<div class="mb-3 flex items-start justify-between gap-3">
			<h2 id="enrich-title" class="skin-title text-lg font-semibold text-ink">
				Enrich from {provider}
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
			aria-activedescendant={candidates.length ? `enrich-opt-${active}` : undefined}
			placeholder="Search {provider} by name…"
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<p class="mt-2 text-xs text-muted" aria-live="polite">
			{#if loading}
				Searching {provider}…
			{:else if error}
				<span class="text-warn">{error}</span>
			{:else if query.trim().length < 2}
				Type at least two characters to search.
			{:else}
				{candidates.length} match{candidates.length === 1 ? '' : 'es'}
			{/if}
		</p>

		<ul id={listId} role="listbox" aria-label="Candidates" class="mt-2 flex-1 overflow-y-auto">
			{#each candidates as c, i (c.external_id)}
				{@const m = matchLabel(c.confidence)}
				<!-- Keyboard nav is handled at the combobox input (↑/↓/Enter via
				     aria-activedescendant), the WAI-ARIA listbox pattern; per-option key
				     handlers would be redundant. -->
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="enrich-opt-{i}"
					role="option"
					aria-selected={i === active}
					onclick={() => confirm(c)}
					onmouseenter={() => (active = i)}
					class="cursor-pointer rounded-theme px-3 py-2 {i === active ? 'bg-surface-2' : ''}"
				>
					<div class="flex items-center justify-between gap-2">
						<span class="truncate text-sm text-ink">{c.label}</span>
						<span class="shrink-0 text-xs {m.accent ? 'text-accent' : 'text-muted'}">{m.text}</span>
					</div>
					{#if c.disambiguation}
						<p class="truncate text-xs text-muted" title={c.disambiguation}>{c.disambiguation}</p>
					{/if}
				</li>
			{/each}
		</ul>

		{#if applying}
			<p class="mt-2 text-xs text-muted">Enriching…</p>
		{/if}
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		.enrich-pop {
			animation: enrich-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes enrich-rise {
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
