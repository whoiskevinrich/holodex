<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { api, ApiError } from '#lib/api.ts';
	import { listScroll } from '#lib/listScroll.svelte.ts';
	import { activity } from '#lib/activity.svelte.ts';
	import { navSearch } from '#lib/navSearch.svelte.ts';
	import { toMessage, videoCount, tagCount } from '#lib/format.ts';
	import { PagedList, settledQuery } from '#lib/pagedList.svelte.ts';
	import { ServerSearch } from '#lib/serverSearch.svelte.ts';
	import LoadMoreButton from '#lib/components/shared/LoadMoreButton.svelte';
	import { findTagByName, cycleMessage } from '#lib/tagHierarchy.ts';
	import type { Category, EntityRef, Tag } from '#lib/types.ts';
	import ListToolbar from '#lib/components/sort/ListToolbar.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import PageActions from '#lib/components/sort/PageActions.svelte';
	import { segmentedToggleClass } from '#lib/components/sort/segmentedToggle.ts';
	import { TAG_SORTS, tagsSchema, type TagType } from '#lib/listState.ts';
	import { listController } from '#lib/listController.svelte.ts';
	import EntityPicker from '#lib/components/entity/EntityPicker.svelte';
	import MergeCanonicalDialog from '#lib/components/entity/MergeCanonicalDialog.svelte';
	import CategoryPicker from '#lib/components/entity/CategoryPicker.svelte';
	import ConfirmDialog from '#lib/components/shared/ConfirmDialog.svelte';
	import DuplicatesBanner from '#lib/components/duplicates/DuplicatesBanner.svelte';
	import WritebackBatchDialog from '#lib/components/writeback/WritebackBatchDialog.svelte';
	import { dismissable } from '#lib/actions/dismissable.ts';
	import { PopoverMenu } from '#lib/actions/popoverMenu.svelte.ts';
	import { shuffleSeed } from '#lib/sortPreference.svelte.ts';
	import ViewToggle from '#lib/components/sort/ViewToggle.svelte';
	import ListTable from '#lib/components/sort/ListTable.svelte';
	import VirtualRows from '#lib/components/shared/VirtualRows.svelte';
	import ScrollPositionChip from '#lib/components/shared/ScrollPositionChip.svelte';
	import { SparseList } from '#lib/sparseList.svelte.ts';
	import { RovingList, WindowVirtualizer, holdAnchorOnViewChange, wholeListPage } from '#lib/windowVirtualizer.svelte.ts';
	import { tagStream, type TagRow } from '#lib/tagStream.ts';
	import { TAG_COLUMNS } from '#lib/listColumns.ts';
	import { PAGE_VIEWS, readView, writeView } from '#lib/viewPreference.svelte.ts';
	import DensitySlider from '#lib/components/sort/DensitySlider.svelte';
	import { effectiveDensity } from '#lib/density.svelte.ts';

	// Sort and the All/Tags/Categories type are one F73 list state (ADR-114): sort sticky +
	// in the URL, type in the URL only (`?type=`), both held by ListController.
	const list = listController(tagsSchema, '/tags');
	const sortBy = $derived(list.state.sort);

	// Unified type filter (HOLODEX-240) + search (driven by the shared nav box,
	// NS2/NS3/HOLODEX-249; a server search since F80 EL5). The type filter is a URL-only
	// query (ADR-114 D1), rendered as tabs under the title (F73 R4).
	const typeFilter = $derived(list.state.query.type);
	const TYPE_TABS: { value: TagType; label: string }[] = [
		{ value: 'all', label: 'All' },
		{ value: 'tags', label: 'Tags' },
		{ value: 'categories', label: 'Categories' }
	];
	// NS2: `navSearch.inPlace` is only true while this route is mounted AND the box's
	// tab matches this page's own scope (Tags) — otherwise it's previewing another
	// type via the overlay panel.
	const search = settledQuery(() => (navSearch.inPlace ? navSearch.query : ''));
	const query = $derived(search.value);

	// One stream of pills, 200 a page (F80 EL8): tags in the chosen sort, then the
	// categories in name order once the tags run out (categories are never shuffled). The
	// Tags and Categories tabs each page only their own kind. On the All tab the categories
	// are only counted alongside the tags, so the stream's "left" count is known from the
	// start; their first page is read once the tags run out (HOLODEX-589).
	const PILL_PAGE = 200;
	const tags = new PagedList<Tag>(
		(offset, limit) => api.listTags({ sort: sortBy, q: query, seed: shuffleSeed.value, limit, offset }),
		PILL_PAGE
	);
	const categories = new PagedList<Category>(
		(offset, limit) => api.pageCategories({ q: query, limit, offset }),
		PILL_PAGE
	);
	// The create form's Tag/Category switch uses the shared segmented-control classes.
	const typeCls = segmentedToggleClass;

	const isOwner = $derived(activity.effectiveOwner); // owner AND Owner view on (F29)

	// Tag identity actions (F43, RD7) — tags have no detail page, so all identity lives in a
	// pill-native "Manage tags" mode: selectable pills + a Merge bar, and a per-pill ⋯ menu
	// (rename / add alias / merge into…). No decision chips. Everything owner-gated.
	let manage = $state(false);

	// Chips, Posters or Table (F82), remembered per page; only the toggle writes it (F73 R7).
	// Manage tags works in every view: each tag's ⋯ menu rides its pill, card or row (P0-8);
	// selecting several for a bulk action stays in Chips.
	let activeView = $state(readView('tags'));
	const baseColumns = TAG_COLUMNS.map((c) => ({
		...c,
		cell: { name: nameCell, videos: countCell, parent: parentCell }[c.id]!
	}));
	// While managing, a trailing ⋯ column; unclipped so its menu can open past the cell.
	const tableColumns = $derived(
		manage
			? [...baseColumns, { id: 'menu', header: 'Actions', hideHeader: true, align: 'right' as const, width: '40px', noClip: true, cell: menuCell }]
			: baseColumns
	);
	const posterCols = $derived(effectiveDensity());

	// Select-to-merge: pick 2+ pills, then choose the surviving name in MergeCanonicalDialog
	// (mirrors /people).
	// The picked tags are kept whole, so a selection survives Load more (EL9).
	let selectedTags = $state<Tag[]>([]);
	let choosing = $state(false); // the "Keep which name?" dialog is open
	let selectHint = $state('');
	const selectedIds = $derived(selectedTags.map((t) => t.id));

	// Bulk writeback actions (HOLODEX-239, ADR-077 D2): on/off stay two separate
	// buttons since a selection can span tags already in different states (spec
	// P0), and both post immediately like the single-tag toggle — no enqueue.
	let bulkBusy = $state(false);
	let bulkError = $state('');
	let bulkSyncOpen = $state(false);
	const bulkScopeLabel = $derived(
		selectedTags.length > 4
			? `${selectedTags.length} tags`
			: selectedTags.map((t) => t.name).join(', ')
	);

	async function bulkSetWriteback(enabled: boolean) {
		if (bulkBusy || selectedIds.length === 0) return;
		bulkBusy = true;
		bulkError = '';
		try {
			// No reload(): the /tags list read does populate writeback_enabled, but
			// the pill list has nowhere to render it, so a full tag refetch here
			// would buy nothing visible.
			await api.setTagsWriteback(selectedIds, enabled);
		} catch (e) {
			bulkError = toMessage(e);
		} finally {
			bulkBusy = false;
		}
	}

	// Per-pill ⋯ menu: one open at a time. The popover swaps between the action list and an
	// inline rename/alias editor; "Merge into…" opens the shared EntityPicker instead.
	// actionConflict/actionNearMiss are extras layered on top of the shared open/close +
	// value/busy/error plumbing (PopoverMenu) — reset alongside it via onOpen/onClose.
	let actionConflict = $state<EntityRef | null>(null);
	// Non-blocking near-miss (P1-5): a fuzzy look-alike surfaced after a successful
	// add/rename — advisory; "Keep both" records keep-separate so it won't nag again.
	let actionNearMiss = $state<EntityRef | null>(null);
	const resetTagMenuExtras = () => {
		actionConflict = null;
		actionNearMiss = null;
	};
	const tagMenu = new PopoverMenu<'menu' | 'rename' | 'alias' | 'parent'>({
		onOpen: resetTagMenuExtras,
		onClose: resetTagMenuExtras
	});
	let firstItem = $state<HTMLButtonElement | null>(null);
	let actionInput = $state<HTMLInputElement | null>(null);
	let mergeInto = $state<Tag | null>(null);

	// Category pill's own reduced ⋯ menu (Rename/Delete only, HOLODEX-240 §2) — kept
	// entirely separate from the tag pill menu above: tag ids and category ids are
	// different id spaces, so sharing one PopoverMenu would risk one pill's menu
	// opening for the other's id.
	const catMenu = new PopoverMenu<'menu' | 'rename'>();
	let catActionInput = $state<HTMLInputElement | null>(null);
	let catDeleting = $state<Category | null>(null); // drives the delete ConfirmDialog
	let catDeleteBusy = $state(false);
	let catDeleteError = $state('');

	// Bulk/single "Add to category…" / "Remove from category…" (§4) — one shared
	// picker instance for both the Manage-bar bulk actions and a tag pill's own
	// ⋯ menu item (tagIds is a single-element array in the latter case).
	let catPicker = $state<{ mode: 'add' | 'remove'; tagIds: number[]; categories?: Category[] } | null>(null);
	let catPickerHint = $state(''); // "select two or more" / "none belong to a category" hints

	// ── Standing "+ New" create pill (HOLODEX-243) ───────────────────────────────────
	// A singleton control, not PopoverMenu (that class models "one-of-many open, keyed
	// by an id"; this pill has no id to key on).
	let createOpen = $state(false);
	let createType = $state<'tag' | 'category'>('tag');
	let createValue = $state('');
	let createBusy = $state(false);
	let createError = $state('');
	let createInput = $state<HTMLInputElement | null>(null);
	let createTrigger = $state<HTMLButtonElement | null>(null);
	// Tag-only: reuses the shared nearMissCard snippet (below), wired to the *new*
	// tag's id rather than an edited existing one.
	let createResult = $state<{ tagId: number; nearMiss: EntityRef } | null>(null);

	// Scroll restoration (HOLODEX-248, ADR-032): keyed on the canonical list view (sort +
	// type, ADR-114 D5) plus the search query, so a mismatch safely skips the restore. On
	// the first load only, Back re-reads the pills that were loaded, then scrolls (EL7);
	// later reloads (rename, merge, category edits) keep the loaded run.
	const scrollKey = $derived(`${list.key}:${query}`);
	let firstLoad = true;

	// Posters and Table scroll the whole list (F80 EL14, HOLODEX-604): one stream of rows,
	// tags then categories on the All tab, read a page at a time where it's viewed. The
	// Chips view keeps the pill pages above and Load more (EL1).
	const stream = tagStream({
		tags: (offset, limit) => api.listTags({ sort: sortBy, q: query, seed: shuffleSeed.value, limit, offset }),
		categories: (offset, limit) => api.pageCategories({ q: query, limit, offset }),
		show: () => ({ tags: showTags, categories: showCategories })
	});
	const wholeList = $derived(activeView !== 'chips');
	// The Chips view shows none of it, so there it reads nothing.
	const rows = new SparseList<TagRow>((offset, limit, fresh) =>
		wholeList ? stream.fetch(offset, limit, fresh) : Promise.resolve({ items: [], total: 0 })
	);
	let rowsEl = $state<HTMLElement>();
	const virt = new WindowVirtualizer(140);
	const roving = new RovingList(virt, () => rows.total);
	wholeListPage({
		id: 'tag-rows',
		list: rows,
		virt,
		roving,
		key: () => `${scrollKey}:${wholeList}`,
		restartOn: () => {
			void sortBy;
			void query;
			void typeFilter;
			void wholeList;
			if (sortBy === 'random') void shuffleSeed.value;
			stream.reset();
		},
		root: () => rowsEl
	});
	// Posters ↔ Table is one list, so the switch keeps the first entry in view.
	holdAnchorOnViewChange(() => activeView, virt, roving);
	function refreshRows() {
		stream.reset();
		return rows.refresh();
	}

	// A merge or delete can use up the tags, so counted categories may need their page.
	function reload() {
		return Promise.all([tags.refresh().then(fillCategories), refreshRows()]);
	}
	function reloadCategories() {
		return Promise.all([categories.refresh(), refreshRows()]);
	}

	$effect(() => {
		void sortBy;
		void query;
		void typeFilter;
		if (sortBy === 'random') void shuffleSeed.value;
		untrack(async () => {
			const snap = firstLoad ? listScroll.take('tags', scrollKey) : null;
			firstLoad = false;
			// A Back restore that had categories on screen reads them in full straight away.
			const countOnly = showTags && !snap?.loadedCategories;
			await Promise.all([
				showTags ? tags.load({ count: snap?.loaded }) : undefined,
				showCategories
					? countOnly
						? categories.count()
						: categories.load({ count: snap?.loadedCategories })
					: undefined
			]);
			await fillCategories();
			if (snap) {
				await tick();
				window.scrollTo(0, snap.scrollY);
			}
		});
	});

	// Stash the run and scroll offset on the way out (e.g. opening a tag or category).
	beforeNavigate(() => {
		listScroll.save('tags', {
			key: scrollKey,
			scrollY: window.scrollY,
			loaded: tags.items.length,
			loadedCategories: visibleCategories.length
		});
	});

	const showTags = $derived(typeFilter !== 'categories');
	const showCategories = $derived(typeFilter !== 'tags');
	const loading = $derived(
		(showTags && tags.loading && tags.items.length === 0) ||
			(showCategories && categories.loading && categories.items.length === 0)
	);
	const visibleTags = $derived(showTags ? tags.items : []);
	// On the All tab the categories wait until every tag is shown (one stream, EL8).
	const tagsDone = $derived(!showTags || !tags.hasMore);
	const visibleCategories = $derived(showCategories && tagsDone ? categories.items : []);
	const resultsCount = $derived((showTags ? tags.total : 0) + (showCategories ? categories.total : 0));
	const pillsLeft = $derived(
		(showTags ? tags.left : 0) +
			(showCategories ? categories.total - visibleCategories.length : 0)
	);
	const pillError = $derived(tags.error || categories.error);
	let pillsEl = $state<HTMLElement>();

	// Once the tags run out, a categories list that was only counted reads its first page.
	async function fillCategories() {
		if (tagsDone && showCategories && categories.items.length === 0 && categories.hasMore)
			await categories.loadMore();
	}

	// Load more serves the stream: tags until they run out, then categories. The click that
	// loads the last tags also reads the first page of categories, so they appear together.
	let pillAnnounce = $state('');
	async function loadMorePills() {
		const before = visibleTags.length + visibleCategories.length;
		const from = showTags && tags.hasMore ? tags : categories;
		await from.loadMore();
		await fillCategories();
		const added = visibleTags.length + visibleCategories.length - before;
		pillAnnounce = added > 0 ? `Loaded ${added} more. ${pillsLeft.toLocaleString()} left.` : '';
		if (pillsLeft === 0) {
			await tick();
			// The first new pill's focus target: its link, or in Manage mode a tag's select
			// button (HOLODEX-589). Only the Chips view has Load more.
			pillsEl?.querySelectorAll<HTMLElement>('[data-pill]')?.[before]?.focus();
		}
	}
	const emptyMessage = $derived(
		typeFilter === 'tags'
			? 'No tags indexed yet.'
			: typeFilter === 'categories'
				? 'No categories yet.'
				: 'No tags or categories indexed yet.'
	);

	const manageActions = $derived([
		{ label: 'Add to category…', onselect: bulkAddToCategory },
		{ label: 'Remove from category…', onselect: bulkRemoveFromCategory },
		...(bulkBusy
			? []
			: [
					{ label: 'Turn off writeback', onselect: () => bulkSetWriteback(false) },
					{ label: 'Turn on writeback', onselect: () => bulkSetWriteback(true) },
					{ label: 'Sync writeback now…', onselect: () => (bulkSyncOpen = true) }
				])
	]);

	function exitManage() {
		manage = false;
		selectedTags = [];
		choosing = false;
		selectHint = '';
		tagMenu.close(false);
	}

	function toggleSelect(t: Tag) {
		selectHint = '';
		selectedTags = selectedIds.includes(t.id)
			? selectedTags.filter((x) => x.id !== t.id)
			: [...selectedTags, t];
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
	async function startAction(kind: 'rename' | 'alias' | 'parent', tag: Tag) {
		tagMenu.action = kind;
		tagMenu.value = kind === 'rename' ? tag.name : '';
		tagMenu.error = '';
		actionConflict = null;
		actionNearMiss = null;
		if (kind === 'parent') {
			// The parent may not be among the loaded pills; read its name.
			parentSearch.set('');
			parentName = tag.parent_tag_id ? '…' : '';
			if (tag.parent_tag_id) {
				api
					.getTag(tag.parent_tag_id)
					.then((r) => (parentName = r.tag.name))
					.catch(() => (parentName = '—'));
			}
		}
		await Promise.resolve();
		actionInput?.focus();
		if (kind === 'rename') actionInput?.select();
	}

	function submitAction(e: SubmitEvent, tagId: number) {
		e.preventDefault();
		tagMenu.submit(async (value) => {
			const res =
				tagMenu.action === 'rename'
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
			else tagMenu.close();
		});
	}

	function mergeConflict(tagId: number) {
		if (!actionConflict) return;
		const targetId = actionConflict.id;
		tagMenu.run(async () => {
			await api.mergeEntities('tag', tagId, targetId);
			tagMenu.close();
			reload();
		});
	}

	// Near-miss hint actions (P1-5): fold the look-alike into the edited tag, or keep both
	// (records keep-separate so the hint never returns for this pair).
	function mergeNearMiss(tagId: number) {
		if (!actionNearMiss) return;
		const targetId = actionNearMiss.id;
		tagMenu.run(async () => {
			await api.mergeEntities('tag', tagId, targetId);
			tagMenu.close();
			reload();
		});
	}

	function keepBoth(tagId: number) {
		if (!actionNearMiss) return;
		const targetId = actionNearMiss.id;
		tagMenu.run(async () => {
			await api.dismissDuplicate('tag', tagId, targetId);
			tagMenu.close();
		});
	}

	// ── Hierarchy: set/clear parent (F50 S8, ADR-075 D1 P1-2) ────────────────────────
	// Typeahead resolves against the already-loaded `tags` list (no new search
	// endpoint, per the design handoff) — an exact case-insensitive name match,
	// excluding the tag itself.
	function applyParent(tag: Tag, parentId: number | null) {
		tagMenu.run(async () => {
			const res = await api.setTagParent(tag.id, parentId);
			if (res.cycle) {
				// Straight passthrough of the ADR-075 D1 server-side cycle guard.
				tagMenu.error = cycleMessage(tag.name);
				return;
			}
			tagMenu.close();
			reload();
		});
	}

	// The parent typeahead and its exact-name check ask the server, so any tag in the
	// library can be the parent, not only the loaded pills (EL10; the tag page's pattern).
	const parentSearch = new ServerSearch<Tag>(async (q) => (await api.search(q)).tags ?? []);
	let parentName = $state('');

	async function submitParent(e: SubmitEvent, tag: Tag) {
		e.preventDefault();
		const name = tagMenu.value.trim();
		if (!name || tagMenu.busy) return;
		let match: Tag | undefined;
		try {
			match = findTagByName((await api.search(name)).tags ?? [], name, tag.id);
		} catch (err) {
			tagMenu.error = toMessage(err);
			return;
		}
		if (!match) {
			tagMenu.error = `No tag named "${name}".`;
			return;
		}
		applyParent(tag, match.id);
	}

	// ── Category pill's own ⋯ menu: Rename / Delete only (HOLODEX-240 §2) ────────────
	async function startCatRename(c: Category) {
		catMenu.action = 'rename';
		catMenu.value = c.name;
		catMenu.error = '';
		await Promise.resolve();
		catActionInput?.focus();
		catActionInput?.select();
	}

	function submitCatRename(e: SubmitEvent, c: Category) {
		e.preventDefault();
		catMenu.submit(
			async (name) => {
				await api.renameCategory(c.id, name);
				catMenu.close();
				reloadCategories();
			},
			(err, name) => (err instanceof ApiError && err.status === 409 ? categoryCollisionMessage(name) : toMessage(err))
		);
	}

	// Shared with submitCreate's category branch below — both hard-409 on an exact
	// collision (HOLODEX-240 §2 / ADR-078 D3), no near-miss step for categories.
	const categoryCollisionMessage = (name: string) => `“${name}” already names a tag or another category.`;

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
		// Add mode searches the server as the owner types (HOLODEX-586), so no list here.
		catPicker = { mode: 'add', tagIds };
	}

	// Remove mode offers only the categories the selected tags belong to. The loaded
	// pills may not include them, so read every category's membership on demand.
	async function openRemoveFromCategory(tagIds: number[]) {
		const ids = new Set(tagIds);
		let all: Category[];
		try {
			all = (await api.listCategories()).items ?? [];
		} catch (err) {
			catPickerHint = toMessage(err);
			return;
		}
		const relevant = all.filter((c) => (c.tag_ids ?? []).some((id) => ids.has(id)));
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
		selectedTags = [];
	}

	// ── Standing "+ New" create pill (HOLODEX-243) ───────────────────────────────────
	async function openCreate() {
		createOpen = true;
		createType = 'tag';
		createValue = '';
		createError = '';
		createResult = null;
		await Promise.resolve();
		createInput?.focus();
	}

	// The popover only renders while createOpen, so nothing reads the other create*
	// fields once closed — openCreate() resets them unconditionally on the next open.
	function closeCreate() {
		createOpen = false;
		createBusy = false;
		createTrigger?.focus();
	}

	// Busy/error-guarded async action, mirroring PopoverMenu.run (the create pill is a
	// singleton control, not keyed by an id, so it doesn't share tagMenu's instance).
	async function guardCreate(action: () => Promise<void>, mapError?: (err: unknown) => string) {
		if (createBusy) return;
		createBusy = true;
		createError = '';
		try {
			await action();
		} catch (err) {
			createError = mapError ? mapError(err) : toMessage(err);
		} finally {
			createBusy = false;
		}
	}

	// Submit branches by type — the two backend calls have genuinely different
	// collision semantics (design handoff §3): tag creation resolves-or-creates
	// silently (no collision, followed by a non-blocking near-miss check); category
	// creation hard-409s on an exact collision (no near-miss, error-only).
	function submitCreate(e: SubmitEvent) {
		e.preventDefault();
		const name = createValue.trim();
		if (!name) return;
		guardCreate(
			async () => {
				if (createType === 'tag') {
					const { tag } = await api.resolveOrCreateTag(name);
					reload();
					const nm = await api.nearMiss('tag', tag.id, name).then((r) => r.near_miss);
					if (nm) createResult = { tagId: tag.id, nearMiss: nm };
					else closeCreate();
				} else {
					await api.createCategory(name);
					reloadCategories();
					closeCreate();
				}
			},
			(err) =>
				createType === 'category' && err instanceof ApiError && err.status === 409
					? categoryCollisionMessage(name)
					: toMessage(err)
		);
	}

	// Near-miss hint actions (mirrors mergeNearMiss/keepBoth above, pointed at the
	// just-created tag instead of an edited existing one).
	function mergeCreateNearMiss() {
		if (!createResult) return;
		const { tagId, nearMiss } = createResult;
		guardCreate(async () => {
			await api.mergeEntities('tag', tagId, nearMiss.id);
			closeCreate();
			reload();
		});
	}

	function keepCreateBoth() {
		if (!createResult) return;
		const { tagId, nearMiss } = createResult;
		guardCreate(async () => {
			await api.dismissDuplicate('tag', tagId, nearMiss.id);
			closeCreate();
		});
	}

</script>

<section class="space-y-4">

	<h1 class="skin-title text-2xl font-semibold text-ink">Tags</h1>

	<!-- All/Tags/Categories (HOLODEX-240) picks which list you're looking at, so it's
	     page-level tabs under the title, not a toolbar control (F73 R4). -->
	<div role="tablist" aria-label="Show" class="flex gap-6 border-b border-rule text-sm">
		{#each TYPE_TABS as t (t.value)}
			<button
				role="tab"
				aria-selected={typeFilter === t.value}
				aria-controls="tags-panel"
				tabindex={typeFilter === t.value ? 0 : -1}
				onclick={() => list.setQuery({ type: t.value })}
				onkeydown={(e) => {
					const i = TYPE_TABS.findIndex((x) => x.value === typeFilter);
					const to = e.key === 'ArrowRight' ? i + 1 : e.key === 'ArrowLeft' ? i - 1 : -9;
					if (to === -9) return;
					e.preventDefault();
					const next = TYPE_TABS[(to + TYPE_TABS.length) % TYPE_TABS.length];
					// currentTarget is null once dispatch ends, so take the tablist now.
					const tablist = (e.currentTarget as HTMLElement).parentElement;
					list.setQuery({ type: next.value });
					tick().then(() => tablist?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus());
				}}
				class="-mb-px border-b-2 pb-2 {typeFilter === t.value
					? 'border-accent text-ink'
					: 'border-transparent text-muted hover:text-ink'}"
			>
				{t.label}
			</button>
		{/each}
	</div>

	<ListToolbar
		reroll={sortBy === 'random' ? () => shuffleSeed.reroll() : undefined}
		actions={isOwner && !manage ? [{ label: 'Manage tags', onselect: () => (manage = true) }] : []}
		mode={manage ? manageBar : undefined}
		count={manage ? manageHint : undefined}
	>
		{#snippet sort()}
			<SortDropdown compact options={TAG_SORTS} sort={list.state.sort} onchange={(v) => list.setSort(v)} />
		{/snippet}
		{#snippet view()}
			<ViewToggle options={PAGE_VIEWS.tags} bind:view={activeView} onchange={(v) => writeView('tags', v)} />
			{#if activeView === 'posters'}
				<DensitySlider />
			{/if}
		{/snippet}
	</ListToolbar>

	{#if query.trim()}
		<p class="text-xs text-muted" aria-live="polite">
			{#if resultsCount}
				{resultsCount} result{resultsCount === 1 ? '' : 's'} for “{query.trim()}”
			{:else}
				No tags or categories match “{query.trim()}”.
			{/if}
		</p>
	{/if}

	<DuplicatesBanner entityType="tag" />

	<!-- Manage mode (F73 D3, the owner's pick): the toolbar row becomes the mode bar —
	     the same pattern as /people's select mode — and the hint plus any warnings take the
	     count line's place. Bulk category/writeback actions (HOLODEX-239: on and off both
	     offered, never one combined toggle) live in the bar's ⋯ once two or more are picked. -->
	{#snippet manageBar()}
		<span class="min-w-0 flex-1 truncate text-sm text-ink" aria-live="polite">Managing · {selectedIds.length} selected</span>
		<button
			onclick={openChoose}
			class="h-8 shrink-0 rounded-theme border border-accent px-3 text-sm text-accent hover:bg-surface-2"
		>
			Merge…
		</button>
		{#if selectedIds.length >= 2}
			<PageActions items={manageActions} label="More actions for the selected tags" />
		{/if}
		<button onclick={exitManage} class="btn-ghost h-8 shrink-0 px-3 text-sm">Done</button>
	{/snippet}
	{#snippet manageHint()}
		{#if activeView === 'chips'}
			Select two or more, then merge; or use a tag’s ⋯ menu to rename, alias, or merge into another.
		{:else}
			Use a tag’s ⋯ menu to rename, alias, or merge into another. Switch to Chips to select several.
		{/if}
		{#if selectHint}<span class="text-warn">{selectHint}</span>{/if}
		{#if catPickerHint}<span class="text-warn">{catPickerHint}</span>{/if}
		{#if bulkError}<span class="text-warn">{bulkError}</span>{/if}
	{/snippet}

	<div id="tags-panel" role="tabpanel">
	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else}
		<!-- A failed load or Load more keeps whatever is loaded and says so above it. -->
		{#if pillError}<p class="pb-2 text-sm text-warn">Couldn’t load tags: {pillError}</p>{/if}
		<div
			bind:this={pillsEl}
			class="flex flex-wrap gap-2"
			use:dismissable={{ enabled: tagMenu.openId !== null, inside: '[data-tag-pill]', onclose: tagMenu.close }}
			use:dismissable={{ enabled: catMenu.openId !== null, inside: '[data-cat-pill]', onclose: catMenu.close }}
			use:dismissable={{ enabled: createOpen, inside: '[data-create-pill]', onclose: closeCreate }}
		>
			{#if isOwner}
				<!-- Standing create pill (HOLODEX-243) — always first, regardless of
				     typeFilter/search/manage, so it never disappears exactly when it's
				     needed most (a fresh empty grid). -->
				<div data-create-pill class="relative inline-flex">
					<button
						type="button"
						bind:this={createTrigger}
						onclick={openCreate}
						aria-expanded={createOpen}
						class="rounded-full border border-dashed border-rule px-3 py-1.5 text-sm text-muted hover:border-accent hover:text-accent"
					>
						+ New
					</button>
					{#if createOpen}
						<div
							class="absolute left-0 top-full z-10 mt-1 w-72 rounded-theme border border-rule bg-surface-2 p-2 shadow-sm"
							aria-label="Create a tag or category"
						>
							{#if createResult}
								{@render nearMissCard(createResult.nearMiss, createBusy, createError, mergeCreateNearMiss, keepCreateBoth)}
							{:else}
								<div class="mb-2 flex overflow-hidden rounded-theme border border-rule text-sm">
									<button type="button" onclick={() => (createType = 'tag')} class={typeCls(createType === 'tag')}>
										Tag
									</button>
									<button
										type="button"
										onclick={() => (createType = 'category')}
										class={typeCls(createType === 'category')}
									>
										Category
									</button>
								</div>
								<form onsubmit={submitCreate} class="space-y-2">
									<input
										bind:this={createInput}
										bind:value={createValue}
										type="text"
										placeholder={createType === 'tag' ? 'Tag name' : 'Category name'}
										aria-label={createType === 'tag' ? 'New tag name' : 'New category name'}
										class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
									/>
									<div class="flex flex-wrap gap-2">
										<button
											type="submit"
											disabled={createBusy}
											class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
										>
											Create
										</button>
										<button
											type="button"
											onclick={closeCreate}
											disabled={createBusy}
											class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
										>
											Cancel
										</button>
									</div>
									{#if createError}
										<p class="text-sm text-warn">{createError}</p>
									{/if}
								</form>
							{/if}
						</div>
					{/if}
				</div>
			{/if}
			{#if activeView === 'chips'}
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
							data-pill
							type="button"
							aria-pressed={selected}
							onclick={() => toggleSelect(t)}
							class="inline-flex items-center gap-1 rounded-full py-1.5 pl-3 pr-2 text-ink"
						>
							{#if selected}<span aria-hidden="true" class="text-accent">✓</span>{/if}
							{t.name}
							<span class="text-xs text-muted">{t.video_count}</span>
						</button>
						<!-- ⋯ opens the per-tag identity menu (shared with Posters cards and Table rows). -->
						{@render tagMenuButton(t, 'inline-flex items-center rounded-r-full border-l border-rule px-2 text-muted hover:text-accent')}

						{@render tagMenuPanel(t)}
					</div>
				{:else}
					<a
						data-pill
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
					<div data-cat-pill class="relative inline-flex max-w-full items-stretch rounded-full border border-accent bg-surface text-sm">
						<a
							data-pill
							href={`/categories/${c.id}`}
							class="inline-flex min-w-0 items-center gap-1.5 rounded-full py-1.5 pl-3 pr-2 text-ink wrap-anywhere hover:text-accent"
						>
							{@render categoryIcon()}
							{c.name} <span class="text-xs text-muted">{tagCount(c.tag_count)}</span>
						</a>
						<button
							type="button"
							bind:this={catMenu.triggers[c.id]}
							onclick={() => catMenu.toggle(c.id)}
							aria-haspopup="menu"
							aria-expanded={catMenu.isOpen(c.id)}
							aria-label={`Category actions: ${c.name}`}
							class="inline-flex items-center rounded-r-full border-l border-rule px-2 text-muted hover:text-accent"
						>
							⋯
						</button>

						{#if catMenu.isOpen(c.id)}
							<div
								role="menu"
								class="absolute right-0 top-full z-10 mt-1 min-w-[11rem] rounded-theme border border-rule bg-surface-2 p-1 shadow-sm"
							>
								{#if catMenu.action === 'menu'}
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
											catMenu.close(false);
										}}
										class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
									>
										Delete
									</button>
								{:else}
									<form onsubmit={(e) => submitCatRename(e, c)} class="space-y-2 p-1">
										<input
											bind:this={catActionInput}
											bind:value={catMenu.value}
											type="text"
											placeholder="New name"
											aria-label={`Rename ${c.name}`}
											class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
										/>
										<div class="flex flex-wrap gap-2">
											<button
												type="submit"
												disabled={catMenu.busy}
												class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
											>
												Rename
											</button>
											<button
												type="button"
												onclick={() => catMenu.close()}
												disabled={catMenu.busy}
												class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
											>
												Cancel
											</button>
										</div>
										{#if catMenu.error}
											<p class="text-sm text-warn">{catMenu.error}</p>
										{/if}
									</form>
								{/if}
							</div>
						{/if}
					</div>
				{:else}
					<a
						data-pill
						href={`/categories/${c.id}`}
						class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-accent bg-surface px-3 py-1.5 text-sm text-ink wrap-anywhere hover:bg-surface-2"
					>
						{@render categoryIcon()}
						{c.name} <span class="text-xs text-muted">{tagCount(c.tag_count)}</span>
					</a>
				{/if}
			{/each}
			{/if}
		</div>
		{#if wholeList && rows.error}
			<p class="py-2 text-sm text-warn">Couldn’t load tags: {rows.error}</p>
		{:else if wholeList && rows.total > 0}
			<ScrollPositionChip {virt} total={rows.total} />
			<div class={isOwner ? 'pt-3' : ''}>
				{#if activeView === 'posters'}
					<!-- Posters (F82 P0-9): the media grid's column rule and the shared density value. -->
					<VirtualRows
						list={rows}
						{virt}
						{roving}
						lanes={posterCols}
						label="Tags"
						noun="tags"
						rowClass="grid gap-4 pb-4"
						bind:root={rowsEl}
					>
						{#snippet entry(r)}{@render posterCard(r)}{/snippet}
						{#snippet placeholder()}
							<!-- The card's own frame, still (handoff, "Placeholders, continued"). -->
							<div>
								<span class="block aspect-video rounded-theme border border-rule/50 bg-surface-2"></span>
								<span class="flex h-[26px] items-end pb-1"><span class="h-2.5 w-3/5 rounded-theme bg-rule"></span></span>
								<span class="flex h-4 items-center"><span class="h-2 w-2/5 rounded-theme bg-rule"></span></span>
							</div>
						{/snippet}
					</VirtualRows>
				{:else}
					<ListTable
						columns={tableColumns}
						key={(r) => (r.kind === 'tag' ? `t${r.tag.id}` : `c${r.cat.id}`)}
						href={(r) => (r.kind === 'tag' ? `/tags/${r.tag.id}` : `/categories/${r.cat.id}`)}
						sort={sortBy}
						sorts={TAG_SORTS}
						owner={isOwner}
						onsort={(s) => list.setSort(s)}
						window={{ list: rows, virt, roving, label: 'Tags', noun: 'tags' }}
						bind:root={rowsEl}
					/>
				{/if}
			</div>
		{/if}
		{#if !wholeList}
			<LoadMoreButton
				left={pillsLeft}
				loading={tags.loadingMore || categories.loadingMore}
				announce={pillAnnounce}
				onclick={loadMorePills}
			/>
		{/if}
		{#if visibleTags.length === 0 && visibleCategories.length === 0 && !query && !pillError}
			<p class="py-2 text-sm text-muted">
				{isOwner ? 'Nothing here yet — create your first one above.' : emptyMessage}
			</p>
		{/if}
	{/if}
	</div>
</section>


<!-- The per-tag identity menu (F43 RD7): one ⋯ trigger and one panel, rendered by the Chips
     pill, the Posters card and the Table row while Manage tags is on (F82 P0-8), so the three
     can't drift. The wrapper that holds them carries data-tag-pill for the dismissable. -->
{#snippet tagMenuButton(t: Tag, cls: string)}
	<button
		type="button"
		bind:this={tagMenu.triggers[t.id]}
		onclick={() => tagMenu.toggle(t.id, () => firstItem?.focus())}
		aria-haspopup="menu"
		aria-expanded={tagMenu.isOpen(t.id)}
		aria-label={`Tag actions: ${t.name}`}
		class={cls}
	>
		⋯
	</button>
{/snippet}
{#snippet tagMenuPanel(t: Tag)}
	{#if tagMenu.isOpen(t.id)}
		<div
			role="menu"
			class="absolute right-0 top-full z-10 mt-1 min-w-[13rem] rounded-theme border border-rule bg-surface-2 p-1 shadow-sm"
		>
			{#if tagMenu.action === 'menu'}
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
						tagMenu.close(false);
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
						tagMenu.close(false);
					}}
					class="block w-full rounded-theme px-3 py-1.5 text-left text-sm text-ink hover:bg-surface"
				>
					Add to category…
				</button>
			{:else if actionNearMiss}
				{@render nearMissCard(actionNearMiss, tagMenu.busy, tagMenu.error, () => mergeNearMiss(t.id), () => keepBoth(t.id))}
			{:else if tagMenu.action === 'parent'}
				<!-- Hierarchy: set/clear parent (P1-2). Typeahead is a <datalist> over the
				     already-loaded tag list -- no new search endpoint. -->
				<form onsubmit={(e) => submitParent(e, t)} class="space-y-2 p-1">
					{#if t.parent_tag_id}
						<p class="text-sm text-ink">
							Parent: {parentName || '—'}
							<button
								type="button"
								onclick={() => applyParent(t, null)}
								disabled={tagMenu.busy}
								class="btn-quiet ml-1"
							>
								Clear
							</button>
						</p>
					{/if}
					<input
						bind:this={actionInput}
						bind:value={tagMenu.value}
						oninput={() => parentSearch.set(tagMenu.value)}
						type="text"
						list={`parent-options-${t.id}`}
						placeholder="New parent tag"
						aria-label={`Set parent for ${t.name}`}
						class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
					/>
					<datalist id={`parent-options-${t.id}`}>
						{#each parentSearch.items.filter((x) => x.id !== t.id) as opt (opt.id)}
							<option value={opt.name}></option>
						{/each}
					</datalist>
					<div class="flex flex-wrap gap-2">
						<button type="submit" disabled={tagMenu.busy} class="btn-accent px-3 py-1.5 text-sm">
							Set parent
						</button>
						<button
							type="button"
							onclick={() => tagMenu.close()}
							disabled={tagMenu.busy}
							class="btn-ghost px-3 py-1.5 text-sm"
						>
							Cancel
						</button>
					</div>
					{#if tagMenu.error}
						<p class="text-sm text-warn">{tagMenu.error}</p>
					{/if}
				</form>
			{:else}
				<!-- Inline rename/alias editor. A collision offers a merge instead of a
				     silent fold (never auto-merge homonyms). -->
				<form onsubmit={(e) => submitAction(e, t.id)} class="space-y-2 p-1">
					<input
						bind:this={actionInput}
						bind:value={tagMenu.value}
						type="text"
						placeholder={tagMenu.action === 'rename' ? 'New name' : 'Add an alias'}
						aria-label={tagMenu.action === 'rename' ? `Rename ${t.name}` : `Add an alias for ${t.name}`}
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
								disabled={tagMenu.busy}
								class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
							>
								Yes, merge them in
							</button>
							<button
								type="button"
								onclick={() => tagMenu.close()}
								disabled={tagMenu.busy}
								class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
							>
								No, keep separate
							</button>
						</div>
					{:else}
						<div class="flex flex-wrap gap-2">
							<button
								type="submit"
								disabled={tagMenu.busy}
								class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
							>
								{tagMenu.action === 'rename' ? 'Rename' : 'Add'}
							</button>
							<button
								type="button"
								onclick={() => tagMenu.close()}
								disabled={tagMenu.busy}
								class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
							>
								Cancel
							</button>
						</div>
					{/if}
					{#if tagMenu.error}
						<p class="text-sm text-warn">{tagMenu.error}</p>
					{/if}
				</form>
			{/if}
		</div>
	{/if}
{/snippet}

<!-- Non-blocking near-miss (P1-5): an edit/create already saved; this is an advisory
     nudge. "Keep both" records keep-separate so it won't nag again. Shared by the
     per-pill ⋯ menu and the standing create pill (HOLODEX-243) — same copy, same
     actions, wired to whichever tag id is relevant. -->
{#snippet nearMissCard(
	target: EntityRef,
	busy: boolean,
	error: string,
	onMerge: () => void,
	onKeepBoth: () => void
)}
	<div class="space-y-2 p-1">
		<p class="text-sm text-ink">
			Saved. Looks a lot like
			<span class="font-semibold">{target.name}</span>
			({videoCount(target.video_count ?? 0)}) — merge them?
		</p>
		<div class="flex flex-wrap gap-2">
			<button
				type="button"
				onclick={onMerge}
				disabled={busy}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
			>
				Merge them in
			</button>
			<button
				type="button"
				onclick={onKeepBoth}
				disabled={busy}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
			>
				Keep both
			</button>
		</div>
		{#if error}
			<p class="text-sm text-warn">{error}</p>
		{/if}
	</div>
{/snippet}

<!-- Decorative tag-glyph icon shared by both category pill variants and the Table view's
     category rows. -->
{#snippet categoryIcon()}
	<svg class="h-3.5 w-3.5 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
		<path stroke-linecap="round" stroke-linejoin="round" d="M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z" />
		<path stroke-linecap="round" stroke-linejoin="round" d="M6 6h.008v.008H6V6z" />
	</svg>
{/snippet}

<!-- Table view cells (F82 handoff, "Columns"); the column list is TAG_COLUMNS. A category's
     count is its tags, and it has no parent. -->
{#snippet nameCell(r: TagRow)}
	{#if r.kind === 'tag'}
		{r.tag.name}
	{:else}
		<span class="inline-flex min-w-0 items-center gap-1.5">{@render categoryIcon()}<span class="truncate">{r.cat.name}</span></span>
	{/if}
{/snippet}
{#snippet countCell(r: TagRow)}{r.kind === 'tag' ? (r.tag.video_count ?? 0) : tagCount(r.cat.tag_count)}{/snippet}
{#snippet parentCell(r: TagRow)}{r.kind === 'tag' ? (r.tag.parent_name ?? '') : ''}{/snippet}
{#snippet menuCell(r: TagRow)}
	{#if r.kind === 'tag'}
		<span data-tag-pill class="relative inline-flex">
			{@render tagMenuButton(r.tag, 'inline-flex h-7 w-7 items-center justify-center rounded-theme text-muted hover:text-accent')}
			{@render tagMenuPanel(r.tag)}
		</span>
	{/if}
{/snippet}

<!-- A tag with no image, or a category (F82 handoff, "Tags Posters view"): a muted tag glyph
     on surface-2, the person silhouette's family. A category's glyph is accent, as its pill's
     border is. Behind the image, so a failed load falls back to it. -->
{#snippet tagPlaceholder(category: boolean)}
	<span class="absolute inset-0 flex items-center justify-center" aria-hidden="true">
		<svg
			class="h-7 w-7 {category ? 'text-accent opacity-80' : 'text-muted opacity-55'}"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="1.5"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z" />
			<path stroke-linecap="round" stroke-linejoin="round" d="M6 6h.008v.008H6V6z" />
		</svg>
	</span>
{/snippet}

<!-- One Posters card (F82 P0-9): a 16:9 frame with the tag's image contained in it (never
     cropped: an upload can be any shape) or the placeholder, then the name and count. The
     card lifts like People's poster cards (app.css .poster-card). While managing, a tag's
     ⋯ sits top-right as a sibling of the link, shown on hover or focus. -->
{#snippet posterCard(r: TagRow)}
	{@const tag = r.kind === 'tag' ? r.tag : undefined}
	{@const name = r.kind === 'tag' ? r.tag.name : r.cat.name}
	{@const href = r.kind === 'tag' ? `/tags/${r.tag.id}` : `/categories/${r.cat.id}`}
	{@const count = r.kind === 'tag' ? videoCount(r.tag.video_count ?? 0) : tagCount(r.cat.tag_count)}
	<div class="poster-card group relative" data-tag-pill={tag ? '' : undefined}>
		<a {href} class="block focus-visible:outline-none">
			<span
				class="poster-card-frame relative block aspect-video overflow-hidden rounded-theme border bg-surface-2 group-has-[a:focus-visible]:ring-2 group-has-[a:focus-visible]:ring-accent {tag?.image_url
					? 'border-transparent'
					: 'border-rule/50'}"
			>
				{@render tagPlaceholder(!tag)}
				{#if tag?.image_url}
					<img
						src={tag.image_url}
						alt={name}
						loading="lazy"
						onerror={(e) => ((e.currentTarget as HTMLImageElement).hidden = true)}
						class="absolute inset-0 h-full w-full bg-surface-2 object-contain"
					/>
				{/if}
			</span>
			<span class="skin-title line-clamp-1 pt-1.5 text-sm font-medium text-ink" title={name}>{name}</span>
		</a>
		<span class="text-xs text-muted">{count}</span>
		{#if manage && tag}
			<span class="absolute right-1.5 top-1.5 z-10">
				{@render tagMenuButton(
					tag,
					`inline-flex h-[22px] w-[26px] items-center justify-center rounded-theme bg-bg/80 text-ink hover:text-accent focus-visible:opacity-100 group-hover:opacity-100 ${tagMenu.isOpen(tag.id) ? 'opacity-100' : 'opacity-0'}`
				)}
				{@render tagMenuPanel(tag)}
			</span>
		{/if}
	</div>
{/snippet}

<!-- "Keep which name?" — choose the surviving tag when merging a multi-select (mirrors
     /people). The others become its aliases and their videos move under it. -->
{#if choosing}
	<MergeCanonicalDialog
		kind="tag"
		items={selectedTags}
		onclose={() => (choosing = false)}
		onmerged={() => {
			selectedTags = [];
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

<!-- Bulk "Sync writeback now for selected" (HOLODEX-239): videoCountHint is null — a
     selected tag's video_count would overcount a video shared by two selected tags
     (VideoIDsForTags dedupes), so the confirm step names the tags instead of a
     possibly-wrong number. -->
{#if bulkSyncOpen}
	<WritebackBatchDialog
		scopeLabel={bulkScopeLabel}
		videoCountHint={null}
		trigger={() => api.syncTagsWriteback(selectedIds)}
		batchStatus={api.writebackBatchStatus}
		onclose={() => (bulkSyncOpen = false)}
	/>
{/if}
