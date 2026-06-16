<script lang="ts">
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { toMessage, videoCount } from '$lib/format';
	import type { Person } from '$lib/types';
	import SortToggle from '$lib/components/SortToggle.svelte';

	let people = $state<Person[]>([]);
	let sort = $state<'name' | 'count'>('name');
	let loading = $state(true);

	// Merge selection (F23, owner-only): pick 2+ people, then choose the canonical
	// one to fold the rest into. See [[ADR-036]].
	let selecting = $state(false);
	let selectedIds = $state<number[]>([]);
	let chooseCanonical = $state(false);
	let canonicalId = $state<number | null>(null);
	let merging = $state(false);
	let mergeError = $state('');

	const isOwner = $derived(activity.isOwner);
	const selectedPeople = $derived(people.filter((p) => selectedIds.includes(p.id)));

	function reload() {
		loading = true;
		api
			.listPeople(sort)
			.then((res) => (people = res.items ?? []))
			.finally(() => (loading = false));
	}

	$effect(() => {
		void sort; // re-run on sort change
		reload();
	});

	function toggle(id: number) {
		selectedIds = selectedIds.includes(id)
			? selectedIds.filter((x) => x !== id)
			: [...selectedIds, id];
	}

	function cancelSelect() {
		selecting = false;
		selectedIds = [];
		chooseCanonical = false;
		canonicalId = null;
		mergeError = '';
	}

	function openChoose() {
		canonicalId = selectedIds[0] ?? null;
		mergeError = '';
		chooseCanonical = true;
	}

	async function confirmMerge() {
		if (!canonicalId || merging) return;
		merging = true;
		mergeError = '';
		try {
			// Fold every other selected person into the chosen canonical one.
			for (const fromId of selectedIds.filter((id) => id !== canonicalId)) {
				await api.mergePersons(canonicalId, fromId);
			}
			cancelSelect();
			reload();
		} catch (e) {
			mergeError = toMessage(e);
		} finally {
			merging = false;
		}
	}
</script>

<section class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h1 class="skin-title text-2xl font-semibold text-ink">People</h1>
		<div class="flex items-center gap-2">
			{#if isOwner}
				{#if selecting}
					<button
						onclick={openChoose}
						disabled={selectedIds.length < 2}
						class="rounded-theme bg-accent px-3 py-1 text-sm font-semibold text-accent-ink disabled:opacity-60"
					>
						Merge {selectedIds.length || ''} selected
					</button>
					<button
						onclick={cancelSelect}
						class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
					>
						Cancel
					</button>
				{:else}
					<button
						onclick={() => (selecting = true)}
						class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
					>
						Merge people…
					</button>
				{/if}
			{/if}
			<SortToggle bind:sort />
		</div>
	</div>

	{#if selecting}
		<p class="text-sm text-muted">Select two or more people, then choose which name to keep.</p>
	{/if}

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if people.length === 0}
		<p class="py-16 text-center text-sm text-muted">No people indexed yet.</p>
	{:else}
		<ul class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each people as p (p.id)}
				<li>
					{#if selecting}
						<label
							class="flex cursor-pointer items-center gap-3 rounded-theme border bg-surface px-4 py-2.5 text-ink {selectedIds.includes(
								p.id
							)
								? 'border-accent'
								: 'border-rule hover:border-accent'}"
						>
							<input
								type="checkbox"
								class="accent-accent"
								checked={selectedIds.includes(p.id)}
								onchange={() => toggle(p.id)}
							/>
							<span class="flex-1 truncate">{p.name}</span>
							<span class="text-xs text-muted">{p.video_count}</span>
						</label>
					{:else}
						<a
							href={`/people/${p.id}`}
							class="flex items-center justify-between rounded-theme border border-rule bg-surface px-4 py-2.5 text-ink hover:border-accent"
						>
							<span class="truncate">{p.name}</span>
							<span class="text-xs text-muted">{p.video_count}</span>
						</a>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>

{#if chooseCanonical}
	<div
		class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
		role="presentation"
		onclick={(e) => {
			if (e.target === e.currentTarget && !merging) chooseCanonical = false;
		}}
	>
		<div
			class="flex w-full max-w-lg flex-col gap-3 rounded-theme border border-rule bg-surface p-4 shadow-xl"
			role="dialog"
			aria-modal="true"
			aria-labelledby="merge-choose-title"
		>
			<h2 id="merge-choose-title" class="skin-title text-lg font-semibold text-ink">
				Keep which name?
			</h2>
			<p class="text-xs text-muted">
				The chosen name stays; the others become its aliases and their videos move under it. Confirm
				these are the same person — this can’t be auto-undone.
			</p>
			<fieldset class="space-y-1">
				{#each selectedPeople as p (p.id)}
					<label class="flex cursor-pointer items-center gap-3 rounded-theme px-2 py-1.5 text-ink hover:bg-surface-2">
						<input type="radio" name="canonical" class="accent-accent" value={p.id} checked={canonicalId === p.id} onchange={() => (canonicalId = p.id)} />
						<span class="flex-1 truncate">{p.name}</span>
						<span class="text-xs text-muted">{videoCount(p.video_count ?? 0)}</span>
					</label>
				{/each}
			</fieldset>
			{#if mergeError}
				<p class="text-sm text-warn">{mergeError}</p>
			{/if}
			<div class="flex flex-wrap items-center justify-end gap-2">
				<button
					onclick={() => (chooseCanonical = false)}
					disabled={merging}
					class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
				>
					Back
				</button>
				<button
					onclick={confirmMerge}
					disabled={merging || !canonicalId}
					class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
				>
					{merging ? 'Merging…' : 'Merge'}
				</button>
			</div>
		</div>
	</div>
{/if}
