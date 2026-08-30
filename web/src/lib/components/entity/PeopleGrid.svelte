<script lang="ts">
	// Reusable People grid section, extracted from the Media detail page's People
	// section so the Film detail page's Cast section (a read-only, derived union of
	// its scenes' people) can share the same tile shape instead of hand-rolling its
	// own copy. Owner editing (per-tile remove badge + PersonPicker add tile) only
	// renders when the caller passes attach/detach — Cast has neither, since it
	// isn't an editable relationship (the underlying people come from the film's
	// attached videos, not a direct link).
	//
	// Poster tiles are sized to match the Films grid's tile size (grid-cols-3/4/6)
	// rather than the previous denser grid-cols-4/5/8 — a deliberate size increase,
	// not a byproduct of the extraction.
	import { personKey } from '$lib/format';
	import type { Person, ResolvedPerson, VideoCollisionRef } from '$lib/types';
	import PersonPoster from '$lib/components/person/PersonPoster.svelte';
	import PersonPicker from './PersonPicker.svelte';
	import PosterTile from './PosterTile.svelte';

	let {
		title,
		people,
		isOwner = false,
		attach,
		detach,
		busyKey = $bindable(null),
		onRemove,
		removeError = ''
	}: {
		title: string;
		people: Person[];
		isOwner?: boolean;
		attach?: (name: string, role: 'actor' | 'director') => Promise<{ ok: true } | { conflict: VideoCollisionRef }>;
		detach?: (name: string, role: 'actor' | 'director') => Promise<{ ok: true } | { conflict: VideoCollisionRef }>;
		// Shared with the caller's own grid-remove control — both mutate the same
		// underlying link, so they must share one busy gate (HOLODEX-272).
		busyKey?: string | null;
		onRemove?: (p: Person) => void;
		removeError?: string;
	} = $props();

	const editable = $derived(isOwner && !!attach && !!detach);
	// Editable people always carry a role (attach/detach require one) — cast once here
	// rather than at each PersonPicker call site.
	const editablePeople = $derived(people as ResolvedPerson[]);
	// Owned here (not inside PersonPicker) so it survives the empty↔populated branch
	// swap below: the two PersonPicker mounts are different template positions, so
	// committing the first attach unmounts one instance and mounts the other — without
	// a shared bindable, the popover would reset to closed right after the first add.
	let personPickerOpen = $state(false);
</script>

{#snippet personPicker(hasPeople: boolean)}
	<PersonPicker
		people={editablePeople}
		{hasPeople}
		{isOwner}
		attach={attach!}
		detach={detach!}
		bind:busyKey
		bind:open={personPickerOpen}
	/>
{/snippet}

{#if editable || people.length}
	<section class="space-y-1.5">
		<h2 class="text-xs uppercase tracking-wide text-muted">{title}</h2>
		{#if people.length}
			<!-- F25: 2:3 poster cards (placeholder when a person has no poster). Composite
			     each-key (id + role) since a dual-role attachment on a video is two entries
			     sharing the same id (ADR-072); Cast's people carry no role, so it falls
			     back to id alone. -->
			<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-6">
				{#each people as p (personKey(p))}
					{@const key = personKey(p)}
					<PosterTile
						href={`/people/${p.id}`}
						label={p.name}
						busy={busyKey === key}
						onRemove={editable ? () => onRemove?.(p) : undefined}
					>
						{#snippet image()}
							<div class="rounded-theme transition group-hover:opacity-90">
								<PersonPoster personId={p.id} name={p.name} />
							</div>
						{/snippet}
					</PosterTile>
				{/each}
				{#if editable}
					<li>
						<PersonPicker
							people={editablePeople}
							hasPeople={true}
							{isOwner}
							attach={attach!}
							detach={detach!}
							bind:busyKey
							bind:open={personPickerOpen}
						/>
					</li>
				{/if}
			</ul>
		{:else if editable}
			<!-- No poster-tile box when the grid is empty (HOLODEX-289-style, matching
			     Studio/Tags' "+ Add X" text CTA) — a lone dashed square with nothing beside
			     it read as less fluid than an inline text button. -->
			<PersonPicker
				people={editablePeople}
				hasPeople={false}
				{isOwner}
				attach={attach!}
				detach={detach!}
				bind:busyKey
				bind:open={personPickerOpen}
			/>
		{/if}
		{#if removeError}
			<p class="text-sm text-warn" aria-live="polite">{removeError}</p>
		{/if}
	</section>
{/if}
