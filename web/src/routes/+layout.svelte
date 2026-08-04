<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { theme, THEMES, THEME_LABELS } from '$lib/theme.svelte';
	import { adminMode } from '$lib/adminMode.svelte';
	import { activity } from '$lib/activity.svelte';
	import { searchHistory } from '$lib/searchHistory.svelte';
	import { navSearch } from '$lib/navSearch.svelte';
	import { dismissable } from '$lib/actions/dismissable';
	import ActivityIndicator from '$lib/components/activity/ActivityIndicator.svelte';
	import SearchResultsPanel from '$lib/components/entity/SearchResultsPanel.svelte';

	let { children } = $props();

	let searchInput = $state<HTMLInputElement | null>(null);
	let searchFormEl = $state<HTMLFormElement | null>(null);
	// The box's expanded focus session — history dropdown OR live results panel,
	// never both. Closed only by Escape or a click outside the box (not blur), so Tab
	// can reach the tab row and result rows inside the panel (NS5).
	let boxOpen = $state(false);
	let activeIdx = $state(-1); // keyboard-highlighted history row, -1 = none
	const hasQuery = $derived(navSearch.query.trim() !== '');
	const showHistory = $derived(boxOpen && !hasQuery && searchHistory.items.length > 0);
	const showPanel = $derived(boxOpen && hasQuery);

	// Apply the saved skin + load search history on mount.
	$effect(() => {
		theme.init();
		adminMode.init();
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

	function closeBox() {
		boxOpen = false;
		activeIdx = -1;
	}

	// Run a search: record it in history, then navigate. Shared by form submit and
	// clicking a history row. Doesn't clear the box's query (matches today's
	// behavior — the submitted term stays visible after navigating to /search).
	function runSearch(term: string) {
		const q = term.trim();
		if (!q) return;
		searchHistory.record(q);
		closeBox();
		goto(`/search?q=${encodeURIComponent(q)}`);
	}

	// Enter with nothing else focused submits the typed term natively via the form;
	// a focused history/panel row handles its own activation instead (NS5).
	function submitSearch(e: Event) {
		e.preventDefault();
		runSearch(navSearch.query);
	}

	function openBox() {
		activeIdx = -1;
		boxOpen = true;
	}

	function onSearchInput(e: Event) {
		navSearch.setQuery((e.currentTarget as HTMLInputElement).value);
		activeIdx = -1;
		// Typing always means "show me results" — reopens the box if a prior Escape
		// left it closed while focus stayed in the input (NS5: Escape doesn't clear
		// the query, so the very next keystroke should resume live filtering).
		boxOpen = true;
	}

	function clearSearch() {
		navSearch.clear();
		searchInput?.focus();
	}

	function pickHistory(q: string) {
		navSearch.setQuery(q);
		runSearch(q);
	}

	// ArrowDown from the input moves the roving highlight into the history list
	// (mirrors the tab row's own ArrowDown-into-first-row behavior below). Enter is
	// handled explicitly rather than relying on native implicit form submission —
	// browsers don't reliably treat Enter as a submit trigger once other buttons
	// share the form (EnrichPicker.svelte's input takes the same explicit approach).
	function historyRowAt(i: number) {
		return searchFormEl?.querySelector<HTMLButtonElement>(`[data-history-index="${i}"]`) ?? null;
	}

	function onInputKeydown(e: KeyboardEvent) {
		if (showHistory && e.key === 'ArrowDown' && searchHistory.items.length) {
			e.preventDefault();
			activeIdx = 0;
			historyRowAt(0)?.focus();
		} else if (e.key === 'Enter') {
			e.preventDefault();
			runSearch(navSearch.query);
		}
	}

	// Roving tabindex within the history list (NS5) — Tab reaches the highlighted
	// row; ↓/↑ move it; Enter/click activate via the row's own native button
	// behavior, matching EnrichPicker.svelte's established pattern.
	function onHistoryRowKey(e: KeyboardEvent, i: number) {
		const items = searchHistory.items;
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			const next = Math.min(i + 1, items.length - 1);
			activeIdx = next;
			historyRowAt(next)?.focus();
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) {
				activeIdx = -1;
				searchInput?.focus();
			} else {
				activeIdx = i - 1;
				historyRowAt(i - 1)?.focus();
			}
		}
	}
</script>

<header class="flex items-center justify-between gap-4 border-b border-rule px-6 py-3">
	<a href="/" class="skin-title text-lg font-semibold tracking-tight text-ink">Holodex</a>

	<form
		bind:this={searchFormEl}
		data-search-box
		onsubmit={submitSearch}
		use:dismissable={{
			enabled: boxOpen,
			inside: '[data-search-box]',
			onclose: (viaEscape) => {
				closeBox();
				if (viaEscape) searchInput?.focus();
			}
		}}
		class="relative max-w-md flex-1"
	>
		<div class="relative">
			<input
				bind:this={searchInput}
				value={navSearch.query}
				oninput={onSearchInput}
				onfocus={openBox}
				onclick={openBox}
				onkeydown={onInputKeydown}
				aria-label="Search everything"
				placeholder="Search everything…  (Ctrl-K)"
				class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 pr-7 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
			/>
			{#if hasQuery}
				<button
					type="button"
					onclick={clearSearch}
					aria-label="Clear search"
					class="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-theme px-1 text-muted hover:text-ink"
				>
					×
				</button>
			{/if}
		</div>

		<!-- History dropdown: shown only on a focused, EMPTY box (QW1) — hides the
		     instant the user types, when the live results panel takes over below.
		     Roving tabindex (NS5): the highlighted row is the lone tab stop. -->
		{#if showHistory}
			<ul
				id="search-history"
				role="listbox"
				aria-label="Recent searches"
				class="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-theme border border-rule bg-surface shadow-lg"
			>
				{#each searchHistory.items as q, i (q)}
					<li role="presentation">
						<div
							class="flex items-center gap-2 {i === activeIdx ? 'bg-surface-2 text-ink' : 'text-ink hover:bg-surface-2'}"
						>
							<button
								data-history-index={i}
								type="button"
								role="option"
								aria-selected={i === activeIdx}
								tabindex={i === activeIdx ? 0 : -1}
								onclick={() => pickHistory(q)}
								onfocus={() => (activeIdx = i)}
								onmouseenter={() => (activeIdx = i)}
								onkeydown={(e) => onHistoryRowKey(e, i)}
								class="flex-1 truncate px-3 py-1.5 text-left text-sm"
							>
								{q}
							</button>
							<button
								type="button"
								aria-label={`Remove "${q}" from history`}
								class="shrink-0 px-2 text-muted hover:text-ink"
								onclick={() => {
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
						onclick={() => {
							searchHistory.clear();
							activeIdx = -1;
						}}
					>
						Clear history
					</button>
				</li>
			</ul>
		{:else if showPanel}
			<SearchResultsPanel
				results={navSearch.results}
				loading={navSearch.loading}
				error={navSearch.error}
				query={navSearch.query}
				bind:activeTab={navSearch.activeTab}
				variant="dropdown"
				onnavigate={closeBox}
			/>
		{/if}
	</form>

	<nav class="flex items-center gap-3 text-sm text-muted">
		<a href="/" class="hover:text-ink">Media</a>
		<a href="/people" class="hover:text-ink">People</a>
		<a href="/studios" class="hover:text-ink">Studios</a>
		<a href="/tags" class="hover:text-ink">Tags</a>

		<!-- Owner chrome, separated from the content nav so the bar reads in tiers (F35):
		     activity · Owner-view toggle · Owner hub · skin picker. Keys/Status/Trash
		     no longer live in the content nav — they're tabs under the Owner gear. -->
		<span class="flex items-center gap-3 border-l border-rule pl-3">
			<ActivityIndicator />

		<!-- Owner-view toggle (F29, relabeled in F35): a binary switch that hides ALL
		     owner-only controls and data for a faithful visitor preview. Owner-only;
		     presentation only — it never changes the admin token or any server gate
		     (ADR-030). ON = accent fill (the active/primary semantic, doubling as the
		     "powers on" indicator); OFF = muted outline. Icon swaps open-eye/eye-slash
		     so meaning isn't color-only. Label tucks away below `sm`, like the skin
		     picker. Backed by the (intentionally still-named) `adminMode` store. -->
		{#if activity.isOwner}
			<button
				type="button"
				role="switch"
				aria-checked={adminMode.enabled}
				aria-label="Owner view"
				title={adminMode.enabled
					? 'Owner view — switch to visitor preview'
					: 'Previewing as visitor — switch to owner view'}
				onclick={() => adminMode.toggle()}
				class="flex items-center gap-1.5 rounded-theme border px-2 py-1 text-xs transition {adminMode.enabled
					? 'border-transparent bg-accent text-accent-ink'
					: 'border-rule text-muted hover:text-ink'}"
			>
				{#if adminMode.enabled}
					<svg
						xmlns="http://www.w3.org/2000/svg"
						fill="none"
						viewBox="0 0 24 24"
						stroke-width="1.5"
						stroke="currentColor"
						class="h-3.5 w-3.5"
						aria-hidden="true"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.183.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z"
						/>
						<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
					</svg>
				{:else}
					<svg
						xmlns="http://www.w3.org/2000/svg"
						fill="none"
						viewBox="0 0 24 24"
						stroke-width="1.5"
						stroke="currentColor"
						class="h-3.5 w-3.5"
						aria-hidden="true"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88"
						/>
					</svg>
				{/if}
				<span class="hidden sm:inline">Owner view</span>
			</button>
		{/if}

		<!-- Owner tools hub entry (F35): a single gear in the owner-chrome cluster that
		     opens /owner (Status · Metadata keys · Trash as tabs) — replacing the three
		     former peer links. Gated on effectiveOwner OR needToken, so a signed-out (or
		     not-yet-authenticated) visitor still has a way back to the token prompt on
		     /owner/status — effectiveOwner alone hid this entirely once
		     signed out, since it also requires Owner view to be on. Visitor view (Owner
		     view off) still hides it while genuinely owner, matching F29's intent.
		     Active (on an /owner route) = text-accent + aria-current; never a fill, so
		     the one solid accent stays the Owner-view ON state / a page's primary action.
		     Label hides below `sm`, like the toggle and skin picker. -->
		{#if activity.effectiveOwner || activity.needToken}
			{@const ownerActive = page.url.pathname.startsWith('/owner')}
			<a
				href="/owner"
				aria-label="Owner tools"
				aria-current={ownerActive ? 'page' : undefined}
				title="Owner tools"
				class="flex items-center gap-1.5 rounded-theme border border-rule px-2 py-1 text-xs transition {ownerActive
					? 'text-accent'
					: 'text-muted hover:text-ink'}"
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					fill="none"
					viewBox="0 0 24 24"
					stroke-width="1.5"
					stroke="currentColor"
					class="h-3.5 w-3.5"
					aria-hidden="true"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 0 1 0-.255c.007-.378-.138-.75-.43-.991l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z"
					/>
					<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
				</svg>
				<span class="hidden sm:inline">Owner</span>
			</a>
		{/if}

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
		</span>
	</nav>
</header>

<!-- Announces Admin-mode changes that don't originate from the toggle itself
     (auto-reveal on owner-only routes, F29 P0-6); the switch announces its own
     manual flips via aria-checked. -->
<p class="sr-only" role="status" aria-live="polite">{adminMode.announcement}</p>

<main class="px-6 py-6">
	{@render children()}
</main>
