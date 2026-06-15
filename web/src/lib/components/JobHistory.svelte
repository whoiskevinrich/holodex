<script lang="ts">
	// 30-day scan history (F21.3): the /keys table idiom, newest first.
	import type { JobRun } from '$lib/types';
	import { formatAgo, formatDurMs } from '$lib/format';

	let { runs }: { runs: JobRun[] } = $props();
</script>

{#if runs.length === 0}
	<p class="py-16 text-center text-sm text-muted">No scans recorded yet.</p>
{:else}
	<div class="overflow-x-auto">
		<table class="w-full text-left text-sm">
			<thead class="text-xs uppercase tracking-wide text-muted">
				<tr class="border-b border-rule">
					<th class="py-2 pr-4">When</th>
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
						<td class="py-2 pr-4 text-ink">{r.trigger}</td>
						<td class="py-2 pr-4 tabular-nums text-muted">{formatDurMs(r.duration_ms)}</td>
						<td class="py-2 pr-4 text-right tabular-nums text-muted">{r.added}</td>
						<td class="py-2 pr-4 text-right tabular-nums text-muted">{r.updated}</td>
						<td class="py-2 pr-4 text-right tabular-nums text-muted">{r.removed}</td>
						<td
							class="py-2 pr-4 text-right tabular-nums {r.errors > 0 ? 'text-warn' : 'text-muted'}"
							>{r.errors}</td
						>
						<td class="py-2">
							{#if r.status === 'error'}
								<span
									class="rounded-theme border border-warn px-1.5 py-0.5 text-[10px] font-semibold text-warn"
									>error</span
								>
							{:else}
								<span class="text-muted">ok</span>
							{/if}
						</td>
					</tr>
					{#if r.error_message}
						<tr>
							<td colspan="8" class="pb-2 pr-4 text-xs text-muted">{r.error_message}</td>
						</tr>
					{/if}
				{/each}
			</tbody>
		</table>
	</div>
{/if}
