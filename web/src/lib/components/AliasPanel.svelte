<script lang="ts">
	// "Aliases" identity panel (F43, ADR-061 — extracted from the F23 person card, now
	// entity-generic). Owner-curated alternate names that drive search + scan routing, plus
	// "Merge a … in…" (EntityPicker) and the homonym collision card ("never a silent merge").
	// Studio additionally gets a Rename affordance (allowRename) — the person page keeps its
	// own F37 name-chip rename and passes allowRename={false}, injecting a rename collision
	// into `conflict`. Reused verbatim on person + studio detail (not tag — RD7). Tokens only.
	import { api } from '$lib/api';
	import { toMessage, videoCount } from '$lib/format';
	import type { EntityKind, EntityRef, PersonAlias } from '$lib/types';
	import EntityPicker from '$lib/components/EntityPicker.svelte';

	let {
		entityType,
		entityId,
		entityName,
		aliases = $bindable([]),
		isOwner,
		allowRename = false,
		conflict = $bindable(null),
		onmerged,
		onrenamed
	}: {
		entityType: EntityKind;
		entityId: number;
		entityName: string;
		aliases: PersonAlias[];
		isOwner: boolean;
		allowRename?: boolean;
		// Externally-injectable merge offer: the person page routes its F37 rename collision
		// here so both entry points share one confirm card.
		conflict?: EntityRef | null;
		onmerged: () => void;
		onrenamed?: () => void;
	} = $props();

	// The EntityKind values ('person' | 'studio' | 'tag') are themselves the singular noun.
	const noun = $derived(entityType);

	let newAlias = $state('');
	let aliasBusy = $state(false);
	let aliasError = $state('');
	let aliasInput = $state<HTMLInputElement | null>(null);
	let mergeOpen = $state(false);

	// Rename (studio): an inline input pre-filled with the current name (mirrors the add
	// input). A collision routes into the shared merge-offer card, never an auto-merge.
	let renaming = $state(false);
	let renameTo = $state('');
	let renameBusy = $state(false);
	let renameError = $state('');
	let renameInput = $state<HTMLInputElement | null>(null);

	async function addAlias(e: SubmitEvent) {
		e.preventDefault();
		const value = newAlias.trim();
		if (!value || aliasBusy) return;
		aliasBusy = true;
		aliasError = '';
		try {
			const res = await api.addEntityAlias(entityType, entityId, value);
			if (res.conflict) {
				// The name already belongs to a real, separate entity — never merge silently
				// (homonyms exist). Surface it; the owner decides.
				conflict = res.conflict;
				return;
			}
			aliases = res.aliases ?? aliases;
			newAlias = '';
			aliasInput?.focus(); // keep focus for quick multi-add
		} catch (err) {
			aliasError = toMessage(err);
		} finally {
			aliasBusy = false;
		}
	}

	async function removeAlias(a: PersonAlias) {
		if (aliasBusy) return;
		aliasError = '';
		const prev = aliases;
		aliases = aliases.filter((x) => x.id !== a.id); // optimistic
		try {
			await api.deleteEntityAlias(entityType, entityId, a.id);
		} catch (err) {
			aliases = prev; // restore on failure
			aliasError = toMessage(err);
		}
	}

	// The owner confirmed the colliding entity is the same one → fold it into this entity.
	async function mergeConflict() {
		if (!conflict) return;
		aliasBusy = true;
		aliasError = '';
		try {
			await api.mergeEntities(entityType, entityId, conflict.id);
			conflict = null;
			newAlias = '';
			onmerged();
		} catch (err) {
			aliasError = toMessage(err);
		} finally {
			aliasBusy = false;
		}
	}

	function startRename() {
		renameTo = entityName;
		renameError = '';
		renaming = true;
		Promise.resolve().then(() => renameInput?.select());
	}

	function cancelRename() {
		renaming = false;
		renameTo = '';
		renameError = '';
	}

	async function submitRename(e: SubmitEvent) {
		e.preventDefault();
		const next = renameTo.trim();
		if (!next || renameBusy) return;
		if (next === entityName) {
			cancelRename();
			return;
		}
		renameBusy = true;
		renameError = '';
		try {
			const res = await api.renameEntity(entityType, entityId, next);
			if (res.conflict) {
				// Name taken — offer to merge that entity in instead (F23 invariant), via the
				// shared collision card. The rename does not happen until resolved.
				conflict = res.conflict;
				cancelRename();
				return;
			}
			cancelRename();
			onrenamed?.();
		} catch (err) {
			renameError = toMessage(err);
		} finally {
			renameBusy = false;
		}
	}
</script>

{#if aliases.length || isOwner}
	<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<h2 class="text-xs uppercase tracking-wide text-muted">Aliases</h2>
			{#if isOwner}
				<div class="flex flex-wrap items-center gap-2">
					{#if allowRename}
						<button
							onclick={startRename}
							class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
						>
							Rename
						</button>
					{/if}
					<button
						onclick={() => (mergeOpen = true)}
						class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
					>
						Merge a {noun} in…
					</button>
				</div>
			{/if}
		</div>
		<p class="text-sm text-muted">
			Searching either name finds this {noun}, and future scans match it too.
		</p>

		{#if renaming}
			<form onsubmit={submitRename} class="flex flex-wrap items-center gap-2">
				<input
					bind:this={renameInput}
					bind:value={renameTo}
					type="text"
					aria-label={`Rename this ${noun}`}
					aria-describedby={renameError ? 'rename-error' : undefined}
					class="min-w-0 flex-1 rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
				/>
				<button
					type="submit"
					disabled={renameBusy}
					class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
				>
					{renameBusy ? 'Renaming…' : 'Rename'}
				</button>
				<button
					type="button"
					onclick={cancelRename}
					disabled={renameBusy}
					class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
				>
					Cancel
				</button>
			</form>
			<p class="text-xs text-muted">
				“{entityName}” is kept as an alias — search and future scans still match it.
			</p>
			{#if renameError}
				<p id="rename-error" class="text-sm text-warn">{renameError}</p>
			{/if}
		{/if}

		<div class="flex flex-wrap gap-2" aria-live="polite">
			{#each aliases as a (a.id)}
				<span
					class="inline-flex items-center gap-1 rounded-full bg-surface-2 px-2.5 py-0.5 text-sm text-ink"
				>
					{a.alias}
					{#if isOwner}
						<button
							onclick={() => removeAlias(a)}
							disabled={aliasBusy}
							aria-label={`Remove alias ${a.alias}`}
							class="leading-none text-muted hover:text-accent focus:text-accent disabled:opacity-60"
						>
							×
						</button>
					{/if}
				</span>
			{/each}
			{#if !aliases.length && isOwner}
				<p class="text-sm text-muted">No aliases yet.</p>
			{/if}
		</div>

		{#if isOwner}
			<form onsubmit={addAlias} class="flex flex-wrap items-center gap-2">
				<input
					bind:this={aliasInput}
					bind:value={newAlias}
					type="text"
					placeholder="Add an alias"
					aria-label="Add an alias"
					aria-describedby={aliasError ? 'alias-error' : undefined}
					class="min-w-0 flex-1 rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
				/>
				<button
					type="submit"
					disabled={aliasBusy}
					class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
				>
					Add
				</button>
			</form>
		{/if}

		{#if conflict}
			<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
				<p class="text-sm text-ink">
					<span class="font-semibold">{conflict.name}</span> ({videoCount(conflict.video_count ?? 0)})
					is already a separate {noun}. Are they the same as {entityName}?
				</p>
				<div class="flex flex-wrap items-center gap-2">
					<button
						onclick={mergeConflict}
						disabled={aliasBusy}
						class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
					>
						Yes, merge them in
					</button>
					<button
						onclick={() => (conflict = null)}
						disabled={aliasBusy}
						class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
					>
						No, keep separate
					</button>
				</div>
			</div>
		{/if}

		{#if aliasError}
			<p id="alias-error" class="text-sm text-warn">{aliasError}</p>
		{/if}
	</section>
{/if}

{#if mergeOpen}
	<EntityPicker
		{entityType}
		canonicalId={entityId}
		canonicalName={entityName}
		onclose={() => (mergeOpen = false)}
		onmerged={onmerged}
	/>
{/if}
