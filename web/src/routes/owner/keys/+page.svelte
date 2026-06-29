<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { MetadataKey } from '$lib/types';
	import { toMessage } from '$lib/format';

	let keys = $state<MetadataKey[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(() => {
		api
			.metadataKeys()
			.then((r) => (keys = r.keys ?? []))
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});
</script>

<section class="mx-auto max-w-4xl space-y-4">
	<header class="space-y-1">
		<h1 class="skin-title text-2xl font-semibold text-ink">Metadata keys</h1>
		<p class="text-sm text-muted">
			Every raw container tag across your library — counts, sample values, and whether a mapping
			covers it. Use this to author <code class="font-mono text-ink">metadata-mappings.yaml</code>.
		</p>
	</header>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">{error}</p>
	{:else if keys.length === 0}
		<p class="py-16 text-center text-sm text-muted">No extended metadata captured yet.</p>
	{:else}
		<table class="w-full text-left text-sm">
			<thead class="text-xs uppercase tracking-wide text-muted">
				<tr class="border-b border-rule">
					<th class="py-2 pr-4">Key</th>
					<th class="py-2 pr-4">Videos</th>
					<th class="py-2 pr-4">Samples</th>
					<th class="py-2">Mapped</th>
				</tr>
			</thead>
			<tbody>
				{#each keys as k (k.source_key)}
					<tr class="border-b border-rule">
						<td class="py-2 pr-4 font-mono text-ink">{k.source_key}</td>
						<td class="py-2 pr-4 tabular-nums text-muted">{k.count}</td>
						<td class="py-2 pr-4 text-muted">{k.samples.join(' · ')}</td>
						<td class="py-2">
							{#if k.mapped}
								<span class="rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink">mapped</span>
							{:else}
								<span class="text-muted">—</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</section>
