<script lang="ts">
	import { api, ApiError } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, videoCount, tagCount, filterByName } from '$lib/format';
	import { PEOPLE_TAG_SORTS, type Category, type EntityRef, type PeopleTagSort, type Tag } from '$lib/types';
	import SortToggle from '$lib/components/sort/SortToggle.svelte';
	import SortReroll from '$lib/components/sort/SortReroll.svelte';
	import EntityPicker from '$lib/components/entity/EntityPicker.svelte';
	import MergeCanonicalDialog from '$lib/components/entity/MergeCanonicalDialog.svelte';
	import CategoryPicker from '$lib/components/entity/CategoryPicker.svelte';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import DuplicatesBanner from '$lib/components/duplicates/DuplicatesBanner.svelte';
	import { dismissable } from '$lib/actions/dismissable';
	import { readSort, writeSort, shuffleSeed } from '$lib/sortPreference.svelte';
	import { seededShuffle } from '$lib/shuffle';

	let tags = $state<Tag[]>([]);
	let categories = $state<Category[]>([]);
	let sort = $state<PeopleTagSort>(readSort('tags', PEOPLE_TAG_SORTS, 'name'));
	let loading = $state(true);

	// Unified type filter + search (HOLODEX-240) — both new to this page. Search
	// filters client-side against the already-loaded, unpaged tag+category lists
	// (personal-library scale, no dedicated search endpoint — same posture
	// EntityPicker/FacetFilter already take).
	let typeFilter = $state<'all' | 'tags' | 'categories'>('all');
	let query = $state('');
	// SortToggle's own cls() helper, duplicated verbatim (it isn't exported) — same
	// segmented-control shell reused for this second, independent toggle.
	const typeCls = (active: boolean) =>
		active ? 'bg-accent px-3 py-1 text-accent-ink' : 'px-3 py-1 text-muted hover:text-ink';

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

	// Category pill's own reduced ⋯ menu (Rename/Delete only, HOLODEX-240 §2) — kept
	// entirely separate from the tag pill menu state above: tag ids and category ids
	// are different id spaces, so sharing `openMenu` would risk one pill's menu
	// opening for the other's id.
	let openCatMenu = $state<number | null>(null);
	let catMenuAction = $state<'menu' | 'rename'>('menu');
	let catActionValue = $state('');
	let catActionBusy = $state(false);
	let catActionError = $state('');
	let catMenuTriggers = $state<Record<number, HTMLButtonElement | null>>({});
	let catActionInput = $state<HTMLInputElement | null>(null);
	let catDeleting = $state<Category | null>(null); // drives the delete ConfirmDialog
	let catDeleteBusy = $state(false);
	let catDeleteError = $state('');

	// Bulk/single "Add to category…" / "Remove from category…" (§4) — one shared
	// picker instance for both the Manage-bar bulk actions and a tag pill's own
	// ⋯ menu item (tagIds is a single-element array in the latter case).
	let catPicker = $state<{ mode: 'add' | 'remove'; tagIds: number[]; categories: Category[] } | null>(null);
	let catPickerHint = $state(''); // "select two or more" / "none belong to a category" hints

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

	function reloadCategories() {
		api.listCategories().then((res) => (categories = res.items ?? []));
	}

	$effect(() => {
		void sort; // re-run on sort change
		reload();
	});

	// Categories don't have a sort toggle of their own — loaded once, always
	// name-ordered (as the server returns them).
	$effect(() => {
		reloadCategories();
	});

	// "Random" shuffles the name-ordered list client-side with the session seed, so
	// the order holds across re-renders and reshuffles only on reroll/new session.
	const shuffled = $derived(sort === 'random' ? seededShuffle(tags, shuffleSeed.value) : tags);
	const displayed = $derived(filterByName(shuffled, query));
	const displayedCategories = $derived(filterByName(categories, query));
	const showTags = $derived(typeFilter !== 'categories');
	const showCategories = $derived(typeFilter !== 'tags');
	const visibleTags = $derived(showTags ? displayed : []);
	const visibleCategories = $derived(showCategories ? displayedCategories : []);
	const resultsCount = $derived(visibleTags.length + visibleCategories.length);
	const emptyMessage = $derived(
		typeFilter === 'tags'
			? 'No tags indexed yet.'
			: typeFilter === 'categories'
				? 'No categories yet.'
				: 'No tags or categories indexed yet.'
	);

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

	// ── Category pill's own ⋯ menu: Rename / Delete only (HOLODEX-240 §2) ────────────
	async function openCatMenuFor(id: number) {
		openCatMenu = id;
		catMenuAction = 'menu';
		catActionValue = '';
		catActionError = '';
	}

	function closeCatMenu(returnFocus = true) {
		const id = openCatMenu;
		openCatMenu = null;
		catMenuAction = 'menu';
		if (returnFocus && id != null) catMenuTriggers[id]?.focus();
	}

	function toggleCatMenu(id: number) {
		if (openCatMenu === id) closeCatMenu();
		else openCatMenuFor(id);
	}

	async function startCatRename(c: Category) {
		catMenuAction = 'rename';
		catActionValue = c.name;
		catActionError = '';
		await Promise.resolve();
		catActionInput?.focus();
		catActionInput?.select();
	}

	async function submitCatRename(e: SubmitEvent, c: Category) {
		e.preventDefault();
		const name = catActionValue.trim();
		if (!name || catActionBusy) return;
		catActionBusy = true;
		catActionError = '';
		try {
			await api.renameCategory(c.id, name);
			closeCatMenu();
			reloadCategories();
		} catch (err) {
			catActionError =
				err instanceof ApiError && err.status === 409
					? `“${name}” already names a tag or another category.`
					: toMessage(err);
		} finally {
			catActionBusy = false;
		}
	}

	async function confirmDeleteCategory() {
		if (!catDeleting || catDeleteBusy) return;
		catDeleteBusy = true;
		catDeleteError = '';
		try {
			await api.deleteCategory(catDeleting.id);
			catDeleting = null;
			reloadCategories();
		} catch (err) {
			catDeleteError = toMessage(err);
		} finally {
			catDeleteBusy = false;
		}
	}

	// ── Bulk/single "Add to category…" / "Remove from category…" (§4) ───────────────
	function openAddToCategory(tagIds: number[]) {
		catPickerHint = '';
		catPicker = { mode: 'add', tagIds, categories };
	}

	function openRemoveFromCategory(tagIds: number[]) {
		const ids = new Set(tagIds);
		const relevant = categories.filter((c) => (c.tag_ids ?? []).some((id) => ids.has(id)));
		if (relevant.length === 0) {
			catPickerHint = "None of the selected tags belong to a category yet.";
			return;
		}
		catPickerHint = '';
		catPicker = { mode: 'remove', tagIds, categories: relevant };
	}

	// Manage-bar bulk actions: 2+ selected, mirroring the Merge button's own
	// "select two or more" hint-on-click pattern rather than a disabled button.
	function bulkAddToCategory() {
		if (selectedIds.length < 2) {
			catPickerHint = 'Select two or more tags to add to a category.';
			return;
		}
		openAddToCategory(selectedIds);
	}

	function bulkRemoveFromCategory() {
		if (selectedIds.length < 2) {
			catPickerHint = 'Select two or more tags to remove from a category.';
			return;
		}
		openRemoveFromCategory(selectedIds);
	}

	function categoryPickerApplied() {
		reloadCategories();
		selectedIds = [];
	}

</script>

<section class="space-y-4">
	<!-- Decorative tag-glyph icon shared by both category pill variants below. -->
	{#snippet categoryIcon()}
		<svg class="h-3.5 w-3.5 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<path stroke-linecap="round" stroke-linejoin="round" d="M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z" />
			<path stroke-linecap="round" stroke-linejoin="round" d="M6 6h.008v.008H6V6z" />
		</svg>
	{/snippet}

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
			<!-- All/Tags/Categories filter (HOLODEX-240) — SortToggle's own shell, reused
			     as-is (no radiogroup semantics; SortToggle itself uses none). -->
			<div class="flex overflow-hidden rounded-theme border border-rule text-sm">
				<button onclick={() => (typeFilter = 'all')} class={typeCls(typeFilter === 'all')}>All</button>
				<button onclick={() => (typeFilter = 'tags')} class={typeCls(typeFilter === 'tags')}>Tags</button>
				<button onclick={() => (typeFilter = 'categories')} class={typeCls(typeFilter === 'categories')}>
					Categories
				</button>
			</div>
		</div>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<input
			type="search"
			bind:value={query}
			placeholder="Search tags and categories…"
			aria-label="Search tags and categories"
			class="w-full max-w-xs rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>
		{#if query.trim()}
			<p class="text-xs text-muted" aria-live="polite">
				{#if resultsCount}
					{resultsCount} result{resultsCount === 1 ? '' : 's'} for “{query.trim()}”
				{:else}
					No tags or categories match “{query.trim()}”.
				{/if}
			</p>
		{/if}
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
			{#if selectedIds.length >= 2}
				<button onclick={bulkAddToCategory} class="btn-ghost px-3 py-1 text-sm">Add to category…</button>
				<button onclick={bulkRemoveFromCategory} class="btn-ghost px-3 py-1 text-sm">
					Remove from category…
				</button>
			{/if}
			{#if selectHint}
				<span class="text-sm text-warn" role="status">{selectHint}</span>
			{/if}
			{#if catPickerHint}
				<span class="text-sm text-warn" role="status">{catPickerHint}</span>
			{/if}
		</div>
	{/if}

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if visibleTags.length === 0 && visibleCategories.length === 0}
		{#if !query.trim()}
			<p class="py-16 text-center text-sm text-muted">{emptyMessage}</p>
		{/if}
	{:else}
		<div
			class="flex flex-wrap gap-2"
			use:dismissable={{ enabled: openMenu !== null, inside: '[data-tag-pill]', onclose: closeMenu }}
			use:dismissable={{ enabled: openCatMenu !== null, inside: '[data-cat-pill]', onclose: closeCatMenu }}
		>
			{#each visibleTags as t (t.id)}
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
									<button
										role="menuitem"
										type="button"
										onclick={() => {
											openAddToCategory([t.id]);
											openMenu = null;
										}}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Add to category…
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

			{#each visibleCategories as c (c.id)}
				{#if manage}
					<!-- Category pill — Manage mode, deliberately asymmetric from tag pills: not
					     selectable (bulk actions only ever target tags), so the body still
					     navigates even while manage is on. Reduced ⋯ menu: Rename/Delete only. -->
					<div data-cat-pill class="relative inline-flex items-stretch rounded-full border border-accent bg-surface text-sm">
						<a
							href={`/categories/${c.id}`}
							class="inline-flex items-center gap-1.5 rounded-full py-1.5 pl-3 pr-2 text-ink hover:text-accent"
						>
							{@render categoryIcon()}
							{c.name} <span class="text-xs text-muted">{tagCount(c.tag_count)}</span>
						</a>
						<button
							type="button"
							bind:this={catMenuTriggers[c.id]}
							onclick={() => toggleCatMenu(c.id)}
							aria-haspopup="menu"
							aria-expanded={openCatMenu === c.id}
							aria-label={`Category actions: ${c.name}`}
							class="inline-flex items-center rounded-r-full border-l border-rule px-2 text-muted hover:text-accent"
						>
							⋯
						</button>

						{#if openCatMenu === c.id}
							<div
								role="menu"
								class="absolute right-0 top-full z-10 mt-1 min-w-[11rem] rounded-theme border border-rule bg-surface-2 p-1 shadow-sm"
							>
								{#if catMenuAction === 'menu'}
									<button
										role="menuitem"
										type="button"
										onclick={() => startCatRename(c)}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Rename
									</button>
									<button
										role="menuitem"
										type="button"
										onclick={() => {
											catDeleting = c;
											catDeleteError = '';
											openCatMenu = null;
										}}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Delete
									</button>
								{:else}
									<form onsubmit={(e) => submitCatRename(e, c)} class="space-y-2 p-1">
										<input
											bind:this={catActionInput}
											bind:value={catActionValue}
											type="text"
											placeholder="New name"
											aria-label={`Rename ${c.name}`}
											class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
										/>
										<div class="flex flex-wrap gap-2">
											<button
												type="submit"
												disabled={catActionBusy}
												class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
											>
												Rename
											</button>
											<button
												type="button"
												onclick={() => closeCatMenu()}
												disabled={catActionBusy}
												class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
											>
												Cancel
											</button>
										</div>
										{#if catActionError}
											<p class="text-sm text-warn">{catActionError}</p>
										{/if}
									</form>
								{/if}
							</div>
						{/if}
					</div>
				{:else}
					<a
						href={`/categories/${c.id}`}
						class="inline-flex items-center gap-1.5 rounded-full border border-accent bg-surface px-3 py-1.5 text-sm text-ink hover:bg-surface-2"
					>
						{@render categoryIcon()}
						{c.name} <span class="text-xs text-muted">{tagCount(c.tag_count)}</span>
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

<!-- Bulk/single "Add to category…" / "Remove from category…" (HOLODEX-240 §4). -->
{#if catPicker}
	<CategoryPicker
		mode={catPicker.mode}
		tagIds={catPicker.tagIds}
		categories={catPicker.categories}
		onclose={() => (catPicker = null)}
		onapplied={categoryPickerApplied}
	/>
{/if}

<!-- Category delete confirm — the only place category delete lives (not duplicated on
     /categories/{id}), avoiding "you just deleted the page you're standing on" handling. -->
{#if catDeleting}
	<ConfirmDialog
		title={`Delete “${catDeleting.name}”?`}
		confirmLabel="Delete"
		busy={catDeleteBusy}
		error={catDeleteError}
		onconfirm={confirmDeleteCategory}
		oncancel={() => (catDeleting = null)}
	>
		{#snippet body()}
			<p>
				{tagCount(catDeleting?.tag_count ?? 0)} will be unassigned from it — the tags themselves
				aren’t affected. This can’t be undone.
			</p>
		{/snippet}
	</ConfirmDialog>
{/if}
