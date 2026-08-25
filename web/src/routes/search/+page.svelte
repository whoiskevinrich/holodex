<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/stores';
	import { beforeNavigate } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { listScroll } from '$lib/listScroll.svelte';
	import { toMessage } from '$lib/format';
	import type { SearchResponse } from '$lib/types';
	import SearchResultsPanel from '$lib/components/entity/SearchResultsPanel.svelte';
	import { type SearchTab } from '$lib/navSearch.svelte';

	let results = $state<SearchResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	// Independent from the nav box's own live-typing tab — landing here always starts
	// on "All" (NS1: "the page renders it full-width as the page body").
	let activeTab = $state<SearchTab>('all');

	const q = $derived($page.url.searchParams.get('q') ?? '');

	// On the first load only, restore the scroll position stashed when we last left these
	// results (← Back from a person/studio/tag/video), once the re-fetched results have
	// painted (HOLODEX-248, ADR-032). Keyed by `q` itself — the only thing that
	// determines this result set, and (unlike a plain page $state) it survives Back
	// unchanged since it comes straight from the URL. Later reloads (query edited in
	// place) stay put.
	let firstLoad = true;

	$effect(() => {
		const term = q;
		if (!term) {
			results = null;
			loading = false;
			error = '';
			return;
		}
		loading = true;
		error = '';
		api
			.searchAll(term, !!activity.caps?.films_enabled)
			.then((res) => {
				results = res;
			})
			.catch((e) => {
				error = toMessage(e);
				results = null;
			})
			.finally(() => {
				loading = false;
				if (firstLoad) {
					firstLoad = false;
					const snap = listScroll.take('search', term);
					if (snap) tick().then(() => window.scrollTo(0, snap.scrollY));
				}
			});
	});

	// Stash the scroll offset on the way out (e.g. opening a result) so ← Back restores
	// where these results were.
	beforeNavigate(() => {
		listScroll.save('search', { key: q, scrollY: window.scrollY });
	});
</script>

<section class="space-y-6">
	<h1 class="skin-title text-xl font-semibold text-ink">
		Results for <span class="text-muted">“{q}”</span>
	</h1>

	<SearchResultsPanel {results} {loading} {error} query={q} bind:activeTab variant="page" />
</section>
