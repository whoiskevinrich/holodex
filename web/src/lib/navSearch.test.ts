import { describe, it, expect } from 'vitest';
import { pageScopeFor } from './navSearch.svelte';

// NS2/NS6 (HOLODEX-249): pageScopeFor is pure and table-driven — every route id
// maps to its in-place scope (or null) with no DOM, per the testing-strategy's
// "plain Vitest table test independent of any component."
describe('pageScopeFor', () => {
	it('maps each top-level list route to its own scope', () => {
		expect(pageScopeFor('/')).toBe('videos');
		expect(pageScopeFor('/people')).toBe('people');
		expect(pageScopeFor('/studios')).toBe('studios');
		expect(pageScopeFor('/tags')).toBe('tags');
	});

	it('maps person/studio/tag detail routes to Videos (NS6)', () => {
		expect(pageScopeFor('/people/[id]')).toBe('videos');
		expect(pageScopeFor('/studios/[id]')).toBe('videos');
		expect(pageScopeFor('/tags/[id]')).toBe('videos');
	});

	it('has no scope for the category detail route (no embedded video list)', () => {
		expect(pageScopeFor('/categories/[id]')).toBeNull();
	});

	it('has no scope for routes with no single-entity list, or no matched route', () => {
		expect(pageScopeFor('/search')).toBeNull();
		expect(pageScopeFor('/owner')).toBeNull();
		expect(pageScopeFor(null)).toBeNull();
	});
});
