// "Searched …" caption state for the Enrich picker (ADR-095 D6, F54 FR9,
// HOLODEX-369). Pure: EnrichPicker.svelte hands it the last response's
// `searched[]` and its own hide conditions and renders whatever comes back.
// Kept out of the component so the state table in the design handoff is
// unit-testable without a DOM (this repo has no component-test harness).

export interface SearchedCaption {
	/** searched[0] — always shown inline, in ink. */
	first: string;
	/** Entries beyond the first; > 0 means the "+N more" toggle renders. */
	more: number;
	/** Every entry in issue order (the first repeats — the list is the record). */
	entries: string[];
}

/** Why the caption is hidden, mirroring the picker's own status-line branches. */
export interface CaptionHide {
	loading: boolean;
	error: string;
	/** The current box text — below two characters the picker never searched. */
	query: string;
}

/**
 * The caption to render for the last `/resolve` response, or null for
 * "render nothing" — no `searched` key or an empty list (never an empty slot),
 * while a new search is in flight, on an error, or under two characters.
 */
export function searchedCaption(
	searched: readonly string[] | undefined,
	hide: CaptionHide
): SearchedCaption | null {
	if (!searched || searched.length === 0) return null;
	if (hide.loading || hide.error || hide.query.trim().length < 2) return null;
	return { first: searched[0], more: searched.length - 1, entries: [...searched] };
}

/** Toggle copy: `+N more` collapsed, `show less` expanded. */
export function moreLabel(more: number, expanded: boolean): string {
	return expanded ? 'show less' : `+${more} more`;
}
