<script lang="ts">
	// Per-kind activity digest (HOLODEX-210, ADR-071 D3): the default view of the
	// job history. Answers "is each job still running, and did anything fail in the
	// last 30 days" without loading every run — its size tracks the number of job
	// kinds, not the run count.
	//
	// Dismiss (HOLODEX-416, ADR-100): the owner clears a failure they have handled
	// from the callout — per row, or every one in the window at once — and the
	// per-kind error count follows. Nothing is deleted; the Log keeps every run.
	// The mutation is applied locally (dismissDigest.ts) and reported up through
	// `onchange`, so the row leaves instantly and the next loadDigest() agrees.
	import { tick } from 'svelte';
	import type { JobDigest, JobRun } from '$lib/types';
	import { api } from '$lib/api';
	import { formatAgo, toMessage } from '$lib/format';
	import JobStatusBadge from '$lib/components/activity/JobStatusBadge.svelte';
	import { totalFailures as sumFailures, withoutFailures, withoutRun } from './dismissDigest';

	let {
		digest,
		isOwner = false,
		onchange,
		onerror
	}: {
		digest: JobDigest;
		isOwner?: boolean;
		onchange?: (next: JobDigest) => void;
		onerror?: (message: string) => void;
	} = $props();

	const totalFailures = $derived(sumFailures(digest));

	// Same composer as ExtractionQueueRow's GHOST: an immediate, row-clearing
	// resolve. `ml-auto shrink-0` keeps it at the end of the row's last line when
	// a long detail wraps; the button itself never wraps.
	const GHOST = 'btn-row btn-ghost px-2 ml-auto shrink-0';

	let dismissing = $state<Record<number, true>>({});
	let dismissingAll = $state(false);
	let list = $state<HTMLUListElement | undefined>();
	let root = $state<HTMLDivElement | undefined>();

	// Focus never lands on <body> after a row leaves: the next row's Dismiss (same
	// index), else the last one, else the section heading once the callout unmounts.
	async function refocus(index: number) {
		await tick();
		const buttons = list?.querySelectorAll<HTMLButtonElement>('button');
		if (buttons?.length) {
			buttons[Math.min(index, buttons.length - 1)].focus();
			return;
		}
		root?.closest('section')?.querySelector<HTMLElement>('h2')?.focus();
	}

	async function dismiss(run: JobRun, index: number) {
		if (dismissing[run.id] || dismissingAll) return;
		dismissing = { ...dismissing, [run.id]: true };
		try {
			await api.dismissJobRun(run.id);
			// Already-dismissed (second tab) answers dismissed:false; the row leaves
			// either way — it is gone server-side.
			onchange?.(withoutRun(digest, run));
			refocus(index);
		} catch (e) {
			onerror?.(`Couldn't dismiss — ${toMessage(e)}`);
		} finally {
			const { [run.id]: _, ...rest } = dismissing;
			dismissing = rest;
		}
	}

	async function dismissAll() {
		if (dismissingAll) return;
		dismissingAll = true;
		try {
			await api.dismissJobFailures();
			onchange?.(withoutFailures(digest));
			refocus(0);
		} catch (e) {
			onerror?.(`Couldn't dismiss — ${toMessage(e)}`);
		} finally {
			dismissingAll = false;
		}
	}
</script>

{#if digest.kinds.length === 0}
	<p class="py-16 text-center text-sm text-muted">No jobs recorded yet.</p>
{:else}
	<div class="space-y-4" bind:this={root}>
		{#if totalFailures > 0}
			<div class="rounded-theme border border-warn bg-surface px-3 py-2" role="alert">
				<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
					<h3 class="text-xs font-semibold uppercase tracking-wide text-warn">
						{totalFailures} recent {totalFailures === 1 ? 'failure' : 'failures'}
						{#if digest.failures.length < totalFailures}
							<span class="font-normal text-muted">· showing the most recent {digest.failures.length}</span>
						{/if}
					</h3>
					{#if isOwner}
						<!-- The count doubles as the "how many am I clearing" cue — no confirm (D3). -->
						<button onclick={dismissAll} disabled={dismissingAll} class="btn-quiet px-2 py-0.5 text-xs"
							>{dismissingAll ? 'Dismissing…' : `Dismiss all ${totalFailures}`}</button
						>
					{/if}
				</div>
				<ul class="mt-1.5 space-y-1" bind:this={list}>
					{#each digest.failures as f, i (f.id)}
						<li class="flex flex-wrap items-baseline gap-x-2 text-sm">
							<span class="text-ink">{f.kind}</span>
							<span class="text-xs text-muted">{formatAgo(f.started_at)}</span>
							<span class="min-w-0 flex-1 wrap-anywhere text-xs text-muted">{f.detail || f.error_message}</span>
							{#if isOwner}
								<button
									onclick={() => dismiss(f, i)}
									disabled={!!dismissing[f.id] || dismissingAll}
									aria-busy={!!dismissing[f.id]}
									aria-label="Dismiss {f.kind} failure from {formatAgo(f.started_at)}"
									class={GHOST}>{dismissing[f.id] ? 'Dismissing…' : 'Dismiss'}</button
								>
							{/if}
						</li>
					{/each}
				</ul>
			</div>
		{/if}

		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm">
				<thead class="text-xs uppercase tracking-wide text-muted">
					<tr class="border-b border-rule">
						<th class="py-2 pr-4">Job</th>
						<th class="py-2 pr-4">Last run</th>
						<th class="py-2 pr-4 text-right">Runs</th>
						<th class="py-2 pr-4 text-right">Errors</th>
						<th class="py-2">Status</th>
					</tr>
				</thead>
				<tbody>
					{#each digest.kinds as k (k.kind)}
						<tr class="border-b border-rule">
							<td class="py-2 pr-4 text-ink">{k.kind}</td>
							<td class="py-2 pr-4 whitespace-nowrap text-muted">{formatAgo(k.last_run)}</td>
							<td class="py-2 pr-4 text-right tabular-nums text-muted">{k.runs}</td>
							<td class="py-2 pr-4 text-right tabular-nums {k.errors > 0 ? 'text-warn' : 'text-muted'}"
								>{k.errors}</td
							>
							<td class="py-2 whitespace-nowrap"
								><JobStatusBadge status={k.last_status} dismissed={k.last_dismissed} /></td
							>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}
