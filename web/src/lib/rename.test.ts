import { describe, expect, it } from 'vitest';
import {
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
