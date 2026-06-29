<script lang="ts">
	// Trash view (F24, ADR-037): owner-only list of soft-deleted items, each with
	// Restore (safe — accent) and Delete permanently (destructive — warn, confirmed).
	// Hidden from non-owners (the header link is gated too, and the API 401s). Tokens
	// only; QA 3 skins.
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { adminMode } from '$lib/adminMode.svelte';
	import { formatAgo, formatUntil, toMessage } from '$lib/format';
	import type { TrashEntry } from '$lib/types';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	let items = $state<TrashEntry[]>([]);
	let loading = $state(true);
	let error = $state('');
	let rowError = $state(''); // inline failure for a restore/purge action
	let confirm = $state<TrashEntry | null>(null); // item pending a permanent-delete confirm
	let busy = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			items = (await api.trash()).items;
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}

	// Owner-gated: only fetch once we know the caller is the owner. caps load app-wide
	// (layout); re-run when isOwner flips (e.g. after the token is entered).
	$effect(() => {
		if (activity.isOwner) {
			// Auto-reveal (F29 P0-6): /trash is an owner-only route, so landing here in
			// visitor view flips Admin mode back on (and announces it) rather than
			// stranding the owner on an empty page.
			adminMode.reveal();
			load();
		} else loading = false;
	});

	async function restore(it: TrashEntry) {
		rowError = '';
		try {
			await api.restoreMedia(it.id);
			items = items.filter((x) => x.id !== it.id);
		} catch (e) {
			rowError = toMessage(e);
		}
	}

	async function purge() {
		if (!confirm || busy) return;
		busy = true;
		rowError = '';
		const id = confirm.id;
		try {
			await api.deleteMedia(id, { purge: true });
			items = items.filter((x) => x.id !== id);
			confirm = null;
		} catch (e) {
			rowError = toMessage(e); // shown on the page; the item stays in Trash to retry
			confirm = null;
		} finally {
			busy = false;
		}
	}
</script>

<section class="mx-auto max-w-4xl space-y-4">
	<header class="space-y-1">
		<h1 class="skin-title text-2xl font-semibold text-ink">Trash</h1>
		<p class="text-sm text-muted">
			Deleted items are hidden from your library and can be restored here
			{#if items.length && items[0].purge_at}until they are permanently removed{:else}until you delete them permanently{/if}.
		</p>
	</header>

	{#if !activity.isOwner}
		<p class="text-sm text-muted">Owner only.</p>
	{:else}
		<AsyncState {loading} {error} loadingText="Loading Trash…">
			{#if rowError}
				<p class="rounded-theme border border-warn bg-surface px-3 py-2 text-sm text-warn" aria-live="polite">
					{rowError}
				</p>
			{/if}
			{#if items.length === 0}
				<p class="py-12 text-center text-sm text-muted">Trash is empty.</p>
			{:else}
				<ul class="space-y-2" aria-live="polite">
					{#each items as it (it.id)}
						<li class="flex flex-wrap items-center gap-3 rounded-theme border border-rule bg-surface p-3">
							<div class="min-w-0 flex-1">
								<p class="truncate text-sm text-ink" title={it.title}>{it.title}</p>
								<p class="text-xs text-muted">
									deleted {formatAgo(it.deleted_at)}{#if it.purge_at} · purges {formatUntil(it.purge_at)}{/if}
								</p>
							</div>
							<div class="flex shrink-0 gap-2">
								<button
									onclick={() => restore(it)}
									class="rounded-theme border border-accent px-3 py-1.5 text-sm text-accent hover:bg-accent/10"
								>
									Restore
								</button>
								<button
									onclick={() => {
										rowError = '';
										confirm = it;
									}}
									class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
								>
									Delete permanently
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</AsyncState>
	{/if}
</section>

{#if confirm}
	<ConfirmDialog
		title="Delete permanently?"
		confirmLabel="Delete permanently"
		{busy}
		onconfirm={purge}
		oncancel={() => (confirm = null)}
	>
		{#snippet body()}
			<p>
				<span class="font-semibold">{confirm?.title}</span> and its file will be
				<span class="font-semibold">permanently removed</span> from disk. This cannot be undone.
			</p>
			<p class="truncate font-mono text-xs text-muted" title={confirm?.path}>{confirm?.path}</p>
		{/snippet}
	</ConfirmDialog>
{/if}
