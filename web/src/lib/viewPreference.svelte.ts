// Per-page persisted display-mode preference for /people (F55, RD1). Same
// validated-read/fallback-on-corrupt shape as sortPreference.svelte.ts's readSort/writeSort,
// but kept as its own module rather than folded in there: the allowed-value set here is a
// fixed 2-literal view mode, not a per-page `allowed` sort array — same pattern, different kind
// of value, mirroring how density.svelte.ts is also its own module alongside sortPreference.

const KEY = 'holodex:view:people';
const VALUES = ['list', 'poster'] as const;
export type PersonView = (typeof VALUES)[number];

export function readView(): PersonView {
	if (typeof localStorage === 'undefined') return 'list';
	try {
		const v = localStorage.getItem(KEY);
		if (v && (VALUES as readonly string[]).includes(v)) return v as PersonView;
	} catch {
		// Malformed or unavailable storage — fall through to the default.
	}
	return 'list';
}

export function writeView(value: PersonView): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(KEY, value);
	} catch {
		// Storage full/unavailable — the preference just won't persist (non-fatal).
	}
}
