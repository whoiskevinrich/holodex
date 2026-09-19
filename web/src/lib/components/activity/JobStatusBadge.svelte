<script lang="ts">
	// The one error/ok badge for a job run's status (HOLODEX-210). Shared by the
	// digest's per-kind rows and the full history log so the treatment can't drift
	// between them. `--warn` (never `--accent`) marks the failing state.
	//
	// `dismissed` (HOLODEX-416, handoff D5): the run did fail and the owner has
	// handled it — the same badge shape with `--rule`/`--muted` in place of
	// `--warn`, plus the `· dismissed` marker. One prop rather than a second
	// component so the digest and the Log read identically. Only meaningful for
	// an error; an ok run is never dismissed.
	let { status, dismissed = false }: { status: string; dismissed?: boolean } = $props();
</script>

{#if status === 'error' && dismissed}
	<span class="rounded-theme border border-rule px-1.5 py-0.5 text-[10px] font-semibold text-muted"
		>error</span
	>
	<span class="text-xs text-muted">· dismissed</span>
{:else if status === 'error'}
	<span class="rounded-theme border border-warn px-1.5 py-0.5 text-[10px] font-semibold text-warn"
		>error</span
	>
{:else}
	<span class="text-muted">ok</span>
{/if}
