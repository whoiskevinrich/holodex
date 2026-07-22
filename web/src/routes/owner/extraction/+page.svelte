<script lang="ts">
	// Extraction review queue tab (F48.6, ADR-067). Unlike Enrichment/Duplicates
	// (fixed-key grouping via groupByKind), rows group by video id — an open, dynamic
	// key set — so grouping/sorting is derived locally: video groups with the most
	// pending fields sort first (clears the most backlog per click), fields within a
	// group render People → Studio → Title → Release date → other. "Keep
	// tag"/"Dismiss" never touch the file and resolve immediately in place; the other
	// actions stage a pending write the owner commits via the sticky commit bar's
	// preview dialog (F48.7a). Tokens only; QA 3 skins.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import ExtractionQueueRow from '$lib/components/ExtractionQueueRow.svelte';
	import ExtractionPreviewDialog from '$lib/components/ExtractionPreviewDialog.svelte';
	import type { ExtractionPreviewItem, ExtractionQueueRow as QueueRow, ExtractionResolveAction } from '$lib/types';

	let rows = $state<QueueRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	let extracting = $state(false); // the /admin/extract-all POST is in flight
	let extractRunning = $state(false); // the background batch is (probably) still producing rows
	let refreshing = $state(false);
	let extractError = $state('');

	// The component may unmount mid-poll (owner navigates away); stop touching
	// state once it does.
	let alive = true;
	$effect(() => () => {
		alive = false;
	});

	// canonical field key -> label, from the shared facet registry (the same
	// labels every other field surface in the app uses) — falls back to a
	// titleized key for anything not present as a facet.
	let labelByField = $state<Record<string, string>>({});

	// The extraction package's field key ("people", from the {people} filename
	// token — internal/extract/pattern.go) differs from the app's canonical field
	// key for the same data ("actors": metadata-mappings.yaml.example declares
	// filename:people as one of "actors"'s *sources*, not a canonical field of its
	// own — see media/[id]/+page.svelte's `f.canonical === 'actors'`). Mirrors
	// internal/extract.WritebackField on the backend: without this alias, the
	// facet-registry lookup below misses and falls back to a titleized "People"
	// instead of the app's actual configured label ("Actors"), showing a
	// different name for the same field than every other surface in the app.
	const FIELD_LABEL_ALIASES: Record<string, string> = { people: 'actors' };
	function fieldLabel(key: string): string {
		const canonical = FIELD_LABEL_ALIASES[key] ?? key;
		return labelByField[canonical] ?? key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
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

	// Reload the queue without the full-screen "Loading…" state, so an
	// auto/manual refresh updates rows in place instead of blanking the list.
	async function refresh() {
		refreshing = true;
		try {
			const queue = await api.extractionQueue();
			if (alive) rows = queue.rows ?? [];
		} catch (e) {
			if (alive) error = toMessage(e);
		} finally {
			if (alive) refreshing = false;
		}
	}

	// /admin/extract-all returns 202 immediately and processes in the
	// background (progress lands in System Activity). There's no completion
	// signal here, so give the owner visible feedback — a running notice plus a
	// bounded auto-refresh — and a manual Refresh for anything still in flight.
	async function extractAll() {
		if (extracting || extractRunning) return;
		extracting = true;
		extractError = '';
		try {
			await api.extractAll();
			extractRunning = true;
			void pollQueue();
		} catch (e) {
			extractError = toMessage(e);
		} finally {
			extracting = false;
		}
	}

	async function pollQueue() {
		for (let i = 0; i < 8 && alive; i++) {
			await new Promise((r) => setTimeout(r, 2500));
			if (!alive) return;
			await refresh();
		}
		if (alive) extractRunning = false;
	}
</script>

<div class="space-y-5">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="max-w-2xl text-sm text-muted">
			Fields the filename and tags disagree on, or that scored below the auto-apply threshold.
			Resolving a row writes it the same way an auto-applied field would.
		</p>
		<div class="flex shrink-0 items-center gap-2">
			<button
				onclick={refresh}
				disabled={refreshing || loading}
				class="rounded-theme border border-rule px-2.5 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				{refreshing ? 'Refreshing…' : 'Refresh'}
			</button>
			<button
				onclick={extractAll}
				disabled={extracting || extractRunning}
				class="btn-accent px-2.5 py-1.5 text-sm"
			>
				{extracting ? 'Starting…' : extractRunning ? 'Extracting…' : 'Extract all'}
			</button>
		</div>
	</div>
	{#if extractRunning}
		<p class="text-sm text-muted" role="status" aria-live="polite">
			Extraction is running in the background — new rows appear below as files are processed. For a
			large library this can take a while; use Refresh to check for more.
		</p>
	{/if}
	{#if extractError}
		<p class="text-sm text-warn" role="alert">{extractError}</p>
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

		<!-- Commit bar. Sticky and unconditionally mounted whenever there are rows, so
		     staging the first pick doesn't shift the queue and the write control stays
		     reachable from row 400 as easily as from row 1 (HOLODEX-199). -->
		<div
			class="sticky bottom-0 flex flex-wrap items-center justify-between gap-3 border-t border-rule bg-surface px-3 py-2.5"
		>
			<p class="text-sm text-muted" aria-live="polite">
				{stagedCount} staged · {rows.length} row{rows.length === 1 ? '' : 's'} left
			</p>
			<div class="flex items-center gap-2">
				<button
					onclick={() => (staged = {})}
					disabled={stagedCount === 0}
					class="rounded-theme border border-rule px-2.5 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:border-transparent disabled:text-muted"
				>
					Clear
				</button>
				<button
					onclick={() => (showPreview = true)}
					disabled={stagedCount === 0}
					class="rounded-theme bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90 disabled:bg-surface-2 disabled:text-muted"
				>
					Review &amp; write {stagedCount}
				</button>
			</div>
		</div>
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
