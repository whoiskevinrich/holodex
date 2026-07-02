<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import type { DecisionSource, ResolvedField, Studio, StudioDetailResponse, Video } from '$lib/types';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import EntityVideos from '$lib/components/EntityVideos.svelte';
	import ProvenanceBadge from '$lib/components/ProvenanceBadge.svelte';
	import SourceSelect from '$lib/components/SourceSelect.svelte';
	import UrlValueList from '$lib/components/UrlValueList.svelte';

	// Studio detail (F38, ADR-053): name header + video grid + a Details section that
	// reuses the F36 source-chip radiogroup with the `record` baseline (RD5). Unlike the
	// person page there is no rename, no aliases, no images, no writeback — a studio's
	// name is derived identity, corrected by editing the studio field on its videos. The
	// Details section is hidden until enrichment or a decision gives it something beyond
	// `name` to curate (so a pre-enrichment studio is just name + videos).
	let studio = $state<Studio | null>(null);
	let videos = $state<Video[]>([]);
	let resolved = $state<ResolvedField[]>([]);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner);

	// Replace fields other than `name` that have a value or (for the owner) a candidate.
	// `name` is read-only identity — never a chip row.
	const replaceFields = $derived(
		resolved.filter(
			(f) =>
				f.canonical !== 'name' &&
				!f.multi &&
				(isOwner
					? f.values.length > 0 || (f.candidates ?? []).length > 0
					: f.values.some((v) => v.trim() !== ''))
		)
	);
	const compactFields = $derived(replaceFields.filter((f) => f.display !== 'long_text'));
	const longFields = $derived(replaceFields.filter((f) => f.display === 'long_text'));
	// Hide the whole Details section until there's something beyond name to show.
	const hasDetails = $derived(replaceFields.length > 0);

	// The provider behind a visitor row's ProvenanceBadge — the winning namespace unless
	// it is the record baseline or a manual literal (mirrors the person page).
	function winnerProvider(f: ResolvedField): string {
		const ns = (f.winning_source ?? '').split(':')[0];
		return ns === 'record' || ns === 'file' || ns === 'manual' ? '' : ns;
	}

	function apply(res: StudioDetailResponse) {
		studio = res.studio;
		videos = res.items ?? [];
		resolved = res.resolved ?? [];
	}

	function load(current: number) {
		loading = true;
		error = '';
		api
			.getStudio(current)
			.then(apply)
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	}

	$effect(() => load(id));

	async function reloadDetail() {
		try {
			apply(await api.getStudio(id));
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
	}

	// Persist a studio field decision then refetch. DB-only — a studio has no file, so
	// selecting the record chip is a standing blank-pin (RD5), so every chip maps to PUT.
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		await api.setStudioFieldDecision(id, canonical, {
			source,
			...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
		});
		await reloadDetail();
	}
</script>

<AsyncState {loading} {error}>
	<EntityVideos
		backHref="/studios"
		backLabel="All studios"
		name={studio?.name ?? ''}
		{videos}
		empty="No videos for this studio."
	>
		{#snippet detail()}
			{#if hasDetails}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
					<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
						{#each compactFields as f (f.canonical)}
							{#if isOwner}
								<div>
									<dt class="mb-1 text-muted">{f.label}:</dt>
									<dd>
										<SourceSelect
											field={f}
											baselineKey="record"
											decide={(s, mv) => decideField(f.canonical, s, mv)}
										/>
									</dd>
								</div>
							{:else}
								<div>
									<dt class="inline text-muted">{f.label}:</dt>
									{#if f.display === 'url'}
										<dd class="inline"><UrlValueList values={f.values} /></dd>
									{:else}
										<dd class="inline text-ink">{f.values.join(', ')}</dd>
									{/if}
									{#if winnerProvider(f)}
										<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
									{/if}
								</div>
							{/if}
						{/each}

						{#each longFields as f (f.canonical)}
							<div class="sm:col-span-2">
								<dt class="inline text-muted">{f.label}:</dt>
								{#if f.values[0]?.trim()}
									<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
								{:else if isOwner}
									<dd class="mt-1 block text-muted">—</dd>
								{/if}
								{#if isOwner}
									<dd class="block">
										<SourceSelect
											field={f}
											baselineKey="record"
											decide={(s, mv) => decideField(f.canonical, s, mv)}
										/>
									</dd>
								{:else if winnerProvider(f)}
									<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
								{/if}
							</div>
						{/each}
					</dl>
				</section>
			{/if}
		{/snippet}
	</EntityVideos>
</AsyncState>
