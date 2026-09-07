import { describe, expect, it } from 'vitest';
import { parseSceneNumberInput, sceneBadgeLabel } from './sceneNumber';

// sceneBadgeLabel is now read by two surfaces — the film detail page's Scenes grid
// (VideoCard) and the media detail page's Films chips — and the design requires them to
// read identically (docs/design/media-detail-films-people-handoff.md §5). Pin the
// vocabulary so a change on one surface can't silently diverge the other.
describe('sceneBadgeLabel', () => {
	it('renders a numbered scene as #N', () => {
		expect(sceneBadgeLabel(4)).toBe('#4');
	});

	it('renders an unnumbered scene as an em-dash, not an empty badge', () => {
		expect(sceneBadgeLabel(null)).toBe('—');
	});

	it('renders a full-film link as "Full", ignoring any scene number', () => {
		expect(sceneBadgeLabel(null, true)).toBe('Full');
		expect(sceneBadgeLabel(7, true)).toBe('Full');
	});

	it('defaults to the scene reading when the caller omits isFullFilm (VideoCard)', () => {
		expect(sceneBadgeLabel(1)).toBe('#1');
		expect(sceneBadgeLabel(null)).toBe('—');
	});
});

describe('parseSceneNumberInput', () => {
	it('treats a cleared field as "no scene number"', () => {
		expect(parseSceneNumberInput('')).toEqual({ value: null });
	});

	it('accepts a positive whole number from either the string or number form', () => {
		expect(parseSceneNumberInput('6')).toEqual({ value: 6 });
		expect(parseSceneNumberInput(6)).toEqual({ value: 6 });
	});

	it('rejects zero, negatives, and fractions', () => {
		for (const bad of [0, -1, 1.5]) {
			expect(parseSceneNumberInput(bad)).toHaveProperty('error');
		}
	});
});
