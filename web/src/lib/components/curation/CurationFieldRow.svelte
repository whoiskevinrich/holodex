<script lang="ts">
	// One curated field row (F30): renders its values as chips and, for the owner,
	// an inline "+ Add" affordance. Mutations call the curation API then ask the
	// parent to reload so resolved[] reflects the new merged state. Tokens only; 3 skins.
	//
	// Entity-generic since F37: by default mutations hit the media curation endpoints
	// (videoId), but a caller may supply its own `curate`/`clearCuration` transport (the
	// person page passes the /people/{id}/curation pair) and hide the "don't write" toggle
	// (`showWriteToggle={false}` — persons have no writeback).
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { CurationRequest, Person, ResolvedField, ResolvedValue } from '$lib/types';
	import CurationChip from './CurationChip.svelte';
	import EntityPickerDialog from '../entity/EntityPickerDialog.svelte';

	let {
		field,
		videoId = 0,
		isOwner,
		people = [],
		personStyle = false,
		showWriteToggle = true,
		curate,
		clearCuration,
		onchanged
	}: {
		field: ResolvedField;
		videoId?: number;
		isOwner: boolean;
		people?: Person[];
		personStyle?: boolean;
		showWriteToggle?: boolean;
		// Optional transport override (F37). When absent, the media endpoints are used.
		curate?: (req: CurationRequest) => Promise<unknown>;
		clearCuration?: (req: CurationRequest) => Promise<unknown>;
		onchanged: () => Promise<void> | void;
	} = $props();

	// The effective transport: caller-supplied, else the media curation client (F30).
	const doCurate = (req: CurationRequest) =>
		curate ? curate(req) : api.curateMedia(videoId, req);
	const doClear = (req: CurationRequest) =>
		clearCuration ? clearCuration(req) : api.clearMediaCuration(videoId, req);

	// For person fields (actors/director), match each value to a linked Person by name
	// (case-insensitive) so the chip can show a headshot + link. Unmatched values (e.g.
	// a director with no Person record, or a manual addition) render as a plain chip.
	const peopleByName = $derived(
		new Map(people.map((p) => [p.name.trim().toLowerCase(), p]))
	);
	function personFor(value: string): { id: number; headshot_version?: number } | undefined {
		if (!personStyle) return undefined;
		return peopleByName.get(value.trim().toLowerCase());
	}

	const items = $derived<ResolvedValue[]>(
		field.items ?? field.values.map((v) => ({ value: v, sources: [] as string[] }))
	);
	// Add + per-value remove are for set (merge-mode) fields; a scalar shows a single
	// overridable chip without add/remove (handoff: scalar = one value).
	const isSet = $derived(!!field.multi);

	let adding = $state(false);
	let draft = $state('');
	let busy = $state(false);
	let error = $state('');
	let pickerOpen = $state(false);

	// Person-typed fields (F40, ADR-072 — actors/director) open the entity-search
	// picker instead of a bare text input, so linking a cast member finds an
	// existing person rather than minting a typo'd near-duplicate. Reuses
	// EntityPickerDialog (built for the Extraction tab's People/Studio edits) —
	// same entity-generic search + inline-create shape the F40 design calls for.
	const isPersonLink = $derived(field.entity_kind === 'person');

	async function run(fn: () => Promise<unknown>) {
		busy = true;
		error = '';
		try {
			await fn();
			await onchanged();
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}

	function add() {
		const v = draft.trim();
		adding = false;
		draft = '';
		if (v) run(() => doCurate({ field: field.canonical, value: v, action: 'add' }));
	}
	function onAddKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			add();
		} else if (e.key === 'Escape') {
			e.preventDefault();
			adding = false;
			draft = '';
		}
	}

	function remove(value: string) {
		run(() => doCurate({ field: field.canonical, value, action: 'suppress' }));
	}
	function toggleWrite(value: string, noWrite: boolean) {
		run(() =>
			noWrite
				? doCurate({ field: field.canonical, value, action: 'nowrite' })
				: doClear({ field: field.canonical, value, action: 'nowrite' })
		);
	}
	async function edit(oldValue: string, newValue: string) {
		const existing = items.find((it) => it.value === oldValue);
		await run(async () => {
			// Edit = drop the old value (clear a manual add, else tombstone a source
			// value) + add the new one.
			if (existing?.manual) {
				await doClear({ field: field.canonical, value: oldValue, action: 'add' });
			} else {
				await doCurate({ field: field.canonical, value: oldValue, action: 'suppress' });
			}
			await doCurate({ field: field.canonical, value: newValue, action: 'add' });
		});
	}
</script>

<div class="flex flex-wrap items-center gap-1.5" class:opacity-60={busy} aria-busy={busy}>
	{#each items as it (it.value)}
		<CurationChip
			item={it}
			{isOwner}
			showRemove={isSet}
			{showWriteToggle}
			person={personFor(it.value)}
			onedit={edit}
			onremove={remove}
			ontogglewrite={toggleWrite}
		/>
	{/each}

	{#if isOwner && isSet}
		{#if isPersonLink}
			<button
				type="button"
				onclick={() => (pickerOpen = true)}
				class="inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted hover:text-accent hover:border-accent focus-visible:text-accent"
			>
				<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>
				Add
			</button>
		{:else if adding}
			<!-- svelte-ignore a11y_autofocus -->
			<input
				bind:value={draft}
				onkeydown={onAddKey}
				onblur={add}
				autofocus
				aria-label={`Add a value to ${field.label}`}
				placeholder="Add…"
				class="inline-block w-28 rounded-theme border border-rule bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent"
			/>
		{:else}
			<button
				type="button"
				onclick={() => (adding = true)}
				class="inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted hover:text-accent hover:border-accent focus-visible:text-accent"
			>
				<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>
				Add
			</button>
		{/if}
	{/if}

	{#if pickerOpen}
		<EntityPickerDialog
			kind="person"
			seedQuery=""
			onclose={() => (pickerOpen = false)}
			onselect={(name) => run(() => doCurate({ field: field.canonical, value: name, action: 'add' }))}
		/>
	{/if}

	{#if error}
		<span class="text-xs text-warn" aria-live="polite">{error}</span>
	{/if}
</div>
