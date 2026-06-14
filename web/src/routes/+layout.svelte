<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { theme, THEMES, THEME_LABELS } from '$lib/theme.svelte';

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
		<a href="/keys" class="hover:text-ink">Keys</a>

		<!-- Skin switcher as a first-class segmented control: each option shows that
		     skin's own accent (the swatch re-scopes --accent via data-theme). -->
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
					<span class="hidden md:inline">{THEME_LABELS[t]}</span>
				</button>
			{/each}
		</div>
	</nav>
</header>

<main class="px-6 py-6">
	{@render children()}
</main>
