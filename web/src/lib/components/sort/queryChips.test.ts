import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import FilterChip from './FilterChip.svelte';
import QueryChipGroup from './QueryChipGroup.svelte';
import AdvancedQueryRow from './AdvancedQueryRow.svelte';

// testing-strategy §38.3 W1/W6 markup: the NOT chip, the OR group's box and its spoken
// logic, and the Advanced query row (query-language-handoff.md "Components").
describe('F87 query chips', () => {
	it('draws a NOT chip outlined, a plain chip filled', () => {
		const not = render(FilterChip, { props: { label: 'not comedy', negated: true, onremove: () => {} } }).body;
		expect(not).toContain('border-accent');
		expect(not).not.toContain('bg-accent ');
		expect(not).toContain('aria-label="Remove filter: not comedy"');
		const plain = render(FilterChip, { props: { label: 'comedy', onremove: () => {} } }).body;
		expect(plain).toContain('bg-accent');
	});

	it('states a group’s logic for a screen reader and wraps an AND set in ( + )', () => {
		const body = render(QueryChipGroup, {
			props: {
				members: [
					[
						{ label: 'science-fiction', negated: false, path: [0, 0, 0] },
						{ label: 'fantasy', negated: false, path: [0, 0, 1] }
					],
					[{ label: 'action', negated: false, path: [0, 1] }]
				],
				onremove: () => {},
				onremovemember: () => {}
			}
		}).body;
		expect(body).toContain('role="group"');
		expect(body).toContain('aria-label="Any of: science-fiction and fantasy, or action"');
		expect(body).toContain('aria-label="Remove group: science-fiction and fantasy, or action"');
		const text = body.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');
		expect(text).toContain('( science-fiction × + fantasy × ) or action ×');
	});

	it('shows the Advanced query row with its own Edit as text', () => {
		const body = render(AdvancedQueryRow, { props: { text: '((a OR b) c) OR d', onedit: () => {} } }).body;
		expect(body).toContain('Advanced query');
		expect(body).toContain('aria-label="Advanced query: ((a OR b) c) OR d"');
		expect(body).toContain('Edit as text');
	});
});
