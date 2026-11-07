import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Video } from '#lib/types.ts';
import '../../../app.css';
import VideoCard from './VideoCard.svelte';

const activity = vi.hoisted(() => ({ effectiveOwner: true }));
vi.mock('#lib/activity.svelte.ts', () => ({ activity }));

// W5: the watched mark (P0-9, testing-strategy §40.6). It needs the API's fact, which goes to the
// owner only, and Owner view on, so the owner's visitor preview shows what a visitor sees.

const video = (over: Partial<Video> = {}) =>
	({ id: 1, title: 'A title', duration_sec: 60, width: 1920, height: 1080, ...over }) as unknown as Video;
const bar = () => document.querySelector('.video-frame span[aria-hidden="true"].bg-accent');

describe('VideoCard watched mark', () => {
	beforeEach(() => {
		activity.effectiveOwner = true;
	});

	it('draws the bar and names it on the link when the API sends the fact', async () => {
		await render(VideoCard, { video: video({ watched: true }) });
		expect(bar()).not.toBeNull();
		expect(document.querySelector('a')?.textContent).toContain(', watched');
	});

	it('draws nothing without the fact', async () => {
		await render(VideoCard, { video: video() });
		expect(bar()).toBeNull();
		expect(document.querySelector('a')?.textContent).not.toContain('watched');
	});

	it('draws nothing while Owner view is off', async () => {
		activity.effectiveOwner = false;
		await render(VideoCard, { video: video({ watched: true }) });
		expect(bar()).toBeNull();
		expect(document.querySelector('a')?.textContent).not.toContain('watched');
	});

	it('keeps the card one height either way', async () => {
		const plain = await render(VideoCard, { video: video() });
		const h = plain.container.getBoundingClientRect().height;
		plain.unmount();
		const marked = await render(VideoCard, { video: video({ watched: true }) });
		expect(marked.container.getBoundingClientRect().height).toBe(h);
	});
});
