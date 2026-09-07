// Track sizing for VideoGrid's `stageAligned` mode (HOLODEX-331 §9.6).
//
// Extracted from VideoGrid rather than left as inline $derived values for the same
// reason filmsPeopleLayout was: the interesting behaviour lives at card counts the dev
// fixture cannot reach (its one film has two scenes), the repo has no component-test
// harness, and the arithmetic is what decides whether the grid holds the stage or grows
// past it. A pure function is the only honest way to pin it.
//
// The rule: cards keep ONE size no matter how many there are — derived from the full
// column count — while the number of declared tracks follows the card count. Those two
// together are what let the CSS (`width: fit-content` with a `min-width` of the stage)
// hold the grid at stage width for a short row and grow it for a long one. Declaring all
// `cols` tracks would hold the grid open at full width even with two cards, because empty
// tracks still occupy their share.

export type StageGridTracks = {
	/** How many tracks to declare — never more than there are cards. */
	trackCount: number;
	/** Fixed px width per track, or 0 before the container has been measured. */
	trackPx: number;
};

export function stageGridTracks(
	cols: number,
	itemCount: number,
	availWidth: number,
	gap: number
): StageGridTracks {
	// Width comes from `cols`, not `trackCount`: a two-scene film must show the same card
	// size as a twenty-scene one, otherwise two cards would each be half the container.
	const trackPx = availWidth > 0 && cols > 0 ? Math.floor((availWidth - (cols - 1) * gap) / cols) : 0;
	return { trackCount: Math.max(0, Math.min(cols, itemCount)), trackPx };
}

/**
 * Width the grid will occupy for a given card count — the value the CSS derives via
 * `fit-content`, restated here so the stage-hold threshold is testable.
 */
export function stageGridWidth(t: StageGridTracks, gap: number): number {
	if (t.trackCount <= 0 || t.trackPx <= 0) return 0;
	return t.trackCount * t.trackPx + (t.trackCount - 1) * gap;
}
