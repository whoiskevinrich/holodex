// Per-page persisted display-mode preference for the list pages (F55 RD1, F82 P0-2). Same
// validated-read/fallback-on-corrupt shape as sortPreference.svelte.ts's readSort/writeSort,
// but kept as its own module rather than folded in there: the allowed values here are a
// page's fixed view modes, not a per-page `allowed` sort array — same pattern, different kind
// of value, mirroring how density.svelte.ts is also its own module alongside sortPreference.
//
// The view is never in the URL, and only an explicit choice writes it (F73 R7): call
// writeView from the toggle's change handler, never from an effect.

// Each page's views in toggle order; the first is the page's default.
export const PAGE_VIEWS = {
	media: ['grid', 'table'],
	people: ['list', 'poster', 'table'],
	tags: ['chips', 'posters', 'table']
} as const;

export type ViewPage = keyof typeof PAGE_VIEWS;
export type PageView<P extends ViewPage> = (typeof PAGE_VIEWS)[P][number];
export type PersonView = PageView<'people'>;
// Every view any page offers — ViewToggle keys its labels and icons by this, so a view added
// above without a label fails the type check rather than rendering a blank cell.
export type AnyView = PageView<ViewPage>;

const key = (page: ViewPage) => `holodex:view:${page}`;

export function readView<P extends ViewPage>(page: P): PageView<P> {
	const values: readonly string[] = PAGE_VIEWS[page];
	const fallback = values[0] as PageView<P>;
	if (typeof localStorage === 'undefined') return fallback;
	try {
		const v = localStorage.getItem(key(page));
		if (v && values.includes(v)) return v as PageView<P>;
	} catch {
		// Malformed or unavailable storage — fall through to the default.
	}
	return fallback;
}

export function writeView<P extends ViewPage>(page: P, value: PageView<P>): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(key(page), value);
	} catch {
		// Storage full/unavailable — the preference just won't persist (non-fatal).
	}
}
