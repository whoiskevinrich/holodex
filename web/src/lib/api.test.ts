import { describe, it, expect } from 'vitest';
import { api } from './api';

// Person image serving URLs (F24, ADR-037). The frontend always appends the active
// skin (so the backend's empty-slot placeholder matches the current skin) and the
// `?v=` cache-buster only when a version is known — a replaced core slot gets a new
// version and so a new, non-cached URL.
describe('personImageURL', () => {
	it('builds skin + version query params', () => {
		expect(api.personImageURL(7, 'headshot', { skin: 'broadcast', version: 42 })).toBe(
			'/api/v1/people/7/image/headshot?skin=broadcast&v=42'
		);
	});

	it('omits v when no version is given', () => {
		expect(api.personImageURL(3, 'banner', { skin: 'cinematheque' })).toBe(
			'/api/v1/people/3/image/banner?skin=cinematheque'
		);
	});

	it('omits the query entirely with no opts', () => {
		expect(api.personImageURL(9, 'poster')).toBe('/api/v1/people/9/image/poster');
	});

	it('omits v when version is 0 (an unfilled slot)', () => {
		expect(api.personImageURL(1, 'headshot', { skin: 'brutalist', version: 0 })).toBe(
			'/api/v1/people/1/image/headshot?skin=brutalist'
		);
	});
});

describe('personGalleryImageURL', () => {
	it('stamps version and skin when present', () => {
		expect(api.personGalleryImageURL(5, 88, { version: 88, skin: 'broadcast' })).toBe(
			'/api/v1/people/5/images/88?skin=broadcast&v=88'
		);
	});

	it('omits the query when no opts', () => {
		expect(api.personGalleryImageURL(5, 88)).toBe('/api/v1/people/5/images/88');
	});
});
