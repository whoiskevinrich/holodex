<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import type { EnrichedField, EnrichSource, Person, Video } from '$lib/types';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import EntityVideos from '$lib/components/EntityVideos.svelte';
	import ProvenanceBadge from '$lib/components/ProvenanceBadge.svelte';
	import EnrichPicker from '$lib/components/EnrichPicker.svelte';

	let person = $state<Person | null>(null);
	let videos = $state<Video[]>([]);
	let enriched = $state<EnrichedField[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Enrichment controls (owner-only, F22). sources is loaded once when the client
	// is confirmed owner; the picker drives a provider resolve→apply.
	let sources = $state<EnrichSource[]>([]);
	let pickerOpen = $state(false);
	let busy = $state('');
	// Action errors render inline in the panel — never via the page-level `error`,
	// which AsyncState uses to replace the whole page.
	let actionError = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.isOwner);
	// v1 enriches People from the first person-capable provider.
	const provider = $derived(sources.find((s) => s.entity_types.includes('person'))?.name ?? '');
	const enrichedByProvider = $derived(provider && enriched.some((f) => f.provider === provider));

	$effect(() => {
		const current = id;
		loading = true;
		error = '';
		api
			.getPerson(current)
			.then((res) => {
				person = res.person;
				videos = res.items ?? [];
				enriched = res.enriched ?? [];
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});

	// Load providers once the client is confirmed owner (the layout polls caps).
	$effect(() => {
		if (isOwner && sources.length === 0) {
			api
				.enrichSources()
				.then((res) => (sources = res.sources ?? []))
				.catch(() => {});
		}
	});

	function onApplied(fields: EnrichedField[]) {
		enriched = fields;
	}

	async function clearProvider() {
		if (!provider) return;
		busy = 'clear';
		actionError = '';
		try {
			await api.enrichClear(id, provider);
			// Clear removed exactly this provider's rows; drop them locally rather
			// than refetching the whole person (+ up to 500 videos).
			enriched = enriched.filter((f) => f.provider !== provider);
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			busy = '';
		}
	}
</script>

<AsyncState {loading} error={error || (!person ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/people"
		backLabel="All people"
		name={person?.name ?? ''}
		{videos}
		empty="No videos for this person."
	>
		{#snippet detail()}
			{#if enriched.length || (isOwner && provider)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Enrichment</h2>
						{#if isOwner && provider}
							<div class="flex flex-wrap items-center gap-2">
								<button
									onclick={() => (pickerOpen = true)}
									class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink"
								>
									Enrich from {provider}
								</button>
								{#if enrichedByProvider}
									<button
										onclick={clearProvider}
										disabled={busy === 'clear'}
										class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
									>
										Clear {provider}
									</button>
								{/if}
							</div>
						{/if}
					</div>

					{#if enriched.length}
						<dl class="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
							{#each enriched as f (f.canonical + f.provider)}
								<div>
									<dt class="inline text-muted">{f.label}:</dt>
									<dd class="inline text-ink">{f.values.join(', ')}</dd>
									<ProvenanceBadge provider={f.provider} label={f.provider} />
								</div>
							{/each}
						</dl>
					{:else}
						<p class="text-sm text-muted">No enrichment yet.</p>
					{/if}

					{#if actionError}
						<p class="text-sm text-warn">{actionError}</p>
					{/if}
				</section>
			{/if}
		{/snippet}
	</EntityVideos>
</AsyncState>

{#if pickerOpen && provider}
	<EnrichPicker
		personId={id}
		personName={person?.name ?? ''}
		{provider}
		onclose={() => (pickerOpen = false)}
		onapplied={onApplied}
	/>
{/if}
