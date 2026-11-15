// F90 Filters panel conditions (provider-and-field-filters-handoff.md): Provider link and
// the Fields rows. People and Studios keep them in their own URL params; Media keeps them as
// `matched:any` and `has:` conditions in ?expr=, so a chip from the panel is the one typed.
import { api } from './api';
import { printTree, type QueryNode } from './query';
export { fieldChipLabel } from './query';

export type LinkState = 'any' | 'linked' | 'unlinked';
/** One Fields row: the field's key and whether it must have a value (else be missing). */
export interface FieldCond {
	key: string;
	has: boolean;
}
/** A list's field the Fields control offers (GET /filter-fields). */
export interface FilterField {
	key: string;
	label: string;
}

// One load per list per page load: the panel and the chips row both read it.
const loaded = new Map<FilterList, Promise<FilterField[]>>();
export type FilterList = 'media' | 'people' | 'studios';

/** The list's Fields choices (GET /filter-fields); [] when they can't load. Owner only. */
export function loadFilterFields(list: FilterList): Promise<FilterField[]> {
	let p = loaded.get(list);
	if (!p) {
		p = api.filterFields(list).then(
			(r) => r.fields,
			() => {
				loaded.delete(list);
				return [];
			}
		);
		loaded.set(list, p);
	}
	return p;
}

/** Drop the loaded Fields choices: a promotion just changed which fields a list offers. */
export function forgetFilterFields(): void {
	loaded.clear();
}

/** The query's top-level conditions with their AND-level index; an OR or deeper tree has none. */
function topLevel(tree: QueryNode): QueryNode[] {
	return tree.kind === 'and' ? (tree.items ?? []) : [tree];
}

/** A plain condition or a NOT over one, as field, value and sign; anything else is null. */
function leafOf(n: QueryNode): { field?: string; value?: string; negated: boolean } | null {
	if (n.kind === 'cond') return { field: n.field, value: n.value, negated: false };
	const x = n.items?.[0];
	if (n.kind === 'not' && x?.kind === 'cond') return { field: x.field, value: x.value, negated: true };
	return null;
}

const isLink = (l: { field?: string; value?: string }) => l.field === 'matched' && l.value === 'any';
const isField = (key: string) => (l: { field?: string; value?: string }) => l.field === 'has' && l.value === key;

/** What Media's panel shows for a parsed query: only top-level `matched:any` and `has:`
 * conditions. An OR, `matched:provider-1` or a `has` inside a group draws nothing and stays
 * as typed (handoff "States and interactions"). */
export function panelConds(tree: QueryNode | null | undefined): { link: LinkState; fields: FieldCond[] } {
	let link: LinkState = 'any';
	const fields: FieldCond[] = [];
	for (const n of tree ? topLevel(tree) : []) {
		const l = leafOf(n);
		if (!l) continue;
		if (isLink(l) && link === 'any') link = l.negated ? 'unlinked' : 'linked';
		else if (l.field === 'has' && l.value && !fields.some((f) => f.key === l.value)) {
			fields.push({ key: l.value, has: !l.negated });
		}
	}
	return { link, fields };
}

/** expr with every top-level condition `match` accepts replaced, in place of the first, by
 * `text` ('' removes them); appended when none matched. With no parse yet (tree null), the
 * raw text gets `text` appended so nothing typed is lost. undefined when nothing is left. */
function rewrite(expr: string | undefined, tree: QueryNode | null | undefined, match: (l: NonNullable<ReturnType<typeof leafOf>>) => boolean, text: string): string | undefined {
	const parts: string[] = [];
	let placed = false;
	if (tree) {
		for (const n of topLevel(tree)) {
			const l = leafOf(n);
			if (l && match(l)) {
				if (!placed && text) parts.push(text);
				placed = true;
				continue;
			}
			parts.push(printTree(n, n.kind === 'or'));
		}
	} else if (expr?.trim()) {
		parts.push(expr.trim());
	}
	if (!placed && text) parts.push(text);
	return parts.join(' ') || undefined;
}

/** Sets the Provider link condition: Any removes it. */
export function withLink(expr: string | undefined, tree: QueryNode | null | undefined, link: LinkState): string | undefined {
	return rewrite(expr, tree, isLink, link === 'any' ? '' : link === 'linked' ? 'matched:any' : '-matched:any');
}

/** Sets one field's condition: null removes it. */
export function withField(expr: string | undefined, tree: QueryNode | null | undefined, key: string, has: boolean | null): string | undefined {
	return rewrite(expr, tree, isField(key), has === null ? '' : `${has ? '' : '-'}has:${key}`);
}

/** People and Studios' filter query, as their URL carries it (?link=&has=&missing=). */
export interface EntityFilterQuery {
	link?: 'linked' | 'unlinked';
	has?: string[];
	missing?: string[];
}

export function parseEntityFilters(p: URLSearchParams): EntityFilterQuery {
	const link = p.get('link');
	const has = p.getAll('has').filter(Boolean);
	const missing = p.getAll('missing').filter(Boolean);
	return {
		...(link === 'linked' || link === 'unlinked' ? { link } : {}),
		...(has.length ? { has } : {}),
		...(missing.length ? { missing } : {})
	};
}

export function entityFiltersToParams(q: EntityFilterQuery): URLSearchParams {
	const p = new URLSearchParams();
	if (q.link) p.set('link', q.link);
	for (const k of q.has ?? []) p.append('has', k);
	for (const k of q.missing ?? []) p.append('missing', k);
	return p;
}

/** The query's rows, has before missing; the panel keeps its own row order on top. */
export function entityConds(q: EntityFilterQuery): FieldCond[] {
	return [...(q.has ?? []).map((key) => ({ key, has: true })), ...(q.missing ?? []).map((key) => ({ key, has: false }))];
}

export function withEntityConds(q: EntityFilterQuery, conds: readonly FieldCond[]): EntityFilterQuery {
	const has = conds.filter((c) => c.has).map((c) => c.key);
	const missing = conds.filter((c) => !c.has).map((c) => c.key);
	const { has: _h, missing: _m, ...rest } = q;
	return { ...rest, ...(has.length ? { has } : {}), ...(missing.length ? { missing } : {}) };
}

/** How many panel choices the Filters button counts. */
export function entityFilterCount(q: EntityFilterQuery): number {
	return (q.link ? 1 : 0) + (q.has?.length ?? 0) + (q.missing?.length ?? 0);
}
