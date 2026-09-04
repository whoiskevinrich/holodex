<script lang="ts">
	// "Aliases" identity panel (F43, ADR-061 — extracted from the F23 person card, now
	// entity-generic). Owner-curated alternate names that drive search + scan routing, plus
	// "Merge a … in…" (EntityPicker) and the homonym collision card ("never a silent merge").
	// Rename lives on the entity's own NameEditControl now (HOLODEX-269, both person and
	// studio) — this panel is add/remove/merge only. Reused verbatim on person + studio
	// detail (not tag — RD7). Tokens only.
	import { api } from '$lib/api';
	import { toMessage, videoCount } from '$lib/format';
	import type { EntityKind, EntityRef, PersonAlias, SkippedAlias } from '$lib/types';
	import EntityPicker from '$lib/components/entity/EntityPicker.svelte';
	import MergeOfferCard from '$lib/components/entity/MergeOfferCard.svelte';

	let {
		entityType,
		entityId,
		entityName,
		aliases = $bindable([]),
		isOwner,
		conflict = $bindable(null),
		skippedAliases = [],
		onmerged
	}: {
		entityType: EntityKind;
		entityId: number;
		entityName: string;
		aliases: PersonAlias[];
		isOwner: boolean;
		conflict?: EntityRef | null;
		// Provider names a collision kept off this entity (F58, ADR-088 D5). Owner-only —
		// the detail payload omits the key entirely for a visitor, so this stays empty.
		skippedAliases?: SkippedAlias[];
		onmerged: () => void;
	} = $props();

	// The EntityKind values ('person' | 'studio' | 'tag') are themselves the singular noun.
	const noun = $derived(entityType);
	// ...but only two of the three pluralize by suffix, and `person` is the one this panel
	// is used on most.
	const nounPlural = $derived(entityType === 'person' ? 'people' : `${entityType}s`);

	// A skipped name carries no provenance of its own — identity_review_queue records the
	// pair and the name, not which provider proposed it — so the line attributes it to the
	// provider on this entity's own chips, and only when there is exactly one. With zero or
	// several, it falls back to wording that names no provider rather than guessing.
	const skippedProvider = $derived.by(() => {
		const sources = [...new Set(aliases.map((a) => a.source).filter(Boolean))];
		return sources.length === 1 ? (sources[0] as string) : '';
	});

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
			Searching any of these finds this {noun}, and future scans match them too.
		</p>

		<div class="flex flex-wrap gap-2" aria-live="polite">
			{#each aliases as a (a.id)}
				<span
					class="inline-flex items-center gap-1 rounded-full bg-surface-2 px-2.5 py-0.5 text-sm text-ink"
				>
					{a.alias}
					{#if a.source}
						<!-- Provenance, not a lesser tier: a provider-supplied name is as real an
						     alias as a typed one, and this chip behaves identically. Deliberately a
						     plain span and NOT ProvenanceBadge — that renders a brand icon standing
						     for "which source won this field", which has no meaning for a value with
						     exactly one origin and no competing candidates. -->
						<span class="rounded-full border border-rule px-1.5 py-px text-[10px] uppercase text-muted">
							{a.source}
						</span>
					{/if}
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

		{#if isOwner && skippedAliases.length}
			<!-- Collision review line (F58, ADR-088 D5): a provider name that already belongs
			     to another entity is skipped, never merged in silently. Square corners — the
			     theming rule forbids rounding a single-sided border. -->
			<div class="border-l-[3px] border-accent bg-surface-2 p-3" aria-live="polite">
				<p class="text-sm text-ink">
					{#if skippedAliases.length === 1}
						1 name{#if skippedProvider}&nbsp;from <span class="uppercase">{skippedProvider}</span
							>{/if} was skipped — <span class="font-semibold">{skippedAliases[0].alias}</span>
						already belongs to another {noun}.
					{:else}
						{skippedAliases.length} names{#if skippedProvider}&nbsp;from <span class="uppercase"
								>{skippedProvider}</span
							>{/if} were skipped because they belong to other {nounPlural}.
					{/if}
					<a href="/owner/duplicates" class="ml-1 text-accent hover:underline">Review</a>
				</p>
			</div>
		{/if}

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
