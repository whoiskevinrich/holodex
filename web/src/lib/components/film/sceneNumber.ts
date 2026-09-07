// Shared scene-number field validation (HOLODEX-326): FilmAttachDialog's attach step
// and EditSceneNumberDialog's edit form both take one nullable positive-integer field
// with identical rules -- one place to keep them in sync.

// bind:value on <input type="number"> coerces to a Number (or '' when cleared), so a
// caller's $state field is effectively `number | ''` at runtime even though svelte-check
// infers a wider `string | number` -- never call .trim() on it.
export function parseSceneNumberInput(
	value: string | number
): { value: number | null } | { error: string } {
	if (value === '') return { value: null };
	const n = Number(value);
	if (!Number.isInteger(n) || n <= 0) {
		return { error: 'Scene number must be a positive whole number, or left blank.' };
	}
	return { value: n };
}

// sceneBadgeLabel is the scene badge's vocabulary, shared by the film detail page's
// Scenes grid (VideoCard) and the media detail page's Films chips (HOLODEX-328). The
// design requires the two surfaces to read identically, so the three cases live in one
// place rather than as parallel ternaries: a numbered scene, an unnumbered one, and a
// full-film link, which has no scene number to show at all.
export function sceneBadgeLabel(sceneNumber: number | null, isFullFilm = false): string {
	if (isFullFilm) return 'Full';
	return sceneNumber === null ? '\u2014' : `#${sceneNumber}`;
}
