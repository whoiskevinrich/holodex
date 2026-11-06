import { describe, expect, it } from 'vitest';
import {
	caretContext,
	condLabel,
	echoParts,
	exprWithout,
	fieldsFor,
	groupLabel,
	insertAt,
	isQueryMode,
	printTree,
	toChips,
	type QueryNode
} from './query';

// Display-tree builders shaped like /query/parse's output (internal/query DisplayNode).
const tag = (id: number, label: string): QueryNode => ({ kind: 'cond', field: 'tag', op: '=', value: String(id), id, label, expr: `tag:${id}` });
const person = (id: number, label: string): QueryNode => ({ kind: 'cond', field: 'person', op: '=', value: String(id), id, label, expr: `person:${id}` });
const not = (x: QueryNode): QueryNode => ({ kind: 'not', items: [x], expr: '' });
const and = (...items: QueryNode[]): QueryNode => ({ kind: 'and', items, expr: '' });
const or = (...items: QueryNode[]): QueryNode => ({ kind: 'or', items, expr: '' });
const fill = (n: QueryNode): QueryNode => {
	n.items?.forEach(fill);
	if (n.kind !== 'cond' && n.kind !== 'word') n.expr = printTree(n);
	return n;
};

const sf = tag(1, 'science-fiction');
const fantasy = tag(2, 'fantasy');
const action = tag(3, 'action');
const comedy = tag(4, 'comedy');
const ford = person(7, 'Harrison Ford');
// Motivating query 1: ((sf fantasy) OR action) -comedy ford
const query1 = () => fill(and(or(and(sf, fantasy), action), not(comedy), ford));

describe('F89 chip labels', () => {
	it('words dates and playlist references from the server label', () => {
		const added: QueryNode = { kind: 'cond', field: 'added', op: '<', value: '<30d', label: 'last 30 days', expr: 'added:<30d' };
		const pl: QueryNode = { kind: 'cond', field: 'playlist', op: '=', value: '4', id: 4, label: 'Favourites', expr: 'playlist:4' };
		expect(condLabel(added)).toBe('added: last 30 days');
		expect(condLabel(pl)).toBe('in: Favourites');
		expect(toChips(fill(not(pl)))).toMatchObject({ kind: 'chips', items: [{ kind: 'chip', chip: { label: 'not in: Favourites', negated: true, path: [] } }] });
	});
});

describe('toChips (W1)', () => {
	it('draws a plain AND as plain chips', () => {
		const m = toChips(fill(and(comedy, { kind: 'cond', field: 'duration', op: '<', lo: 120, expr: 'duration:<120s' })));
		expect(m).toMatchObject({
			kind: 'chips',
			items: [
				{ kind: 'chip', chip: { label: 'comedy', negated: false, path: [0] } },
				{ kind: 'chip', chip: { label: 'Duration < 2 min', negated: false, path: [1] } }
			]
		});
	});

	it('draws query 1 as one group, a NOT chip and a person chip', () => {
		const m = toChips(query1());
		expect(m.kind).toBe('chips');
		if (m.kind !== 'chips') return;
		expect(m.items).toHaveLength(3);
		const [group, notChip, fordChip] = m.items;
		expect(group.kind).toBe('group');
		if (group.kind === 'group') {
			expect(group.members.map((s) => s.map((c) => c.label))).toEqual([['science-fiction', 'fantasy'], ['action']]);
			expect(groupLabel(group.members)).toBe('science-fiction and fantasy, or action');
		}
		expect(notChip).toMatchObject({ kind: 'chip', chip: { label: 'not comedy', negated: true, path: [1] } });
		expect(fordChip.kind === 'chip' && fordChip.chip.label).toBe('Harrison Ford');
	});

	it('shows only the Advanced row past one OR level', () => {
		const deep = fill(and(or(and(or(sf, fantasy), action), comedy), ford));
		expect(toChips(deep)).toEqual({ kind: 'advanced', text: deep.expr });
		const notGroup = fill(not(or(sf, action)));
		expect(toChips(notGroup).kind).toBe('advanced');
	});

	it('labels a bare word as title or Tag', () => {
		const m = toChips({ kind: 'word', label: 'comedy', expr: 'comedy' });
		expect(m.kind === 'chips' && m.items[0].kind === 'chip' && m.items[0].chip.label).toBe('comedy · title or Tag');
	});
});

describe('exprWithout (W2)', () => {
	it('removes a member of an AND set, dropping its parentheses', () => {
		expect(exprWithout(query1(), [0, 0, 1])).toBe('(tag:1 OR tag:3) -tag:4 person:7');
	});
	it('collapses a group left with one member into a plain chip', () => {
		const q = exprWithout(query1(), [0, 1]);
		expect(q).toBe('tag:1 tag:2 -tag:4 person:7');
	});
	it('removes the whole group with its ×', () => {
		expect(exprWithout(query1(), [0])).toBe('-tag:4 person:7');
	});
	it('removes a NOT chip, and the last chip leaves an empty query', () => {
		expect(exprWithout(query1(), [1])).toBe('(tag:1 tag:2 OR tag:3) person:7');
		expect(exprWithout(comedy, [])).toBe('');
	});
});

describe('echoParts (W3)', () => {
	it('slices at UTF-16 offsets into three runs', () => {
		const text = 'é person:"Zoë A" tag:sciencefiction';
		const start = text.indexOf('sciencefiction');
		expect(echoParts(text, { start, end: start + 14 })).toEqual(['é person:"Zoë A" tag:', 'sciencefiction', '']);
	});
	it('clamps offsets past the text', () => {
		expect(echoParts('abc', { start: 2, end: 99 })).toEqual(['ab', 'c', '']);
	});
});

describe('query-mode lexer (W4)', () => {
	it('keeps plain words in plain search', () => {
		for (const s of ['harrison', 'harrison ford', 'or not', 'well-known', '-']) expect(isQueryMode(s)).toBe(false);
	});
	it('switches on a field, a negation, an operator or a parenthesis', () => {
		for (const s of ['tag:', 'a -x', 'a OR b', '(a', 'NOT a', '-tag:comedy']) expect(isQueryMode(s)).toBe(true);
	});
	it('reports the field under the caret', () => {
		const t1 = 'tag:sci';
		expect(caretContext(t1, t1.length)).toEqual({ kind: 'value', field: 'tag', prefix: 'sci', start: 4, end: 7 });
		const t2 = 'tag:a person:"Harrison F';
		expect(caretContext(t2, t2.length)).toMatchObject({ kind: 'value', field: 'person', prefix: 'Harrison F' });
		const t3 = 'person:"Harrison Ford"';
		expect(caretContext(t3, t3.length)).toEqual({ kind: 'none' });
		expect(caretContext('a pers', 6)).toEqual({ kind: 'field', prefix: 'pers', start: 2, end: 6 });
		expect(caretContext('-tag:co', 7)).toMatchObject({ kind: 'value', field: 'tag', prefix: 'co', start: 5 });
	});
	it('inserts a suggestion with a trailing space', () => {
		expect(insertAt('tag:sci', 4, 7, 'science-fiction')).toEqual({ text: 'tag:science-fiction ', caret: 20 });
	});
	it('never offers matched to a visitor', () => {
		expect(fieldsFor(false)).not.toContain('matched');
		expect(fieldsFor(true)).toContain('matched');
	});
});

describe('mapped fields and the F89 dates', () => {
	const cond = (x: Partial<QueryNode>): QueryNode => ({ kind: 'cond', expr: '', ...x });
	it('labels a mapped field by its own label, a date in the server\'s words', () => {
		expect(condLabel(cond({ field: 'actors', field_label: 'Actors', label: 'Ann Lee' }))).toBe('Actors: Ann Lee');
		expect(condLabel(cond({ field: 'updated', label: 'last 30 days' }))).toBe('updated: last 30 days');
		expect(condLabel(cond({ field: 'released', label: '2026' }))).toBe('released: 2026');
		expect(condLabel(cond({ field: 'director', label: 'gone', missing: true }))).toBe('director: gone');
	});
	it('takes the key glyph from the server, with no list of fields', () => {
		const chipFor = (x: QueryNode) => (toChips(x) as { items: { kind: string; chip: { ownerOnly?: boolean } }[] }).items[0].chip;
		expect(chipFor(cond({ field: 'container', label: 'x', owner_only: true })).ownerOnly).toBe(true);
		expect(chipFor(cond({ field: 'director', label: 'x' })).ownerOnly).toBeUndefined();
	});
	it('offers the reader\'s mapped fields after the built-ins', () => {
		expect(fieldsFor(false, ['director'])).toContain('director');
		expect(fieldsFor(false)).toContain('released');
		expect(fieldsFor(false)).not.toContain('matched');
	});
});
