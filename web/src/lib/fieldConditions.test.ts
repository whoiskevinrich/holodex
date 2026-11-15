// F90 Filters panel conditions (testing-strategy §42; provider-and-field-filters-handoff.md,
// "States and interactions"): what Media's panel reads from a parsed query, how it writes
// back, and People and Studios' URL params.
import { describe, expect, it } from 'vitest';
import {
	entityConds,
	entityFilterCount,
	entityFiltersToParams,
	panelConds,
	parseEntityFilters,
	withEntityConds,
	withField,
	withLink
} from './fieldConditions';
import { toChips, type QueryNode } from './query';

const cond = (field: string, value: string, extra: Partial<QueryNode> = {}): QueryNode => ({
	kind: 'cond',
	expr: `${field}:${value}`,
	field,
	value,
	...extra
});
const not = (n: QueryNode): QueryNode => ({ kind: 'not', expr: `-${n.expr}`, items: [n] });
const and = (...items: QueryNode[]): QueryNode => ({ kind: 'and', expr: items.map((i) => i.expr).join(' '), items });
const or = (...items: QueryNode[]): QueryNode => ({ kind: 'or', expr: items.map((i) => i.expr).join(' OR '), items });

describe('Media panel reads the query', () => {
	it('shows top-level Linked and field rows', () => {
		const t = and(cond('matched', 'any'), cond('has', 'director'), not(cond('has', 'poster')), cond('tag', 'noir'));
		expect(panelConds(t)).toEqual({
			link: 'linked',
			fields: [
				{ key: 'director', has: true },
				{ key: 'poster', has: false }
			]
		});
		expect(panelConds(not(cond('matched', 'any'))).link).toBe('unlinked');
	});
	it('draws nothing for an OR, a named provider or a has inside a group', () => {
		expect(panelConds(or(cond('has', 'director'), cond('has', 'actors')))).toEqual({ link: 'any', fields: [] });
		expect(panelConds(cond('matched', 'provider-1')).link).toBe('any');
		expect(panelConds(and(cond('tag', 'noir'), not(and(cond('has', 'director'), cond('tag', 'x'))))).fields).toEqual([]);
		expect(panelConds(null)).toEqual({ link: 'any', fields: [] });
	});
});

describe('Media panel writes the query', () => {
	const t = and(cond('tag', 'noir'), cond('has', 'director'), cond('studio', '3'));
	it('flips a field in place and removes it', () => {
		expect(withField(t.expr, t, 'director', false)).toBe('tag:noir -has:director studio:3');
		expect(withField(t.expr, t, 'director', null)).toBe('tag:noir studio:3');
	});
	it('appends a new condition and sets or clears the link', () => {
		expect(withField(t.expr, t, 'actors', true)).toBe('tag:noir has:director studio:3 has:actors');
		expect(withLink(t.expr, t, 'unlinked')).toBe('tag:noir has:director studio:3 -matched:any');
		const linked = and(cond('matched', 'any'), cond('tag', 'noir'));
		expect(withLink(linked.expr, linked, 'unlinked')).toBe('-matched:any tag:noir');
		expect(withLink(linked.expr, linked, 'any')).toBe('tag:noir');
		expect(withLink('matched:any', cond('matched', 'any'), 'any')).toBeUndefined();
	});
	it('keeps an OR and a named provider as typed', () => {
		const q = and(or(cond('has', 'director'), cond('has', 'actors')), cond('matched', 'provider-1'));
		expect(withLink(q.expr, q, 'linked')).toBe('(has:director OR has:actors) matched:provider-1 matched:any');
	});
	it('appends to the raw text before the parse returns', () => {
		expect(withField('tag:noir', null, 'director', true)).toBe('tag:noir has:director');
		expect(withField(undefined, null, 'director', true)).toBe('has:director');
	});
});

describe('the chips read the panel’s words', () => {
	const labels = (n: QueryNode) => {
		const m = toChips(n);
		return m.kind === 'chips' ? m.items.map((i) => (i.kind === 'chip' ? [i.chip.label, i.chip.negated, !!i.chip.ownerOnly] : null)) : null;
	};
	it('Linked, Not linked, Linked: provider, Label: has|missing, all owner-only', () => {
		const t = and(
			cond('matched', 'any'),
			not(cond('matched', 'any')),
			cond('matched', 'provider-1', { label: 'provider-1' }),
			cond('has', 'director', { field_label: 'Director' }),
			not(cond('has', 'poster', { field_label: 'Poster' }))
		);
		expect(labels(t)).toEqual([
			['Linked', false, true],
			['Not linked', true, true],
			['Linked: provider-1', false, true],
			['Director: has', false, true],
			['Poster: missing', true, true]
		]);
	});
});

describe('People and Studios URL params', () => {
	it('round-trips and drops an unknown link value', () => {
		const q = parseEntityFilters(new URLSearchParams('link=unlinked&has=bio&missing=birthdate&has=website'));
		expect(q).toEqual({ link: 'unlinked', has: ['bio', 'website'], missing: ['birthdate'] });
		expect(parseEntityFilters(entityFiltersToParams(q))).toEqual(q);
		expect(parseEntityFilters(new URLSearchParams('link=maybe'))).toEqual({});
		expect(entityFilterCount(q)).toBe(4);
	});
	it('rows map to has and missing and back', () => {
		const q = { link: 'linked' as const, has: ['bio'] };
		const rows = [...entityConds(q), { key: 'birthdate', has: false }];
		expect(withEntityConds(q, rows)).toEqual({ link: 'linked', has: ['bio'], missing: ['birthdate'] });
		expect(withEntityConds(q, [])).toEqual({ link: 'linked' });
	});
});
