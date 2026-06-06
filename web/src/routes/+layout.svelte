<script lang="ts">
	import '../app.css';

	let { children } = $props();

	// Dark mode is default (F8); toggle persists to localStorage.
	let dark = $state(true);

	// Load the saved preference once on mount (default: dark).
	$effect(() => {
		dark = localStorage.getItem('holodex-theme') !== 'light';
	});

	// Apply + persist whenever the theme changes.
	$effect(() => {
		document.documentElement.classList.toggle('dark', dark);
		localStorage.setItem('holodex-theme', dark ? 'dark' : 'light');
	});

	const toggleTheme = () => (dark = !dark);
</script>

<header class="flex items-center justify-between border-b border-zinc-800 px-6 py-3">
	<a href="/" class="text-lg font-semibold tracking-tight">Holodex</a>
	<nav class="flex items-center gap-4 text-sm text-zinc-400">
		<a href="/" class="hover:text-zinc-100">Media</a>
		<a href="/people" class="hover:text-zinc-100">People</a>
		<a href="/tags" class="hover:text-zinc-100">Tags</a>
		<button
			onclick={toggleTheme}
			class="rounded border border-zinc-700 px-2 py-1 hover:bg-zinc-800"
			aria-label="Toggle color theme"
		>
			{dark ? '☾' : '☀'}
		</button>
	</nav>
</header>

<main class="px-6 py-6">
	{@render children()}
</main>
