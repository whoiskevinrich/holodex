import { describe, expect, it } from 'vitest';
import type { VideoCompareSide, VideoDuplicatePair, VideoWork } from '$lib/types';
import {
	carryLines,
	factRows,
	labelError,
	sideSummary,
	videoDisclosureId,
	videoFocusLandingIds,
	workSummary
} from './videoPairs';
import { groupId, QUEUE_ID } from './queue';

// F76 (HOLODEX-521): the Videos group's pure rules — row summary, fact table presence,
// carry-over lines, label validation and the focus ladder.

const noWork: VideoWork = { playlists: 0, films: [], edits: 0, manual_tags: 0 };

function side(over: Partial<VideoCompareSide> = {}): VideoCompareSide {
	return {
		id: 1,
		title: 'Harbor Lights',
		file_path: '/m/a.mkv',
		file_name: 'a.mkv',
		folder: '/m',
		file_size: 6_657_199_309,
		duration_sec: 2054,
		width: 3840,
		height: 2160,
		video_codec: 'hevc',
		bitrate_kbps: 24100,
		container: 'mkv',
		work: noWork,
		if_kept: noWork,
		...over
	};
}

const pair = (a: number, b: number): VideoDuplicatePair => ({
	a: { id: a, title: 't', width: 1920, height: 1080, duration_sec: 60 },
	b: { id: b, title: 't', width: 1920, height: 1080, duration_sec: 60 }
});

describe('sideSummary', () => {
	it('reads as the resolution bucket and duration', () => {
		expect(sideSummary({ width: 3840, duration_sec: 2054 })).toBe('4K · 34:14');
		expect(sideSummary({ width: 1920, duration_sec: 3845 })).toBe('FHD · 1:04:05');
	});
});

describe('factRows', () => {
	it('keeps a row when either side has a value, leaving the other cell blank', () => {
		const rows = factRows(side(), side({ video_codec: undefined }));
		expect(rows.find((r) => r.key === 'codec')).toEqual({ key: 'codec', label: 'Video codec', a: 'hevc', b: '' });
	});

	it('drops a row neither side has, so Edition and Part appear only when set', () => {
		const keys = factRows(side(), side()).map((r) => r.key);
		expect(keys).not.toContain('edition');
		expect(keys).not.toContain('part');
		expect(keys).not.toContain('work');
		expect(factRows(side({ edition: 'Extended' }), side()).map((r) => r.key)).toContain('edition');
	});

	it('uses the media page formats', () => {
		const rows = Object.fromEntries(factRows(side(), side()).map((r) => [r.key, r.a]));
		expect(rows.resolution).toBe('4K · 3840×2160');
		expect(rows.duration).toBe('34:14');
		expect(rows.bitrate).toBe('24.1 Mbps');
	});
});

describe('workSummary and carryLines', () => {
	const work: VideoWork = {
		playlists: 2,
		films: [{ film_id: 3, name: 'Harbor Lights (2019)', scene_number: 3, is_full_film: false }],
		edits: 3,
		manual_tags: 1
	};

	it('summarizes the owner work compactly, empty when there is none', () => {
		expect(workSummary(work)).toBe('2 playlists · film link · 3 edits · 1 tag you added');
		expect(workSummary(noWork)).toBe('');
	});

	it('lists only what moves, naming the film and its scene', () => {
		expect(carryLines(work)).toEqual([
			'2 playlist places',
			'Film link — Harbor Lights (2019), scene 3',
			'3 field edits',
			'1 tag you added'
		]);
		expect(carryLines(noWork)).toEqual([]);
	});
});

describe('labelError', () => {
	it('needs at least one edition', () => {
		expect(labelError('edition', ' ', '')).toBe('Give at least one file an edition.');
		expect(labelError('edition', "Director's Cut", '')).toBe('');
	});

	it('needs two different positive part numbers', () => {
		expect(labelError('part', '1', '')).toBe('Give both files a part number.');
		expect(labelError('part', '0', '2')).toBe('Give both files a part number.');
		expect(labelError('part', 'one', '2')).toBe('Give both files a part number.');
		expect(labelError('part', '1', '01')).toBe('The two files need different part numbers.');
		expect(labelError('part', '1', '2')).toBe('');
	});
});

describe('videoFocusLandingIds', () => {
	it('lands on the next row, then the Videos heading, then the queue', () => {
		const a = pair(1, 2);
		const b = pair(3, 4);
		expect(videoFocusLandingIds(a, [a, b])).toEqual([videoDisclosureId(b), groupId('video'), QUEUE_ID]);
		expect(videoFocusLandingIds(b, [a, b])).toEqual([groupId('video'), QUEUE_ID]);
	});
});
