<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, videoCount, tagCount, aliasHint } from '$lib/format';
	import { findTagByName, cycleMessage } from '$lib/tagHierarchy';
	import type { Category, EntityRef, Tag, Video } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import EntityVideos from '$lib/components/entity/EntityVideos.svelte';
	import FilmsRow from '$lib/components/entity/FilmsRow.svelte';
	import { filmsRow } from '$lib/filmsRow.svelte';
	import CategoryPicker from '$lib/components/entity/CategoryPicker.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import WritebackBatchDialog from '$lib/components/writeback/WritebackBatchDialog.svelte';
	import NameEditControl from '$lib/components/entity/NameEditControl.svelte';
	import MergeOfferCard from '$lib/components/entity/MergeOfferCard.svelte';

	let tag = $state<Tag | null>(null);
	let videos = $state<Video[]>([]);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	let toggleBusy = $state(false);
	let toggleError = $state('');
	let syncOpen = $state(false);

	$effect(() => {
		const current = id;
		loading = true;
		error = '';
		api
			.getTag(current)
			.then((res) => {
				tag = res.tag;
				videos = res.items ?? [];
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});

	// Films row (F56, design handoff §5) — see filmsRow.svelte.ts.
	const filmsState = filmsRow(() => id, 'tagId');

	async function reloadTag() {
		if (!tag) return;
		try {
			const res = await api.getTag(tag.id);
			tag = res.tag;
			videos = res.items ?? [];
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
	}

	// Rename (HOLODEX-269, docked-pencil NameEditControl beside the hero name). A 409
	// shows the merge offer inline; these two only cover the inline verdict's own
	// busy/error (NameEditControl owns the rest of the rename flow itself).
	let renameMergeBusy = $state(false);
	let renameMergeError = $state('');
	// Non-blocking near-miss advisory (F43 P1-5, mirrors AliasPanel's flagNearMiss) — a
	// fuzzy look-alike surfaced after a successful rename, distinct from the exact-name
	// `conflict` above.
	let nearMiss = $state<EntityRef | null>(null);
	let nearMissBusy = $state(false);
	let nearMissError = $state('');

	async function commitTagRename(value: string): Promise<{ ok: true } | { conflict: EntityRef }> {
		const res = await api.renameEntity('tag', id, value);
		if (res.conflict) return { conflict: res.conflict };
		await reloadTag();
		// Advisory-only fuzzy look-alike check, same as AliasPanel's post-add flagNearMiss and
		// the tag list page's manage-mode rename — must never block the rename that already
		// succeeded.
		try {
			nearMiss = (await api.nearMiss('tag', id, value)).near_miss;
		} catch {
			nearMiss = null;
		}
		return { ok: true };
	}

	async function mergeRenameConflict(mergeConflict: EntityRef, resolve: () => void) {
		renameMergeBusy = true;
		renameMergeError = '';
		try {
			await api.mergeEntities('tag', id, mergeConflict.id);
			resolve();
			await reloadTag();
		} catch (e) {
			renameMergeError = toMessage(e);
		} finally {
			renameMergeBusy = false;
		}
	}

	async function mergeNearMiss() {
		if (!nearMiss) return;
		nearMissBusy = true;
		nearMissError = '';
		try {
			await api.mergeEntities('tag', id, nearMiss.id);
			nearMiss = null;
			await reloadTag();
		} catch (e) {
			nearMissError = toMessage(e);
		} finally {
			nearMissBusy = false;
		}
	}

	async function keepNearMissSeparate() {
		if (!nearMiss) return;
		nearMissBusy = true;
		nearMissError = '';
		try {
			await api.dismissDuplicate('tag', id, nearMiss.id);
			nearMiss = null;
		} catch (e) {
			nearMissError = toMessage(e);
		} finally {
			nearMissBusy = false;
		}
	}

	// Posts immediately, no confirm step (HOLODEX-239, ADR-077 D1) — toggling the
	// flag alone never enqueues a write, so it's the lowest-stakes control on the
	// card. tag is reassigned from the PATCH response so the label/glyph flip
	// without a refetch.
	async function toggleWriteback() {
		if (!tag || toggleBusy) return;
		toggleBusy = true;
		toggleError = '';
		try {
			tag = (await api.setTagWriteback(tag.id, !tag.writeback_enabled)).tag;
		} catch (e) {
			toggleError = toMessage(e);
		} finally {
			toggleBusy = false;
		}
	}

	// ── Parent (F50 S8, ADR-075 D1, HOLODEX-259) ─────────────────────────────────────
	// Typeahead resolves against a lazily-loaded full tag list, same discipline as the
	// /tags list's Manage-mode parent control (existing-tags-only, no create-on-typo) —
	// the matching logic itself is shared via $lib/tagHierarchy.
	let allTags = $state<Tag[]>([]);
	let parentOpen = $state(false);
	let parentValue = $state('');
	let parentBusy = $state(false);
	let parentError = $state('');
	let parentInput = $state<HTMLInputElement | null>(null);

	async function openParentAdd() {
		parentValue = '';
		parentError = '';
		parentOpen = true;
		if (allTags.length === 0) allTags = (await api.listTags('name')).items ?? [];
		await Promise.resolve();
		parentInput?.focus();
	}

	function closeParentAdd() {
		parentOpen = false;
		parentError = '';
	}

	// × on the chip clears immediately, no confirm — the lowest-stakes control on
	// this card (same precedent as the writeback toggle above).
	async function applyParent(parentId: number | null) {
		if (!tag || parentBusy) return;
		parentBusy = true;
		parentError = '';
		try {
			const res = await api.setTagParent(tag.id, parentId);
			if (res.cycle) {
				parentError = cycleMessage(tag.name);
				return;
			}
			await reloadTag();
			closeParentAdd();
		} catch (e) {
			parentError = toMessage(e);
		} finally {
			parentBusy = false;
		}
	}

	function submitParentAdd(e: SubmitEvent) {
		e.preventDefault();
		if (!tag) return;
		const name = parentValue.trim();
		if (!name || parentBusy) return;
		const match = findTagByName(allTags, name, tag.id);
		if (!match) {
			parentError = `No tag named "${name}".`;
			return;
		}
		applyParent(match.id);
	}

	// ── Children (F50 S8, ADR-075 D1, HOLODEX-259) ───────────────────────────────────
	// "+ Add child" resolves-or-creates by name, same idiom as categories/[id]'s
	// "+ Add tag". A brand-new tag or a childless root attaches immediately (low blast
	// radius); a candidate that already has its own parent or children interrupts with
	// a confirm first, since attaching it here would relocate an established branch —
	// see docs/design/tag-detail-hierarchy-reparent-confirm-handoff.md.
	let childOpen = $state(false);
	let childValue = $state('');
	let childBusy = $state(false);
	let childError = $state('');
	let childInput = $state<HTMLInputElement | null>(null);
	let childRemoveBusy = $state<number | null>(null);

	let confirmCandidate = $state<Tag | null>(null);
	let confirmBusy = $state(false);
	let confirmError = $state('');

	async function openChildAdd() {
		childValue = '';
		childError = '';
		childOpen = true;
		await Promise.resolve();
		childInput?.focus();
	}

	function closeChildAdd() {
		childOpen = false;
		childValue = '';
		childError = '';
	}

	function childCycleMessage(childName: string, parentName: string): string {
		return `Can't move "${childName}" here — ${parentName} is already under it.`;
	}

	// Returns true on a cycle (caller surfaces the error in whichever surface was
	// showing — the add-child form or the confirm dialog); reloads the current tag's
	// Children on success.
	async function attachChild(childId: number): Promise<boolean> {
		if (!tag) return false;
		const res = await api.setTagParent(childId, tag.id);
		if (res.cycle) return true;
		await reloadTag();
		return false;
	}

	async function submitChild(e: SubmitEvent) {
		e.preventDefault();
		if (!tag) return;
		const name = childValue.trim();
		if (!name || childBusy) return;
		childBusy = true;
		childError = '';
		try {
			const { tag: resolved } = await api.resolveOrCreateTag(name);
			const candidate = (await api.getTag(resolved.id)).tag;
			const hasSubtree = (candidate.ancestors?.length ?? 0) > 0 || (candidate.children?.length ?? 0) > 0;
			if (!hasSubtree) {
				if (await attachChild(candidate.id)) {
					childError = childCycleMessage(candidate.name, tag.name);
					return;
				}
				closeChildAdd();
			} else {
				// Focus the input before opening the dialog so ConfirmDialog captures it
				// as the trigger and returns focus here on cancel/close, not the "Add"
				// button (per the handoff's §3/§5 focus-restoration requirement).
				childInput?.focus();
				confirmCandidate = candidate;
			}
		} catch (err) {
			childError = toMessage(err);
		} finally {
			childBusy = false;
		}
	}

	async function confirmMove() {
		if (!confirmCandidate || !tag) return;
		confirmBusy = true;
		confirmError = '';
		try {
			if (await attachChild(confirmCandidate.id)) {
				confirmError = childCycleMessage(confirmCandidate.name, tag.name);
				return;
			}
			confirmCandidate = null;
			closeChildAdd();
		} catch (err) {
			confirmError = toMessage(err);
		} finally {
			confirmBusy = false;
		}
	}

	function cancelMove() {
		confirmCandidate = null;
		confirmError = '';
	}

	async function removeChild(childId: number) {
		if (!tag || childRemoveBusy) return;
		childRemoveBusy = childId;
		childError = '';
		try {
			await api.setTagParent(childId, null);
			await reloadTag();
		} catch (e) {
			childError = toMessage(e);
		} finally {
			childRemoveBusy = null;
		}
	}

	// ── Categories (HOLODEX-240, ADR-078, HOLODEX-259) ───────────────────────────────
	// Exact port of categories/[id]'s member-tag idiom, direction-inverted: this tag
	// owns many categories rather than a category owning many tags.
	let allCategories = $state<Category[]>([]);
	let catPickerOpen = $state(false);
	let catRemoveBusy = $state<number | null>(null);
	let catError = $state('');

	async function openCatPicker() {
		catError = '';
		if (allCategories.length === 0) allCategories = (await api.listCategories()).items;
		catPickerOpen = true;
	}

	async function removeCategory(categoryId: number) {
		if (!tag || catRemoveBusy) return;
		catRemoveBusy = categoryId;
		catError = '';
		try {
			await api.unassignCategoryTags(categoryId, [tag.id]);
			await reloadTag();
		} catch (e) {
			catError = toMessage(e);
		} finally {
			catRemoveBusy = null;
		}
	}
</script>

{#snippet entityChip(href: string, name: string, onRemove: () => void, busy: boolean, ariaLabel: string)}
	<!-- The curation-chip idiom (categories/[id], media/[id]'s Tags section). -->
	<span
		class="curation-chip group relative inline-flex items-center gap-1 rounded-full border border-rule bg-surface-2 px-2.5 py-1 text-sm text-ink"
	>
		<a href={href} class="hover:text-accent focus-visible:text-accent">{name}</a>
		<span class="curation-actions ml-0.5 inline-flex items-center">
			<button
				type="button"
				onclick={onRemove}
				disabled={busy}
				aria-label={ariaLabel}
				class="rounded p-0.5 -m-0.5 text-muted hover:text-accent focus-visible:text-accent"
			>
				×
			</button>
		</span>
	</span>
{/snippet}

{#snippet plainLink(href: string, name: string)}
	<a href={href} class="rounded-theme bg-surface-2 px-2.5 py-1 text-sm text-ink hover:text-accent">{name}</a>
{/snippet}

<AsyncState {loading} error={error || (!tag ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/tags"
		backLabel="All tags"
		{videos}
		empty="No videos for this tag."
		scrollKey={`tag:${id}`}
	>
		{#snippet hero()}
			<!-- Ancestor breadcrumb (F50 S8, ADR-075 D1 P1-3) — scoped to this page only
			     (skipped on media-page chips and /tags pills, per the design handoff). -->
			{#if tag?.ancestors?.length}
				<p class="text-sm text-muted">{tag.ancestors.join(' › ')} ›</p>
			{/if}
			<NameEditControl
				name={tag?.name ?? ''}
				{isOwner}
				onCommit={commitTagRename}
				label="tag"
				headingClass="skin-title text-2xl font-semibold text-ink"
				hint={tag ? aliasHint(tag.name) : undefined}
			>
				{#snippet verdict(c, resolve)}
					<MergeOfferCard
						noun="tag"
						entityName={tag?.name ?? ''}
						conflict={c}
						busy={renameMergeBusy}
						error={renameMergeError}
						onmerge={() => mergeRenameConflict(c, resolve)}
						onkeepseparate={() => {
							renameMergeError = '';
							resolve();
						}}
					/>
				{/snippet}
			</NameEditControl>
			{#if nearMiss}
				<!-- Non-blocking near-miss (P1-5): the rename already saved; this is an advisory
				     nudge, distinct from the blocking exact-name conflict above (mirrors AliasPanel). -->
				<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
					<p class="text-sm text-ink">
						Saved. Looks a lot like <span class="font-semibold">{nearMiss.name}</span>
						({videoCount(nearMiss.video_count ?? 0)}) — merge them?
					</p>
					<div class="flex flex-wrap items-center gap-2">
						<button onclick={mergeNearMiss} disabled={nearMissBusy} class="btn-accent px-3 py-1.5 text-sm">
							Yes, merge them in
						</button>
						<button
							onclick={keepNearMissSeparate}
							disabled={nearMissBusy}
							class="btn-ghost px-3 py-1.5 text-sm"
						>
							No, keep separate
						</button>
					</div>
					{#if nearMissError}
						<p class="text-sm text-warn">{nearMissError}</p>
					{/if}
				</div>
			{/if}
			<p class="text-sm text-muted">{videoCount(videos.length)}</p>
		{/snippet}

		{#snippet detail()}
			<!-- Hierarchy & categories (F50/HOLODEX-240, HOLODEX-259): parent, direct
			     children, and category memberships. Read states are visible to every
			     visitor (matching the ancestor breadcrumb above); only the add/×
			     affordances are owner-gated. -->
			{#if tag}
				{@const t = tag}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<h2 class="text-xs uppercase tracking-wide text-muted">Hierarchy &amp; categories</h2>

					<div class="space-y-1.5">
						<h3 class="text-xs uppercase tracking-wide text-muted">Parent</h3>
						<div class="flex flex-wrap items-center gap-2">
							{#if t.parent_tag_id}
								{@const parentName = t.ancestors?.[t.ancestors.length - 1] ?? '—'}
								{#if isOwner}
									{@render entityChip(
										`/tags/${t.parent_tag_id}`,
										parentName,
										() => applyParent(null),
										parentBusy,
										`Clear parent ${parentName}`
									)}
								{:else}
									{@render plainLink(`/tags/${t.parent_tag_id}`, parentName)}
								{/if}
							{:else if !isOwner}
								<p class="text-sm text-muted">No parent — this is a root tag.</p>
							{/if}

							{#if isOwner}
								{#if parentOpen}
									<form onsubmit={submitParentAdd} class="inline-flex flex-wrap items-center gap-2">
										<input
											bind:this={parentInput}
											bind:value={parentValue}
											type="text"
											list="parent-options"
											placeholder="Parent tag"
											aria-label="Set parent"
											class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
										/>
										<datalist id="parent-options">
											{#each allTags.filter((x) => x.id !== t.id) as opt (opt.id)}
												<option value={opt.name}></option>
											{/each}
										</datalist>
										<button type="submit" disabled={parentBusy} class="btn-accent px-3 py-1.5 text-sm">
											Set parent
										</button>
										<button
											type="button"
											onclick={closeParentAdd}
											disabled={parentBusy}
											class="btn-quiet px-3 py-1.5 text-sm"
										>
											Cancel
										</button>
									</form>
								{:else}
									<button type="button" onclick={openParentAdd} class="btn-quiet px-3 py-1.5 text-sm">
										+ Set parent
									</button>
								{/if}
							{/if}
						</div>
						{#if parentError}<p class="text-sm text-warn">{parentError}</p>{/if}
					</div>

					<div class="space-y-1.5 border-t border-rule pt-3">
						<h3 class="text-xs uppercase tracking-wide text-muted">
							Children{t.children?.length ? ` · ${t.children.length}` : ''}
						</h3>
						<div class="flex flex-wrap items-center gap-2">
							{#each t.children ?? [] as c (c.id)}
								{#if isOwner}
									{@render entityChip(
										`/tags/${c.id}`,
										c.name,
										() => removeChild(c.id),
										childRemoveBusy === c.id,
										`Remove child ${c.name}`
									)}
								{:else}
									{@render plainLink(`/tags/${c.id}`, c.name)}
								{/if}
							{/each}
							{#if !t.children?.length && !isOwner}
								<p class="text-sm text-muted">No children.</p>
							{/if}

							{#if isOwner}
								{#if childOpen}
									<form onsubmit={submitChild} class="inline-flex flex-wrap items-center gap-2">
										<input
											bind:this={childInput}
											bind:value={childValue}
											type="text"
											placeholder="Add a child"
											aria-label="Add a child"
											class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
										/>
										<button type="submit" disabled={childBusy} class="btn-accent px-3 py-1.5 text-sm">
											Add
										</button>
										<button
											type="button"
											onclick={closeChildAdd}
											disabled={childBusy}
											class="btn-quiet px-3 py-1.5 text-sm"
										>
											Cancel
										</button>
									</form>
								{:else}
									<button type="button" onclick={openChildAdd} class="btn-quiet px-3 py-1.5 text-sm">
										+ Add child
									</button>
								{/if}
							{/if}
						</div>
						{#if childError}<p class="text-sm text-warn">{childError}</p>{/if}
					</div>

					<div class="space-y-1.5 border-t border-rule pt-3">
						<h3 class="text-xs uppercase tracking-wide text-muted">
							Categories{t.categories?.length ? ` · ${t.categories.length}` : ''}
						</h3>
						<div class="flex flex-wrap items-center gap-2">
							{#each t.categories ?? [] as c (c.id)}
								{#if isOwner}
									{@render entityChip(
										`/categories/${c.id}`,
										c.name,
										() => removeCategory(c.id),
										catRemoveBusy === c.id,
										`Remove category ${c.name}`
									)}
								{:else}
									{@render plainLink(`/categories/${c.id}`, c.name)}
								{/if}
							{/each}
							{#if !t.categories?.length && !isOwner}
								<p class="text-sm text-muted">No categories.</p>
							{/if}

							{#if isOwner}
								<button type="button" onclick={openCatPicker} class="btn-quiet px-3 py-1.5 text-sm">
									+ Add category
								</button>
							{/if}
						</div>
						{#if catError}<p class="text-sm text-warn">{catError}</p>{/if}
					</div>
				</section>
			{/if}

			<!-- Details (HOLODEX-239, ADR-077): writeback inclusion toggle + manual sync
			     trigger, the tag-scoped analog of CurationChip's "don't write" glyph. Two
			     dt/dd rows in one card, not a single hard-coded control.
			     Non-owners see nothing (mirrors people/[id]'s activity.effectiveOwner gating). -->
			{#if isOwner && tag}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
					<dl class="space-y-3 text-sm">
						<div class="flex items-center justify-between gap-3">
							<div>
								<dt class="text-ink">File writeback</dt>
								<dd class="text-xs text-muted">
									{#if tag.writeback_enabled}
										Included in this tag's videos' Genre field on write.
									{:else}
										Excluded from Genre writeback — stays searchable in Holodex.
									{/if}
								</dd>
							</div>
							<button
								type="button"
								onclick={toggleWriteback}
								disabled={toggleBusy}
								aria-pressed={!tag.writeback_enabled}
								title={tag.writeback_enabled
									? 'Exclude from file writeback'
									: 'Include in file writeback'}
								aria-label={tag.writeback_enabled
									? `Exclude ${tag.name} from file writeback`
									: `Include ${tag.name} in file writeback`}
								class="shrink-0 rounded-theme border p-1.5 {tag.writeback_enabled
									? 'border-rule text-muted hover:text-ink'
									: 'border-accent text-accent'}"
							>
								<svg
									class="h-4 w-4"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									aria-hidden="true"
								>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="M9 13h6m-6 4h6m4 4H5a2 2 0 01-2-2V5a2 2 0 012-2h8l6 6v10a2 2 0 01-2 2z"
									/>
								</svg>
							</button>
						</div>
						{#if toggleError}
							<p class="text-xs text-warn">{toggleError}</p>
						{/if}

						<div class="flex items-center justify-between gap-3 border-t border-rule pt-3">
							<div>
								<dt class="text-ink">Sync to files</dt>
								<dd class="text-xs text-muted">
									Push this tag's current decision out to already-written files.
								</dd>
							</div>
							<button
								type="button"
								onclick={() => (syncOpen = true)}
								disabled={!tag.video_count}
								title={tag.video_count ? undefined : 'No videos carry this tag'}
								class="btn-accent shrink-0 px-3 py-1.5 text-sm"
							>
								Sync writeback now
							</button>
						</div>
					</dl>
				</section>
			{/if}
		{/snippet}
		{#snippet footer()}
			<FilmsRow films={filmsState.films} />
		{/snippet}
	</EntityVideos>
</AsyncState>

{#if syncOpen && tag}
	{@const t = tag}
	<WritebackBatchDialog
		scopeLabel={t.name}
		videoCountHint={t.video_count ?? 0}
		trigger={() => api.syncTagWriteback(t.id)}
		batchStatus={api.writebackBatchStatus}
		onclose={() => (syncOpen = false)}
	/>
{/if}

{#if catPickerOpen && tag}
	{@const t = tag}
	<CategoryPicker
		tagIds={[t.id]}
		mode="add"
		categories={allCategories}
		onclose={() => (catPickerOpen = false)}
		onapplied={reloadTag}
	/>
{/if}

{#if confirmCandidate && tag}
	{@const candidate = confirmCandidate}
	{@const parentName = candidate.ancestors?.[candidate.ancestors.length - 1]}
	<ConfirmDialog
		title={`Move "${candidate.name}" here?`}
		confirmLabel="Move it here"
		variant="destructive"
		busy={confirmBusy}
		error={confirmError}
		onconfirm={confirmMove}
		oncancel={cancelMove}
	>
		{#snippet body()}
			{#if parentName}
				<p>
					“{candidate.name}” is currently under “{parentName}”. Moving it here removes it from that
					parent — {parentName}'s other children are unaffected.
				</p>
			{/if}
			{#if candidate.children?.length}
				<p>
					“{candidate.name}” has {tagCount(candidate.children.length)} of its own. They'll move here
					along with it — nothing is deleted, but its whole branch relocates.
				</p>
			{/if}
		{/snippet}
	</ConfirmDialog>
{/if}
