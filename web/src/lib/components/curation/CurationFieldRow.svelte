<script lang="ts">
	// One curated field row (F30): renders its values as chips and, for the owner,
	// an inline "+ Add" affordance. Mutations call the curation API then ask the
	// parent to reload so resolved[] reflects the new merged state. Tokens only; QA Cinémathèque.
	//
	// Entity-generic since F37: by default mutations hit the media curation endpoints
	// (videoId), but a caller may supply its own `curate`/`clearCuration` transport (the
	// person page passes the /people/{id}/curation pair) and hide the "don't write" toggle
	// (`showWriteToggle={false}` — persons have no writeback).
	import { tick } from 'svelte';
	import { api } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import { rowProvenance } from '#lib/chipProvenance.ts';
	import type { CurationRequest, Person, ResolvedField, ResolvedValue } from '#lib/types.ts';
	import CurationChip from './CurationChip.svelte';
	import ProvenanceBadge from '../enrichment/ProvenanceBadge.svelte';
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
	// HOLODEX-144: one row badge when every value shares a provenance, else a 12px icon on
	// each provider value (chip-row-provenance-handoff.md).
	const provenance = $derived(rowProvenance(items));
	// Add + per-value remove are for set (merge-mode) fields; a scalar shows a single
	// overridable chip without add/remove (handoff: scalar = one value).
	const isSet = $derived(!!field.multi);

	// Removed (N) (F30.3g, source-choices-restore-handoff.md): the owner's removed values, listed
	// under the row when the disclosure is open. Opening it is UI-only; nothing is sent.
	const removed = $derived(isOwner && isSet ? (field.removed ?? []) : []);
	const removedId = $derived(`removed-${field.canonical}`);
	let showRemoved = $state(false);
	let announce = $state('');
	let rowEl = $state<HTMLDivElement | null>(null);

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

	// run applies one mutation and reloads; true when it landed.
	async function run(fn: () => Promise<unknown>): Promise<boolean> {
		busy = true;
		error = '';
		try {
			const result = await fn();
			// curateMedia/setFieldDecision resolve (rather than throw) on a composite-key
			// collision, carrying `{conflict}` — treat that as a failure too, or the
			// caller never learns the edit didn't apply (HOLODEX-272).
			if (result && typeof result === 'object' && 'conflict' in result && (result as { conflict?: unknown }).conflict) {
				error = 'This change conflicts with another video with the same title, people, date, and studio.';
				return false;
			}
			await onchanged();
			return true;
		} catch (e) {
			error = toMessage(e);
			return false;
		} finally {
			busy = false;
		}
	}

	// Same match key as the curation store (trim + lowercase): a typed value meets its removal
	// in any casing.
	const curationKey = (s: string) => s.trim().toLowerCase();

	// addValue adds a typed or picked value. A value the owner removed earlier comes back by
	// lifting its removal, since a plain add stays hidden behind it; one no source sends now is
	// then added too, so what the owner typed appears.
	function addValue(v: string) {
		const prior = removed.find((r) => curationKey(r.value) === curationKey(v));
		if (!prior) {
			run(() => doCurate({ field: field.canonical, value: v, action: 'add' }));
			return;
		}
		restore(prior.value, !!prior.not_sent, v);
	}

	async function restore(value: string, notSent: boolean, typed?: string) {
		const restores = [...(rowEl?.querySelectorAll<HTMLButtonElement>('[data-restore]') ?? [])];
		const at = restores.findIndex((b) => b.getAttribute('aria-label') === `Restore ${value}`);
		const ok = await run(async () => {
			await doClear({ field: field.canonical, value, action: 'suppress' });
			if (notSent) await doCurate({ field: field.canonical, value: typed ?? value, action: 'add' });
		});
		if (!ok) {
			// The clear may have landed before the follow-up add failed: reload so the row
			// shows what the store holds now.
			if (notSent) void onchanged();
			return;
		}
		announce = notSent && !typed ? `Restored ${value}. No source sends it now.` : `Restored ${value}.`;
		await tick();
		const left = [...(rowEl?.querySelectorAll<HTMLButtonElement>('[data-restore]') ?? [])];
		const next = left[Math.min(Math.max(at, 0), left.length - 1)];
		(next ?? rowEl?.querySelector<HTMLButtonElement>('[data-add]'))?.focus();
	}

	function add() {
		const v = draft.trim();
		adding = false;
		draft = '';
		if (v) addValue(v);
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

	async function remove(value: string) {
		if (!(await run(() => doCurate({ field: field.canonical, value, action: 'suppress' })))) return;
		await tick();
		announce = `Removed ${value}. ${removed.length} removed.`;
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

<div bind:this={rowEl} class="flex flex-wrap items-center gap-1.5" class:opacity-60={busy} aria-busy={busy}>
	{#each items as it, i (it.value)}
		<CurationChip
			item={it}
			{isOwner}
			showRemove={isSet}
			{showWriteToggle}
			person={personFor(it.value)}
			mark={provenance.marks[i]}
			onedit={edit}
			onremove={remove}
			ontogglewrite={toggleWrite}
		/>
	{/each}
	{#if provenance.badge}
		<ProvenanceBadge
			provider={provenance.badge.provider}
			label={provenance.badge.provider}
			manual={provenance.badge.manual}
		/>
	{/if}

	{#if isOwner && isSet}
		{#if isPersonLink}
			<button
				type="button"
				data-add
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
				data-add
				onclick={() => (adding = true)}
				class="inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted hover:text-accent hover:border-accent focus-visible:text-accent"
			>
				<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>
				Add
			</button>
		{/if}
	{/if}

	{#if removed.length > 0}
		<!-- Removed (N) toggle: a UI-only disclosure (.btn-quiet), absent when nothing is removed. -->
		<button
			type="button"
			onclick={() => (showRemoved = !showRemoved)}
			aria-expanded={showRemoved}
			aria-controls={removedId}
			aria-label={`Removed values for ${field.label}, ${removed.length}`}
			class="btn-quiet inline-flex min-h-6 items-center gap-1 px-1 text-xs {showRemoved ? 'text-ink' : ''}"
		>
			Removed ({removed.length})
			<svg class="h-3 w-3 {showRemoved ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6"/></svg>
		</button>
		{#if showRemoved}
			<div id={removedId} role="group" aria-label={`Removed from ${field.label}`} class="flex basis-full flex-wrap items-center gap-1.5">
				{#each removed as r (r.value)}
					<CurationChip
						item={{ value: r.value, sources: [] }}
						{isOwner}
						removed={{ notSent: !!r.not_sent, onrestore: () => restore(r.value, !!r.not_sent) }}
					/>
				{/each}
			</div>
		{/if}
	{/if}
	<span class="sr-only" aria-live="polite">{announce}</span>

	{#if pickerOpen}
		<EntityPickerDialog
			kind="person"
			seedQuery=""
			onclose={() => (pickerOpen = false)}
			onselect={(name) => addValue(name)}
		/>
	{/if}

	{#if error}
		<span class="text-xs text-warn" aria-live="polite">{error}</span>
	{/if}
</div>
