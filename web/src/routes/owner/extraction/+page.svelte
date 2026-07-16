<script lang="ts">
	// Extraction review queue tab (F48.6, ADR-067). Unlike Enrichment/Duplicates
	// (fixed-key grouping via groupByKind), rows group by video id — an open, dynamic
	// key set — so grouping/sorting is derived locally: video groups with the most
	// pending fields sort first (clears the most backlog per click), fields within a
	// group render People → Studio → Title → Release date → other. "Accept
	// tag"/"Dismiss" never touch the file and resolve immediately in place; the other
	// actions stage a pending write the owner commits via the preview dialog (F48.7a).
	// Tokens only; QA 3 skins.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import ExtractionQueueRow from '$lib/components/ExtractionQueueRow.svelte';
	import ExtractionPreviewDialog from '$lib/components/ExtractionPreviewDialog.svelte';
	import type { ExtractionPreviewItem, ExtractionQueueRow as QueueRow, ExtractionResolveAction } from '$lib/types';

	let rows = $state<QueueRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	let extracting = $state(false);
	let extractError = $state('');

	// canonical field key -> label, from the shared facet registry (the same
	// labels every other field surface in the app uses) — falls back to a
	// titleized key for anything not present as a facet.
	let labelByField = $state<Record<string, string>>({});
	function fieldLabel(key: string): string {
		return labelByField[key] ?? key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
	}

	const FIELD_ORDER: Record<string, number> = { people: 0, studio: 1, title: 2, release_date: 3 };
	function fieldRank(key: string): number {
		return FIELD_ORDER[key] ?? 99;
	}
	function isEntityField(key: string): boolean {
		return key === 'people' || key === 'studio';
	}

	interface VideoGroup {
		videoId: number;
		videoTitle: string;
		filePath: string;
		rows: QueueRow[];
	}

	const groups = $derived.by((): VideoGroup[] => {
		const byVideo = new Map<number, QueueRow[]>();
		for (const row of rows) {
			const list = byVideo.get(row.video_id);
			if (list) list.push(row);
			else byVideo.set(row.video_id, [row]);
		}
		const out: VideoGroup[] = [...byVideo.entries()].map(([videoId, items]) => {
			const sorted = [...items].sort(
				(a, b) => fieldRank(a.field_key) - fieldRank(b.field_key) || a.field_key.localeCompare(b.field_key)
			);
			return { videoId, videoTitle: sorted[0].video_title, filePath: sorted[0].file_path, rows: sorted };
		});
		out.sort((a, b) => b.rows.length - a.rows.length || a.videoTitle.localeCompare(b.videoTitle));
		return out;
	});

	// reviewId -> the owner's staged-but-unwritten pick (F48.7a). Cleared once the
	// row is dropped (submitted, or the row itself disappears from the queue).
	let staged = $state<Record<number, { action: ExtractionResolveAction; value: string }>>({});
	const stagedCount = $derived(Object.keys(staged).length);
	let showPreview = $state(false);

	const rowsById = $derived(new Map(rows.map((r) => [r.id, r])));

	const previewItems = $derived.by((): ExtractionPreviewItem[] => {
		const items: ExtractionPreviewItem[] = [];
		for (const [idStr, pick] of Object.entries(staged)) {
			const id = Number(idStr);
			const row = rowsById.get(id);
			if (!row) continue;
			items.push({
				reviewId: id,
				videoTitle: row.video_title,
				fieldLabel: fieldLabel(row.field_key),
				oldValue: row.tag_value,
				newValue: pick.value,
				action: pick.action
			});
		}
		return items;
	});

	function stage(row: QueueRow, action: ExtractionResolveAction, value: string) {
		staged = { ...staged, [row.id]: { action, value } };
	}
	function unstage(reviewId: number) {
		if (!(reviewId in staged)) return;
		const next = { ...staged };
		delete next[reviewId];
		staged = next;
	}
	function dropRow(reviewId: number) {
		rows = rows.filter((r) => r.id !== reviewId);
		unstage(reviewId);
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [queue, facets] = await Promise.all([api.extractionQueue(), api.facets()]);
			rows = queue.rows ?? [];
			labelByField = Object.fromEntries(facets.facets.map((f) => [f.canonical, f.label]));
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
	});

	async function extractAll() {
		if (extracting) return;
		extracting = true;
		extractError = '';
		try {
			await api.extractAll();
		} catch (e) {
			extractError = toMessage(e);
		} finally {
			extracting = false;
		}
	}
</script>

<div class="space-y-5">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="max-w-2xl text-sm text-muted">
			Fields the filename and tags disagree on, or that scored below the auto-apply threshold.
			Resolving a row writes it the same way an auto-applied field would.
		</p>
		<button
			onclick={extractAll}
			disabled={extracting}
			class="shrink-0 rounded-theme border border-rule px-2.5 py-1.5 text-sm text-accent hover:bg-surface-2 disabled:opacity-60"
		>
			{extracting ? 'Starting…' : 'Extract all'}
		</button>
	</div>
	{#if extractError}
		<p class="text-sm text-warn" role="alert">{extractError}</p>
	{/if}

	{#if stagedCount > 0}
		<div class="flex justify-end">
			<button
				onclick={() => (showPreview = true)}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90"
			>
				Review {stagedCount} change{stagedCount === 1 ? '' : 's'}
			</button>
		</div>
	{/if}

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else if groups.length === 0}
		<p class="py-16 text-center text-sm text-muted">Nothing left to review.</p>
	{:else}
		{#each groups as g (g.videoId)}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<h3 class="truncate border-b border-rule px-3 pb-2 pt-3 text-sm font-medium text-ink" title={g.filePath}>
					{g.videoTitle}
				</h3>
				{#each g.rows as row (row.id)}
					<ExtractionQueueRow
						{row}
						fieldLabel={fieldLabel(row.field_key)}
						isEntityField={isEntityField(row.field_key)}
						staged={staged[row.id]}
						onstage={(action, value) => stage(row, action, value)}
						onunstage={() => unstage(row.id)}
						resolveTag={() => api.resolveExtractionReview(row.id, 'tag')}
						dismiss={() => api.dismissExtractionReview(row.id)}
						onhandled={() => dropRow(row.id)}
					/>
				{/each}
			</section>
		{/each}
	{/if}
</div>

{#if showPreview}
	<ExtractionPreviewDialog
		items={previewItems}
		onclose={() => (showPreview = false)}
		onsubmitted={(resolvedIds) => resolvedIds.forEach(dropRow)}
		resolve={(id, action, value) => api.resolveExtractionReview(id, action, value)}
	/>
{/if}
