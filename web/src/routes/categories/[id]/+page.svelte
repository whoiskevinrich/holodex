<script lang="ts">
	// Category detail (HOLODEX-240 §3) — deliberately sparse sibling of tags/[id]:
	// no video grid, no ancestor breadcrumb, no video-count hero line (categories
	// are flat and don't attach to videos directly, spec Non-Goals). The member-tag
	// chip section is the exact curation-chip idiom already shipped on the media
	// page's Tags section (media/[id]/+page.svelte), adapted from a video's tags to
	// a category's: add resolves-or-creates the tag by name (no video attach) then
	// assigns it; remove unassigns rather than detaching a video link.
	import { page } from '$app/stores';
	import { api, ApiError } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, tagCount, videoCount } from '$lib/format';
	import type { Category, EntityRef } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import TagLinkChip from '$lib/components/entity/TagLinkChip.svelte';

	let category = $state<Category | null>(null);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	function load() {
		loading = true;
		error = '';
		return api
			.getCategory(id)
			.then((res) => (category = res.category))
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	}

	$effect(() => {
		void id; // re-run if the route param changes
		load();
	});

	// ── Rename (pencil button → inline form; no ⋯ menu on this page) ────────────────
	let renaming = $state(false);
	let renameValue = $state('');
	let renameBusy = $state(false);
	let renameError = $state('');
	let renameInput = $state<HTMLInputElement | null>(null);

	async function openRename() {
		if (!category) return;
		renameValue = category.name;
		renameError = '';
		renaming = true;
		await Promise.resolve();
		renameInput?.focus();
		renameInput?.select();
	}

	function closeRename() {
		renaming = false;
		renameError = '';
	}

	async function submitRename(e: SubmitEvent) {
		e.preventDefault();
		if (!category || renameBusy) return;
		const name = renameValue.trim();
		if (!name) return;
		renameBusy = true;
		renameError = '';
		try {
			const res = await api.renameCategory(category.id, name);
			category = res.category;
			closeRename();
		} catch (err) {
			renameError =
				err instanceof ApiError && err.status === 409
					? `“${name}” already names a tag or another category.`
					: toMessage(err);
		} finally {
			renameBusy = false;
		}
	}

	// ── Member-tag add/remove — mirrors media/[id]'s Tags section verbatim ──────────
	let tagAddOpen = $state(false);
	let tagAddValue = $state('');
	let tagInput = $state<HTMLInputElement | null>(null);
	let tagBusy = $state(false);
	let tagError = $state('');
	let tagNearMiss = $state<EntityRef | null>(null);
	let tagJustAdded = $state<EntityRef | null>(null);

	function resetTagForm() {
		tagAddValue = '';
		tagError = '';
		tagNearMiss = null;
		tagJustAdded = null;
	}

	async function openTagAdd() {
		resetTagForm();
		tagAddOpen = true;
		await Promise.resolve();
		tagInput?.focus();
	}

	function closeTagAdd() {
		resetTagForm();
		tagAddOpen = false;
	}

	async function runTagAction(fn: () => Promise<void>, formatError?: (err: unknown) => string) {
		if (tagBusy) return;
		tagBusy = true;
		tagError = '';
		try {
			await fn();
		} catch (err) {
			tagError = formatError ? formatError(err) : toMessage(err);
		} finally {
			tagBusy = false;
		}
	}

	function submitTagAdd(e: SubmitEvent) {
		e.preventDefault();
		if (!category) return;
		const name = tagAddValue.trim();
		if (!name) return;
		const categoryId = category.id;
		runTagAction(
			async () => {
				const { tag } = await api.resolveOrCreateTag(name);
				const res = await api.assignCategoryTags(categoryId, [tag.id]);
				category = res.category;
				tagJustAdded = { id: tag.id, name: tag.name };
				const nm = await api.nearMiss('tag', tag.id, name).then((r) => r.near_miss);
				if (nm) tagNearMiss = nm;
				else closeTagAdd();
			},
			(err) =>
				err instanceof ApiError && err.status === 422 ? `'${name}' is on the deny-list.` : toMessage(err)
		);
	}

	// "Use existing": swap the just-added tag for the near-miss it looks like —
	// resolve-or-create the near-miss's exact name (no new row, since it already
	// exists), assign that, then unassign the tag the add just created/resolved.
	async function useTagNearMiss() {
		if (!category || !tagNearMiss || !tagJustAdded) return;
		const categoryId = category.id;
		const nearMissName = tagNearMiss.name;
		const justAddedId = tagJustAdded.id;
		await runTagAction(async () => {
			const { tag } = await api.resolveOrCreateTag(nearMissName);
			// Two independent writes (different tag ids) — run concurrently, then
			// reload once: either response alone could race the other's write and
			// miss it, so neither is safe to trust as the final state on its own.
			await Promise.all([
				api.assignCategoryTags(categoryId, [tag.id]),
				api.unassignCategoryTags(categoryId, [justAddedId])
			]);
			await load();
			closeTagAdd();
		});
	}

	function removeTag(tagId: number) {
		if (!category) return;
		const categoryId = category.id;
		runTagAction(async () => {
			const res = await api.unassignCategoryTags(categoryId, [tagId]);
			category = res.category;
		});
	}
</script>

<AsyncState {loading} error={error || (!category ? 'Not found.' : '')}>
	{#if category}
		<section class="space-y-4">
			<div>
				{#if renaming}
					<form onsubmit={submitRename} class="flex flex-wrap items-center gap-2">
						<input
							bind:this={renameInput}
							bind:value={renameValue}
							type="text"
							aria-label="Rename category"
							class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-lg text-ink focus:border-accent focus:outline-none"
						/>
						<button type="submit" disabled={renameBusy} class="btn-accent px-3 py-1.5 text-sm">Rename</button>
						<button type="button" onclick={closeRename} disabled={renameBusy} class="btn-quiet px-3 py-1.5 text-sm">
							Cancel
						</button>
						{#if renameError}<p class="w-full text-sm text-warn">{renameError}</p>{/if}
					</form>
				{:else}
					<div class="flex items-center gap-2">
						<h1 class="skin-title text-2xl font-semibold text-ink">{category.name}</h1>
						{#if isOwner}
							<button
								aria-label="Rename category"
								onclick={openRename}
								class="rounded-theme border border-rule p-1.5 text-muted hover:border-accent hover:text-ink"
							>
								<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z"
									/>
								</svg>
							</button>
						{/if}
					</div>
				{/if}
				<p class="text-sm text-muted">{tagCount(category.tags?.length ?? 0)}</p>
			</div>

			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
				<div class="flex flex-wrap items-center gap-2">
					{#each category.tags ?? [] as t (t.id)}
						<TagLinkChip tag={t} busy={tagBusy} onremove={isOwner ? removeTag : undefined} />
					{/each}

					{#if isOwner}
						{#if tagAddOpen}
							<form onsubmit={submitTagAdd} class="inline-flex items-center gap-2">
								<input
									bind:this={tagInput}
									bind:value={tagAddValue}
									type="text"
									placeholder="Add a tag"
									aria-label="Add a tag"
									class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
								/>
								<button type="submit" disabled={tagBusy} class="btn-accent px-3 py-1.5 text-sm">Add</button>
								<button type="button" onclick={closeTagAdd} disabled={tagBusy} class="btn-quiet px-3 py-1.5 text-sm">
									Cancel
								</button>
							</form>
						{:else}
							<button type="button" onclick={openTagAdd} class="btn-quiet px-3 py-1.5 text-sm">+ Add tag</button>
						{/if}
					{/if}
				</div>

				{#if tagNearMiss}
					<!-- Non-blocking near-miss nudge (verbatim copy from media/[id]'s tagNearMiss card)
					     — the attach already succeeded; this only offers to consolidate onto the look-alike. -->
					<div class="flex flex-wrap items-center gap-2 rounded-theme border border-rule bg-surface-2 px-3 py-2">
						<p class="text-sm text-ink">
							Looks a lot like <span class="font-semibold">{tagNearMiss.name}</span>
							({videoCount(tagNearMiss.video_count ?? 0)}) — use that instead?
						</p>
						<button type="button" onclick={useTagNearMiss} disabled={tagBusy} class="btn-accent px-3 py-1.5 text-sm">
							Use existing
						</button>
						<button type="button" onclick={closeTagAdd} disabled={tagBusy} class="btn-ghost px-3 py-1.5 text-sm">
							Add as new anyway
						</button>
					</div>
				{/if}
				{#if tagError}
					<p class="text-sm text-warn">{tagError}</p>
				{/if}
			</section>
		</section>
	{/if}
</AsyncState>
