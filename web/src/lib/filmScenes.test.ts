import { describe, expect, it } from 'vitest';
import { sortScenes } from './filmScenes';

describe('sortScenes', () => {
	it('puts numbered scenes in order, unnumbered after them in API order', () => {
		const s = [
			{ id: 'a', scene_number: 3 },
			{ id: 'b', scene_number: null },
			{ id: 'c', scene_number: 1 },
			{ id: 'd' },
			{ id: 'e', scene_number: 2 }
		];
		expect(sortScenes(s).map((x) => x.id)).toEqual(['c', 'e', 'a', 'b', 'd']);
	});

	it('leaves its input alone', () => {
		const s = [{ scene_number: 2 }, { scene_number: 1 }];
		sortScenes(s);
		expect(s.map((x) => x.scene_number)).toEqual([2, 1]);
	});
});
