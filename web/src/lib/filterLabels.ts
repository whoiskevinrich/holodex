import { api } from '$lib/api';
import type { MediaQuery } from '$lib/listState';

// The human words for an active /media filter: browse's removable chips (F73 R3/R5) and
// a smart playlist's filter summary (F75 handoff §4) read the same labels from here, so
// "Tag: noir" can't say one thing in the toolbar and another under a playlist's title.

// Entity scope (?person/tag/studio_id/category_id) carries ids, so its names are looked up.
const SCOPES = [
	{ key: 'person', label: 'Person', load: (id: number) => api.getPerson(id).then((r) => r.person.display_name ?? r.person.name) },
	{ key: 'studio_id', label: 'Studio', load: (id: number) => api.getStudio(id).then((r) => r.studio.name) },
	{ key: 'tag', label: 'Tag', load: (id: number) => api.getTag(id).then((r) => r.tag.name) },
	{ key: 'category', label: 'Category', load: (id: number) => api.getCategory(id).then((r) => r.category.name) }
] as const;

const scopeIds = (query: MediaQuery, key: (typeof SCOPES)[number]['key']) =>
	(query[key] as number[] | undefined) ?? [];

/** Looks up every scope name `query` names that `names` (a `$state` record keyed
 *  `key:id`) doesn't hold yet: '…' while loading, 'unknown' when the lookup fails. */
export function loadScopeNames(query: MediaQuery, names: Record<string, string>): void {
	for (const s of SCOPES) {
		for (const id of scopeIds(query, s.key)) {
			const k = `${s.key}:${id}`;
			if (k in names) continue;
			names[k] = '…';
			s.load(id)
				.then((name) => (names[k] = name))
				.catch(() => (names[k] = 'unknown'));
		}
	}
}

export interface FilterLabel {
	/** Stable identity for a keyed each — labels can repeat ("Person: …" while loading). */
	id: string;
	label: string;
	kind: 'filter' | 'scope';
	/** The query patch that drops this filter. */
	clear: Partial<MediaQuery>;
}

const range = (a?: number, b?: number, unit = '') =>
	a && b ? `${a}–${b}${unit}` : a ? `≥ ${a}${unit}` : `≤ ${b}${unit}`;

/** Every active filter and entity scope in `query` except the title search `q`, in chip
 *  order. `facetLabel` names a mapped facet (falls back to its canonical key). */
export function filterLabels(
	query: MediaQuery,
	names: Record<string, string>,
	facetLabel: (canonical: string) => string | undefined = () => undefined
): FilterLabel[] {
	const out: FilterLabel[] = [];
	for (const s of SCOPES) {
		const ids = scopeIds(query, s.key);
		for (const id of ids) {
			out.push({
				id: `${s.key}:${id}`,
				label: `${s.label}: ${names[`${s.key}:${id}`] ?? '…'}`,
				kind: 'scope',
				clear: { [s.key]: ids.filter((x) => x !== id) }
			});
		}
	}
	if (query.resolution && query.resolution !== 'All')
		out.push({ id: 'resolution', label: query.resolution, kind: 'filter', clear: { resolution: 'All' } });
	if (query.duration_min || query.duration_max)
		out.push({
			id: 'duration',
			label: `Duration ${range(query.duration_min, query.duration_max, ' min')}`,
			kind: 'filter',
			clear: { duration_min: undefined, duration_max: undefined }
		});
	if (query.year_min || query.year_max)
		out.push({
			id: 'year',
			label: range(query.year_min, query.year_max),
			kind: 'filter',
			clear: { year_min: undefined, year_max: undefined }
		});
	for (const [canonical, value] of Object.entries(query.mapped ?? {})) {
		if (!value) continue;
		out.push({
			id: `mapped:${canonical}`,
			label: `${facetLabel(canonical) ?? canonical}: ${value}`,
			kind: 'filter',
			clear: { mapped: { ...query.mapped, [canonical]: '' } }
		});
	}
	return out;
}
