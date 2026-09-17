import { describe, expect, it } from 'vitest';
import { partBadgeLabel } from './partBadge';

// Four surfaces read this label and the design requires them to read identically
// (docs/design/media-parts-handoff.md §5); pin the vocabulary so one surface can't
// silently diverge from the others.
describe('partBadgeLabel', () => {
	it('renders the long form, "Part N"', () => {
		expect(partBadgeLabel('2')).toBe('Part 2');
		expect(partBadgeLabel(2)).toBe('Part 2');
	});

	it("does not normalise — that is the lifter's job, and a curated value is verbatim (RD8)", () => {
		expect(partBadgeLabel(' 02 ')).toBe('Part 02');
		expect(partBadgeLabel('two')).toBe('Part two');
	});
});
