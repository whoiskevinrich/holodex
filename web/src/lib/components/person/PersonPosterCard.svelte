<script lang="ts">
	// Poster-grid card for the People index (F55) — a name/count block over the shared
	// 2:3 .portrait-frame well. Border/focus-ring are Tailwind utilities (matching
	// people/+page.svelte's own conditional-border idiom and the app's existing
	// focus-visible:ring-* convention); the hover-lift transform/shadow lives in
	// app.css's .poster-card-frame block, scoped to this card so PersonAvatar/PersonBanner/
	// the person-detail PersonPoster are unaffected.
	import type { Person } from '$lib/types';
	import PersonImageFrame from './PersonImageFrame.svelte';

	let { person, eager = false }: { person: Person; eager?: boolean } = $props();

	// poster_version, NOT headshot_version (P0-6) — the two image roles are independently
	// fillable; a person can have a headshot with no poster, or vice versa.
	const hasPoster = $derived((person.poster_version ?? 0) > 0);
</script>

<a href={`/people/${person.id}`} class="poster-card group relative block">
	<PersonImageFrame
		personId={person.id}
		role="poster"
		name={person.name}
		version={person.poster_version}
		{eager}
		frameClass={`portrait-frame--2x3 w-full poster-card-frame ${hasPoster ? 'border-transparent' : 'border-rule'} group-focus-visible:ring-2 group-focus-visible:ring-accent`}
	/>
	<div class="space-y-0.5 pt-1.5">
		<h3 class="skin-title line-clamp-1 text-sm font-medium text-ink" title={person.name}>
			{person.name}
		</h3>
		<span class="text-xs text-muted">{person.video_count}</span>
	</div>
</a>
