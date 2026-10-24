import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import CurationFieldRow from './CurationFieldRow.svelte';
import type { ResolvedField, ResolvedValue } from '#lib/types.ts';

// HOLODEX-144 (chip-row-provenance-handoff.md): a row of one provenance ends with one badge;
// a mixed row marks only its provider values. No chip carries the old ·source text.

function field(items: ResolvedValue[]): ResolvedField {
	return { canonical: 'actors', label: 'Actors', values: items.map((i) => i.value), items, multi: true } as ResolvedField;
}

async function row(items: ResolvedValue[], isOwner = false) {
	const screen = await render(CurationFieldRow, { field: field(items), isOwner, onchanged: () => {} });
	return screen.container;
}

describe('CurationFieldRow provenance', () => {
	it('ends a single-provider row with one badge and leaves the chips plain', async () => {
		const el = await row([
			{ value: 'Lena Ortiz', sources: ['provider-1'] },
			{ value: 'Mara Voss', sources: ['provider-1', 'file'] }
		]);
		expect(el.querySelectorAll('[title="from provider-1"]')).toHaveLength(1);
		expect(el.querySelectorAll('.curation-chip [title^="from"]')).toHaveLength(0);
		expect(el.textContent).not.toContain('·');
	});

	it('marks only the provider values of a mixed row, with no row badge', async () => {
		const el = await row([
			{ value: 'Ana Keßler', sources: ['provider-1'] },
			{ value: 'Rui Dias', sources: ['file'] },
			{ value: 'Mara Voss', sources: ['provider-1', 'file'] }
		]);
		const marks = el.querySelectorAll('.curation-chip [title^="from"]');
		expect([...marks].map((m) => m.closest('.curation-chip')?.getAttribute('aria-label'))).toEqual([
			'Ana Keßler, from provider-1',
			'Mara Voss, from provider-1 + file'
		]);
		expect(marks[1].getAttribute('title')).toBe('from provider-1 and file');
		expect(el.querySelector('[aria-label^="source:"]')).toBeNull();
		expect(el.textContent).not.toContain('·');
	});

	it('shows the file pill after an all-file row', async () => {
		const el = await row([{ value: 'Rui Dias', sources: ['file'] }]);
		expect(el.querySelector('[aria-label="source: from file"]')?.textContent?.trim()).toBe('file');
	});

	it('claims no provenance for values without sources', async () => {
		const el = await row([{ value: 'Rui Dias', sources: [] }]);
		expect(el.querySelector('[aria-label^="source:"]')).toBeNull();
		expect(el.querySelectorAll('[title^="from"]')).toHaveLength(0);
	});

	it('keeps the source in each chip’s accessible name', async () => {
		const el = await row([
			{ value: 'Ana Keßler', sources: ['provider-1'] },
			{ value: 'Rui Dias', sources: ['file'] }
		]);
		const labels = [...el.querySelectorAll('.curation-chip')].map((c) => c.getAttribute('aria-label'));
		expect(labels).toEqual(['Ana Keßler, from provider-1', 'Rui Dias, from file']);
	});

	it('puts the owner’s + Add after the row badge', async () => {
		const el = await row([{ value: 'Lena Ortiz', sources: ['provider-1'] }], true);
		const badge = el.querySelector('[title="from provider-1"]')!;
		const add = [...el.querySelectorAll('button')].find((b) => b.textContent?.includes('Add'))!;
		expect(badge.compareDocumentPosition(add) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});
});
