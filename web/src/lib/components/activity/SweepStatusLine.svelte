<script lang="ts">
	// Entity refresh sweep status line for the /people and /studios list pages (F66
	// RD10, handoff Option D). Reads the shared activity poll's `sweep` block and
	// renders only what concerns THIS kind: a running line with live counts, or the
	// last finished sweep's done line until it is dismissed. Idle, or a sweep of the
	// other kind, renders nothing. Owner + Admin mode only.
	import { activity } from '$lib/activity.svelte';
	import { formatDurMs } from '$lib/format';
	import type { SweepKind } from '$lib/types';
	import { doneLineFor, skippedText, sweepFinished, type SweepEdge } from './sweepLine';

	let { kind, onfinished }: { kind: SweepKind; onfinished?: () => void } = $props();

	const plural = $derived(kind === 'person' ? 'people' : 'studios');
	const isOwner = $derived(activity.effectiveOwner);
	const sweep = $derived(activity.data?.sweep);

	const running = $derived(sweep?.state === 'running' && sweep.kind === kind ? sweep : null);

	// The done line is derived from last_run plus a page-local dismissed batch, so a
	// reload of the poll never resurrects a line the owner dismissed; leaving the
	// route drops the component and the memory with it (by design).
	let dismissedBatch = $state('');
	const done = $derived(doneLineFor(sweep, kind, dismissedBatch));

	// Fire onfinished exactly once on this kind's running→idle edge (sweepLine.ts).
	const edge: SweepEdge = { sawRunning: false };
	$effect(() => {
		if (sweepFinished(edge, sweep, kind)) onfinished?.();
	});

	const batchHref = $derived(done ? `/owner/status?batch=${encodeURIComponent(done.batch_id)}` : '');

	const n = (v: number) => v.toLocaleString();

	// One line, always (F73 divergence D1, chosen by the owner): the counts truncate with
	// an ellipsis and the full text in the tooltip, while Details (and Dismiss) stay pinned
	// at the end so the ellipsis can never swallow them — and so is a failure count, in warn,
	// because a failure must never hide inside the ellipsis. The full report is System Activity.
	const runningText = $derived(
		running
			? `Refreshing ${n(running.done)} of ${n(running.total)} · linked ${n(running.linked)}` +
					(running.needs_review ? ` · ${n(running.needs_review)} need review` : '')
			: ''
	);
	const doneText = $derived(
		done && !done.error
			? `Refreshed ${n(done.total)} ${plural} in ${formatDurMs(done.duration_ms)} · linked ${n(done.linked)}` +
					(done.needs_review ? ` · ${n(done.needs_review)} need review` : '') +
					(done.skipped
						? ` · skipped ${n(done.skipped)}` +
							(done.skipped_providers.length ? ` (${skippedText(done.skipped_providers)})` : '')
						: '') +
					(done.stale_skipped ? ` · ${n(done.stale_skipped)} recently refreshed` : '')
			: ''
	);
</script>

{#if isOwner}
	{#if running}
		<p class="flex min-w-0 items-baseline gap-1 text-sm text-muted" role="status" aria-live="polite">
			<span class="min-w-0 truncate" title={runningText}>{runningText}</span>
			{#if running.failed}<span class="shrink-0 text-warn">· {n(running.failed)} failed</span>{/if}
			<a href="/owner/status" class="shrink-0 text-accent hover:underline">· Details</a>
		</p>
	{:else if done?.error}
		<p class="flex min-w-0 items-baseline gap-1 text-sm text-warn" role="alert">
			<span class="min-w-0 truncate" title={done.error}>Couldn't refresh {plural}: {done.error}</span>
			<a href="/owner/status" class="shrink-0 text-accent hover:underline">· Details</a>
		</p>
	{:else if done}
		<p class="flex min-w-0 items-baseline gap-1 text-sm text-muted" role="status" aria-live="polite">
			<span class="min-w-0 truncate" title={doneText}>{doneText}</span>
			{#if done.failed}<span class="shrink-0 text-warn">· {n(done.failed)} failed</span>{/if}
			<a href={batchHref} class="shrink-0 text-accent hover:underline">· Details</a>
			<button
				type="button"
				onclick={() => (dismissedBatch = done.batch_id)}
				class="shrink-0 text-muted hover:underline">· Dismiss</button
			>
		</p>
	{/if}
{/if}
