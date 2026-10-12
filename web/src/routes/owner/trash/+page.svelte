<script lang="ts">
	// Trash view (F24, ADR-037): owner-only list of soft-deleted items, each with
	// Restore (safe — accent) and Delete permanently (destructive — warn, confirmed).
	// A tab under the Owner hub (F35); the /owner gate hides it from non-owners and the
	// API 401s. Tokens only; QA Cinémathèque.
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { formatAgo, formatUntil, toMessage } from '$lib/format';
	import type { TrashEntry } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';

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
	// (layout); re-run when isOwner flips (e.g. after the token is entered). Auto-reveal
	// (F29 P0-6) now fires once at the /owner layout gate (F35), not per page.
	$effect(() => {
		if (activity.isOwner) {
			load();
		} else loading = false;
	});

	// One list holds media and films (F78 RD7); each row restores and purges through
	// its own kind's endpoints. Rows are keyed by kind too — a film and a video can
	// share an id.
	const rowKey = (it: TrashEntry) => `${it.kind}:${it.id}`;

	async function restore(it: TrashEntry) {
		rowError = '';
		try {
			await (it.kind === 'film' ? api.restoreFilm(it.id) : api.restoreMedia(it.id));
			items = items.filter((x) => rowKey(x) !== rowKey(it));
		} catch (e) {
			rowError = toMessage(e);
		}
	}

	async function purge() {
		if (!confirm || busy) return;
		busy = true;
		rowError = '';
		const it = confirm;
		try {
			await (it.kind === 'film'
				? api.deleteFilm(it.id, { purge: true })
				: api.deleteMedia(it.id, { purge: true }));
			items = items.filter((x) => rowKey(x) !== rowKey(it));
			confirm = null;
		} catch (e) {
			rowError = toMessage(e); // shown on the page; the item stays in Trash to retry
			confirm = null;
		} finally {
			busy = false;
		}
	}
</script>

<!-- Kept narrower than the stage, same reasoning as owner/keys: these rows are
     `flex` with a `flex-1` title and `shrink-0` buttons, so full stage width would put a
     row's Delete button ~2400px from its title (HOLODEX-331). -->
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
					{#each items as it (rowKey(it))}
						<li class="flex flex-wrap items-center gap-3 rounded-theme border border-rule bg-surface p-3">
							<div class="min-w-0 flex-1">
								<!-- Kind label (F78 RD7): a status encoding, not an affordance. -->
								<p class="flex min-w-0 items-center gap-2 text-sm text-ink">
									<span class="shrink-0 rounded-theme border border-rule px-1.5 text-xs text-muted"
										>{it.kind === 'film' ? 'Film' : 'Media'}</span
									>
									<span class="truncate" title={it.title}>{it.title}</span>
								</p>
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
			{#if confirm?.kind === 'film'}
				<!-- Dialog C's copy (delete-film-handoff.md); no path, a film has no file. -->
				<p>
					<span class="font-semibold">{confirm.title}</span>, its cast, images and aliases will be
					<span class="font-semibold">permanently removed</span>. This cannot be undone.
				</p>
				<p class="text-xs text-muted">Its linked videos stay in your library. No media file is touched.</p>
			{:else}
				<p>
					<span class="font-semibold">{confirm?.title}</span> and its file will be
					<span class="font-semibold">permanently removed</span> from disk. This cannot be undone.
				</p>
				<p class="truncate font-mono text-xs text-muted" title={confirm?.path}>{confirm?.path}</p>
			{/if}
		{/snippet}
	</ConfirmDialog>
{/if}
