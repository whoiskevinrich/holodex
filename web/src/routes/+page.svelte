<script lang="ts">
	// Browse view placeholder (F3). Confirms backend connectivity until the
	// media grid + filter bar land.
	let status = $state<'checking' | 'ok' | 'down'>('checking');

	$effect(() => {
		fetch('/api/v1/ping')
			.then((r) => (status = r.ok ? 'ok' : 'down'))
			.catch(() => (status = 'down'));
	});
</script>

<section class="space-y-4">
	<h1 class="text-2xl font-semibold">Library</h1>
	<p class="text-zinc-400">
		Browse grid, filters, and global search land here (Phase 1: F3, F4).
	</p>

	<div class="inline-flex items-center gap-2 rounded border border-zinc-800 px-3 py-2 text-sm">
		<span class="text-zinc-500">API:</span>
		{#if status === 'checking'}
			<span class="text-amber-400">checking…</span>
		{:else if status === 'ok'}
			<span class="text-emerald-400">connected</span>
		{:else}
			<span class="text-red-400">unreachable — is the Go server running on :7800?</span>
		{/if}
	</div>
</section>
