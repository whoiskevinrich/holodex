// The thumbnail retry shared by VideoCard and the Media Table's VideoThumb.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('#lib/api.ts', () => ({ api: { thumbnailReload: (url: string, n: number) => `${url}#${n}` } }));

import { MAX_RETRIES, ThumbnailLoad, retryDelay } from './thumbnailLoad.svelte';

describe('retryDelay', () => {
	it('doubles from 2s, caps at 30s, and jitters ±50%', () => {
		expect(retryDelay(0, 0.5)).toBe(2000);
		expect(retryDelay(1, 0.5)).toBe(4000);
		expect(retryDelay(4, 0.5)).toBe(30000); // 32s, capped
		expect(retryDelay(0, 0)).toBe(1000);
		expect(retryDelay(0, 0.999)).toBeCloseTo(2998, 0);
	});
});

describe('ThumbnailLoad', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('retries a 404 with a fresh src, then gives up after MAX_RETRIES', () => {
		const t = new ThumbnailLoad(() => '/thumb/7');
		expect(t.src).toBe('/thumb/7#0');
		for (let i = 1; i <= MAX_RETRIES; i++) {
			t.onerror();
			expect(t.attempt).toBe(i - 1); // nothing changes until the delay passes
			vi.runOnlyPendingTimers();
			expect(t.attempt).toBe(i);
			expect(t.src).toBe(`/thumb/7#${i}`);
		}
		expect(t.gaveUp).toBe(false);
		t.onerror();
		expect(t.gaveUp).toBe(true);
		expect(vi.getTimerCount()).toBe(0);
	});

	it('a load marks it loaded', () => {
		const t = new ThumbnailLoad(() => '/thumb/7');
		t.onload();
		expect(t.loaded).toBe(true);
		expect(t.gaveUp).toBe(false);
	});
});
