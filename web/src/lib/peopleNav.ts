// Pure helpers for the People-list A–Z jump-navigation (F25). Extracted from the
// page component so the edge-case-laden bucketing logic is unit-testable.

// firstLetter buckets a name under A–Z (case-folded, leading space trimmed) or '#'
// for anything that doesn't start with an ASCII letter (digits, symbols, and
// accented/non-Latin initials — matching the page's plain A–Z bar).
export function firstLetter(name: string): string {
	const c = name.trim().charAt(0).toUpperCase();
	return c >= 'A' && c <= 'Z' ? c : '#';
}

// letterAnchors maps each letter to the index of the FIRST name under it (names are
// alphabetical when the list is name-sorted), used to anchor the jump targets.
export function letterAnchors(names: string[]): Record<string, number> {
	const m: Record<string, number> = {};
	names.forEach((n, i) => {
		const L = firstLetter(n);
		if (!(L in m)) m[L] = i;
	});
	return m;
}
