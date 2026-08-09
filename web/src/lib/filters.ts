// Single source of truth for translating MediaFilters <-> URL query params, so
// the API client (api.ts) and the browse page's shareable-URL sync (+page.svelte)
// can never drift apart (F4.7 shareable filters).
import type { MediaFilters, Resolution, SortOrder } from './types';

// The default sort; omitted from the URL so a pristine browse view stays at `/`.
export const DEFAULT_SORT: SortOrder = 'added_desc';

// The single source of truth for the media sort options: their values, labels, and
// order (F12.1). SortDropdown renders this; SORT_ORDERS derives the value set used
// to validate a persisted preference (SP1) so a stale/unknown localStorage value
// safely falls back to DEFAULT_SORT. "random" is a seeded shuffle (ADR-045).
// completeness_asc/desc (F55.5) carry an `ownerOnly` flag — the server 401s a
// non-owner request using them, so SortDropdown filters them out unless owner=true.
export const MEDIA_SORTS: readonly { value: SortOrder; label: string; ownerOnly?: boolean }[] = [
	{ value: 'added_desc', label: 'Date added — newest' },
	{ value: 'added_asc', label: 'Date added — oldest' },
	{ value: 'title_asc', label: 'Title — A→Z' },
	{ value: 'title_desc', label: 'Title — Z→A' },
	{ value: 'duration_desc', label: 'Duration — longest' },
	{ value: 'duration_asc', label: 'Duration — shortest' },
	{ value: 'resolution_desc', label: 'Resolution — highest' },
	{ value: 'resolution_asc', label: 'Resolution — lowest' },
	{ value: 'random', label: 'Random' },
	{ value: 'completeness_desc', label: 'Completeness — most complete', ownerOnly: true },
	{ value: 'completeness_asc', label: 'Completeness — least complete', ownerOnly: true }
];

export const SORT_ORDERS: readonly SortOrder[] = MEDIA_SORTS.map((s) => s.value);

// filtersToParams serializes filters to URLSearchParams. When paging is false,
// limit/offset are omitted (used for the shareable browse URL).
export function filtersToParams(f: MediaFilters, paging = true): URLSearchParams {
	const p = new URLSearchParams();
	if (f.q) p.set('q', f.q);
	for (const id of f.person ?? []) p.append('person', String(id));
	for (const id of f.tag ?? []) p.append('tag', String(id));
	for (const id of f.studio_id ?? []) p.append('studio_id', String(id));
	for (const id of f.category ?? []) p.append('category_id', String(id));
	if (f.duration_min) p.set('duration_min', String(f.duration_min));
	if (f.duration_max) p.set('duration_max', String(f.duration_max));
	if (f.resolution && f.resolution !== 'All') p.set('resolution', f.resolution);
	if (f.year_min) p.set('year_min', String(f.year_min));
	if (f.year_max) p.set('year_max', String(f.year_max));
	if (f.sort && f.sort !== DEFAULT_SORT) p.set('sort', f.sort);
	for (const canonical of f.missing_facet ?? []) p.append('missing_facet', canonical);
	for (const [k, v] of Object.entries(f.mapped ?? {})) {
		if (v) p.set(k, v); // configurable mapped-field filter, keyed by canonical (F20.5)
	}
	if (paging) {
		if (f.limit) p.set('limit', String(f.limit));
		if (f.offset) p.set('offset', String(f.offset));
		// The shuffle seed is fetch mechanics, not a shareable filter (like
		// limit/offset): it rides the API request so paged "Load more" tiles under one
		// seed (ADR-045), but stays out of the bookmarkable URL. Only meaningful for
		// the random sort.
		if (f.sort === 'random' && f.seed != null) p.set('seed', String(f.seed));
	}
	return p;
}

// mappedFromParams extracts selected values for the given mapped-field canonical
// names from the URL (F20.5). The canonical names aren't known until the facet
// list loads, so the browse page calls this once facets arrive — keeping all
// URL→state parsing in this module rather than inside the facet component.
export function mappedFromParams(p: URLSearchParams, canonicals: string[]): Record<string, string> {
	const out: Record<string, string> = {};
	for (const c of canonicals) {
		const v = p.get(c);
		if (v) out[c] = v;
	}
	return out;
}

// paramsToFilters is the inverse parser (initializes browse state from the URL).
export function paramsToFilters(p: URLSearchParams): MediaFilters {
	const num = (k: string) => (p.get(k) ? Number(p.get(k)) : undefined);
	return {
		q: p.get('q') || undefined,
		resolution: (p.get('resolution') as Resolution) || 'All',
		duration_min: num('duration_min'),
		duration_max: num('duration_max'),
		year_min: num('year_min'),
		year_max: num('year_max'),
		person: p.getAll('person').map(Number).filter(Boolean),
		tag: p.getAll('tag').map(Number).filter(Boolean),
		studio_id: p.getAll('studio_id').map(Number).filter(Boolean),
		category: p.getAll('category_id').map(Number).filter(Boolean),
		sort: (p.get('sort') as SortOrder) || DEFAULT_SORT,
		missing_facet: p.getAll('missing_facet').filter(Boolean)
	};
}
