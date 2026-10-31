import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { resolveSelection, sourceChips } from '#lib/f36.ts';
import type { FieldCandidate, ResolvedField } from '#lib/types.ts';
import SourceImageTiles from './SourceImageTiles.svelte';

// F85 testing-strategy §37.4 W2–W4: groups, the no-poster line, one roving radiogroup.

const IMG = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';
const FRAME: FieldCandidate = { source: 'local:frame', value: 'local:frame#aa', image_url: IMG, frame_at: 760 };
const UPLOAD: FieldCandidate = { source: 'local:upload', value: 'local:upload#bb', image_url: IMG };
const P1: FieldCandidate = { source: 'provider:provider-1', provider: 'provider-1', value: IMG };

async function mount(candidates: FieldCandidate[], providers: string[] = [], decision?: ResolvedField['decision']) {
	const field: ResolvedField = { canonical: 'poster_url', label: 'Poster', display: 'image_url', values: [], candidates, decision };
	const chips = sourceChips(field).filter((c) => !c.manual || c.value);
	const selection = resolveSelection(field, chips);
	const screen = await render(SourceImageTiles, { field, chips, providers, selection, stagedKey: selection.key });
	return screen.container;
}
const radios = (el: HTMLElement) => [...el.querySelectorAll<HTMLElement>('[role=radio]')];
const checkedKey = (el: HTMLElement) => el.querySelector('[aria-checked=true]')?.getAttribute('data-seg');

describe('SourceImageTiles', () => {
	it('renders a no-poster provider as caption + text, with no radio (W2)', async () => {
		const el = await mount([{ source: 'file', value: '' }], ['provider-2']);
		const group = el.querySelector('[data-tile-group="provider:provider-2"]')!;
		expect(group.textContent).toContain('provider-2');
		expect(group.textContent).toContain('no poster');
		expect(group.querySelector('[role=radio]')).toBeNull();
		expect(group.querySelector('.border-dashed')).toBeNull();
	});

	it('shows the no cover / art placeholder for a file with no cover art (W2)', async () => {
		const el = await mount([{ source: 'file', value: '' }, P1]);
		const file = el.querySelector('[data-seg=file]')!;
		expect(file.querySelector('img')).toBeNull();
		expect(file.textContent).toContain('no cover');
		expect(file.textContent).toContain('art');
	});

	it('is one radiogroup whose arrows walk every tile across groups, skipping no-poster providers (W3)', async () => {
		const el = await mount([{ source: 'file', value: '' }, P1, FRAME, UPLOAD], ['provider-1', 'provider-2'], {
			source: 'local:upload',
			standing: true
		});
		expect(el.querySelectorAll('[role=radiogroup]')).toHaveLength(1);
		expect(radios(el).map((r) => r.dataset.seg)).toEqual(['file', 'local:frame', 'local:upload', 'provider:provider-1']);
		expect(radios(el).filter((r) => r.tabIndex === 0).map((r) => r.dataset.seg)).toEqual(['local:upload']);

		radios(el)[2].focus();
		await userEvent.keyboard('{ArrowRight}'); // last Holodex tile → first tile of the next group
		expect(checkedKey(el)).toBe('provider:provider-1');
		expect(document.activeElement?.getAttribute('data-seg')).toBe('provider:provider-1');
		await userEvent.keyboard('{ArrowRight}'); // last tile wraps to ·file, never the no-poster group
		expect(checkedKey(el)).toBe('file');
		await userEvent.keyboard('{ArrowLeft}');
		expect(checkedKey(el)).toBe('provider:provider-1');
	});

	it('marks selection beyond colour and titles the frame tile (W4)', async () => {
		const el = await mount([{ source: 'file', value: '' }, FRAME], [], { source: 'local:frame', standing: true });
		const frame = el.querySelector<HTMLElement>('[data-seg="local:frame"]')!;
		expect(frame.getAttribute('aria-checked')).toBe('true');
		expect(frame.querySelector('.bg-accent')).not.toBeNull();
		expect(frame.getAttribute('aria-label')).toBe('Poster from frame');
		expect(frame.title).toBe('frame at 12:40');
		expect(el.querySelector('[data-seg=file]')!.getAttribute('aria-checked')).toBe('false');
	});
});
