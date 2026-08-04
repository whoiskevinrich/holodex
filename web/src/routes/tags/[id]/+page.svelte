<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, videoCount } from '$lib/format';
	import type { Tag, Video } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import EntityVideos from '$lib/components/entity/EntityVideos.svelte';
	import WritebackBatchDialog from '$lib/components/writeback/WritebackBatchDialog.svelte';

	let tag = $state<Tag | null>(null);
	let videos = $state<Video[]>([]);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	let toggleBusy = $state(false);
	let toggleError = $state('');
	let syncOpen = $state(false);

	$effect(() => {
		const current = id;
		loading = true;
		error = '';
		api
			.getTag(current)
			.then((res) => {
				tag = res.tag;
				videos = res.items ?? [];
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});

	// Posts immediately, no confirm step (HOLODEX-239, ADR-077 D1) — toggling the
	// flag alone never enqueues a write, so it's the lowest-stakes control on the
	// card. tag is reassigned from the PATCH response so the label/glyph flip
	// without a refetch.
	async function toggleWriteback() {
		if (!tag || toggleBusy) return;
		toggleBusy = true;
		toggleError = '';
		try {
			tag = (await api.setTagWriteback(tag.id, !tag.writeback_enabled)).tag;
		} catch (e) {
			toggleError = toMessage(e);
		} finally {
			toggleBusy = false;
		}
	}
</script>

<AsyncState {loading} error={error || (!tag ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/tags"
		backLabel="All tags"
		name={tag?.name ?? ''}
		{videos}
		empty="No videos for this tag."
		scrollKey={`tag:${id}`}
	>
		{#snippet hero()}
			<!-- Ancestor breadcrumb (F50 S8, ADR-075 D1 P1-3) — scoped to this page only
			     (skipped on media-page chips and /tags pills, per the design handoff). -->
			{#if tag?.ancestors?.length}
				<p class="text-sm text-muted">{tag.ancestors.join(' › ')} ›</p>
			{/if}
			<h1 class="skin-title text-2xl font-semibold text-ink">{tag?.name ?? ''}</h1>
			<p class="text-sm text-muted">{videoCount(videos.length)}</p>
		{/snippet}

		{#snippet detail()}
			<!-- Details (HOLODEX-239, ADR-077): writeback inclusion toggle + manual sync
			     trigger, the tag-scoped analog of CurationChip's "don't write" glyph. Two
			     dt/dd rows in one card, not a single hard-coded control — a future
			     tag-categories row (spec P2) slots in as a third row without restructuring.
			     Non-owners see nothing (mirrors people/[id]'s activity.effectiveOwner gating). -->
			{#if isOwner && tag}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
					<dl class="space-y-3 text-sm">
						<div class="flex items-center justify-between gap-3">
							<div>
								<dt class="text-ink">File writeback</dt>
								<dd class="text-xs text-muted">
									{#if tag.writeback_enabled}
										Included in this tag's videos' Genre field on write.
									{:else}
										Excluded from Genre writeback — stays searchable in Holodex.
									{/if}
								</dd>
							</div>
							<button
								type="button"
								onclick={toggleWriteback}
								disabled={toggleBusy}
								aria-pressed={!tag.writeback_enabled}
								title={tag.writeback_enabled
									? 'Exclude from file writeback'
									: 'Include in file writeback'}
								aria-label={tag.writeback_enabled
									? `Exclude ${tag.name} from file writeback`
									: `Include ${tag.name} in file writeback`}
								class="shrink-0 rounded-theme border p-1.5 {tag.writeback_enabled
									? 'border-rule text-muted hover:text-ink'
									: 'border-accent text-accent'}"
							>
								<svg
									class="h-4 w-4"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									aria-hidden="true"
								>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="M9 13h6m-6 4h6m4 4H5a2 2 0 01-2-2V5a2 2 0 012-2h8l6 6v10a2 2 0 01-2 2z"
									/>
								</svg>
							</button>
						</div>
						{#if toggleError}
							<p class="text-xs text-warn">{toggleError}</p>
						{/if}

						<div class="flex items-center justify-between gap-3 border-t border-rule pt-3">
							<div>
								<dt class="text-ink">Sync to files</dt>
								<dd class="text-xs text-muted">
									Push this tag's current decision out to already-written files.
								</dd>
							</div>
							<button
								type="button"
								onclick={() => (syncOpen = true)}
								disabled={!tag.video_count}
								title={tag.video_count ? undefined : 'No videos carry this tag'}
								class="btn-accent shrink-0 px-3 py-1.5 text-sm"
							>
								Sync writeback now
							</button>
						</div>
					</dl>
				</section>
			{/if}
		{/snippet}
	</EntityVideos>
</AsyncState>

{#if syncOpen && tag}
	{@const t = tag}
	<WritebackBatchDialog
		scopeLabel={t.name}
		videoCountHint={t.video_count ?? 0}
		trigger={() => api.syncTagWriteback(t.id)}
		batchStatus={api.writebackBatchStatus}
		onclose={() => (syncOpen = false)}
	/>
{/if}
