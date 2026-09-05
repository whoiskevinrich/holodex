import { describe, it, expect } from 'vitest';
import {
	buildPreviewItems,
	groupByVideo,
	isEntityField,
	makeFieldLabel,
	sortRows,
	stagePick,
	unstagePick,
	type StagedPicks
} from './extraction';
import type { ExtractionQueueRow } from './types';

function row(over: Partial<ExtractionQueueRow> & { id: number }): ExtractionQueueRow {
	return {
		video_id: 1,
		video_title: 'Film A',
		file_path: '/m/a.mkv',
		field_key: 'title',
		filename_value: 'New',
		tag_value: 'Old',
		confidence: 0.5,
		...over
	} as ExtractionQueueRow;
}

describe('makeFieldLabel', () => {
	it('resolves the people -> actors alias so both surfaces show the configured label', () => {
		// Without the alias this falls back to a titleized "People" while every other
		// field surface in the app says "Actors" for the same data.
		const label = makeFieldLabel({ actors: 'Actors', title: 'Title' });
		expect(label('people')).toBe('Actors');
	});

	it('titleizes an unregistered key rather than showing the raw key', () => {
		expect(makeFieldLabel({})('release_date')).toBe('Release Date');
	});

	it('prefers the registry label over the titleized fallback', () => {
		expect(makeFieldLabel({ title: 'Name' })('title')).toBe('Name');
	});
});

describe('field ordering', () => {
	it('sortRows puts a video’s fields in People, Studio, Title, Release date order, ties by key', () => {
		const sorted = sortRows([
			row({ id: 3, field_key: 'title' }),
			row({ id: 1, field_key: 'people' }),
			row({ id: 4, field_key: 'zz_custom' }),
			row({ id: 2, field_key: 'studio' })
		]);
		expect(sorted.map((r) => r.field_key)).toEqual(['people', 'studio', 'title', 'zz_custom']);
	});

	it('treats only People and Studio as entity fields', () => {
		expect(isEntityField('people')).toBe(true);
		expect(isEntityField('studio')).toBe(true);
		expect(isEntityField('title')).toBe(false);
		expect(isEntityField('release_date')).toBe(false);
	});
});

describe('groupByVideo', () => {
	it('sorts the video with the most pending fields first', () => {
		const groups = groupByVideo([
			row({ id: 1, video_id: 7, video_title: 'Solo', field_key: 'title' }),
			row({ id: 2, video_id: 9, video_title: 'Busy', field_key: 'title' }),
			row({ id: 3, video_id: 9, video_title: 'Busy', field_key: 'people' })
		]);
		expect(groups.map((g) => g.videoId)).toEqual([9, 7]);
		expect(groups[0].rows.map((r) => r.field_key)).toEqual(['people', 'title']);
	});

	it('returns a single group when every row shares one video', () => {
		const groups = groupByVideo([row({ id: 1 }), row({ id: 2, field_key: 'studio' })]);
		expect(groups).toHaveLength(1);
		expect(groups[0].filePath).toBe('/m/a.mkv');
	});

	it('is empty for an empty queue rather than throwing', () => {
		expect(groupByVideo([])).toEqual([]);
	});
});

describe('staging', () => {
	it('stages and unstages without mutating the previous object', () => {
		const empty: StagedPicks = {};
		const one = stagePick(empty, 5, 'filename', 'New');
		expect(empty).toEqual({});
		expect(one[5]).toEqual({ action: 'filename', value: 'New' });

		const back = unstagePick(one, 5);
		expect(one[5]).toBeDefined(); // the staged copy is untouched
		expect(back).toEqual({});
	});

	it('returns the same object when unstaging something not staged', () => {
		const staged = stagePick({}, 1, 'manual', 'X');
		expect(unstagePick(staged, 999)).toBe(staged);
	});

	it('overwrites an existing pick for the same row', () => {
		const staged = stagePick(stagePick({}, 1, 'filename', 'A'), 1, 'manual', 'B');
		expect(Object.keys(staged)).toHaveLength(1);
		expect(staged[1]).toEqual({ action: 'manual', value: 'B' });
	});
});

describe('buildPreviewItems', () => {
	const label = makeFieldLabel({ title: 'Title', actors: 'Actors' });

	function index(rows: ExtractionQueueRow[]) {
		return new Map(rows.map((r) => [r.id, r]));
	}

	it('builds an old -> new diff row per staged pick', () => {
		const rows = index([row({ id: 1, field_key: 'title', tag_value: 'Old Title' })]);
		const items = buildPreviewItems(stagePick({}, 1, 'filename', 'New Title'), rows, label);
		expect(items).toEqual([
			{
				reviewId: 1,
				videoTitle: 'Film A',
				fieldLabel: 'Title',
				oldValue: 'Old Title',
				newValue: 'New Title',
				action: 'filename'
			}
		]);
	});

	it('carries the aliased label through to the preview', () => {
		const rows = index([row({ id: 2, field_key: 'people', tag_value: '' })]);
		const items = buildPreviewItems(stagePick({}, 2, 'manual', 'Rin Hoshino'), rows, label);
		expect(items[0].fieldLabel).toBe('Actors');
	});

	it('skips a staged pick whose row has left the queue', () => {
		// The row was resolved or dismissed in another tab; rendering it against a
		// missing row would produce an item with no old value and no title.
		const items = buildPreviewItems(stagePick({}, 42, 'filename', 'X'), index([row({ id: 1 })]), label);
		expect(items).toEqual([]);
	});
});
