import { describe, expect, it } from 'vitest';
import { cutsNothing, editionFileName, handleBounds, reviewSummary, sanitizeEditionName, trimSummary } from './trimPicker';

const FILE = { interval: 0.04, duration: 100 };
const first = 0;
const last = 99.96;

describe('handleBounds', () => {
	it('stops each handle one frame short of the other', () => {
		const r = { inTime: 10, outTime: 20, ...FILE };
		expect(handleBounds('in', r, first, last)).toEqual({ min: 0, max: 19.96 });
		expect(handleBounds('out', r, first, last)).toEqual({ min: 10.04, max: last });
	});
	it('never gives a range that crosses, on a very short file', () => {
		const r = { inTime: 0, outTime: 0.04, interval: 0.04, duration: 0.08 };
		const a = handleBounds('in', r, 0, 0.04);
		const b = handleBounds('out', r, 0, 0.04);
		expect(a.max).toBeGreaterThanOrEqual(a.min);
		expect(b.max).toBeGreaterThanOrEqual(b.min);
	});
});

describe('cutsNothing', () => {
	it('is true only with both handles on the file ends', () => {
		expect(cutsNothing({ inTime: 0, outTime: last, interval: 0.04 }, first, last)).toBe(true);
		expect(cutsNothing({ inTime: 0.04, outTime: last, interval: 0.04 }, first, last)).toBe(false);
		expect(cutsNothing({ inTime: 0, outTime: last - 0.04, interval: 0.04 }, first, last)).toBe(false);
	});
});

describe('trimSummary', () => {
	it('names both ends', () => {
		expect(trimSummary({ inTime: 12, outTime: 89.96, ...FILE }, first, last)).toBe(
			'Cuts 0:12 from the start and 0:10 from the end. Keeps 1:18.'
		);
	});
	it('names only the end that is cut', () => {
		expect(trimSummary({ inTime: 12, outTime: last, ...FILE }, first, last)).toBe('Cuts 0:12 from the start. Keeps 1:28.');
		expect(trimSummary({ inTime: 0, outTime: 89.96, ...FILE }, first, last)).toBe('Cuts 0:10 from the end. Keeps 1:30.');
	});
	it('asks for a handle when nothing is cut', () => {
		expect(trimSummary({ inTime: 0, outTime: last, ...FILE }, first, last)).toBe('Move a handle to choose what to cut.');
	});
	it('says so when a cut is under a second, not 0:00', () => {
		expect(trimSummary({ inTime: 0.4, outTime: last, ...FILE }, first, last)).toBe(
			'Cuts less than a second from the start. Keeps 1:39.'
		);
	});
});

describe('reviewSummary', () => {
	it('states the kept length, the re-encode and what stays', () => {
		expect(reviewSummary({ inTime: 12, outTime: 89.96, ...FILE })).toBe(
			'Keeps 1:18 of 1:40. The video is re-encoded. Audio, subtitles and tags are kept, and chapters follow the cut.'
		);
	});
});

// The same cases as internal/maintenance/trim_test.go's TestEditionTarget, so the preview shows what
// the server writes.
describe('editionFileName', () => {
	for (const [path, name, want] of [
		['The Long Road (2004).mkv', 'Trimmed', 'The Long Road (2004) {edition-Trimmed}.mkv'],
		['The Long Road (2004) {edition-Theatrical}.mkv', 'Trimmed', 'The Long Road (2004) {edition-Trimmed}.mkv'],
		['a {edition-Old} b.mp4', 'New', 'a b {edition-New}.mp4'],
		['a.mp4', '../../etc/passwd', 'a {edition-etcpasswd}.mp4'],
		['a.mp4', '...hidden', 'a {edition-hidden}.mp4'],
		['a.mp4', 'x}{y', 'a {edition-xy}.mp4'],
		['a.mp4', 'bad\x00name\n', 'a {edition-badname}.mp4'],
		['a.mp4', '  ', ''],
		['a.mp4', '///', ''],
		['a.mp4', '..', '']
	]) {
		it(`${path} + ${JSON.stringify(name)}`, () => {
			expect(editionFileName(path, name)).toBe(want);
		});
	}
	it('drops what can’t be in a name', () => {
		expect(sanitizeEditionName(' a:b*c?"d<e>f|g ')).toBe('abcdefg');
	});
});
