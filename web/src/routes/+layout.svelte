<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { theme, THEMES, THEME_LABELS } from '$lib/theme.svelte';
	import { activity } from '$lib/activity.svelte';
	import { searchHistory } from '$lib/searchHistory.svelte';
	import ActivityIndicator from '$lib/components/ActivityIndicator.svelte';

	let { children } = $props();

	let searchTerm = $state('');
	let searchInput = $state<HTMLInputElement | null>(null);
	let historyOpen = $state(false);
	let activeIdx = $state(-1); // keyboard-highlighted history row, -1 = none
	// Show the dropdown only on a focused, empty box with history — so it hides the
	// instant the user types (QW1). Derived once, used by the input + the panel.
	const showHistory = $derived(historyOpen && !searchTerm.trim() && searchHistory.items.length > 0);

	// Apply the saved skin + load search history on mount.
	$effect(() => {
		theme.init();
		searchHistory.init();
	});

	// Poll system activity app-wide so the header indicator reflects background
	// work on every page (F21.5); ref-counted with the /status page.
	$effect(() => {
		activity.start();
		return () => activity.stop();
	});

	// Ctrl-/Cmd-K focuses the global search (F4.10).
	$effect(() => {
		function onKey(e: KeyboardEvent) {
			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
				e.preventDefault();
				searchInput?.focus();
			}
		}
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	// Run a search: record it in history, then navigate. Shared by form submit and
	// clicking a history row.
	function runSearch(term: string) {
		const q = term.trim();
		if (!q) return;
		searchHistory.record(q);
		historyOpen = false;
		activeIdx = -1;
		goto(`/search?q=${encodeURIComponent(q)}`);
	}

	function submitSearch(e: Event) {
		e.preventDefault();
		runSearch(searchTerm);
	}

	// Open the history dropdown when focusing the (empty) box. The markup also gates
	// on an empty input, so the panel hides the instant the user types (QW1).
	function openHistory() {
		activeIdx = -1;
		historyOpen = true;
	}

	function pickHistory(q: string) {
		searchTerm = q;
		runSearch(q);
	}

	// Keyboard nav within the history dropdown: ↓/↑ move the highlight, Enter runs the
	// highlighted query, Esc closes. With nothing highlighted, Enter falls through to
	// the form's submit (runs the typed term).
	function onSearchKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			historyOpen = false;
			activeIdx = -1;
			return;
		}
		const items = searchHistory.items;
		if (!historyOpen || searchTerm.trim() || items.length === 0) return;
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			activeIdx = (activeIdx + 1) % items.length;
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			activeIdx = activeIdx <= 0 ? items.length - 1 : activeIdx - 1;
		} else if (e.key === 'Enter' && activeIdx >= 0) {
			e.preventDefault();
			pickHistory(items[activeIdx]);
		}
	}
</script>

<header class="flex items-center justify-between gap-4 border-b border-rule px-6 py-3">
	<a href="/" class="skin-title text-lg font-semibold tracking-tight text-ink">Holodex</a>

	<form onsubmit={submitSearch} class="relative max-w-md flex-1">
		<input
			bind:this={searchInput}
			bind:value={searchTerm}
			onfocus={openHistory}
			onblur={() => (historyOpen = false)}
			onkeydown={onSearchKeydown}
			role="combobox"
			aria-expanded={showHistory}
			aria-controls="search-history"
			aria-activedescendant={showHistory && activeIdx >= 0 ? `sh-opt-${activeIdx}` : undefined}
			placeholder="Search everything…  (Ctrl-K)"
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<!-- History dropdown: open only on a focused, EMPTY box, so it hides the instant
		     the user types (QW1). Row actions use onmousedown so they fire before the
		     input's blur closes the panel; remove/clear preventDefault to keep focus. -->
		{#if showHistory}
			<ul
				id="search-history"
				role="listbox"
				class="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-theme border border-rule bg-surface shadow-lg"
			>
				{#each searchHistory.items as q, i (q)}
					<li id={`sh-opt-${i}`} role="option" aria-selected={i === activeIdx}>
						<div
							class="flex items-center gap-2 px-3 py-1.5 text-sm {i === activeIdx
								? 'bg-surface-2 text-ink'
								: 'text-ink hover:bg-surface-2'}"
						>
							<button type="button" class="flex-1 truncate text-left" onmousedown={() => pickHistory(q)}>
								{q}
							</button>
							<button
								type="button"
								aria-label={`Remove "${q}" from history`}
								class="shrink-0 text-muted hover:text-ink"
								onmousedown={(e) => {
									e.preventDefault();
									searchHistory.remove(q);
									activeIdx = -1; // list shifted — drop the (now stale) highlight
								}}
							>
								×
							</button>
						</div>
					</li>
				{/each}
				<li>
					<button
						type="button"
						class="block w-full border-t border-rule px-3 py-1.5 text-left text-xs text-muted hover:text-ink"
						onmousedown={(e) => {
							e.preventDefault();
							searchHistory.clear();
							activeIdx = -1;
						}}
					>
						Clear history
					</button>
				</li>
			</ul>
		{/if}
	</form>

	<nav class="flex items-center gap-3 text-sm text-muted">
		<a href="/" class="hover:text-ink">Media</a>
		<a href="/people" class="hover:text-ink">People</a>
		<a href="/tags" class="hover:text-ink">Tags</a>

		<!-- Library tools, separated from the content nav so the bar reads in two groups. -->
		<span class="flex items-center gap-3 border-l border-rule pl-3">
			<a href="/keys" class="hover:text-ink">Keys</a>
			<a href="/status" class="hover:text-ink">Status</a>
			<!-- Trash is owner-only (F24): the link renders only for the owner; the page
			     and API are independently gated. -->
			{#if activity.isOwner}
				<a href="/trash" class="hover:text-ink">Trash</a>
			{/if}
		</span>

		<ActivityIndicator />

		<!-- Skin switcher as a first-class segmented control: each option shows that
		     skin's own accent (the swatch re-scopes --accent via data-theme). Only the
		     active skin shows its label, keeping the bar compact. -->
		<div
			class="flex items-center gap-0.5 rounded-theme border border-rule p-0.5"
			role="group"
			aria-label="Theme skin"
		>
			{#each THEMES as t (t)}
				<button
					type="button"
					onclick={() => theme.set(t)}
					aria-pressed={theme.current === t}
					title={THEME_LABELS[t]}
					class="flex items-center gap-1.5 rounded-theme px-2 py-1 text-xs transition {theme.current ===
					t
						? 'bg-surface-2 text-ink'
						: 'text-muted hover:text-ink'}"
				>
					<span data-theme={t} class="h-2.5 w-2.5 rounded-full bg-accent ring-1 ring-black/20"></span>
					{#if theme.current === t}<span class="hidden sm:inline">{THEME_LABELS[t]}</span>{/if}
				</button>
			{/each}
		</div>
	</nav>
</header>

<main class="px-6 py-6">
	{@render children()}
</main>
