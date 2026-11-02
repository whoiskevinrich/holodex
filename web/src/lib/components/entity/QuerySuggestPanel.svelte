<script lang="ts">
	// The nav box's panel in F87 query mode (query-language-handoff.md): in place of
	// SearchResultsPanel's tabs, suggestions for the field under the caret, or — when the
	// server refused the query — the error echo, its sentence and "Did you mean" fixes.
	// A text combobox's list: the options aren't focusable and the keyboard stays on the
	// input (aria-activedescendant, this folder's "Keyboard lists" rule).
	import { echoParts, type QueryError } from '#lib/query.ts';
	import type { QuerySuggestion } from '#lib/queryAssist.svelte.ts';

	let {
		text,
		error,
		suggestions,
		highlight,
		heading,
		listId,
		errorId,
		isOwner,
		onpick,
		onhover
	}: {
		text: string;
		error: QueryError | null;
		suggestions: QuerySuggestion[];
		highlight: number;
		/** The group header over the suggestion rows (Tags, People, Fields, Did you mean). */
		heading: string;
		listId: string;
		errorId: string;
		/** The owner-only field (matched) appears in the syntax help for the owner only. */
		isOwner: boolean;
		onpick: (i: number) => void;
		onhover: (i: number) => void;
	} = $props();

	const echo = $derived(error ? echoParts(text, error) : null);
	let syntaxOpen = $state(false);
</script>

<div
	class="absolute left-0 right-0 top-full z-50 mt-1 max-h-[70vh] overflow-y-auto rounded-theme border border-rule bg-surface shadow-lg max-sm:fixed max-sm:bottom-0 max-sm:top-[var(--header-height,3.5rem)] max-sm:mt-0 max-sm:max-h-none max-sm:rounded-none max-sm:border-x-0"
>
	{#if error && echo}
		<div class="px-3 pt-2.5 pb-1.5">
			<!-- Three text nodes sliced at the server's offsets; never {@html} (perimeter). -->
			<p class="break-all font-mono text-[13px] text-ink">
				{echo[0]}<span class="query-bad">{echo[1]}</span>{echo[2]}
			</p>
			<p id={errorId} class="mt-1.5 text-sm text-warn">{error.message}</p>
		</div>
	{/if}
	{#if suggestions.length}
		<p class="px-3 pt-2 pb-1 text-xs font-medium uppercase tracking-wide text-muted">{heading}</p>
		<ul id={listId} role="listbox" aria-label={heading}>
			{#each suggestions as s, i (s.label + i)}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id={`${listId}-${i}`}
					role="option"
					aria-selected={i === highlight}
					onmousedown={(e) => e.preventDefault()}
					onclick={() => onpick(i)}
					onmouseenter={() => onhover(i)}
					class="flex cursor-pointer items-center justify-between gap-3 px-3 py-1.5 text-sm text-ink {i === highlight
						? 'bg-surface-2'
						: 'hover:bg-surface-2'}"
				>
					<span class="truncate">{s.label}</span>
					{#if s.sub}<span class="shrink-0 text-xs text-muted">{s.sub}</span>{/if}
				</li>
			{/each}
		</ul>
	{/if}
	{#if syntaxOpen}
		<div class="border-t border-rule px-3 py-2 text-xs text-muted">
			<p>
				Fields: <span class="font-mono">tag: person: studio: category: duration: year: resolution:</span>.
				Next to each other = all must match; <span class="font-mono">OR</span>; <span class="font-mono">-</span> or
				<span class="font-mono">NOT</span> excludes; ( ) groups; quote names with spaces.
			</p>
			<ul class="mt-1 space-y-0.5 font-mono text-[13px] text-ink">
				<li>((tag:science-fiction tag:fantasy) OR tag:action) -tag:comedy person:"Harrison Ford"</li>
				{#if isOwner}<li>studio:Paramount -matched:any</li>{/if}
				<li>tag:comedy duration:&lt;2m</li>
			</ul>
		</div>
	{/if}
	<div class="flex items-center justify-between border-t border-rule px-3 py-1.5 text-xs text-muted">
		<span>Enter searches videos · Tab inserts suggestion</span>
		<button
			type="button"
			class="underline hover:text-ink"
			aria-expanded={syntaxOpen}
			onmousedown={(e) => e.preventDefault()}
			onclick={() => (syntaxOpen = !syntaxOpen)}>Syntax</button
		>
	</div>
</div>

<style>
	/* The bad part of a refused query: a wavy warn underline (handoff "error"). */
	.query-bad {
		text-decoration: underline wavy var(--warn);
		text-underline-offset: 3px;
	}
</style>
