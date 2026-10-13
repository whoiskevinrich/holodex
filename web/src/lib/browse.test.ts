import { beforeEach, describe, expect, it } from 'vitest';
import { browseCache, forgetVideo, type BrowseSnapshot } from './browse.svelte';
import type { Video } from './types';

const vid = (id: number) => ({ id }) as Video;

function snap(over: Partial<BrowseSnapshot> = {}): BrowseSnapshot {
	return { key: 'sort=title_asc', videos: [vid(1), vid(2), vid(3)], total: 10, offset: 0, scrollY: 900, ...over };
}

beforeEach(() => browseCache.clear());

describe('forgetVideo (HOLODEX-478)', () => {
	it('drops a removed video from the pending snapshot, keeping it pending with its scroll', () => {
		browseCache.save(snap());
		forgetVideo(2);
		expect(browseCache.take('sort=title_asc')).toEqual(snap({ videos: [vid(1), vid(3)], total: 9, offset: -1 }));
	});

	it('shifts the paging offset so the next "Load more" page skips nothing', () => {
		// Two pages loaded (offset is the last page's start); the server list shrank by one,
		// so the next page starts one earlier than it would have.
		browseCache.save(snap({ offset: 50 }));
		forgetVideo(1);
		expect(browseCache.take('sort=title_asc')?.offset).toBe(49);
	});

	it('leaves the snapshot alone when the video is not in the loaded set', () => {
		browseCache.save(snap());
		forgetVideo(99);
		expect(browseCache.take('sort=title_asc')).toEqual(snap());
	});

	it('is a no-op with no pending snapshot', () => {
		forgetVideo(2);
		expect(browseCache.take('sort=title_asc')).toBeNull();
	});
});
