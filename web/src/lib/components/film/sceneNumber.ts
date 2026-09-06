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
