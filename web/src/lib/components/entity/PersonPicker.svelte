<script lang="ts">
	// People relationship-edit popover (HOLODEX-272) — the multi-select sibling of
	// StudioPicker (:271): unlike a single-valued field, a video's People list is a
	// SET, so this shows what's already attached (each independently removable)
	// alongside search/create, and stays open across commits instead of closing on
	// the first one. Every attach/detach commits through the caller-supplied
	// attach/detach props (media/[id]/+page.svelte's attachPerson/detachPerson,
	// which POST /media/{id}/curation — a link IS a curation add, ADR-072 RD1) and
	// may 409 on the People composite-key collision gate (HOLODEX-270/271's
	// mechanism, reused). Unlike StudioPicker there's no `verdict` prop: the grid's
	// own remove control (outside this component entirely) shares the same
	// attach/detach calls, so the conflict verdict is owned by the page, not this
	// popover — on a conflict this just closes, and the page renders the shared
	// CollisionOfferCard where the tile was.
	import { api } from '$lib/api';
	import { personKey, toMessage } from '$lib/format';
	import type { Person, ResolvedPerson, VideoCollisionRef } from '$lib/types';
	import PickerShell, { focusOptionIn } from './PickerShell.svelte';

	let {
		people,
		isOwner,
		attach,
		detach,
		busyKey = $bindable(null)
	}: {
		people: ResolvedPerson[];
		isOwner: boolean;
		attach: (name: string, role: 'actor' | 'director') => Promise<{ ok: true } | { conflict: VideoCollisionRef }>;
		detach: (name: string, role: 'actor' | 'director') => Promise<{ ok: true } | { conflict: VideoCollisionRef }>;
		// Shared with the video page's grid-remove control (HOLODEX-272 review fix) —
		// both surfaces mutate the same video's people, so they must share one busy
		// gate or a grid remove and a picker commit can race on the same video.
		busyKey?: string | null;
	} = $props();

	const ROLES: ('actor' | 'director')[] = ['actor', 'director'];

	let open = $state(false);
	let commitError = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let input = $state<HTMLInputElement | null>(null);

	let query = $state('');
	let candidates = $state<Person[]>([]);
	let active = $state(0);
	let searchLoading = $state(false);
	let searchError = $state('');
	let searchId = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let selectedRole = $state<Record<string, 'actor' | 'director'>>({});

	const trimmedQuery = $derived(query.trim());
	const showCreateRow = $derived(
		trimmedQuery.length >= 2 && !candidates.some((c) => c.name.toLowerCase() === trimmedQuery.toLowerCase())
	);
	const optionCount = $derived(candidates.length + (showCreateRow ? 1 : 0));

	function roleLabel(role: 'actor' | 'director') {
		return role === 'actor' ? 'Actor' : 'Director';
	}

	// A search result's available roles are the two minus whichever this video
	// already links this person id under — a dual-role attach is two separate
	// commits (video_people's PK is (video_id, person_id, role), ADR-072), not one.
	function availableRoles(personId: number): ('actor' | 'director')[] {
		const taken = new Set(people.filter((p) => p.id === personId).map((p) => p.role));
		return ROLES.filter((r) => !taken.has(r));
	}

	// Validates the remembered selection against the CURRENT available-roles list —
	// a role picked earlier in this session (e.g. 'actor') can go stale mid-session
	// once that role is taken by a just-committed attach, and resubmitting it would
	// silently target the wrong role (HOLODEX-272 review fix).
	function roleFor(key: string, avail: ('actor' | 'director')[]): 'actor' | 'director' {
		const chosen = selectedRole[key];
		return chosen && avail.includes(chosen) ? chosen : avail[0];
	}

	function setRole(key: string, role: 'actor' | 'director') {
		selectedRole = { ...selectedRole, [key]: role };
	}

	function openPicker() {
		query = '';
		candidates = [];
		active = 0;
		searchError = '';
		commitError = '';
		selectedRole = {};
		open = true;
		Promise.resolve().then(() => input?.focus());
	}

	function closePicker() {
		open = false;
		clearTimeout(timer);
	}

	function onInput() {
		clearTimeout(timer);
		const q = query.trim();
		if (q.length < 2) {
			candidates = [];
			return;
		}
		timer = setTimeout(() => void search(q), 300);
	}

	async function search(q: string) {
		const id = ++searchId;
		searchLoading = true;
		searchError = '';
		try {
			const res = await api.search(q);
			if (id !== searchId) return;
			candidates = res.people ?? [];
			active = 0;
		} catch (e) {
			if (id !== searchId) return;
			searchError = toMessage(e);
			candidates = [];
		} finally {
			if (id === searchId) searchLoading = false;
		}
	}

	// Multi-select: on success the popover stays open (unlike StudioPicker, which
	// closes) so more people can be added in the same session; a conflict still
	// closes it, matching StudioPicker's handoff to its caller's verdict slot.
	async function commitAttach(key: string, name: string, role: 'actor' | 'director') {
		if (busyKey) return;
		busyKey = key;
		commitError = '';
		try {
			const res = await attach(name, role);
			if ('conflict' in res) {
				closePicker();
				return;
			}
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	async function commitDetach(p: ResolvedPerson) {
		if (busyKey) return;
		busyKey = personKey(p);
		commitError = '';
		try {
			const res = await detach(p.name, p.role);
			if ('conflict' in res) {
				closePicker();
				return;
			}
		} catch (e) {
			commitError = toMessage(e);
		} finally {
			busyKey = null;
		}
	}

	function pickAt(i: number) {
		if (i < candidates.length) {
			const c = candidates[i];
			const avail = availableRoles(c.id);
			if (!avail.length) return;
			void commitAttach(`search:${c.id}`, c.name, roleFor(`search:${c.id}`, avail));
		} else if (showCreateRow) {
			void commitAttach('create', trimmedQuery, roleFor('create', ROLES));
		}
	}

	function onSearchKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' && optionCount) {
			e.preventDefault();
			focusOption(0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			pickAt(active);
		}
	}

	function onOptionKey(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			pickAt(i);
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusOption((i + 1) % optionCount);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (i === 0) input?.focus();
			else focusOption(i - 1);
		}
	}

	function focusOption(i: number) {
		active = i;
		focusOptionIn(dialogEl, 'person-search-opt', i);
	}
</script>

{#if isOwner}
	<button
		type="button"
		aria-haspopup="dialog"
		onclick={openPicker}
		class="flex aspect-[2/3] w-full flex-col items-center justify-center gap-1 rounded-theme border border-dashed border-rule text-muted hover:border-accent hover:text-accent"
	>
		<svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<path stroke-linecap="round" stroke-linejoin="round" d="M12 5v14M5 12h14" />
		</svg>
		<span class="text-xs">Add person</span>
	</button>
{/if}

{#snippet roleToggle(key: string, avail: ('actor' | 'director')[], ariaLabel: string)}
	<div role="group" aria-label={ariaLabel} class="flex shrink-0 gap-1">
		{#each avail as r (r)}
			<button
				type="button"
				aria-pressed={roleFor(key, avail) === r}
				onclick={(e) => {
					e.stopPropagation();
					setRole(key, r);
				}}
				class="rounded-full border px-2 py-0.5 text-xs {roleFor(key, avail) === r
					? 'border-accent bg-accent text-accent-ink'
					: 'border-rule text-muted'}"
			>
				{roleLabel(r)}
			</button>
		{/each}
	</div>
{/snippet}

{#if open}
	<PickerShell titleId="person-picker-title" onclose={closePicker} bind:dialogEl>
		{#snippet header()}
			<h2 id="person-picker-title" class="skin-title text-lg font-semibold text-ink">Add person</h2>
		{/snippet}

		{#if people.length}
			<ul class="mb-3 flex flex-wrap gap-1.5">
				{#each people as p (p.id + ':' + p.role)}
					<li
						class="inline-flex items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-ink"
					>
						<span class="max-w-[10rem] truncate">{p.name}</span>
						<span class="text-muted">{roleLabel(p.role)}</span>
						<button
							type="button"
							aria-label={`Remove ${p.name} (${roleLabel(p.role)})`}
							disabled={busyKey === personKey(p)}
							onclick={() => commitDetach(p)}
							class="text-muted hover:text-accent disabled:cursor-default"
						>
							{busyKey === personKey(p) ? '…' : '×'}
						</button>
					</li>
				{/each}
			</ul>
		{/if}

		<!-- svelte-ignore a11y_role_has_required_aria_props -->
		<input
			bind:this={input}
			bind:value={query}
			oninput={onInput}
			onkeydown={onSearchKey}
			role="combobox"
			aria-expanded={optionCount > 0}
			aria-controls="person-search-options"
			placeholder="Search people by name…"
			class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
		/>

		<p class="mt-2 text-xs text-muted" aria-live="polite">
			{#if searchLoading}
				Searching…
			{:else if searchError}
				<span class="text-warn">{searchError}</span>
			{:else if trimmedQuery.length < 2}
				Type at least two characters to search.
			{:else if candidates.length}
				{candidates.length} match{candidates.length === 1 ? '' : 'es'} — Tab or ↑/↓ to choose, then click or press
				Enter
			{:else}
				No matches for "{trimmedQuery}".
			{/if}
		</p>

		<ul id="person-search-options" role="listbox" aria-label="People" class="mt-2 flex-1 overflow-y-auto">
			{#each candidates as c, i (c.id)}
				{@const avail = availableRoles(c.id)}
				{@const key = `search:${c.id}`}
				<li
					id="person-search-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={i === active}
					aria-disabled={busyKey !== null || !avail.length}
					onclick={() => pickAt(i)}
					onkeydown={(e) => onOptionKey(e, i)}
					onfocus={() => (active = i)}
					onmouseenter={() => (active = i)}
					class="cursor-pointer rounded-theme border-l-2 px-3 py-2 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<div class="flex items-center justify-between gap-2">
						<span class="truncate text-sm text-ink">{c.name}{busyKey === key ? '…' : ''}</span>
						{#if !avail.length}
							<span class="shrink-0 text-xs text-muted">Already attached as Actor, Director</span>
						{:else}
							{@render roleToggle(key, avail, `Role for ${c.name}`)}
						{/if}
					</div>
				</li>
			{/each}
			{#if showCreateRow}
				{@const i = candidates.length}
				<li
					id="person-search-opt-{i}"
					role="option"
					tabindex={i === active ? 0 : -1}
					aria-selected={i === active}
					aria-disabled={busyKey !== null}
					onclick={() => pickAt(i)}
					onkeydown={(e) => onOptionKey(e, i)}
					onfocus={() => (active = i)}
					onmouseenter={() => (active = i)}
					class="cursor-pointer rounded-theme border-l-2 px-3 py-2 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<div class="flex items-center justify-between gap-2">
						<span class="text-xs text-accent">Use "{trimmedQuery}" as a new person{busyKey === 'create' ? '…' : ''}</span
						>
						{@render roleToggle('create', ROLES, `Role for ${trimmedQuery}`)}
					</div>
				</li>
			{/if}
		</ul>

		{#if commitError}
			<p class="mt-2 text-sm text-warn" aria-live="polite">{commitError}</p>
		{/if}
	</PickerShell>
{/if}
