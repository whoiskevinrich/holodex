<script lang="ts">
	// Per-kind activity digest (HOLODEX-210, ADR-071 D3): the default view of the
	// job history. Answers "is each job still running, and did anything fail in the
	// last 30 days" without loading every run — its size tracks the number of job
	// kinds, not the run count.
	import type { JobDigest } from '$lib/types';
	import { formatAgo } from '$lib/format';
	import JobStatusBadge from '$lib/components/activity/JobStatusBadge.svelte';

	let { digest }: { digest: JobDigest } = $props();

	// The true failure count is the sum of each kind's error count, which is always
	// on the wire — the inline `failures` list is capped (digestFailureCap), so its
	// length under-reports after a bad batch. Show the real total and note when the
	// list is only the most recent slice of it.
	const totalFailures = $derived(digest.kinds.reduce((n, k) => n + k.errors, 0));
</script>

{#if digest.kinds.length === 0}
	<p class="py-16 text-center text-sm text-muted">No jobs recorded yet.</p>
{:else}
	<div class="space-y-4">
		{#if totalFailures > 0}
			<div class="rounded-theme border border-warn bg-surface px-3 py-2" role="alert">
				<h3 class="text-xs font-semibold uppercase tracking-wide text-warn">
					{totalFailures} recent {totalFailures === 1 ? 'failure' : 'failures'}
					{#if digest.failures.length < totalFailures}
						<span class="font-normal text-muted">· showing the most recent {digest.failures.length}</span>
					{/if}
				</h3>
				<ul class="mt-1.5 space-y-1">
					{#each digest.failures as f (f.id)}
						<li class="flex flex-wrap items-baseline gap-x-2 text-sm">
							<span class="text-ink">{f.kind}</span>
							<span class="text-xs text-muted">{formatAgo(f.started_at)}</span>
							<span class="text-xs text-muted">{f.detail || f.error_message}</span>
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
							<td class="py-2"><JobStatusBadge status={k.last_status} /></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}
