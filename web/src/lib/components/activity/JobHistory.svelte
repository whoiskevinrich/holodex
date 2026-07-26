<script lang="ts">
	// 30-day job history (F21.3 + F22.6b): scans and enrich runs, newest first.
	import type { JobRun } from '$lib/types';
	import { formatAgo, formatDurMs, toMessage } from '$lib/format';
	import { api } from '$lib/api';
	import JobStatusBadge from '$lib/components/activity/JobStatusBadge.svelte';

	let { runs }: { runs: JobRun[] } = $props();

	// The count columns only apply to scans; other kinds (enrich) show "—" and
	// describe themselves in the detail row instead.
	const count = (r: JobRun, n: number): number | string => (r.kind === 'scan' ? n : '—');

	// Revert (F48.9d, ADR-067). The batch id is a column on the run (ADR-071); it
	// used to be recovered by regexing the "· batch <id>" suffix out of the
	// free-text detail line, which never matched a merge-propagation batch —
	// those are named "merge-person-N-M" and the pattern required digits — so
	// Revert was silently missing for the one case shared batches exist for.
	function batchId(r: JobRun): string | null {
		return r.kind === 'writeback' && r.batch_id ? r.batch_id : null;
	}

	interface RevertStatus {
		state: 'reverting' | 'done' | 'error';
		error?: string;
	}
	let reverts = $state<Record<number, RevertStatus>>({});

	// A reverted batch's own Revert control disappears (nothing to re-revert from
	// this button) — the revert itself lands as a new job run with its own batch
	// id, which gets its own Revert button on the next history refresh (F48.9c).
	async function revert(r: JobRun) {
		const id = batchId(r);
		if (!id || reverts[r.id]?.state === 'reverting') return;
		reverts = { ...reverts, [r.id]: { state: 'reverting' } };
		try {
			await api.revertWritebackBatch(id);
			reverts = { ...reverts, [r.id]: { state: 'done' } };
		} catch (e) {
			reverts = { ...reverts, [r.id]: { state: 'error', error: toMessage(e) } };
		}
	}
</script>

{#if runs.length === 0}
	<p class="py-16 text-center text-sm text-muted">No jobs recorded yet.</p>
{:else}
	<div class="overflow-x-auto">
		<table class="w-full text-left text-sm">
			<thead class="text-xs uppercase tracking-wide text-muted">
				<tr class="border-b border-rule">
					<th class="py-2 pr-4">When</th>
					<th class="py-2 pr-4">Kind</th>
					<th class="py-2 pr-4">Trigger</th>
					<th class="py-2 pr-4">Duration</th>
					<th class="py-2 pr-4 text-right">Added</th>
					<th class="py-2 pr-4 text-right">Updated</th>
					<th class="py-2 pr-4 text-right">Removed</th>
					<th class="py-2 pr-4 text-right">Errors</th>
					<th class="py-2">Status</th>
				</tr>
			</thead>
			<tbody>
				{#each runs as r (r.id)}
					<tr class="border-b border-rule">
						<td class="py-2 pr-4 whitespace-nowrap text-muted">{formatAgo(r.started_at)}</td>
						<td class="py-2 pr-4 text-ink">{r.kind}</td>
						<td class="py-2 pr-4 text-muted">{r.trigger}</td>
						<td class="py-2 pr-4 tabular-nums text-muted">{formatDurMs(r.duration_ms)}</td>
						<td class="py-2 pr-4 text-right tabular-nums text-muted">{count(r, r.added)}</td>
						<td class="py-2 pr-4 text-right tabular-nums text-muted">{count(r, r.updated)}</td>
						<td class="py-2 pr-4 text-right tabular-nums text-muted">{count(r, r.removed)}</td>
						<td
							class="py-2 pr-4 text-right tabular-nums {r.kind === 'scan' && r.errors > 0
								? 'text-warn'
								: 'text-muted'}"
							>{count(r, r.errors)}</td
						>
						<td class="py-2"><JobStatusBadge status={r.status} /></td>
					</tr>
					{#if r.detail || r.error_message}
						{@const batch = batchId(r)}
						<tr>
							<td colspan="9" class="pb-2 pr-4">
								<div class="flex flex-wrap items-center justify-between gap-2">
									<p class="text-xs text-muted">
										{r.detail}{#if r.detail && r.error_message} — {/if}{r.error_message}
									</p>
									{#if batch}
										{#if reverts[r.id]?.state === 'done'}
											<span class="shrink-0 text-xs text-muted">Reverted</span>
										{:else if reverts[r.id]?.state === 'error'}
											<span class="shrink-0 text-xs text-warn" role="alert">{reverts[r.id].error}</span>
										{:else}
											<button
												onclick={() => revert(r)}
												disabled={reverts[r.id]?.state === 'reverting'}
												aria-busy={reverts[r.id]?.state === 'reverting'}
												class="btn-ghost shrink-0 px-2.5 py-1.5 text-xs"
											>
												{reverts[r.id]?.state === 'reverting' ? 'Reverting…' : 'Revert'}
											</button>
										{/if}
									{/if}
								</div>
							</td>
						</tr>
					{/if}
				{/each}
			</tbody>
		</table>
	</div>
{/if}
