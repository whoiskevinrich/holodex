import { describe, expect, it } from 'vitest';
import { provenanceKey, rowProvenance } from './chipProvenance';
import type { ResolvedValue } from './types';

const v = (value: string, sources: string[], manual = false): ResolvedValue => ({ value, sources, manual });

describe('provenanceKey', () => {
	it('keys a provider value by its first provider, even when the file agrees', () => {
		expect(provenanceKey(v('a', ['file', 'provider-1', 'provider-2']))).toEqual({ kind: 'provider', provider: 'provider-1' });
	});
	it('keys Holodex’s own sources as the baseline, never as a provider', () => {
		for (const s of ['file', 'record', 'computed', 'tag']) expect(provenanceKey(v('a', [s]))).toEqual({ kind: 'baseline' });
	});
	it('keys a custom value as manual, by flag or by source', () => {
		expect(provenanceKey(v('a', ['provider-1'], true))).toEqual({ kind: 'manual' });
		expect(provenanceKey(v('a', ['manual']))).toEqual({ kind: 'manual' });
	});
	it('claims nothing for a value with no sources', () => {
		expect(provenanceKey(v('a', []))).toEqual({ kind: 'unknown' });
	});
});

describe('rowProvenance', () => {
	it('gives a single-provider row one badge and no chip marks', () => {
		expect(rowProvenance([v('a', ['provider-1']), v('b', ['provider-1', 'file'])])).toEqual({
			badge: { provider: 'provider-1', manual: false },
			marks: ['', '']
		});
	});
	it('gives an all-baseline row the file badge', () => {
		expect(rowProvenance([v('a', ['file']), v('b', ['file'])]).badge).toEqual({ provider: '', manual: false });
	});
	it('gives an all-custom row the custom badge', () => {
		expect(rowProvenance([v('a', ['manual'], true)]).badge).toEqual({ provider: '', manual: true });
	});
	it('marks only the provider values of a mixed row, with no badge', () => {
		expect(rowProvenance([v('a', ['provider-1']), v('b', ['file']), v('c', ['manual'], true), v('d', ['provider-2'])])).toEqual({
			badge: null,
			marks: ['provider-1', '', '', 'provider-2']
		});
	});
	it('treats two different providers as a mixed row', () => {
		expect(rowProvenance([v('a', ['provider-1']), v('b', ['provider-2'])]).badge).toBeNull();
	});
	it('shows nothing for a row whose sources are unknown, or an empty row', () => {
		expect(rowProvenance([v('a', []), v('b', [])])).toEqual({ badge: null, marks: ['', ''] });
		expect(rowProvenance([])).toEqual({ badge: null, marks: [] });
	});
});
