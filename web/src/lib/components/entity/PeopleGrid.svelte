<script lang="ts">
	// Reusable People grid section, extracted from the Media detail page's People
	// section so the Film detail page's Cast section (a read-only, derived union of
	// its scenes' people) can share the same tile shape instead of hand-rolling its
	// own copy. Owner editing (per-tile remove badge + PersonPicker add tile) only
	// renders when the caller passes attach/detach — Cast has neither, since it
	// isn't an editable relationship (the underlying people come from the film's
	// attached videos, not a direct link).
	//
	// Poster tiles are a FIXED w-20, not a responsive grid (HOLODEX-328): the media
	// detail page sets this section beside the Films chips, and a responsive grid
	// column can't stay the same width as Films' fixed tile across breakpoints. The
	// two poster rows must line up, so both are flex-wrap rows of the same tile. The
	// film page's Cast section rides the same change — accepted deliberately, since
	// two surfaces drawing people posters at two different sizes is the inconsistency
	// one layer up. If a caller ever needs a different size, add a `tileWidth` prop
	// rather than forking the component.
	//
	// An EMPTY editable grid renders the bare "+ Add person" CTA with NO section and
	// NO heading (HOLODEX-328): a heading over nothing reads as a section that failed
	// to load, and the media detail page positions that CTA itself.
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

{#if people.length}
	<section class="space-y-1.5">
		<h2 class="text-xs uppercase tracking-wide text-muted">{title}</h2>
		<!-- F25: 2:3 poster cards (placeholder when a person has no poster). Composite
		     each-key (id + role) since a dual-role attachment on a video is two entries
		     sharing the same id (ADR-072); Cast's people carry no role, so it falls
		     back to id alone. -->
		<ul class="flex flex-wrap gap-3">
			{#each people as p (personKey(p))}
				<PosterTile
					href={`/people/${p.id}`}
					name={p.display_name ?? p.name}
					personId={p.id}
					onRemove={editable ? () => onRemove?.(p) : undefined}
					busy={busyKey === personKey(p)}
				>
					{#snippet poster()}
						<div class="rounded-theme transition group-hover:opacity-90">
							<!-- poster_version is the ?v= cache-buster: the image route is served
							     immutable, so a versionless URL would pin the first poster forever. -->
							<PersonPoster personId={p.id} name={p.display_name ?? p.name} version={p.poster_version} />
						</div>
					{/snippet}
				</PosterTile>
			{/each}
			{#if editable}
				<li class="w-20 shrink-0">{@render personPicker(true)}</li>
			{/if}
		</ul>
		{#if removeError}
			<p class="text-sm text-warn" aria-live="polite">{removeError}</p>
		{/if}
	</section>
{:else if editable}
	<!-- No poster-tile box when the grid is empty (HOLODEX-289-style, matching
	     Studio/Tags' "+ Add X" text CTA) — a lone dashed square with nothing beside it
	     read as less fluid than an inline text button. No <section>/<h2> either
	     (HOLODEX-328): an empty section's heading labels nothing, and the caller places
	     this CTA in its own layout flow. removeError isn't rendered here because there
	     are no tiles to remove. -->
	{@render personPicker(false)}
{/if}
