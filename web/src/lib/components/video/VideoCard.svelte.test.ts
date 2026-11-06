import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Video } from '#lib/types.ts';
import '../../../app.css';
import VideoCard from './VideoCard.svelte';

// W5: the watched mark (P0-9, testing-strategy §40.6). It follows the API's fact alone: the
// card never reads isOwner, and the API sends the fact to the owner only.

const video = (over: Partial<Video> = {}) =>
	({ id: 1, title: 'A title', duration_sec: 60, width: 1920, height: 1080, ...over }) as unknown as Video;
const bar = () => document.querySelector('.video-frame span[aria-hidden="true"].bg-accent');

describe('VideoCard watched mark', () => {
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

	it('keeps the card one height either way', async () => {
		const plain = await render(VideoCard, { video: video() });
		const h = plain.container.getBoundingClientRect().height;
		plain.unmount();
		const marked = await render(VideoCard, { video: video({ watched: true }) });
		expect(marked.container.getBoundingClientRect().height).toBe(h);
	});
});
