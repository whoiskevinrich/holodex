// ListController: the page-side half of ADR-114 (testing-strategy §20.1, risks 1–2).
import { describe, it, expect, vi, beforeEach } from 'vitest';

type AfterNav = (n: { to: { url: URL } | null }) => void;
const nav = vi.hoisted(() => ({
	goto: vi.fn(),
	replaceState: vi.fn(),
	afterNavigate: vi.fn(),
	last: null as null | ((n: { to: { url: URL } | null }) => void)
}));
vi.mock('$app/navigation', () => ({
	goto: nav.goto,
	replaceState: nav.replaceState,
	afterNavigate: (cb: AfterNav) => {
		nav.last = cb;
	}
}));

import { ListController } from './listController.svelte';
import { peopleSchema, tagsSchema, type ListSchema } from './listState';

function memoryStorage(): Storage {
	const m = new Map<string, string>();
	return {
		get length() {
			return m.size;
		},
		clear: () => m.clear(),
		getItem: (k) => m.get(k) ?? null,
		key: (i) => [...m.keys()][i] ?? null,
		removeItem: (k) => void m.delete(k),
		setItem: (k, v) => void m.set(k, String(v))
	};
}

function at(search: string, pathname = '/') {
	vi.stubGlobal('location', { search, pathname });
}
function navigateTo(path: string) {
	const url = new URL(path, 'http://x');
	at(url.search, url.pathname);
	nav.last?.({ to: { url } });
}

// Pages get the popstate listener through listController()'s $effect; here it's
// subscribed by hand and unsubscribed after each test.
const listeners: (() => void)[] = [];
function make<S extends string, Q>(schema: ListSchema<S, Q>, path: string): ListController<S, Q> {
	const c = new ListController(schema, path);
	listeners.push(c.listen());
	return c;
}

beforeEach(() => {
	while (listeners.length) listeners.pop()!();
	vi.stubGlobal('window', new EventTarget());
	vi.stubGlobal('localStorage', memoryStorage());
	vi.clearAllMocks();
	nav.last = null;
});

describe('ListController', () => {
	it('arrives from the URL without writing storage (risk 1)', () => {
		localStorage.setItem('holodex:sort:people', 'name');
		const set = vi.spyOn(localStorage, 'setItem');
		at('?sort=count');
		const c = make(peopleSchema, '/people');
		expect(c.state.sort).toBe('count');
		expect(set).not.toHaveBeenCalled();
	});

	it('writes the restored sort into a bare URL', () => {
		localStorage.setItem('holodex:sort:people', 'count');
		at('');
		make(peopleSchema, '/people');
		expect(nav.replaceState).toHaveBeenCalledWith('/people?sort=count', {});
	});

	it('a same-route nav to the bare path clears the query and keeps the saved sort (risk 2)', () => {
		localStorage.setItem('holodex:sort:tags', 'count');
		at('?sort=count&type=categories');
		const c = make(tagsSchema, '/tags');
		expect(c.state.query.type).toBe('categories');
		navigateTo('/tags');
		expect(c.state.query.type).toBe('all');
		expect(c.state.sort).toBe('count');
	});

	it('Back to an entry restores that entry’s query', () => {
		at('');
		const c = make(tagsSchema, '/tags');
		navigateTo('/tags?type=tags');
		expect(c.state.query.type).toBe('tags');
	});

	it('Back to a same-page entry re-reads the address bar (shallow entries skip afterNavigate)', () => {
		at('', '/tags');
		const c = make(tagsSchema, '/tags');
		at('?type=categories', '/tags');
		window.dispatchEvent(new Event('popstate'));
		expect(c.state.query.type).toBe('categories');
	});

	it('ignores a popstate that lands on another route', () => {
		at('?type=tags', '/tags');
		const c = make(tagsSchema, '/tags');
		at('?type=categories', '/tags/5');
		window.dispatchEvent(new Event('popstate'));
		expect(c.state.query.type).toBe('tags');
	});

	it('ignores navigations to other routes', () => {
		at('?type=tags');
		const c = make(tagsSchema, '/tags');
		navigateTo('/tags/5');
		expect(c.state.query.type).toBe('tags');
	});

	it('setSort writes the preference once and commits the URL', () => {
		at('');
		const c = make(peopleSchema, '/people');
		const set = vi.spyOn(localStorage, 'setItem');
		c.setSort('random');
		expect(set).toHaveBeenCalledExactlyOnceWith('holodex:sort:people', 'random');
		expect(nav.replaceState).toHaveBeenLastCalledWith('/people?sort=random', {});
	});

	it('setQuery commits without touching storage', () => {
		at('');
		const c = make(tagsSchema, '/tags');
		const set = vi.spyOn(localStorage, 'setItem');
		c.setQuery({ type: 'categories' });
		expect(set).not.toHaveBeenCalled();
		expect(nav.replaceState).toHaveBeenLastCalledWith('/tags?type=categories', {});
	});

	it('an owner-only sort in the URL applies once owner capabilities resolve', () => {
		at('?sort=completeness_asc');
		const c = make(peopleSchema, '/people');
		expect(c.state.sort).toBe('name');
		expect(nav.replaceState).not.toHaveBeenCalled();
		c.setOwner(true);
		expect(c.state.sort).toBe('completeness_asc');
	});

	it('key is the canonical query string', () => {
		at('?type=tags&sort=count');
		const c = make(tagsSchema, '/tags');
		expect(c.key).toBe('sort=count&type=tags');
	});
});
