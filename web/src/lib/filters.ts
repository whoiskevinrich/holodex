// Single source of truth for translating MediaFilters <-> URL query params, so
// the API client (api.ts) and the browse page's shareable-URL sync (+page.svelte)
// can never drift apart (F4.7 shareable filters).
import type { MediaFilters, Resolution, SortOrder } from './types';

// The default sort; omitted from the URL so a pristine browse view stays at `/`.
export const DEFAULT_SORT: SortOrder = 'added_desc';

// filtersToParams serializes filters to URLSearchParams. When paging is false,
// limit/offset are omitted (used for the shareable browse URL).
export function filtersToParams(f: MediaFilters, paging = true): URLSearchParams {
	const p = new URLSearchParams();
	if (f.q) p.set('q', f.q);
	for (const id of f.person ?? []) p.append('person', String(id));
	for (const id of f.tag ?? []) p.append('tag', String(id));
	if (f.duration_min) p.set('duration_min', String(f.duration_min));
	if (f.duration_max) p.set('duration_max', String(f.duration_max));
	if (f.resolution && f.resolution !== 'All') p.set('resolution', f.resolution);
	if (f.year_min) p.set('year_min', String(f.year_min));
	if (f.year_max) p.set('year_max', String(f.year_max));
	if (f.sort && f.sort !== DEFAULT_SORT) p.set('sort', f.sort);
	for (const [k, v] of Object.entries(f.mapped ?? {})) {
		if (v) p.set(k, v); // configurable mapped-field filter, keyed by canonical (F20.5)
	}
	if (paging) {
		if (f.limit) p.set('limit', String(f.limit));
		if (f.offset) p.set('offset', String(f.offset));
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
		sort: (p.get('sort') as SortOrder) || DEFAULT_SORT
	};
}
