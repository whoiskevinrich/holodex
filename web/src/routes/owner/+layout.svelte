<script lang="ts">
	// Owner area shell (F35). Folds the former /status, /keys, /trash pages into one
	// tabbed hub at /owner, so the header carries a single "Owner" entry instead of
	// three peer links. Owner-only end to end; the server gate (ADR-030) stays the
	// sole authority — this layout is navigation, not a security boundary.
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { activity } from '$lib/activity.svelte';
	import { adminMode } from '$lib/adminMode.svelte';

	let { children } = $props();

	const tabs = [
		{ href: '/owner/status', label: 'Status' },
		{ href: '/owner/keys', label: 'Metadata keys' },
		{ href: '/owner/fields', label: 'Attached keys' },
		{ href: '/owner/duplicates', label: 'Duplicates' },
		{ href: '/owner/enrichment', label: 'Enrichment' },
		{ href: '/owner/extraction', label: 'Extraction' },
		{ href: '/owner/trash', label: 'Trash' }
	];

	// Owner gate + single auto-reveal (P0-6), consolidated here so the nested pages
	// don't each re-implement it. `activity.caps` is fetched app-wide by the root
	// layout's polling (activity.start()), so we don't fetch it here — we just wait
	// for it. No redirect on the initial null flash. An owner landing in visitor view
	// flips Owner view back on; a caller who still needs to unlock (auth required, not
	// yet owner) is left on the page so /owner/status can show the token form — the
	// way back in; a genuine non-owner is sent home.
	$effect(() => {
		if (!activity.caps) return;
		if (activity.isOwner) {
			adminMode.reveal();
		} else if (!activity.needToken) {
			goto('/', { replaceState: true });
		}
	});

	const activeTab = $derived(page.url.pathname);
</script>

<section class="mx-auto max-w-5xl space-y-5">
	<header class="space-y-1">
		<h1 class="skin-title text-2xl font-semibold text-ink">Owner</h1>
		<p class="text-sm text-muted">Owner tools — visible only in your view, hidden from visitors.</p>
	</header>

	<!-- Tab row. Active tab uses bg-surface-2 (the skin-picker's active idiom), NOT
	     bg-accent — the solid accent stays reserved for a page's one primary action. -->
	<nav class="flex flex-wrap gap-2 border-b border-rule pb-3" aria-label="Owner tools">
		{#each tabs as t (t.href)}
			{@const active = activeTab === t.href}
			<a
				href={t.href}
				aria-current={active ? 'page' : undefined}
				class="rounded-theme px-3 py-1.5 text-sm transition {active
					? 'bg-surface-2 text-ink'
					: 'text-muted hover:text-ink'}"
			>
				{t.label}
			</a>
		{/each}
	</nav>

	{@render children()}
</section>
