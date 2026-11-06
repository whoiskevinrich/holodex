import { describe, expect, it } from 'vitest';
import { ApiError } from './api';
import { allChips, capReadout, chipAtFault, draftDirty, draftQueryString, missingProblem, staleNotice, toProblem } from './playlistDefinition';
import { printNames, toChips, type ChipCond, type QueryNode } from './query';

const pl = (id: number, label: string, missing = false): QueryNode => ({
	kind: 'cond',
	field: 'playlist',
	op: '=',
	value: String(id),
	id,
	label,
	expr: `playlist:${id}`,
	...(missing ? { missing: true } : {})
});
const matched: QueryNode = { kind: 'cond', field: 'matched', op: '=', value: 'any', label: 'any', expr: 'matched:any' };
const and = (...items: QueryNode[]): QueryNode => ({ kind: 'and', items, expr: '' });

describe('the draft (W4)', () => {
	const saved = { qs: 'expr=tag%3A1', sort: 'added_desc', cap: null };
	it('asks only when something differs, typed text included', () => {
		expect(draftDirty({ ...saved }, saved, 'tag:a', 'tag:a')).toBe(false);
		expect(draftDirty({ ...saved }, saved, ' tag:a ', 'tag:a')).toBe(false);
		expect(draftDirty({ ...saved, qs: '' }, saved, '', '')).toBe(true);
		expect(draftDirty({ ...saved, sort: 'title_asc' }, saved, '', '')).toBe(true);
		expect(draftDirty({ ...saved, cap: 25 }, saved, '', '')).toBe(true);
		expect(draftDirty({ ...saved }, saved, 'tag:ab', 'tag:a')).toBe(true);
	});
	it('keeps a stored missing_facet, which list state drops', () => {
		const qs = draftQueryString({ expr: 'tag:1' }, ['poster']);
		expect(new URLSearchParams(qs).get('expr')).toBe('tag:1');
		expect(new URLSearchParams(qs).getAll('missing_facet')).toEqual(['poster']);
	});
});

describe('capReadout (P0-2)', () => {
	it.each([
		[31, null, false, undefined, '31 match'],
		[31, 25, false, undefined, '31 match, showing 25'],
		[31, 25, true, undefined, '31 match, showing a fresh 25 each play'],
		[10, 25, false, undefined, '10 match'],
		[31, 25, false, 12, '31 match, showing 25 (was 12)'],
		[31, 25, false, 25, '31 match, showing 25']
	])('%i matches, cap %s', (m, cap, random, saved, want) => {
		expect(capReadout(m, cap, random, saved)).toBe(want);
	});
});

describe('problems and their chips', () => {
	const model = toChips(and(pl(4, 'Favourites'), matched));
	const chip = (field: string) => allChips(model).find((c) => c.field === field) as ChipCond;

	it('marks the owner-only chips on an own-condition refusal, naming one in the fix', () => {
		const e = new ApiError(400, '/playlists/1', 'x', { error: 'A public playlist can’t use owner-only conditions.', owner_only: true });
		const p = toProblem(e, model);
		expect(p.fix).toBe('Remove “matched”, or make the playlist private.');
		expect(chipAtFault(chip('matched'), p)).toBe(true);
		expect(chipAtFault(chip('playlist'), p)).toBe(false);
	});

	it('marks the direct reference a deep problem came through', () => {
		const e = new ApiError(400, '/playlists/1', 'x', {
			error: 'This would make a loop.',
			fix: 'Remove “in: Favourites”.',
			stale_refs: [{ kind: 'loop', via: 4, value: '1,4,9,1' }]
		});
		const p = toProblem(e, model);
		expect(p.fix).toBe('Remove “in: Favourites”.');
		expect(chipAtFault(chip('playlist'), p)).toBe(true);
		expect(chipAtFault(chip('matched'), p)).toBe(false);
	});

	it('shows a deleted reference as its id, marked, and keeps the id in the field text', () => {
		const tree = pl(14, '#14', true);
		const m = toChips(tree);
		expect(allChips(m)[0].label).toBe('in: #14');
		expect(missingProblem(m)?.sentence).toBe('This definition refers to a playlist that was deleted.');
		expect(chipAtFault(allChips(m)[0], null)).toBe(true);
		expect(printNames(tree)).toBe('playlist:14');
		expect(missingProblem(model)).toBeNull();
	});

	it('words a server error with no body as itself', () => {
		expect(toProblem(new Error('offline'), null).sentence).toBe('offline');
	});
});

describe('staleNotice', () => {
	it('offers Make private for a public playlist problem, never for a deletion', () => {
		expect(staleNotice({ kind: 'owner_only', key: 'playlist', value: '4', name: 'A', via_name: 'B' }).makePrivate).toBe(true);
		expect(staleNotice({ kind: 'owner_only', key: 'playlist', value: '4', name: 'A', via_name: 'B' }).detail).toContain('“B”');
		expect(staleNotice({ kind: 'missing', key: 'playlist', value: '4' }).makePrivate).toBeUndefined();
	});
	it('names a loop by its path', () => {
		expect(staleNotice({ kind: 'loop', name: 'A → B → A' }).detail).toContain('A → B → A');
	});
});
