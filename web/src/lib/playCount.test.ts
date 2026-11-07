import { describe, expect, it, vi } from 'vitest';
import { PlayCounter } from './playCount';

// W3: what counts as a play (P0-7, SEC-9, testing-strategy §40.6). A real element fires
// timeupdate about every quarter second, and `seeking` before the position jumps.

const DURATION = 100;

function setup(isOwner = true) {
	const post = vi.fn();
	const c = new PlayCounter(() => isOwner, post);
	const play = (from: number, to: number) => {
		for (let t = from; t <= to; t += 0.25) c.timeupdate(t, DURATION);
	};
	const seek = (to: number) => {
		c.seeking();
		c.timeupdate(to, DURATION);
	};
	return { c, post, play, seek };
}

describe('PlayCounter', () => {
	it('does not count sampling the first minutes', () => {
		const { post, play } = setup();
		play(0, 30);
		expect(post).not.toHaveBeenCalled();
	});

	it('does not count seeking straight to the end', () => {
		const { post, play, seek } = setup();
		play(0, 5);
		seek(99);
		play(99, 100);
		expect(post).not.toHaveBeenCalled();
	});

	it('counts playing through, then seeking within the last 10%, once', () => {
		const { post, play, seek } = setup();
		play(0, 91);
		seek(98);
		play(98, 100);
		expect(post).toHaveBeenCalledTimes(1);
		expect(post.mock.calls[0][0]).toBeCloseTo(0.9, 1);
	});

	it('counts a watch one seek breaks up by the time played, not the position', () => {
		const { post, play, seek } = setup();
		play(0, 45);
		seek(50);
		play(50, 94);
		expect(post).not.toHaveBeenCalled();
		play(94, 96);
		expect(post).toHaveBeenCalledTimes(1);
	});

	it('counts watching twice as two plays', () => {
		const { post, play, seek } = setup();
		play(0, 100);
		seek(0);
		play(0, 100);
		expect(post).toHaveBeenCalledTimes(2);
	});

	it('starts over for another video', () => {
		const { c, post, play } = setup();
		play(0, 80);
		c.reset();
		play(0, 80);
		expect(post).not.toHaveBeenCalled();
	});

	it('posts nothing with no owner session', () => {
		const { post, play } = setup(false);
		play(0, 100);
		expect(post).not.toHaveBeenCalled();
	});

	it('waits for a known length', () => {
		const post = vi.fn();
		const c = new PlayCounter(() => true, post);
		for (let t = 0; t < 5; t += 0.25) c.timeupdate(t, NaN);
		expect(post).not.toHaveBeenCalled();
	});
});
