<script lang="ts">
	// "Aliases" identity panel (F43, ADR-061 — extracted from the F23 person card, now
	// entity-generic). Owner-curated alternate names that drive search + scan routing, plus
	// "Merge a … in…" (EntityPicker) and the homonym collision card ("never a silent merge").
	// Rename lives on the entity's own NameEditControl now (HOLODEX-269, both person and
	// studio) — this panel is add/remove/merge only. Reused verbatim on person + studio
	// detail (not tag — RD7). Tokens only.
	import { api } from '$lib/api';
	import { toMessage, videoCount } from '$lib/format';
	import type { EntityKind, EntityRef, PersonAlias } from '$lib/types';
	import EntityPicker from '$lib/components/entity/EntityPicker.svelte';
	import MergeOfferCard from '$lib/components/entity/MergeOfferCard.svelte';

	let {
		entityType,
		entityId,
		entityName,
		aliases = $bindable([]),
		isOwner,
		conflict = $bindable(null),
		onmerged
	}: {
		entityType: EntityKind;
		entityId: number;
		entityName: string;
		aliases: PersonAlias[];
		isOwner: boolean;
		conflict?: EntityRef | null;
		onmerged: () => void;
	} = $props();

	// The EntityKind values ('person' | 'studio' | 'tag') are themselves the singular noun.
	const noun = $derived(entityType);

	let newAlias = $state('');
	let aliasBusy = $state(false);
	let aliasError = $state('');
	let aliasInput = $state<HTMLInputElement | null>(null);
	let mergeOpen = $state(false);

	// Non-blocking near-miss (P1-5): after a successful add/rename, a fuzzy look-alike is
	// surfaced as an advisory nudge (distinct from the exact-name `conflict` above). Studio
	// only — person has no near-miss endpoint (`api.nearMiss` excludes it), so we guard.
	let nearMiss = $state<EntityRef | null>(null);

	async function flagNearMiss(name: string) {
		if (entityType === 'person') return; // no person near-miss endpoint (RD-scoped)
		try {
			nearMiss = (await api.nearMiss(entityType, entityId, name)).near_miss;
		} catch {
			// Advisory only — a failed look-alike probe must never block the completed edit.
			nearMiss = null;
		}
	}

	// Fold the look-alike into this entity, or keep both (records keep-separate so the hint
	// never returns for this pair).
	async function mergeNearMiss() {
		if (!nearMiss || aliasBusy) return;
		aliasBusy = true;
		aliasError = '';
		try {
			await api.mergeEntities(entityType, entityId, nearMiss.id);
			nearMiss = null;
			onmerged();
		} catch (err) {
			aliasError = toMessage(err);
		} finally {
			aliasBusy = false;
		}
	}

	async function keepBoth() {
		if (!nearMiss || aliasBusy) return;
		aliasBusy = true;
		aliasError = '';
		try {
			await api.dismissDuplicate(entityType, entityId, nearMiss.id);
			nearMiss = null;
		} catch (err) {
			aliasError = toMessage(err);
		} finally {
			aliasBusy = false;
		}
	}

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
			await flagNearMiss(value);
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

</script>

{#if aliases.length || isOwner}
	<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<h2 class="text-xs uppercase tracking-wide text-muted">Aliases</h2>
			{#if isOwner}
				<div class="flex flex-wrap items-center gap-2">
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
							class="leading-none text-muted hover:text-accent focus:text-accent"
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
			<MergeOfferCard
				{noun}
				{entityName}
				{conflict}
				busy={aliasBusy}
				onmerge={mergeConflict}
				onkeepseparate={() => (conflict = null)}
			/>
		{/if}

		{#if nearMiss}
			<!-- Non-blocking near-miss (P1-5): the edit already saved; this is an advisory nudge,
			     visually lighter than the exact-name conflict card above. -->
			<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
				<p class="text-sm text-ink">
					Saved. Looks a lot like <span class="font-semibold">{nearMiss.name}</span>
					({videoCount(nearMiss.video_count ?? 0)}) — merge them?
				</p>
				<div class="flex flex-wrap items-center gap-2">
					<button
						onclick={mergeNearMiss}
						disabled={aliasBusy}
						class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
					>
						Merge them in
					</button>
					<button
						onclick={keepBoth}
						disabled={aliasBusy}
						class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
					>
						Keep both
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
