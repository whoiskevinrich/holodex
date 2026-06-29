<script lang="ts">
	// One curated field row (F30): renders its values as chips and, for the owner,
	// an inline "+ Add" affordance. Mutations call the curation API then ask the
	// parent to reload so resolved[] reflects the new merged state. Tokens only; 3 skins.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { Person, ResolvedField, ResolvedValue } from '$lib/types';
	import CurationChip from './CurationChip.svelte';

	let {
		field,
		videoId,
		isOwner,
		people = [],
		personStyle = false,
		onchanged
	}: {
		field: ResolvedField;
		videoId: number;
		isOwner: boolean;
		people?: Person[];
		personStyle?: boolean;
		onchanged: () => Promise<void> | void;
	} = $props();

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
		if (v) run(() => api.curateMedia(videoId, { field: field.canonical, value: v, action: 'add' }));
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
		run(() => api.curateMedia(videoId, { field: field.canonical, value, action: 'suppress' }));
	}
	function toggleWrite(value: string, noWrite: boolean) {
		run(() =>
			noWrite
				? api.curateMedia(videoId, { field: field.canonical, value, action: 'nowrite' })
				: api.clearMediaCuration(videoId, { field: field.canonical, value, action: 'nowrite' })
		);
	}
	async function edit(oldValue: string, newValue: string) {
		const existing = items.find((it) => it.value === oldValue);
		await run(async () => {
			// Edit = drop the old value (clear a manual add, else tombstone a source
			// value) + add the new one.
			if (existing?.manual) {
				await api.clearMediaCuration(videoId, { field: field.canonical, value: oldValue, action: 'add' });
			} else {
				await api.curateMedia(videoId, { field: field.canonical, value: oldValue, action: 'suppress' });
			}
			await api.curateMedia(videoId, { field: field.canonical, value: newValue, action: 'add' });
		});
	}
</script>

<div class="flex flex-wrap items-center gap-1.5" class:opacity-60={busy} aria-busy={busy}>
	{#each items as it (it.value)}
		<CurationChip
			item={it}
			{isOwner}
			showRemove={isSet}
			person={personFor(it.value)}
			onedit={edit}
			onremove={remove}
			ontogglewrite={toggleWrite}
		/>
	{/each}

	{#if isOwner && isSet}
		{#if adding}
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

	{#if error}
		<span class="text-xs text-warn" aria-live="polite">{error}</span>
	{/if}
</div>
