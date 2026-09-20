<script lang="ts">
	// Poster-grid card for the People index (F55) — a name/count block over the shared
	// 2:3 .portrait-frame well. Border/focus-ring are Tailwind utilities (matching
	// people/+page.svelte's own conditional-border idiom and the app's existing
	// focus-visible:ring-* convention); the hover-lift transform/shadow lives in
	// app.css's .poster-card-frame block, scoped to this card so PersonAvatar/PersonBanner/
	// the person-detail PersonPoster are unaffected.
	import type { Person } from '$lib/types';
	import PersonImageFrame from './PersonImageFrame.svelte';
	import CompletenessRing from '$lib/components/completeness/CompletenessRing.svelte';

	let { person, eager = false }: { person: Person; eager?: boolean } = $props();

	// poster_version, NOT headshot_version (P0-6) — the two image roles are independently
	// fillable; a person can have a headshot with no poster, or vice versa.
	const hasPoster = $derived((person.poster_version ?? 0) > 0);
</script>

<!-- Stretched link (F65.8): the <a> is unpositioned and its ::after covers the whole
     card, so the poster, the name AND the count line all navigate — while the
     completeness ring, a <button> since F65.8, sits as a sibling above the stretch
     (`relative z-[1]`) instead of nesting inside the link. `.poster-card` moves to the
     wrapper so the hover-lift and the `:has(a:focus-visible)` focus-lift in app.css
     still key off the whole card. -->
<div class="poster-card relative">
	<a href={`/people/${person.id}`} class="group block after:absolute after:inset-0 after:content-['']">
		<PersonImageFrame
			personId={person.id}
			role="poster"
			name={person.name}
			version={person.poster_version}
			{eager}
			frameClass={`portrait-frame--2x3 w-full poster-card-frame ${hasPoster ? 'border-transparent' : 'border-rule'} group-focus-visible:ring-2 group-focus-visible:ring-accent`}
		/>
		<h3 class="skin-title line-clamp-1 pt-1.5 text-sm font-medium text-ink" title={person.name}>
			{person.name}
		</h3>
	</a>
	<span class="flex items-center gap-1.5 pt-0.5">
		{#if person.completeness}
			<!-- Owner-only by payload (F65.4/F65.5): the row rule applied to the caption —
			     trailing, before the count, so the caption's name line never moves. -->
			<span class="relative z-[1] inline-flex">
				<CompletenessRing
					required={person.completeness.required}
					extras={person.completeness.extras}
					size="row"
					entity={{ kind: 'person', id: person.id }}
				/>
			</span>
		{/if}
		<span class="text-xs text-muted">{person.video_count}</span>
	</span>
</div>
