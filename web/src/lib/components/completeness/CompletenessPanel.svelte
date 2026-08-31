<script lang="ts">
	// Per-entity completeness breakdown panel (F55.13-15, design handoff
	// docs/design/entity-completeness-handoff.md §2 DD4-DD8). Shared across
	// video/person/studio detail pages — the panel itself is entity-agnostic;
	// only the not-applicable toggle (DD8, video-only in v1) needs an id.
	import ProvenanceBadge from '$lib/components/enrichment/ProvenanceBadge.svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { Completeness } from '$lib/types';

	let {
		completeness,
		videoId,
		onchanged
	}: {
		completeness: Completeness | null | undefined;
		/** Present only on the media detail page — the not-applicable mutation
		 * (F55.10) is video-only, and v1's only UI target is
		 * external_provider_id, a video-only registry field. */
		videoId?: number;
		/** Called after a successful not-applicable toggle so the parent can
		 * refetch the detail response — this component doesn't own
		 * `completeness`, so it can't recompute the score locally. */
		onchanged?: () => void;
	} = $props();

	const GROUPS = [
		{ criticality: 'critical', title: 'Critical' },
		{ criticality: 'nice_to_have', title: 'Nice to have' }
	];

	let busy = $state(false);
	let error = $state('');
	let expanded = $state(false);

	async function toggleNotApplicable(canonical: string, notApplicable: boolean) {
		if (!videoId || busy) return;
		busy = true;
		error = '';
		try {
			if (notApplicable) {
				await api.clearFacetNotApplicable(videoId, canonical);
			} else {
				await api.setFacetNotApplicable(videoId, canonical);
			}
			onchanged?.();
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

{#if completeness}
	<section class="space-y-1.5">
		<h2 class="text-xs uppercase tracking-wide text-muted">Completeness</h2>
		<div class="space-y-4 rounded-theme border border-rule bg-surface p-4">
			{#if completeness.facets.length === 0}
				<!-- Only theoretically reachable today (every scored facet not-applicable) —
				     spec §7 edge case: defend against a zero-denominator score render. -->
				<p class="text-sm text-muted">No scored facets.</p>
			{:else}
				<div class="space-y-1.5">
					<div class="flex items-center justify-between gap-3">
						<span class="font-display text-2xl text-ink">{completeness.score}%</span>
						<button
							type="button"
							onclick={() => (expanded = !expanded)}
							aria-expanded={expanded}
							aria-controls="completeness-facets"
							aria-label={expanded ? 'Hide completeness details' : 'Show completeness details'}
							title={expanded ? 'Hide details' : 'Show details'}
							class="btn-quiet flex h-7 w-7 shrink-0 items-center justify-center rounded-theme hover:bg-surface-2"
						>
							<svg
								class="h-4 w-4 transition-transform duration-200 motion-reduce:transition-none"
								class:rotate-180={expanded}
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								aria-hidden="true"
							>
								<path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6" />
							</svg>
						</button>
					</div>
					<div
						class="h-1.5 w-full overflow-hidden rounded-theme bg-surface-2"
						role="img"
						aria-label={`${completeness.score}% complete`}
					>
						<div class="h-full bg-accent" style={`width: ${completeness.score}%`}></div>
					</div>
					{#if completeness.actionability === undefined}
						<p class="text-xs text-muted">Fully complete</p>
					{:else}
						<p class="text-xs text-muted">
							{Math.round(completeness.actionability * 100)}% of missing facets have a cached candidate
							ready
						</p>
					{/if}
				</div>

				{#if error}<p class="text-xs text-warn" role="alert">{error}</p>{/if}

				<div
					id="completeness-facets"
					class="overflow-hidden transition-[max-height] duration-200 ease-out motion-reduce:transition-none"
					style="max-height: {expanded ? '2000px' : '0px'}"
					inert={!expanded}
				>
					<div class="space-y-4">
						{#each GROUPS as group (group.criticality)}
							{@const facets = completeness.facets.filter((f) => f.criticality === group.criticality)}
							{#if facets.length}
								<div class="space-y-1.5">
									<h3 class="text-xs uppercase tracking-wide text-muted">{group.title}</h3>
									<div class="space-y-1.5">
										{#each facets as f (f.canonical)}
											<div class="flex items-center justify-between gap-3 text-sm">
												<span class="text-ink">{f.label}</span>
												<div class="flex shrink-0 items-center gap-1.5">
													{#if f.not_applicable}
														<span class="text-xs text-muted">Not applicable</span>
													{:else if f.tier === 'curated'}
														<span
															class="rounded-full border border-accent px-2 py-0.5 text-xs text-accent"
														>
															Curated
														</span>
													{:else if f.tier === 'provider'}
														<ProvenanceBadge provider={f.provider} label={f.provider} />
													{:else}
														<span
															class="rounded-full border border-dashed border-rule px-2 py-0.5 text-xs text-muted"
														>
															Missing
														</span>
													{/if}
													{#if videoId && f.canonical === 'external_provider_id'}
														<button
															type="button"
															onclick={() => toggleNotApplicable(f.canonical, !!f.not_applicable)}
															disabled={busy}
															aria-pressed={!!f.not_applicable}
															title={f.not_applicable
																? 'Mark applicable again'
																: 'Mark not applicable'}
															aria-label={f.not_applicable
																? `Mark ${f.label} applicable again`
																: `Mark ${f.label} not applicable`}
															class="{f.not_applicable
																? 'btn-accent'
																: 'btn-ghost'} rounded-theme p-1"
														>
															<svg
																class="h-3.5 w-3.5"
																viewBox="0 0 24 24"
																fill="none"
																stroke="currentColor"
																stroke-width="2"
																aria-hidden="true"
															>
																<path
																	stroke-linecap="round"
																	stroke-linejoin="round"
																	d="M18.364 18.364A9 9 0 105.636 5.636a9 9 0 0012.728 12.728zM6 6l12 12"
																/>
															</svg>
														</button>
													{/if}
												</div>
											</div>
										{/each}
									</div>
								</div>
							{/if}
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</section>
{/if}
