<script lang="ts">
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, videoCount } from '$lib/format';
	import { PEOPLE_TAG_SORTS, type EntityRef, type PeopleTagSort, type Tag } from '$lib/types';
	import SortToggle from '$lib/components/sort/SortToggle.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import EntityPicker from '$lib/components/entity/EntityPicker.svelte';
	import MergeCanonicalDialog from '$lib/components/entity/MergeCanonicalDialog.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import { dismissable } from '$lib/actions/dismissable';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';

	let tags = $state<Tag[]>([]);
	let sort = $state<PeopleTagSort>(readSort('tags', PEOPLE_TAG_SORTS, 'name'));
	let loading = $state(true);

	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)

	// Tag identity actions (F43, RD7) — tags have no detail page, so all identity lives in a
	// pill-native "Manage tags" mode: selectable pills + a Merge bar, and a per-pill ⋯ menu
	// (rename / add alias / merge into…). No decision chips. Everything owner-gated.
	let manage = $state(false);

	// Select-to-merge: pick 2+ pills, then choose the surviving name in MergeCanonicalDialog
	// (mirrors /people).
	let selectedIds = $state<number[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open
	let selectHint = $state('');
	const selectedTags = $derived(tags.filter((t) => selectedIds.includes(t.id)));

	// Per-pill ⋯ menu: one open at a time. The popover swaps between the action list and an
	// inline rename/alias editor; "Merge into…" opens the shared EntityPicker instead.
	let openMenu = $state<number | null>(null);
	let menuAction = $state<'menu' | 'rename' | 'alias' | 'parent'>('menu');
	let actionValue = $state('');
	let actionBusy = $state(false);
	let actionError = $state('');
	let actionConflict = $state<EntityRef | null>(null);
	// Non-blocking near-miss (P1-5): a fuzzy look-alike surfaced after a successful
	// add/rename — advisory; "Keep both" records keep-separate so it won't nag again.
	let actionNearMiss = $state<EntityRef | null>(null);
	let menuTriggers = $state<Record<number, HTMLButtonElement | null>>({});
	let firstItem = $state<HTMLButtonElement | null>(null);
	let actionInput = $state<HTMLInputElement | null>(null);
	let mergeInto = $state<Tag | null>(null);

	// Persist the chosen sort per page (SP1).
	$effect(() => {
		writeSort('tags', sort);
	});

	function reload() {
		loading = true;
		api
			.listTags(sort)
			.then((res) => (tags = res.items ?? []))
			.finally(() => (loading = false));
	}

	$effect(() => {
		void sort; // re-run on sort change
		reload();
	});

	// "Random" shuffles the name-ordered list client-side with the session seed, so
	// the order holds across re-renders and reshuffles only on reroll/new session.
	const displayed = $derived(sort === 'random' ? seededShuffle(tags, shuffleSeed.value) : tags);

	function exitManage() {
		manage = false;
		selectedIds = [];
		choosing = false;
		selectHint = '';
		closeMenu(false);
	}

	function toggleSelect(id: number) {
		selectHint = '';
		selectedIds = selectedIds.includes(id)
			? selectedIds.filter((x) => x !== id)
			: [...selectedIds, id];
	}

	// Merge bar: the button stays enabled below 2 selected (CDS: avoid disabled) and
	// answers with a hint instead of merging.
	function openChoose() {
		if (selectedIds.length < 2) {
			selectHint = 'Select two or more tags to merge.';
			return;
		}
		choosing = true;
	}

	// ── Per-pill ⋯ menu ────────────────────────────────────────────────────────────────
	async function openMenuFor(id: number) {
		openMenu = id;
		menuAction = 'menu';
		actionValue = '';
		actionError = '';
		actionConflict = null;
		actionNearMiss = null;
		await Promise.resolve();
		firstItem?.focus();
	}

	function closeMenu(returnFocus = true) {
		const id = openMenu;
		openMenu = null;
		menuAction = 'menu';
		actionConflict = null;
		actionNearMiss = null;
		if (returnFocus && id != null) menuTriggers[id]?.focus();
	}

	function toggleMenu(id: number) {
		if (openMenu === id) closeMenu();
		else openMenuFor(id);
	}

	async function startAction(kind: 'rename' | 'alias' | 'parent', tag: Tag) {
		menuAction = kind;
		actionValue = kind === 'rename' ? tag.name : '';
		actionError = '';
		actionConflict = null;
		actionNearMiss = null;
		await Promise.resolve();
		actionInput?.focus();
		if (kind === 'rename') actionInput?.select();
	}

	async function submitAction(e: SubmitEvent, tagId: number) {
		e.preventDefault();
		const value = actionValue.trim();
		if (!value || actionBusy) return;
		actionBusy = true;
		actionError = '';
		try {
			const res =
				menuAction === 'rename'
					? await api.renameEntity('tag', tagId, value)
					: await api.addEntityAlias('tag', tagId, value);
			if (res.conflict) {
				// The name already belongs to another tag — offer to merge it in (never a
				// silent fold), mirroring the person/studio collision card.
				actionConflict = res.conflict;
				return;
			}
			reload();
			// Surface a fuzzy look-alike as a non-blocking hint; keep the menu open for it,
			// else close.
			const nm = await api.nearMiss('tag', tagId, value).then((r) => r.near_miss);
			if (nm) actionNearMiss = nm;
			else closeMenu();
		} catch (err) {
			actionError = toMessage(err);
		} finally {
			actionBusy = false;
		}
	}

	async function mergeConflict(tagId: number) {
		if (!actionConflict || actionBusy) return;
		actionBusy = true;
		actionError = '';
		try {
			await api.mergeEntities('tag', tagId, actionConflict.id);
			closeMenu();
			reload();
		} catch (err) {
			actionError = toMessage(err);
		} finally {
			actionBusy = false;
		}
	}

	// Near-miss hint actions (P1-5): fold the look-alike into the edited tag, or keep both
	// (records keep-separate so the hint never returns for this pair).
	async function mergeNearMiss(tagId: number) {
		if (!actionNearMiss || actionBusy) return;
		actionBusy = true;
		actionError = '';
		try {
			await api.mergeEntities('tag', tagId, actionNearMiss.id);
			closeMenu();
			reload();
		} catch (err) {
			actionError = toMessage(err);
		} finally {
			actionBusy = false;
		}
	}

	async function keepBoth(tagId: number) {
		if (!actionNearMiss || actionBusy) return;
		actionBusy = true;
		actionError = '';
		try {
			await api.dismissDuplicate('tag', tagId, actionNearMiss.id);
			closeMenu();
		} catch (err) {
			actionError = toMessage(err);
		} finally {
			actionBusy = false;
		}
	}

	// ── Hierarchy: set/clear parent (F50 S8, ADR-075 D1 P1-2) ────────────────────────
	// Typeahead resolves against the already-loaded `tags` list (no new search
	// endpoint, per the design handoff) — an exact case-insensitive name match,
	// excluding the tag itself.
	async function applyParent(tag: Tag, parentId: number | null) {
		if (actionBusy) return;
		actionBusy = true;
		actionError = '';
		try {
			const res = await api.setTagParent(tag.id, parentId);
			if (res.cycle) {
				// Straight passthrough of the ADR-075 D1 server-side cycle guard.
				actionError = `Can't set ${tag.name} as its own ancestor.`;
				return;
			}
			closeMenu();
			reload();
		} catch (err) {
			actionError = toMessage(err);
		} finally {
			actionBusy = false;
		}
	}

	function submitParent(e: SubmitEvent, tag: Tag) {
		e.preventDefault();
		const name = actionValue.trim();
		if (!name || actionBusy) return;
		const match = tags.find((x) => x.id !== tag.id && x.name.toLowerCase() === name.toLowerCase());
		if (!match) {
			actionError = `No tag named "${name}".`;
			return;
		}
		applyParent(tag, match.id);
	}

</script>

<section class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h1 class="skin-title text-2xl font-semibold text-ink">Tags</h1>
		<div class="flex items-center gap-2">
			{#if isOwner}
				<button
					onclick={() => (manage ? exitManage() : (manage = true))}
					class="rounded-theme border px-3 py-1 text-sm {manage
						? 'border-accent text-accent'
						: 'border-rule text-ink hover:bg-surface-2'}"
				>
					{manage ? 'Done' : 'Manage tags'}
				</button>
			{/if}
			{#if sort === 'random'}
				<SortReroll onreroll={() => shuffleSeed.reroll()} />
			{/if}
			<SortToggle bind:sort />
		</div>
	</div>

	<DuplicatesBanner entityType="tag" />

	{#if manage}
		<!-- Merge bar: select 2+ pills, then choose the surviving name. Mirrors /people's
		     multi-select semantics, pill-adapted (F43 §4). -->
		<div
			class="flex flex-wrap items-center gap-3 rounded-theme border border-rule bg-surface px-3 py-2"
		>
			<span class="text-sm text-muted">
				{selectedIds.length} selected — select two or more, then merge; or use a tag’s ⋯ menu to
				rename, alias, or merge into another.
			</span>
			<button
				onclick={openChoose}
				class="rounded-theme border border-accent px-3 py-1 text-sm text-accent hover:bg-surface-2"
			>
				Merge…
			</button>
			{#if selectHint}
				<span class="text-sm text-warn" role="status">{selectHint}</span>
			{/if}
		</div>
	{/if}

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if tags.length === 0}
		<p class="py-16 text-center text-sm text-muted">No tags indexed yet.</p>
	{:else}
		<div
			class="flex flex-wrap gap-2"
			use:dismissable={{ enabled: openMenu !== null, inside: '[data-tag-pill]', onclose: closeMenu }}
		>
			{#each displayed as t (t.id)}
				{#if manage}
					{@const selected = selectedIds.includes(t.id)}
					<div
						data-tag-pill
						class="relative inline-flex items-stretch rounded-full border text-sm {selected
							? 'border-accent bg-surface-2'
							: 'border-rule bg-surface'}"
					>
						<!-- Pill body toggles selection (aria-pressed); the ✓ is decorative. -->
						<button
							type="button"
							aria-pressed={selected}
							onclick={() => toggleSelect(t.id)}
							class="inline-flex items-center gap-1 rounded-full py-1.5 pl-3 pr-2 text-ink"
						>
							{#if selected}<span aria-hidden="true" class="text-accent">✓</span>{/if}
							{t.name}
							<span class="text-xs text-muted">{t.video_count}</span>
						</button>
						<!-- ⋯ opens the per-pill identity menu. -->
						<button
							type="button"
							bind:this={menuTriggers[t.id]}
							onclick={() => toggleMenu(t.id)}
							aria-haspopup="menu"
							aria-expanded={openMenu === t.id}
							aria-label={`Tag actions: ${t.name}`}
							class="inline-flex items-center rounded-r-full border-l border-rule px-2 text-muted hover:text-accent"
						>
							⋯
						</button>

						{#if openMenu === t.id}
							<div
								role="menu"
								class="absolute right-0 top-full z-10 mt-1 min-w-[13rem] rounded-theme border border-rule bg-surface-2 p-1 shadow-sm"
							>
								{#if menuAction === 'menu'}
									<button
										bind:this={firstItem}
										role="menuitem"
										type="button"
										onclick={() => startAction('rename', t)}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Rename
									</button>
									<button
										role="menuitem"
										type="button"
										onclick={() => startAction('alias', t)}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Add alias
									</button>
									<button
										role="menuitem"
										type="button"
										onclick={() => startAction('parent', t)}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										{t.parent_tag_id ? 'Change parent…' : 'Set parent…'}
									</button>
									<button
										role="menuitem"
										type="button"
										onclick={() => {
											mergeInto = t;
											openMenu = null;
										}}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Merge into…
									</button>
								{:else if actionNearMiss}
									<!-- Non-blocking near-miss (P1-5): the edit already saved; this is an
									     advisory nudge. "Keep both" records keep-separate so it won't nag again. -->
									<div class="space-y-2 p-1">
										<p class="text-sm text-ink">
											Saved. Looks a lot like
											<span class="font-semibold">{actionNearMiss.name}</span>
											({videoCount(actionNearMiss.video_count ?? 0)}) — merge them?
										</p>
										<div class="flex flex-wrap gap-2">
											<button
												type="button"
												onclick={() => mergeNearMiss(t.id)}
												disabled={actionBusy}
												class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
											>
												Merge them in
											</button>
											<button
												type="button"
												onclick={() => keepBoth(t.id)}
												disabled={actionBusy}
												class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
											>
												Keep both
											</button>
										</div>
										{#if actionError}
											<p class="text-sm text-warn">{actionError}</p>
										{/if}
									</div>
								{:else if menuAction === 'parent'}
									<!-- Hierarchy: set/clear parent (P1-2). Typeahead is a <datalist> over the
									     already-loaded tag list -- no new search endpoint. -->
									<form onsubmit={(e) => submitParent(e, t)} class="space-y-2 p-1">
										{#if t.parent_tag_id}
											{@const parentName = tags.find((x) => x.id === t.parent_tag_id)?.name}
											<p class="text-sm text-ink">
												Parent: {parentName ?? '—'}
												<button
													type="button"
													onclick={() => applyParent(t, null)}
													disabled={actionBusy}
													class="btn-quiet ml-1"
												>
													Clear
												</button>
											</p>
										{/if}
										<input
											bind:this={actionInput}
											bind:value={actionValue}
											type="text"
											list={`parent-options-${t.id}`}
											placeholder="New parent tag"
											aria-label={`Set parent for ${t.name}`}
											class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
										/>
										<datalist id={`parent-options-${t.id}`}>
											{#each tags.filter((x) => x.id !== t.id) as opt (opt.id)}
												<option value={opt.name}></option>
											{/each}
										</datalist>
										<div class="flex flex-wrap gap-2">
											<button type="submit" disabled={actionBusy} class="btn-accent px-3 py-1.5 text-sm">
												Set parent
											</button>
											<button
												type="button"
												onclick={() => closeMenu()}
												disabled={actionBusy}
												class="btn-ghost px-3 py-1.5 text-sm"
											>
												Cancel
											</button>
										</div>
										{#if actionError}
											<p class="text-sm text-warn">{actionError}</p>
										{/if}
									</form>
								{:else}
									<!-- Inline rename/alias editor. A collision offers a merge instead of a
									     silent fold (never auto-merge homonyms). -->
									<form onsubmit={(e) => submitAction(e, t.id)} class="space-y-2 p-1">
										<input
											bind:this={actionInput}
											bind:value={actionValue}
											type="text"
											placeholder={menuAction === 'rename' ? 'New name' : 'Add an alias'}
											aria-label={menuAction === 'rename' ? `Rename ${t.name}` : `Add an alias for ${t.name}`}
											class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
										/>
										{#if actionConflict}
											<p class="text-sm text-ink">
												<span class="font-semibold">{actionConflict.name}</span>
												({videoCount(actionConflict.video_count ?? 0)}) is already a separate tag. Merge
												them in?
											</p>
											<div class="flex flex-wrap gap-2">
												<button
													type="button"
													onclick={() => mergeConflict(t.id)}
													disabled={actionBusy}
													class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
												>
													Yes, merge them in
												</button>
												<button
													type="button"
													onclick={() => closeMenu()}
													disabled={actionBusy}
													class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
												>
													No, keep separate
												</button>
											</div>
										{:else}
											<div class="flex flex-wrap gap-2">
												<button
													type="submit"
													disabled={actionBusy}
													class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
												>
													{menuAction === 'rename' ? 'Rename' : 'Add'}
												</button>
												<button
													type="button"
													onclick={() => closeMenu()}
													disabled={actionBusy}
													class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
												>
													Cancel
												</button>
											</div>
										{/if}
										{#if actionError}
											<p class="text-sm text-warn">{actionError}</p>
										{/if}
									</form>
								{/if}
							</div>
						{/if}
					</div>
				{:else}
					<a
						href={`/tags/${t.id}`}
						class="rounded-full border border-rule bg-surface px-3 py-1.5 text-sm text-ink hover:border-accent"
					>
						{t.name} <span class="text-xs text-muted">{t.video_count}</span>
					</a>
				{/if}
			{/each}
		</div>
	{/if}
</section>

<!-- "Keep which name?" — choose the surviving tag when merging a multi-select (mirrors
     /people). The others become its aliases and their videos move under it. -->
{#if choosing}
	<MergeCanonicalDialog
		kind="tag"
		items={selectedTags}
		onclose={() => (choosing = false)}
		onmerged={() => {
			selectedIds = [];
			reload();
		}}
	/>
{/if}

<!-- "Merge into…" from a pill's ⋯ menu: fold another tag into this one (this tag survives). -->
{#if mergeInto}
	<EntityPicker
		entityType="tag"
		canonicalId={mergeInto.id}
		canonicalName={mergeInto.name}
		onclose={() => (mergeInto = null)}
		onmerged={() => {
			mergeInto = null;
			reload();
		}}
	/>
{/if}
