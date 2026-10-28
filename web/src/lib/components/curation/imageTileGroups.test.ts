import { describe, expect, it } from 'vitest';
import { resolveSelection, sourceChips } from '#lib/f36.ts';
import type { FieldCandidate, ResolvedField } from '#lib/types.ts';
import { needsDecision } from '#lib/writebackCockpit.ts';
import { imageTileGroups, tileImage, tileTitle } from './imageTileGroups.ts';

// F85 testing-strategy §37.4 W1 (design QA 10.1): the Poster row's groups.

const FILE: FieldCandidate = { source: 'file', value: '', image_url: '/api/v1/media/7/poster/sources/file?v=1' };
const FRAME: FieldCandidate = { source: 'local:frame', value: 'local:frame#aa', image_url: '/api/v1/media/7/poster/sources/frame?v=aa', frame_at: 760.5 };
const UPLOAD: FieldCandidate = { source: 'local:upload', value: 'local:upload#bb', image_url: '/api/v1/media/7/poster/sources/upload?v=bb' };
const AUTO: FieldCandidate = { source: 'local:automatic', value: 'local:automatic#cc', image_url: '/api/v1/media/7/poster/sources/automatic?v=cc' };
const P1: FieldCandidate = { source: 'provider:provider-1', provider: 'provider-1', value: 'https://img.example/p1.jpg' };

function poster(candidates: FieldCandidate[], decision?: ResolvedField['decision'], values: string[] = []): ResolvedField {
	return { canonical: 'poster_url', label: 'Poster', display: 'image_url', values, candidates, decision };
}
const tiles = (f: ResolvedField) => sourceChips(f).filter((c) => !c.manual || c.value);
const shape = (f: ResolvedField, providers: string[] = []) =>
	imageTileGroups(tiles(f), providers).map((g) => [g.caption, g.chips.map((c) => c.key)]);

describe('imageTileGroups', () => {
	it('orders the groups file → Holodex → Automatic → providers', () => {
		expect(shape(poster([FILE, P1, FRAME, UPLOAD, AUTO]))).toEqual([
			['In the file', ['file']],
			['Holodex', ['local:frame', 'local:upload']],
			['Automatic', ['local:automatic']],
			['provider-1', ['provider:provider-1']]
		]);
	});

	it('omits Holodex with neither image and Automatic with no generated frame', () => {
		expect(shape(poster([FILE, P1]))).toEqual([
			['In the file', ['file']],
			['provider-1', ['provider:provider-1']]
		]);
	});

	it('gives a matched provider with no poster a group with an empty list, in provider order', () => {
		expect(shape(poster([FILE, P1]), ['provider-2', 'provider-1'])).toEqual([
			['In the file', ['file']],
			['provider-2', []],
			['provider-1', ['provider:provider-1']]
		]);
	});

	it('gives no group to a provider folded into another provider tile', () => {
		const p2 = { ...P1, source: 'provider:provider-2' as const, provider: 'provider-2' };
		expect(shape(poster([FILE, P1, p2]), ['provider-1', 'provider-2'])).toEqual([
			['In the file', ['file']],
			['provider-1', ['provider:provider-1']]
		]);
	});

	it('makes every group a list, even of one tile', () => {
		for (const g of imageTileGroups(tiles(poster([FILE, FRAME, AUTO, P1])), ['provider-3'])) {
			expect(Array.isArray(g.chips)).toBe(true);
		}
	});
});

describe('tile images and titles', () => {
	const f = poster([{ source: 'file', value: '' }, P1, FRAME]);
	const chip = (key: string) => sourceChips(f).find((c) => c.key === key)!;

	it('shows the served source image, never a local token, and a placeholder for a file with no art', () => {
		expect(tileImage(f, chip('local:frame'))).toBe(FRAME.image_url);
		expect(tileImage(f, chip('provider:provider-1'))).toBe(P1.value);
		expect(tileImage(f, chip('file'))).toBe('');
	});

	it('titles the frame tile with its picked time (P1-1)', () => {
		expect(tileTitle(f, chip('local:frame'))).toBe('frame at 12:40');
		expect(tileTitle(f, chip('provider:provider-1'))).toBeUndefined();
	});
});

describe('local poster selection', () => {
	it('selects a decided local chip, standing', () => {
		const f = poster([FILE, FRAME], { source: 'local:frame', standing: true }, ['local:frame#aa']);
		expect(resolveSelection(f, sourceChips(f))).toEqual({ key: 'local:frame', pending: false });
	});

	it('falls back to the file when the chosen image is no longer stored', () => {
		const f = poster([FILE], { source: 'local:upload', standing: true });
		expect(resolveSelection(f, sourceChips(f))).toEqual({ key: 'file', pending: false });
	});

	it('lets ·file replace a local decision whose image is gone', () => {
		const f = poster([FILE], { source: 'local:upload', standing: true });
		expect(needsDecision(f, sourceChips(f), { key: 'file', custom: '' })).toBe(true);
	});

	it('never makes the automatic frame the RD6 pending winner', () => {
		const f = poster([{ source: 'file', value: '' }, AUTO], { source: 'file', standing: false });
		expect(resolveSelection(f, sourceChips(f))).toEqual({ key: 'file', pending: false });
	});
});
