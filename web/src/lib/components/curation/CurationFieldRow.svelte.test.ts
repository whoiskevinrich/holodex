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

// F30.3g (testing-strategy §34.4, source-choices-restore-handoff.md): the owner's Removed (N).
describe('CurationFieldRow Removed (N)', () => {
	type Req = { field: string; value: string; action: string };
	function owned(items: ResolvedValue[], removed: { value: string; not_sent?: boolean }[]) {
		const calls: string[] = [];
		const f = { ...field(items), canonical: 'genres', label: 'Genres', removed } as ResolvedField;
		const props = {
			field: f,
			isOwner: true,
			curate: async (r: Req) => void calls.push(`curate ${r.action} ${r.value}`),
			clearCuration: async (r: Req) => void calls.push(`clear ${r.action} ${r.value}`),
			onchanged: () => {}
		};
		return { calls, props };
	}
	const toggle = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('[aria-controls="removed-genres"]');

	it('renders no toggle when nothing is removed', async () => {
		const { props } = owned([{ value: 'Drama', sources: ['file'] }], []);
		const { container } = await render(CurationFieldRow, props);
		expect(toggle(container)).toBeNull();
	});

	it('opens a line of dashed removed chips without sending anything', async () => {
		const { calls, props } = owned([{ value: 'Drama', sources: ['file'] }], [
			{ value: 'Noir' },
			{ value: 'Western', not_sent: true }
		]);
		const { container } = await render(CurationFieldRow, props);
		const t = toggle(container)!;
		expect(t.textContent).toContain('Removed (2)');
		expect(t.getAttribute('aria-expanded')).toBe('false');
		t.click();
		await expect.poll(() => container.querySelector('#removed-genres')).not.toBeNull();
		expect(t.getAttribute('aria-expanded')).toBe('true');
		const chips = [...container.querySelectorAll('#removed-genres .curation-chip')];
		expect(chips.map((c) => c.getAttribute('aria-label'))).toEqual([
			'Noir, removed',
			'Western, removed, not sent by any source now'
		]);
		expect(chips.every((c) => c.className.includes('border-dashed'))).toBe(true);
		expect(container.querySelector('#removed-genres [aria-label="Edit Noir"]')).toBeNull();
		expect(calls).toEqual([]);
	});

	it('restores through the clear transport and announces it', async () => {
		const { calls, props } = owned([], [{ value: 'Western', not_sent: true }]);
		const { container } = await render(CurationFieldRow, props);
		toggle(container)!.click();
		await expect.poll(() => container.querySelector('[aria-label="Restore Western"]')).not.toBeNull();
		container.querySelector<HTMLButtonElement>('[aria-label="Restore Western"]')!.click();
		await expect.poll(() => calls).toEqual(['clear suppress Western', 'curate add Western']);
		await expect.poll(() => container.querySelector('[aria-live="polite"].sr-only')?.textContent).toBe(
			'Restored Western. No source sends it now.'
		);
	});

	it('moves focus to + Add once the last removal is restored', async () => {
		const { props } = owned([{ value: 'Drama', sources: ['file'] }], [{ value: 'Noir' }]);
		const screen = await render(CurationFieldRow, props);
		const { container } = screen;
		toggle(container)!.click();
		await expect.poll(() => container.querySelector('[aria-label="Restore Noir"]')).not.toBeNull();
		// The parent reloads the field without the removal, as the page does after onchanged.
		props.onchanged = () => screen.rerender({ ...props, field: { ...props.field, removed: [] } });
		await screen.rerender(props);
		container.querySelector<HTMLButtonElement>('[aria-label="Restore Noir"]')!.click();
		await expect.poll(() => document.activeElement?.hasAttribute('data-add')).toBe(true);
	});

	it('shows + Add and the toggle on a field whose every value was removed', async () => {
		const { props } = owned([], [{ value: 'Noir' }]);
		const { container } = await render(CurationFieldRow, props);
		expect(container.querySelector('[data-add]')).not.toBeNull();
		expect(toggle(container)).not.toBeNull();
	});

	it('lifts a removal when the owner types the value back into + Add', async () => {
		const { calls, props } = owned([{ value: 'Drama', sources: ['file'] }], [{ value: 'Noir' }]);
		const { container } = await render(CurationFieldRow, props);
		container.querySelector<HTMLButtonElement>('[data-add]')!.click();
		await expect.poll(() => container.querySelector('input')).not.toBeNull();
		const el = container.querySelector<HTMLInputElement>('input')!;
		el.value = 'noir';
		el.dispatchEvent(new Event('input', { bubbles: true }));
		el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
		await expect.poll(() => calls).toEqual(['clear suppress Noir']);
	});

	it('shows a visitor no toggle and no removed chips', async () => {
		const { props } = owned([{ value: 'Drama', sources: ['file'] }], [{ value: 'Noir' }]);
		const { container } = await render(CurationFieldRow, { ...props, isOwner: false });
		expect(toggle(container)).toBeNull();
		expect(container.textContent).not.toContain('Noir');
	});
});
