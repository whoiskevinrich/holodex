<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { theme, THEMES, THEME_LABELS, type Theme } from '$lib/theme.svelte';

	let { children } = $props();

	let searchTerm = $state('');
	let searchInput = $state<HTMLInputElement | null>(null);

	// Apply the saved skin on mount (data-theme on <html>).
	$effect(() => {
		theme.init();
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

	function submitSearch(e: Event) {
		e.preventDefault();
		if (searchTerm.trim()) goto(`/search?q=${encodeURIComponent(searchTerm.trim())}`);
	}
</script>

<header class="flex items-center justify-between gap-4 border-b border-rule px-6 py-3">
	<a href="/" class="skin-title text-lg font-semibold tracking-tight text-ink">Holodex</a>

	<form onsubmit={submitSearch} class="relative max-w-md flex-1">
		<input
			bind:this={searchInput}
			bind:value={searchTerm}
			placeholder="Search everything…  (Ctrl-K)"
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>
	</form>

	<nav class="flex items-center gap-4 text-sm text-muted">
		<a href="/" class="hover:text-ink">Media</a>
		<a href="/people" class="hover:text-ink">People</a>
		<a href="/tags" class="hover:text-ink">Tags</a>

		<label class="sr-only" for="skin">Skin</label>
		<select
			id="skin"
			value={theme.current}
			onchange={(e) => theme.set(e.currentTarget.value as Theme)}
			class="rounded-theme border border-rule bg-surface px-2 py-1 text-xs text-ink outline-none focus:border-accent"
			aria-label="Theme skin"
		>
			{#each THEMES as t (t)}
				<option value={t}>{THEME_LABELS[t]}</option>
			{/each}
		</select>
	</nav>
</header>

<main class="px-6 py-6">
	{@render children()}
</main>
