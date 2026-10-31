<script module lang="ts">
	import type { StudioDuplicateEvidence } from '#lib/types.ts';
	import { api } from '#lib/api.ts';

	// One read per studio per session (P0-5): reopening a panel, or a studio that sits in
	// two pairs, costs nothing. A failed read is dropped, so Retry asks again.
	const evidence = new Map<number, Promise<StudioDuplicateEvidence>>();

	function loadEvidence(id: number): Promise<StudioDuplicateEvidence> {
		let p = evidence.get(id);
		if (!p) {
			p = api.studioDuplicateEvidence(id).catch((err) => {
				evidence.delete(id);
				throw err;
			});
			evidence.set(id, p);
		}
		return p;
	}

	/** After a merge the survivor's name, counts and aliases have changed, and it may sit
	 *  in another pair; the next open must re-read it. */
	export function invalidateStudioEvidence(id: number): void {
		evidence.delete(id);
	}
</script>

<script lang="ts">
	// The panel a STUDIO pair expands into (F70 P0-10, HOLODEX-598): two columns of where
	// each spelling came from — the sources that carry the name now with their video
	// counts, up to three sample posters captioned with the source that won each, and the
	// aliases with their origin. `DuplicateComparePanel`'s sibling: same well, grid, column
	// shell and footer, different data.
	//
	// It describes provenance and judges nothing (Goal 3): no verdict word, no "which came
	// first", no precedence control and no provider-value comparison — that is the
	// SourceBadge layer, and this panel is not it. Every verdict comes from the row's
	// snippets, as in the person panel.
	import type { DuplicatePair, EntityRef } from '#lib/types.ts';
	import { onMount, type Snippet } from 'svelte';
	import { videoCount, refLabel, sortExternalLinks } from '#lib/format.ts';
	import ProviderLinkBadge from '#lib/components/enrichment/ProviderLinkBadge.svelte';
	import { profileHref, sourceLabel } from './queue';

	let {
		pair,
		panelId,
		matchLabel,
		matchWeak,
		verdicts,
		mergeInto
	}: {
		pair: DuplicatePair;
		panelId: string;
		matchLabel: string;
		matchWeak: boolean;
		verdicts: Snippet;
		mergeInto: Snippet<[EntityRef, EntityRef]>;
	} = $props();

	interface Side {
		data: StudioDuplicateEvidence | null;
		failed: boolean;
	}
	let sides = $state<Record<'a' | 'b', Side>>({
		a: { data: null, failed: false },
		b: { data: null, failed: false }
	});
	// A poster with no cover yet keeps the themed empty frame, never a broken-image glyph.
	let brokenPosters = $state(new Set<number>());

	// onMount rather than `$effect`, as in the person panel: the row's `{#if}` creates this
	// fresh and its `pair` never changes under it.
	onMount(() => {
		load('a', pair.a);
		load('b', pair.b);
	});

	async function load(key: 'a' | 'b', ref: EntityRef) {
		sides[key].failed = false;
		try {
			sides[key].data = await loadEvidence(ref.id);
		} catch {
			sides[key].failed = true;
		}
	}

	/** Provider namespaces named by the side's own link labels, where it has them. */
	function labels(data: StudioDuplicateEvidence): Record<string, string> {
		return Object.fromEntries(data.external_links.map((l) => [l.provider, l.label]));
	}
</script>

{#snippet column(key: 'a' | 'b', ref: EntityRef, other: EntityRef)}
	{@const side = sides[key]}
	{@const data = side.data}
	{@const name = data?.name ?? refLabel(ref)}
	{@const names = data ? labels(data) : {}}
	<section aria-label={name} class="min-w-0 rounded-theme border border-rule bg-surface p-3">
		<span class="flex items-center gap-1.5">
			<a
				href={profileHref('studio', ref.id)}
				target="_blank"
				rel="noopener"
				aria-label={`Open ${name} in a new tab`}
				class="skin-title truncate font-display text-sm font-semibold text-ink hover:text-accent">{name}</a
			>
			<span aria-hidden="true" class="shrink-0 text-xs text-muted">↗</span>
		</span>
		<span class="block text-xs text-muted">{videoCount(data?.video_count ?? ref.video_count ?? 0)}</span>

		{#if data?.sources.length}
			<p class="mt-3 text-xs text-muted">Name came from</p>
			<dl class="mt-1 space-y-0.5 text-xs">
				{#each data.sources as s (s.source)}
					<div class="flex justify-between gap-3">
						<dt class="text-ink">{sourceLabel(s.source, names)}</dt>
						<dd class="tabular-nums text-muted">{videoCount(s.video_count)}</dd>
					</div>
				{/each}
			</dl>
		{/if}

		<!-- Loading holds three empty frames so nothing shifts; a studio with no videos, or a
		     side that failed, reserves nothing. -->
		{#if data ? data.posters.length : !side.failed}
			<div class="mt-3 grid grid-cols-3 gap-1">
				{#each data?.posters ?? [0, 1, 2].map((i) => ({ video_id: -1 - i, source: '' })) as p (p.video_id)}
					<figure class="min-w-0">
						<div class="video-frame">
							{#if p.video_id > 0 && !brokenPosters.has(p.video_id)}
								<img
									src={api.thumbnailURL(p.video_id)}
									alt=""
									loading="lazy"
									decoding="async"
									onerror={() => (brokenPosters = new Set(brokenPosters).add(p.video_id))}
									class="h-full w-full object-cover"
								/>
							{/if}
						</div>
						<!-- The one place the panel shows a winner: the source that won this video's
						     studio name. Nothing resolved → no caption, never a placeholder. -->
						{#if p.source}
							<figcaption class="mt-0.5 truncate text-xs text-muted">{sourceLabel(p.source, names)}</figcaption>
						{/if}
					</figure>
				{/each}
			</div>
		{/if}

		{#if data}
			<p class="mt-3 text-xs text-muted">Also known as</p>
			{#if data.aliases.length}
				<dl class="mt-1 space-y-0.5 text-xs">
					{#each data.aliases as a (a.id)}
						<div class="flex justify-between gap-3">
							<dt class="truncate text-ink">{a.alias}</dt>
							<!-- No source → the owner's: typed, or kept by a merge or rename. -->
							<dd class="shrink-0 text-muted">{a.source ? sourceLabel(a.source, names) : 'you'}</dd>
						</div>
					{/each}
				</dl>
			{:else}
				<p class="mt-1 text-xs text-muted">No other names</p>
			{/if}
		{/if}

		{#if side.failed}
			<p class="mt-2 text-xs text-warn" role="alert">Couldn't load this side.</p>
			<button type="button" onclick={() => load(key, ref)} class="btn-row btn-quiet mt-1">
				Retry
			</button>
		{:else if data}
			<!-- Every link here opens a new tab (P0-9), so the queue page never moves. -->
			<div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-rule pt-2 text-xs">
				<a href={`/studios/${ref.id}#videos`} target="_blank" rel="noopener" class="text-accent hover:text-ink"
					>Videos<span aria-hidden="true" class="ml-0.5 text-muted">↗</span></a
				>
				{#each sortExternalLinks(data.external_links) as link (link.provider)}
					<span class="inline-flex items-center gap-0.5">
						<ProviderLinkBadge {link} entityName={name} />
						{#if link.url}<span aria-hidden="true" class="text-muted">↗</span>{/if}
					</span>
				{/each}
			</div>
		{/if}
		<div class="mt-2 border-t border-rule pt-2">
			{@render mergeInto(ref, other)}
		</div>
	</section>
{/snippet}

<div
	id={panelId}
	role="group"
	aria-label={`Compare ${refLabel(pair.a)} and ${refLabel(pair.b)}`}
	class="border-t border-rule bg-surface-2 px-3 py-3"
>
	{#if matchLabel}
		<p class="mb-3 text-xs" class:text-warn={matchWeak} class:text-muted={!matchWeak}>
			{matchLabel}
		</p>
	{/if}
	<div class="field-grid gap-4">
		{@render column('a', pair.a, pair.b)}
		{@render column('b', pair.b, pair.a)}
	</div>
	<div class="mt-3 flex flex-wrap items-center justify-end gap-2 border-t border-rule pt-3">
		{@render verdicts()}
	</div>
</div>
