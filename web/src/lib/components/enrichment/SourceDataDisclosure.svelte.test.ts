import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import SourceDataDisclosure from './SourceDataDisclosure.svelte';
import type { EnrichedField } from '#lib/types.ts';

// HOLODEX-97 (media-page-restructure-handoff.md Part C): one disclosure, a source switcher,
// one raw key/value table, and image fields as URL text rather than a second poster.

const file = [
	{ source_key: 'title', value: 'From the file' },
	{ source_key: 'date', value: '2017' }
];
const tmdb: EnrichedField[] = [
	{ canonical: 'title', label: 'Title', values: ['Blade Runner 2049'], provider: 'tmdb' },
	{ canonical: 'poster_url', label: 'Poster', display: 'image_url', values: ['https://img/p.jpg'], provider: 'tmdb' }
];

describe('SourceDataDisclosure', () => {
	it('is closed at rest and counts every source and key', async () => {
		const screen = await render(SourceDataDisclosure, { file, providers: new Map([['tmdb', tmdb]]) });
		const trigger = screen.getByRole('button', { name: /Source data · 2 sources, 4 keys/ });
		await expect.element(trigger).toHaveAttribute('aria-expanded', 'false');
		const panel = document.getElementById(trigger.element().getAttribute('aria-controls')!);
		expect(panel?.hidden).toBe(true);
	});

	it('opens on File and switches sources with the arrow keys', async () => {
		const screen = await render(SourceDataDisclosure, { file, providers: new Map([['tmdb', tmdb]]) });
		await screen.getByRole('button', { name: /Source data/ }).click();
		await expect.element(screen.getByRole('radio', { name: 'File' })).toHaveAttribute('aria-checked', 'true');
		await expect.element(screen.getByText('From the file')).toBeVisible();

		(screen.getByRole('radio', { name: 'File' }).element() as HTMLElement).focus();
		await userEvent.keyboard('{ArrowRight}');
		await expect.element(screen.getByRole('radio', { name: 'tmdb' })).toHaveAttribute('aria-checked', 'true');
		expect(document.activeElement).toBe(screen.getByRole('radio', { name: 'tmdb' }).element());
		await expect.element(screen.getByText('Blade Runner 2049')).toBeVisible();
	});

	it('shows an image field as its URL, never an image', async () => {
		const screen = await render(SourceDataDisclosure, { file: [], providers: new Map([['tmdb', tmdb]]) });
		await screen.getByRole('button', { name: /Source data · 1 source, 2 keys/ }).click();
		await expect.element(screen.getByText('https://img/p.jpg')).toBeVisible();
		expect(document.querySelector('img')).toBeNull();
		// One source needs no switcher.
		await expect.element(screen.getByRole('radiogroup')).not.toBeInTheDocument();
	});

	it('renders nothing when no source has data', async () => {
		const screen = await render(SourceDataDisclosure, { file: [], providers: new Map() });
		await expect.element(screen.getByRole('button', { name: /Source data/ })).not.toBeInTheDocument();
	});
});
