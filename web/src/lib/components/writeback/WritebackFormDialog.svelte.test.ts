import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { ResolvedField } from '#lib/types.ts';
import WritebackFormDialog from './WritebackFormDialog.svelte';

// F85 testing-strategy §37.4 W5: the Poster row is a cockpit row, and the old upload note and
// `cover art / in file` placeholder are gone.

const IMG = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';

function poster(over: Partial<ResolvedField> = {}): ResolvedField {
	return {
		canonical: 'poster_url',
		label: 'Poster',
		display: 'image_url',
		values: ['local:upload#bb'],
		winning_source: 'local:upload:poster_url',
		decision: { source: 'local:upload', standing: true },
		in_sync: false,
		write_target: 'cover.jpg',
		candidates: [
			{ source: 'file', value: '', image_url: IMG },
			{ source: 'local:frame', value: 'local:frame#aa', image_url: IMG, frame_at: 12 },
			{ source: 'local:upload', value: 'local:upload#bb', image_url: IMG }
		],
		...over
	};
}

async function mount(field: ResolvedField) {
	const writeback = vi.fn(async () => ({}));
	const decide = vi.fn(async () => ({}));
	await render(WritebackFormDialog, {
		fields: [field],
		videoId: 7,
		filePath: '/media/clip.mkv',
		providers: ['provider-1'],
		onclose: () => {},
		onenqueued: () => {},
		writeback,
		decide
	});
	return { writeback, decide };
}

describe('WritebackFormDialog Poster row', () => {
	it('writes a decided upload: ↧, counted in the footer, its token sent unchanged', async () => {
		const { writeback, decide } = await mount(poster());
		const upload = document.querySelector<HTMLElement>('[data-seg="local:upload"]')!;
		expect(upload.getAttribute('aria-checked')).toBe('true');
		expect([...document.querySelectorAll('svg title')].map((t) => t.textContent)).toContain('Will be written to the file');
		const write = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Write 1 field to file'))!;
		expect(write).toBeTruthy();
		// The `no poster` group for a matched provider with no candidate.
		expect(document.querySelector('[data-tile-group="provider:provider-1"]')?.textContent).toContain('no poster');

		write.click();
		await vi.waitFor(() => expect(writeback).toHaveBeenCalled());
		expect(decide).not.toHaveBeenCalled();
		expect(writeback).toHaveBeenCalledWith(7, {
			fields: [{ field: 'poster_url', values: ['local:upload#bb'], source: 'local:upload:poster_url' }]
		});
	});

	it('decides a re-pick before writing it', async () => {
		const { writeback, decide } = await mount(poster());
		document.querySelector<HTMLElement>('[data-seg="local:frame"]')!.click();
		await vi.waitFor(() => expect(document.querySelector('[data-seg="local:frame"]')?.getAttribute('aria-checked')).toBe('true'));
		[...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Write 1 field'))!.click();
		await vi.waitFor(() => expect(writeback).toHaveBeenCalled());
		expect(decide).toHaveBeenCalledWith('poster_url', 'local:frame', undefined);
	});

	it('shows no upload note and no cover art / in file placeholder', async () => {
		await mount(poster({ candidates: [{ source: 'file', value: '' }, { source: 'local:upload', value: 'local:upload#bb', image_url: IMG }] }));
		const text = document.body.textContent ?? '';
		expect(text).not.toContain('Your uploaded poster stays on the page');
		expect(text).not.toContain('in file');
		expect(document.querySelector('[data-seg=file]')?.textContent).toContain('no cover');
	});

	it('opens an undecided poster on a file with cover art as ·file, in sync (backend 8eb96dba)', async () => {
		const { writeback } = await mount(
			poster({
				values: ['file#ff'],
				winning_source: 'file:poster_url',
				decision: { source: 'file', standing: false },
				in_sync: true,
				candidates: [
					{ source: 'file', value: 'file#ff', image_url: IMG },
					{ source: 'provider:provider-1', provider: 'provider-1', value: IMG }
				]
			})
		);
		const file = document.querySelector('[data-seg=file]')!;
		expect(file.getAttribute('aria-checked')).toBe('true');
		expect(file.getAttribute('aria-label')).toBe('Poster from file');
		expect(document.body.textContent).toContain('file poster');
		expect(document.body.textContent).toContain('— matches the file');
		expect(document.body.textContent).not.toContain('pending');
		expect([...document.querySelectorAll('img')].every((i) => !i.getAttribute('src')?.startsWith('file#'))).toBe(true);
		const write = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Write 0 fields'))!;
		expect(write).toBeTruthy();
		expect(write.disabled).toBe(true);
		expect(writeback).not.toHaveBeenCalled();
	});

	it('collapses a written local poster to = with its tile image, never the token', async () => {
		await mount(poster({ in_sync: true }));
		const text = document.body.textContent ?? '';
		expect(text).toContain('— matches the file');
		expect(text).toContain('upload poster');
		expect(text).not.toContain('local:upload#');
		expect([...document.querySelectorAll('img')].every((i) => !i.getAttribute('src')?.startsWith('local:'))).toBe(true);
	});
});
