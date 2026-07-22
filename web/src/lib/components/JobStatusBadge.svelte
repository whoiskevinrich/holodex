<script lang="ts">
	// The one error/ok badge for a job run's status (HOLODEX-210). Shared by the
	// digest's per-kind rows and the full history log so the treatment can't drift
	// between them. `--warn` (never `--accent`) marks the failing state.
	let { status }: { status: string } = $props();
</script>

{#if status === 'error'}
	<span class="rounded-theme border border-warn px-1.5 py-0.5 text-[10px] font-semibold text-warn"
		>error</span
	>
{:else}
	<span class="text-muted">ok</span>
{/if}
