<script lang="ts">
	// "Keep which name?" — step two of a multi-select merge (F43, ADR-061): given 2+ selected
	// entities, choose which one survives; the rest fold into it (their videos move, their names
	// become aliases). Shared by /people and /tags (HOLODEX-163 — was two verbatim copies).
	// Entity-generic via `kind`; api.mergeEntities does the fold. The parent owns selection: it
	// passes the chosen `items` and reacts to `onmerged` (reload + clear) / `onclose` (dismiss).
	// Modal chrome (focus trap, Esc, backdrop, focus-return) delegates to ConfirmDialog.
	import { api } from '$lib/api';
	import { toMessage, videoCount } from '$lib/format';
	import type { EntityKind, EntityRef } from '$lib/types';
	import ConfirmDialog from '../shared/ConfirmDialog.svelte';

	let {
		kind,
		items,
		onclose,
		onmerged
	}: {
		kind: EntityKind;
		items: EntityRef[];
		onclose: () => void;
		onmerged: () => void;
	} = $props();

	// Per-entity noun for the confirm copy — the only textual delta across the three.
	const NOUNS: Record<EntityKind, string> = { person: 'person', studio: 'studio', tag: 'tag' };
	const noun = $derived(NOUNS[kind]);

	// The survivor defaults to the first selected until the owner picks another via the radios.
	let canonicalId = $state<number | null>(null);
	const canonical = $derived(canonicalId ?? items[0]?.id ?? null);
	let merging = $state(false);
	let mergeError = $state('');

	async function confirmMerge() {
		if (canonical == null || merging) return;
		merging = true;
		mergeError = '';
		try {
			// Fold every other selected entity into the chosen survivor.
			for (const from of items.filter((e) => e.id !== canonical)) {
				await api.mergeEntities(kind, canonical, from.id);
			}
			onmerged();
			onclose();
		} catch (e) {
			mergeError = toMessage(e);
		} finally {
			merging = false;
		}
	}
</script>

<ConfirmDialog
	title="Keep which name?"
	confirmLabel="Merge"
	cancelLabel="Back"
	busy={merging}
	error={mergeError}
	variant="accent"
	onconfirm={confirmMerge}
	oncancel={onclose}
>
	{#snippet body()}
		<p class="text-xs text-muted">
			The chosen name stays; the others become its aliases and their videos move under it. Confirm
			these are the same {noun} — this can’t be auto-undone.
		</p>
		<fieldset class="space-y-1">
			{#each items as e (e.id)}
				<label class="flex cursor-pointer items-center gap-3 rounded-theme px-2 py-1.5 text-ink hover:bg-surface-2">
					<input type="radio" name="merge-canonical" class="accent-accent" value={e.id} checked={canonical === e.id} onchange={() => (canonicalId = e.id)} />
					<span class="flex-1 truncate">{e.name}</span>
					<span class="text-xs text-muted">{videoCount(e.video_count ?? 0)}</span>
				</label>
			{/each}
		</fieldset>
	{/snippet}
</ConfirmDialog>
