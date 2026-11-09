import { describe, expect, it } from 'vitest';
import {
	DEFAULT_CHARACTERS,
	characterExample,
	characterOrder,
	draftTable,
	parseCharacters,
	sameTable,
	sanitizeValue,
	changedSince,
	failureLine,
	lockLine,
	plainName,
	readBack,
	reasonChip,
	renameableCount,
	sentence,
	type RenamePlanRow
} from './rename';

// File names wording (F86, testing-strategy §39.6): the pure helpers the plan, the card and the
// video page share.

const row = (over: Partial<RenamePlanRow>): RenamePlanRow => ({
	video_id: 1,
	title: 'T',
	current: 'a.mkv',
	template: 0,
	reason: 'will-rename',
	...over
});

describe('renameableCount', () => {
	const counts = { 'will-rename': 1204, 'out-of-date': 37, clash: 3 };
	it('counts the server totals of the reasons that rename', () => {
		expect(renameableCount(counts, 'all')).toBe(1241);
		expect(renameableCount(counts, 'out-of-date')).toBe(37);
		expect(renameableCount(counts, 'clash')).toBe(0);
	});
});

describe('reasonChip', () => {
	it('marks refusals warn and fine states muted; renaming rows carry none', () => {
		expect(reasonChip(row({ reason: 'clash' }))).toEqual({ label: 'clash', refusal: true });
		expect(reasonChip(row({ reason: 'bad-name', bad_name: 'too-long' }))).toEqual({ label: 'too long', refusal: true });
		expect(reasonChip(row({ reason: 'bad-name', bad_name: 'reserved' }))?.label).toBe('reserved name');
		expect(reasonChip(row({ reason: 'locked' }))).toEqual({ label: 'locked', refusal: false });
		expect(reasonChip(row({ reason: 'held' }))?.refusal).toBe(false);
		expect(reasonChip(row({ reason: 'no-template' }))?.refusal).toBe(false);
		expect(reasonChip(row({ reason: 'will-rename' }))).toBeNull();
		expect(reasonChip(row({ reason: 'out-of-date' }))).toBeNull();
	});
});

describe('lockLine', () => {
	it('phrases each of the three lock reasons (handoff call 7)', () => {
		const at = '2026-10-12T10:00:00Z';
		expect(lockLine('auto', at)).toMatch(/^Locked when you renamed it outside Holodex on .+\.$/);
		expect(lockLine('owner', at)).toBe('Locked by you.');
		expect(lockLine('undo', at)).toMatch(/^Locked when you undid a rename on .+\.$/);
	});
});

describe('readBack', () => {
	it('names the value that holds the literal ending its token', () => {
		const r = row({
			reason: 'wont-read-back',
			template: 0,
			written: { studio: ['Acme [EU]'], title: ['The Long Afternoon'] },
			parsed: { studio: ['Acme [EU'], title: ['The Long Afternoon'] }
		});
		expect(readBack(r, ['[{studio}] {title}'])).toEqual({
			token: 'studio',
			value: 'Acme [EU]',
			breaker: ']',
			template: 1,
			readsAs: 'Acme [EU'
		});
	});

	it('finds a multi-character separator, and has no breaker when the value holds none', () => {
		const r = row({
			reason: 'wont-read-back',
			template: 0,
			written: { title: ['Ada - Redux'], studio: ['Acme'] },
			parsed: { title: ['Ada'], studio: ['Redux - Acme'] }
		});
		expect(readBack(r, ['{title} - {studio}'])?.breaker).toBe(' - ');
		const plain = row({ reason: 'wont-read-back', template: 0, written: { title: ['x'] }, parsed: { title: ['y'] } });
		expect(readBack(plain, ['{title}'])?.breaker).toBeUndefined();
	});

	it('blames the token whose value holds its breaker, not the first token that read back wrong', () => {
		// The title's " (" ends {title} early, so people reads back wrong too; people comes first.
		const r = row({
			reason: 'wont-read-back',
			template: 0,
			written: { people: ['Ada Lovelace'], title: ['Engines (Part 2)'] },
			parsed: { people: ['Part 2) (Ada Lovelace'], title: ['Engines'] }
		});
		expect(readBack(r, ['{title} ({people})'])).toEqual({
			token: 'title',
			value: 'Engines (Part 2)',
			breaker: ' (',
			template: 1,
			readsAs: 'Engines'
		});
		// An optional token is written {title?} in the template; its breaker is still found.
		expect(readBack(r, ['{title?} ({people}) {provider-1-id}'])?.token).toBe('title');
	});
});

describe('wording', () => {
	it('strips the id a clash settled with', () => {
		expect(plainName('Ada Lovelace (2023) {hx-412}.mkv')).toBe('Ada Lovelace (2023).mkv');
		expect(plainName('Ada.mkv')).toBe('Ada.mkv');
	});

	it('lists every changed field', () => {
		expect(changedSince([{ token: 'title', recorded: ['a'], current: ['b'] }])).toBe('Title changed since Holodex named it.');
		expect(
			changedSince([
				{ token: 'title', recorded: [], current: [] },
				{ token: 'studio', recorded: [], current: [] }
			])
		).toBe('Title, studio changed since Holodex named it.');
	});

	it('turns a stale plan into the refresh line', () => {
		expect(failureLine('the name changed since the plan; refresh the plan')).toBe('The name changed since you looked.');
		expect(failureLine('the name is taken')).toBe('Not renamed: the name is taken.');
	});

	it('turns a server refusal into a sentence', () => {
		expect(sentence('add {title} or an identity token: without one, unrelated videos could get the same name')).toBe(
			'Add {title} or an identity token. Without one, unrelated videos could get the same name.'
		);
	});
});

// HOLODEX-633: the character table's example column mirrors internal/rename/name.go's SanitizeValue
// (the same cases as name_test.go), so the browser's preview reads like the server's answer.
describe('sanitizeValue', () => {
	it('applies the default table like the server', () => {
		const cases: Record<string, string> = {
			'a\t\n b': 'a b',
			'  Ada  ': 'Ada',
			'a{b}c': 'abc',
			'Café': 'Café',
			'../etc': '..+etc',
			'Star Wars: Episode IV': 'Star Wars- Episode IV',
			'a : b': 'a- b',
			'Fire | Ice': 'Fire- Ice',
			'a - b': 'a - b',
			'M*A*S*H': 'M-A-S-H',
			'The "Thing"': "The 'Thing'",
			'AC/DC': 'AC+DC',
			'Love; Death': 'Love, Death',
			'Airplane!': 'Airplane',
			'Who Goes There?': 'Who Goes There',
			'a:b*c?': 'a-b-c',
			' : a': 'a',
			'*batteries': 'batteries',
			'{edition-x}': 'edition-x',
			'a​b': 'ab'
		};
		for (const [input, want] of Object.entries(cases)) expect(sanitizeValue(input, DEFAULT_CHARACTERS), input).toBe(want);
	});

	it('applies an owner table in one pass, still removing what no file name may hold', () => {
		const chars = { ':': ' - ', '&': ' and ', '!': ';', ';': '!', '-': '-' };
		const cases: Record<string, string> = {
			'Fast & Furious': 'Fast and Furious',
			'Fast&Furious': 'Fast and Furious',
			'a: b': 'a - b',
			'Wow!': 'Wow;',
			'a;b': 'a!b',
			'a?b': 'ab',
			'a - b': 'a- b'
		};
		for (const [input, want] of Object.entries(cases)) expect(sanitizeValue(input, chars), input).toBe(want);
	});
});

describe('characterExample', () => {
	it('runs a fixed sample through the row stand-in alone, a<c>b for an added character', () => {
		expect(characterExample(':', '-')).toEqual({ sample: 'Star Wars: Episode IV', result: 'Star Wars- Episode IV' });
		expect(characterExample('"', "'")).toEqual({ sample: 'The "Thing"', result: "The 'Thing'" });
		expect(characterExample('?', '')).toEqual({ sample: 'Who Goes There?', result: 'Who Goes There' });
		expect(characterExample('&', ' and ')).toEqual({ sample: 'Fast & Furious', result: 'Fast and Furious' });
		expect(characterExample('@', ' at ')).toEqual({ sample: 'a@b', result: 'a at b' });
	});
});

describe('character table helpers', () => {
	it('orders the nine first, then the others in table order', () => {
		expect(characterOrder({ '!': '', ';': ',', '&': 'and', ':': '-' })).toEqual([
			':', '"', '/', '\\', '|', '*', '?', '<', '>', '!', ';', '&'
		]);
	});

	it('has no table for a row with no character or a repeated one', () => {
		expect(draftTable([{ char: '&', standIn: 'and' }])).toEqual({ '&': 'and' });
		expect(draftTable([{ char: '', standIn: 'x' }])).toBeNull();
		expect(draftTable([{ char: '&', standIn: 'a' }, { char: '&', standIn: 'b' }])).toBeNull();
	});

	it('compares tables by content, and reads a bad setting as the default', () => {
		expect(sameTable({ a: '1', b: '2' }, { b: '2', a: '1' })).toBe(true);
		expect(sameTable({ a: '1' }, { a: '2' })).toBe(false);
		expect(sameTable({ a: '1' }, { a: '1', b: '' })).toBe(false);
		expect(parseCharacters('{"&":"and"}')).toEqual({ '&': 'and' });
		expect(parseCharacters('nope')).toEqual(DEFAULT_CHARACTERS);
		expect(parseCharacters('{"&":1}')).toEqual(DEFAULT_CHARACTERS);
	});
});
