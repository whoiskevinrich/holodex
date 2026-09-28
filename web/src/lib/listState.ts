// The list-state contract every list page shares (ADR-114, spec F73 R7/R8).
//
// Two kinds of state, two homes (D1):
//  - the *preference* — the sort — lives in localStorage (`holodex:sort:<page>`, via
//    sortPreference) AND in the URL `sort` param; a valid URL value wins (D2).
//  - the *query* — filters, scope chips, a tab — lives in the URL only. Nothing here ever
//    reads the legacy SP5 `holodex:filters:*` keys (HOLODEX-474 deletes them).
//
// Parsing is pure and never writes storage: arriving by a shared link, Back or a redirect
// must not overwrite the visitor's saved preference (D3). Only `chooseSort`, called from a
// control's change handler, writes. Every URL change goes through `commit` — SvelteKit's
// `replaceState`, never `pushState` and never raw `history.replaceState`, which wipes the
// router state and breaks Back.
import { goto, replaceState } from '$app/navigation';
import { readSort, writeSort } from './sortPreference.svelte';
import { DEFAULT_SORT, MEDIA_SORTS, filtersToParams, paramsToFilters } from './filters';
import type { MediaFilters, PeopleTagSort, SortOrder } from './types';

export interface SortOption<S extends string> {
	value: S;
	label: string;
	ownerOnly?: boolean;
}

// One page's contract: which sorts it offers and how its query round-trips the URL.
// `toParams` omits defaults so a pristine view keeps a bare path.
export interface ListSchema<S extends string, Q> {
	page: string;
	sorts: readonly SortOption<S>[];
	defaultSort: S;
	parseQuery(p: URLSearchParams): Q;
	toParams(q: Q): URLSearchParams;
}

// A page's whole list state. One `sort`, never a second sort-shaped field: a second one
// is how People/Studios came to show two active sorts at once (HOLODEX-473).
export interface ListState<S extends string, Q> {
	sort: S;
	query: Q;
}

export function sortValues<S extends string>(schema: ListSchema<S, unknown>, owner: boolean): S[] {
	return schema.sorts.filter((s) => owner || !s.ownerOnly).map((s) => s.value);
}

// parseList derives the state from the URL (+ the saved sort) and reports whether the URL
// should be rewritten to show the resolved sort. Pure apart from reading localStorage.
//
// `syncUrl` is true when the URL's sort doesn't show what's on screen: absent while a
// non-default sort was restored, or not a sort this page has at all. An owner-only sort
// seen while `owner` is false is left in the URL: capabilities load asynchronously, so an
// owner is briefly indistinguishable from a visitor, and stripping the param then would
// lose the owner's sort before their capabilities arrive.
export function parseList<S extends string, Q>(
	schema: ListSchema<S, Q>,
	params: URLSearchParams,
	owner: boolean
): { state: ListState<S, Q>; syncUrl: boolean } {
	const allowed = sortValues(schema, owner);
	const fromUrl = params.get('sort');
	const urlKnown = fromUrl != null && schema.sorts.some((s) => s.value === fromUrl);
	const sort =
		fromUrl != null && (allowed as string[]).includes(fromUrl)
			? (fromUrl as S)
			: readSort(schema.page, allowed, schema.defaultSort);
	return {
		state: { sort, query: schema.parseQuery(params) },
		syncUrl: fromUrl == null ? sort !== schema.defaultSort : !urlKnown
	};
}

// serialize is canonical (D5): default sort omitted, keys in sorted order (repeated keys
// keep their relative order), so equal states always produce the same string.
export function serialize<S extends string, Q>(schema: ListSchema<S, Q>, state: ListState<S, Q>): URLSearchParams {
	const p = schema.toParams(state.query);
	p.delete('sort');
	if (state.sort !== schema.defaultSort) p.set('sort', state.sort);
	p.sort();
	return p;
}

// The snapshot key ADR-032's browseCache / listScroll restore against (D5).
export function stateKey<S extends string, Q>(schema: ListSchema<S, Q>, state: ListState<S, Q>): string {
	return serialize(schema, state).toString();
}

// The one storage write: call it from the sort control's change handler, never from an
// effect watching derived state (D3).
export function chooseSort<S extends string>(schema: ListSchema<S, unknown>, sort: S): void {
	writeSort(schema.page, sort);
}

// On a hard load, SvelteKit's replaceState throws until the router has finished its own
// async initialise; an arrival-time sync can land in that window, so it retries once on
// the next macrotask (the deferral the Media page already relied on).
export function commit(path: string, params: URLSearchParams): void {
	const qs = params.toString();
	const url = qs ? `${path}?${qs}` : path;
	try {
		replaceState(url, {});
	} catch {
		setTimeout(() => replaceState(url, {}), 0);
	}
}

// Leaving a page whose entity was just removed (D4, generalising HOLODEX-41): back to
// wherever it was opened from when that was in-app, else the bare list.
export function exitAfterRemoval(cameFromInApp: boolean, listHref: string): void {
	if (cameFromInApp) history.back();
	else void goto(listHref);
}

// ---- Page schemas --------------------------------------------------------------------

// Labels are "Field — direction" and at most 20 characters: a native <select> is as wide
// as its widest option, and 20 is what fits the 375px toolbar in the widest skin
// (design handoff, Responsive).
const COMPLETENESS_SORTS = [
	{ value: 'completeness_desc', label: 'Completeness — most', ownerOnly: true },
	{ value: 'completeness_asc', label: 'Completeness — least', ownerOnly: true }
] as const;

export type EntitySort = PeopleTagSort | 'completeness_asc' | 'completeness_desc';
export type FilmSort = 'name' | 'random';
export type TagType = 'all' | 'tags' | 'categories';

export const ENTITY_SORTS: readonly SortOption<EntitySort>[] = [
	{ value: 'name', label: 'Name — A→Z' },
	{ value: 'count', label: 'Videos — most' },
	{ value: 'random', label: 'Random' },
	...COMPLETENESS_SORTS
];
// Tags carry no completeness score, so no Owner entries.
export const TAG_SORTS: readonly SortOption<PeopleTagSort>[] = ENTITY_SORTS.filter(
	(s): s is SortOption<PeopleTagSort> => !s.ownerOnly
);
export const FILM_SORTS: readonly SortOption<FilmSort>[] = [
	{ value: 'name', label: 'Name — A→Z' },
	{ value: 'random', label: 'Random' }
];

type NoQuery = Record<string, never>;
const noQuery = {
	parseQuery: (): NoQuery => ({}),
	toParams: () => new URLSearchParams()
};

export const peopleSchema: ListSchema<EntitySort, NoQuery> = {
	page: 'people',
	sorts: ENTITY_SORTS,
	defaultSort: 'name',
	...noQuery
};
export const studiosSchema: ListSchema<EntitySort, NoQuery> = {
	page: 'studios',
	sorts: ENTITY_SORTS,
	defaultSort: 'name',
	...noQuery
};
export const filmsSchema: ListSchema<FilmSort, NoQuery> = {
	page: 'films',
	sorts: FILM_SORTS,
	defaultSort: 'name',
	...noQuery
};

const TAG_TYPES: readonly TagType[] = ['all', 'tags', 'categories'];
export const tagsSchema: ListSchema<PeopleTagSort, { type: TagType }> = {
	page: 'tags',
	sorts: TAG_SORTS,
	defaultSort: 'name',
	parseQuery: (p) => {
		const t = p.get('type');
		return { type: TAG_TYPES.includes(t as TagType) ? (t as TagType) : 'all' };
	},
	toParams: (q) => new URLSearchParams(q.type === 'all' ? '' : { type: q.type })
};

// Media's query is its existing filter codec (filters.ts) minus the sort, which this
// module owns. Mapped-facet keys (?genre=…) aren't known until the facet list loads, so
// every param the codec doesn't own passes through as `mapped`; dropping them here would
// strip a facet from the URL on the first sync. `missing_facet` is retired from list
// pages (F73 R6): a stale one in a bookmark is dropped, not left filtering with no chip.
const MEDIA_OWN_PARAMS = new Set([
	'q', 'person', 'tag', 'studio_id', 'category_id', 'duration_min', 'duration_max',
	'resolution', 'year_min', 'year_max', 'sort', 'missing_facet', 'limit', 'offset', 'seed'
]);
export type MediaQuery = Omit<MediaFilters, 'sort' | 'limit' | 'offset' | 'seed' | 'missing_facet'>;
export const mediaSchema: ListSchema<SortOrder, MediaQuery> = {
	page: 'media',
	sorts: MEDIA_SORTS,
	defaultSort: DEFAULT_SORT,
	parseQuery: (p) => {
		const { sort: _sort, missing_facet: _missing, ...query } = paramsToFilters(p);
		const mapped: Record<string, string> = {};
		for (const [k, v] of p) if (v && !MEDIA_OWN_PARAMS.has(k)) mapped[k] = v;
		return Object.keys(mapped).length ? { ...query, mapped } : query;
	},
	toParams: (q) => filtersToParams(q, false)
};
