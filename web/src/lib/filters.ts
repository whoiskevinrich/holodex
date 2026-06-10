// Single source of truth for translating MediaFilters <-> URL query params, so
// the API client (api.ts) and the browse page's shareable-URL sync (+page.svelte)
// can never drift apart (F4.7 shareable filters).
import type { MediaFilters, Resolution } from './types';

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
	if (paging) {
		if (f.limit) p.set('limit', String(f.limit));
		if (f.offset) p.set('offset', String(f.offset));
	}
	return p;
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
		tag: p.getAll('tag').map(Number).filter(Boolean)
	};
}
