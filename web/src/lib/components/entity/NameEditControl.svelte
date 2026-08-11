<script lang="ts">
	// Docked-pencil rename control (HOLODEX-269) — replaces Person's SourceSelect/onadopt
	// intercept, Studio's AliasPanel-embedded rename form, and Tag's list-page-only rename with
	// one shared mechanism, and adds a rename affordance to Video Title where none existed. At
	// rest, a non-owner and an owner-not-hovering see the same heading text; the pencil is present
	// in the DOM for an owner (keyboard-reachable) but invisible until hover/focus
	// (.name-edit-row/.name-edit-pencil in app.css, mirroring .curation-actions). On a name
	// collision, onCommit resolves {conflict} and the caller-supplied `verdict` snippet (typically
	// MergeOfferCard) renders inline in place of the edit form — no modal, no navigation.
	import type { Snippet } from 'svelte';
	import type { EntityRef } from '$lib/types';

	let {
		name,
		isOwner,
		onCommit,
		label,
		headingClass,
		hint,
		trailing,
		verdict
	}: {
		name: string;
		isOwner: boolean;
		onCommit: (value: string) => Promise<{ ok: true } | { conflict: EntityRef }>;
		label: string;
		headingClass: string;
		// Optional note shown under the open edit form (e.g. "kept as an alias") — entities on
		// the identity spine (person/studio/tag) pass one, Video Title (no alias mechanism) omits it.
		hint?: string;
		// Optional content rendered beside the heading, inside the same hover-reveal row (e.g.
		// Person's nationality flags) — kept out of the edit form, which takes over the row.
		trailing?: Snippet;
		verdict?: Snippet<[EntityRef, () => void]>;
	} = $props();

	let editing = $state(false);
	let busy = $state(false);
	let error = $state('');
	let value = $state('');
	let conflict = $state<EntityRef | null>(null);
	let input = $state<HTMLInputElement | null>(null);
	let pencil = $state<HTMLButtonElement | null>(null);

	function focusPencil() {
		Promise.resolve().then(() => pencil?.focus());
	}

	function startEdit() {
		value = name;
		error = '';
		editing = true;
		Promise.resolve().then(() => input?.select());
	}

	function closeEdit() {
		editing = false;
		value = '';
		error = '';
	}

	function cancelEdit() {
		closeEdit();
		focusPencil();
	}

	async function commit(e: SubmitEvent) {
		e.preventDefault();
		const next = value.trim();
		if (!next || busy) return;
		if (next === name) {
			closeEdit();
			return;
		}
		busy = true;
		error = '';
		try {
			const res = await onCommit(next);
			if ('conflict' in res) {
				conflict = res.conflict;
				closeEdit();
				return;
			}
			closeEdit();
			focusPencil();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Rename failed.';
		} finally {
			busy = false;
		}
	}

	function resolveConflict() {
		conflict = null;
		focusPencil();
	}
</script>

{#if editing}
	<form onsubmit={commit} class="flex flex-wrap items-center gap-2">
		<input
			bind:this={input}
			bind:value
			type="text"
			aria-label={`Rename this ${label}`}
			aria-describedby={error ? 'name-edit-error' : undefined}
			class="min-w-0 flex-1 rounded-theme border border-rule bg-surface px-3 py-1.5 text-lg text-ink focus:border-accent focus:outline-none"
		/>
		<button type="submit" disabled={busy} class="btn-accent px-3 py-1.5 text-sm">
			{busy ? 'Saving…' : 'Save'}
		</button>
		<button type="button" onclick={cancelEdit} disabled={busy} class="btn-ghost px-3 py-1.5 text-sm">
			Cancel
		</button>
		{#if hint}
			<p class="w-full text-xs text-muted">{hint}</p>
		{/if}
		{#if error}
			<p id="name-edit-error" class="w-full text-sm text-warn">{error}</p>
		{/if}
	</form>
{:else}
	<div class="name-edit-row flex items-center gap-2">
		<h1 class={headingClass}>{name}</h1>
		{#if trailing}{@render trailing()}{/if}
		{#if isOwner}
			<button
				bind:this={pencil}
				type="button"
				aria-label={`Rename this ${label}`}
				onclick={startEdit}
				class="name-edit-pencil rounded-theme border border-rule p-1.5 text-muted hover:border-accent hover:text-ink"
			>
				<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z"
					/>
				</svg>
			</button>
		{/if}
	</div>
{/if}

{#if conflict && verdict}
	{@render verdict(conflict, resolveConflict)}
{/if}
