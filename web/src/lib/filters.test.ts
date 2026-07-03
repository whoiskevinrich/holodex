import { describe, it, expect } from 'vitest';
import { filtersToParams, paramsToFilters } from './filters';

// The studio-entity browse filter (F38, HOLODEX-120): repeatable ?studio_id round-trips
// through the shared filters<->URL codec, keeping api.ts and the browse page in lockstep.
describe('studio_id filter (F38)', () => {
	it('serializes each studio id as a repeatable studio_id param', () => {
		const p = filtersToParams({ studio_id: [3, 7] }, false);
		expect(p.getAll('studio_id')).toEqual(['3', '7']);
	});

	it('omits studio_id entirely when none are selected', () => {
		expect(filtersToParams({ studio_id: [] }, false).has('studio_id')).toBe(false);
		expect(filtersToParams({}, false).has('studio_id')).toBe(false);
	});

	it('parses repeatable studio_id back into a number[]', () => {
		const p = new URLSearchParams('studio_id=3&studio_id=7');
		expect(paramsToFilters(p).studio_id).toEqual([3, 7]);
	});

	it('drops non-numeric / zero studio_id values on parse', () => {
		const p = new URLSearchParams('studio_id=3&studio_id=abc&studio_id=0');
		expect(paramsToFilters(p).studio_id).toEqual([3]);
	});
});
