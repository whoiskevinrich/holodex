import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Video } from '#lib/types.ts';

vi.mock('$app/navigation', () => ({ beforeNavigate: () => {} }));
const { default: VirtualVideos } = await import('./__fixtures__/VirtualVideos.svelte');

// The video card's row-height contract (F80 EL15, HOLODEX-604, testing-strategy §32.2):
// every card is one height whatever its title, and the placeholder matches it, so the
// scrollbar, the chip and Back land on the right video in the longest list.

const TOTAL = 400;
const TITLES = ['A', 'x'.repeat(40), 'A long title, word after word, '.repeat(50)];
const video = (i: number) =>
	({ id: i + 1, title: TITLES[i % 3], duration_sec: 60, width: 1920, height: 1080 }) as unknown as Video;

describe('VirtualVideoGrid', () => {
	it('draws loaded cards (one-line, two-line and over-long titles) and placeholders at one height', async () => {
		let release = () => {};
		const held = new Promise<void>((r) => (release = r));
		const fetch = vi.fn(async (offset: number, limit: number) => {
			if (offset > 0) await held; // page 2 onwards stays placeholders until released
			return { items: Array.from({ length: Math.min(limit, TOTAL - offset) }, (_, k) => video(offset + k)), total: TOTAL };
		});
		render(VirtualVideos, { fetch });
		const cells = () => [...document.querySelectorAll<HTMLElement>('[role="list"] [data-index]')];
		await expect.poll(() => cells().filter((c) => c.getAttribute('aria-busy') !== 'true').length).toBeGreaterThan(2);
		const loaded = cells().filter((c) => c.getAttribute('aria-busy') !== 'true');
		const heights = new Set(loaded.map((c) => c.offsetHeight));
		expect(heights.size, `loaded card heights ${[...heights]}`).toBe(1);

		// Scroll down to the held page: its cells are placeholders, at the same height.
		const cardHeight = [...heights][0];
		window.scrollTo(0, document.documentElement.scrollHeight / 2);
		window.dispatchEvent(new Event('scroll'));
		await expect.poll(() => cells().some((c) => c.getAttribute('aria-busy') === 'true')).toBe(true);
		const busy = cells().filter((c) => c.getAttribute('aria-busy') === 'true');
		expect(new Set(busy.map((c) => c.offsetHeight))).toEqual(new Set([cardHeight]));
		release();
		window.scrollTo(0, 0);
	});
});
