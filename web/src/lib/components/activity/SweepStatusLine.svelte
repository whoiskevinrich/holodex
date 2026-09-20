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
</script>

{#if isOwner}
	{#if running}
		<p class="text-sm text-muted" role="status" aria-live="polite">
			Refreshing {plural} in the background — {n(running.done)} of {n(running.total)}
			{' · '}linked {n(running.linked)}{#if running.needs_review}{' · '}{n(running.needs_review)} need review{/if}{#if running.failed}{' · '}<span
					class="text-warn">{n(running.failed)} failed</span
				>{/if}. You can leave this page.
		</p>
	{:else if done?.error}
		<p class="text-sm text-warn" role="alert">
			Couldn't refresh {plural}: {done.error}. Try again from
			<a href="/owner/status" class="text-accent hover:underline">System Activity</a>.
		</p>
	{:else if done}
		<p class="text-sm text-muted wrap-anywhere" role="status" aria-live="polite">
			Refreshed {n(done.total)} {plural} in {formatDurMs(done.duration_ms)} —
			<a href={batchHref} class="text-accent hover:underline">Linked {n(done.linked)}</a
			>{#if done.needs_review}{' · '}<a href={batchHref} class="text-accent hover:underline"
					>{n(done.needs_review)} need review</a
				>{/if}{#if done.failed}{' · '}<span class="text-warn">{n(done.failed)} failed</span
				>{/if}{#if done.skipped}{' · '}Skipped {n(done.skipped)}{#if done.skipped_providers.length}
					({skippedText(done.skipped_providers)}){/if}{/if}{#if done.stale_skipped}{' · '}{n(
					done.stale_skipped
				)} recently refreshed{/if}{' · '}<a href={batchHref} class="text-accent hover:underline"
				>View in System Activity</a
			>{' · '}<button
				type="button"
				onclick={() => (dismissedBatch = done.batch_id)}
				class="text-muted hover:underline">Dismiss</button
			>
		</p>
	{/if}
{/if}
