// Media detail Films + People layout rule (HOLODEX-328,
// docs/design/media-detail-films-people-handoff.md §2).
//
// Extracted from media/[id]/+page.svelte rather than left as inline $derived booleans
// because it is the one piece of that section with a real state matrix — four link
// states x owner/visitor, plus the films_enabled capability — and the repo has no
// component-test harness, only lib unit tests. A pure function is the honest way to
// pin the matrix the handoff documents.
//
// The rule: a side renders its heading and tiles only when it HAS content; an empty
// side degrades to a bare "+ Add ..." text CTA (owner only, no heading, no dashed
// box); the two stack vertically unless BOTH are populated, in which case they sit
// side by side. Two bare CTAs share one line rather than burning two rows.

export interface FilmsPeopleInput {
	/** The films_enabled capability. When false, Films is hidden for everyone. */
	filmsEnabled: boolean;
	isOwner: boolean;
	filmCount: number;
	peopleCount: number;
}

/**
 * What one side renders: its full section, the bare add CTA, or nothing at all.
 *
 * Note the template only branches on `people !== 'hidden'` — PeopleGrid decides
 * section-vs-CTA internally from the same people array. People's third value exists so
 * `row` can be computed, not because the page renders three People variants.
 */
export type SideLayout = 'section' | 'cta' | 'hidden';

export interface FilmsPeopleLayout {
	/** 'hidden' means the row contributes nothing to the page — no wrapper, no anchor. */
	row: 'hidden' | 'inline-ctas' | 'stacked' | 'side-by-side';
	films: SideLayout;
	people: SideLayout;
}

export function filmsPeopleLayout({
	filmsEnabled,
	isOwner,
	filmCount,
	peopleCount
}: FilmsPeopleInput): FilmsPeopleLayout {
	// A visitor never sees a CTA, so an empty side is simply absent for them — which is
	// what makes "neither linked, visitor" render nothing rather than an empty state.
	const films: SideLayout = !filmsEnabled ? 'hidden' : filmCount > 0 ? 'section' : isOwner ? 'cta' : 'hidden';
	const people: SideLayout = peopleCount > 0 ? 'section' : isOwner ? 'cta' : 'hidden';

	// Stated in the order the design doc's state table lists them. 'stacked' is the
	// fall-through and by far the most common outcome, so it is the default rather than
	// the tail of a ternary staircase.
	let row: FilmsPeopleLayout['row'] = 'stacked';
	if (films === 'hidden' && people === 'hidden') row = 'hidden';
	else if (films === 'section' && people === 'section') row = 'side-by-side';
	else if (films === 'cta' && people === 'cta') row = 'inline-ctas';

	return { row, films, people };
}
