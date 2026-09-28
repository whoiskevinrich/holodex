// The ADR-114 list-state contract (testing-strategy §20.1, cases U1–U9 and S1–S3).
import { describe, it, expect, vi, beforeEach, expectTypeOf } from 'vitest';

const nav = vi.hoisted(() => ({ goto: vi.fn(), replaceState: vi.fn(), pushState: vi.fn() }));
vi.mock('$app/navigation', () => nav);

import {
	ENTITY_SORTS,
	FILM_SORTS,
	TAG_SORTS,
	chooseSort,
	commit,
	exitAfterRemoval,
	filmsSchema,
	mediaSchema,
	parseList,
	peopleSchema,
	serialize,
	stateKey,
	studiosSchema,
	tagsSchema,
	type ListState
} from './listState';
import { MEDIA_SORTS } from './filters';

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

const q = (s: string) => new URLSearchParams(s);

// Vitest runs in node here: no DOM, so `history` is stubbed alongside storage.
const hist = { back: vi.fn(), replaceState: vi.fn(), pushState: vi.fn() };

beforeEach(() => {
	vi.stubGlobal('localStorage', memoryStorage());
	vi.stubGlobal('history', hist);
	vi.clearAllMocks();
});

describe('U1 sort precedence: URL → saved → default', () => {
	it('a valid URL sort beats a valid saved one', () => {
		localStorage.setItem('holodex:sort:people', 'random');
		expect(parseList(peopleSchema, q('sort=count'), false).state.sort).toBe('count');
	});
	it('the saved sort fills an absent URL sort', () => {
		localStorage.setItem('holodex:sort:people', 'count');
		expect(parseList(peopleSchema, q(''), false).state.sort).toBe('count');
	});
	it('invalid values at either level fall through to the default without throwing', () => {
		localStorage.setItem('holodex:sort:people', 'garbage');
		expect(parseList(peopleSchema, q('sort=nonsense'), false).state.sort).toBe('name');
	});
});

describe('U2 owner-only sorts never reach a visitor', () => {
	it('drops a completeness sort from the URL for a visitor, keeps it for the owner', () => {
		expect(parseList(peopleSchema, q('sort=completeness_asc'), false).state.sort).toBe('name');
		expect(parseList(peopleSchema, q('sort=completeness_asc'), true).state.sort).toBe('completeness_asc');
	});
	it('drops a saved completeness sort for a visitor', () => {
		localStorage.setItem('holodex:sort:studios', 'completeness_desc');
		expect(parseList(studiosSchema, q(''), false).state.sort).toBe('name');
		expect(parseList(studiosSchema, q(''), true).state.sort).toBe('completeness_desc');
	});
	it('leaves an owner-only URL sort in place while capabilities may still be loading', () => {
		// An owner is briefly indistinguishable from a visitor; rewriting the URL then would
		// lose the owner's sort before their capabilities resolve.
		expect(parseList(peopleSchema, q('sort=completeness_asc'), false).syncUrl).toBe(false);
	});
	it('Media applies the same rule to its owner-only entries', () => {
		expect(parseList(mediaSchema, q('sort=completeness_desc'), false).state.sort).toBe('added_desc');
		expect(parseList(mediaSchema, q('sort=completeness_desc'), true).state.sort).toBe('completeness_desc');
	});
});

describe('U3 query state comes only from the URL', () => {
	it('ignores legacy SP5 saved filters on a bare URL', () => {
		localStorage.setItem('holodex:filters:media', 'resolution=4k&year_min=2015');
		localStorage.setItem('holodex:filters:tags', JSON.stringify('categories'));
		const media = parseList(mediaSchema, q(''), true).state.query;
		expect(media.resolution).toBe('All');
		expect(media.year_min).toBeUndefined();
		expect(parseList(tagsSchema, q(''), true).state.query.type).toBe('all');
	});
	it('reads the query from the URL', () => {
		expect(parseList(tagsSchema, q('type=categories'), false).state.query.type).toBe('categories');
		expect(parseList(mediaSchema, q('resolution=4K&studio_id=3'), false).state.query.studio_id).toEqual([3]);
	});
});

describe('U4 only the change handler writes storage', () => {
	it('parsing (arrival by link, Back or redirect) makes no storage writes', () => {
		const set = vi.spyOn(localStorage, 'setItem');
		parseList(peopleSchema, q('sort=count'), true);
		parseList(mediaSchema, q('sort=title_asc&resolution=4K'), true);
		parseList(tagsSchema, q('type=tags'), false);
		expect(set).not.toHaveBeenCalled();
	});
	it('choosing a sort writes exactly that page key', () => {
		const set = vi.spyOn(localStorage, 'setItem');
		chooseSort(peopleSchema, 'count');
		expect(set).toHaveBeenCalledTimes(1);
		expect(set).toHaveBeenCalledWith('holodex:sort:people', 'count');
	});
});

describe('U5 serialize is canonical and round-trips', () => {
	it('omits the default sort and sorts keys', () => {
		expect(serialize(peopleSchema, { sort: 'name', query: {} }).toString()).toBe('');
		const s = serialize(mediaSchema, {
			sort: 'title_asc',
			query: { year_min: 2015, resolution: '4K', studio_id: [3] }
		});
		expect(s.toString()).toBe('resolution=4K&sort=title_asc&studio_id=3&year_min=2015');
	});
	it('never carries the shuffle seed', () => {
		const s = serialize(mediaSchema, { sort: 'random', query: {} });
		expect(s.has('seed')).toBe(false);
	});
	it('round-trips parse(serialize(state)) for every schema', () => {
		const cases = [
			[peopleSchema, { sort: 'count', query: {} }],
			[studiosSchema, { sort: 'completeness_asc', query: {} }],
			[filmsSchema, { sort: 'random', query: {} }],
			[tagsSchema, { sort: 'count', query: { type: 'categories' } }]
		] as const;
		for (const [schema, state] of cases) {
			const back = parseList(schema as typeof peopleSchema, serialize(schema as typeof peopleSchema, state as never), true);
			expect(back.state).toEqual(state);
		}
		const media = { sort: 'duration_desc', query: { resolution: '4K', year_min: 2015, person: [4] } } as const;
		const parsed = parseList(mediaSchema, serialize(mediaSchema, media as never), true).state;
		expect(parsed.sort).toBe('duration_desc');
		expect(serialize(mediaSchema, parsed).toString()).toBe(serialize(mediaSchema, media as never).toString());
	});
});

describe('Media query edge cases', () => {
	it('passes mapped-facet params through a round trip (their keys load later)', () => {
		const url = q('genre=Drama&resolution=4K&sort=title_asc');
		const { state } = parseList(mediaSchema, url, false);
		expect(state.query.mapped).toEqual({ genre: 'Drama' });
		expect(serialize(mediaSchema, state).toString()).toBe('genre=Drama&resolution=4K&sort=title_asc');
	});
	it('drops the retired missing_facet filter (F73 R6) instead of filtering with no chip', () => {
		const { state } = parseList(mediaSchema, q('missing_facet=poster&year_min=2015'), true);
		expect(state.query).not.toHaveProperty('missing_facet');
		expect(state.query.mapped).toBeUndefined();
		expect(serialize(mediaSchema, state).toString()).toBe('year_min=2015');
	});
});

describe('U6 the address bar shows the restored sort', () => {
	it('asks for a URL sync when a non-default sort was restored from storage', () => {
		localStorage.setItem('holodex:sort:people', 'count');
		expect(parseList(peopleSchema, q(''), false).syncUrl).toBe(true);
	});
	it('does not when the resolved sort is the default', () => {
		expect(parseList(peopleSchema, q(''), false).syncUrl).toBe(false);
	});
	it('does not when the URL already carries the sort', () => {
		expect(parseList(peopleSchema, q('sort=count'), false).syncUrl).toBe(false);
	});
	it('does when the URL sort is garbage and a saved one replaced it', () => {
		localStorage.setItem('holodex:sort:people', 'random');
		expect(parseList(peopleSchema, q('sort=bogus'), false).syncUrl).toBe(true);
	});
	it('does when a garbage URL sort resolved to the default, so the junk leaves the URL', () => {
		expect(parseList(peopleSchema, q('sort=bogus'), false).syncUrl).toBe(true);
	});
});

describe('U7 commit replaces, never pushes', () => {
	it('calls SvelteKit replaceState once with the canonical URL', () => {
		commit('/people', serialize(peopleSchema, { sort: 'count', query: {} }));
		expect(nav.replaceState).toHaveBeenCalledTimes(1);
		expect(nav.replaceState).toHaveBeenCalledWith('/people?sort=count', {});
		expect(nav.pushState).not.toHaveBeenCalled();
		expect(hist.replaceState).not.toHaveBeenCalled();
		expect(hist.pushState).not.toHaveBeenCalled();
	});
	it('keeps a pristine view at the bare path', () => {
		commit('/people', new URLSearchParams());
		expect(nav.replaceState).toHaveBeenCalledWith('/people', {});
	});
	it('retries on the next macrotask when the router is still initialising', () => {
		vi.useFakeTimers();
		nav.replaceState.mockImplementationOnce(() => {
			throw new Error('router not started');
		});
		commit('/people', new URLSearchParams('sort=count'));
		vi.runAllTimers();
		vi.useRealTimers();
		expect(nav.replaceState).toHaveBeenCalledTimes(2);
		expect(nav.replaceState).toHaveBeenLastCalledWith('/people?sort=count', {});
	});
});

describe('U8 exitAfterRemoval', () => {
	it('goes back when the page was reached in-app', () => {
		exitAfterRemoval(true, '/');
		expect(hist.back).toHaveBeenCalledTimes(1);
		expect(nav.goto).not.toHaveBeenCalled();
	});
	it('goes to the bare list after a direct arrival', () => {
		exitAfterRemoval(false, '/people');
		expect(nav.goto).toHaveBeenCalledWith('/people');
		expect(hist.back).not.toHaveBeenCalled();
	});
});

describe('U9 equal views share one snapshot key', () => {
	it('is independent of the order filters were entered in', () => {
		const a = mediaSchema.parseQuery(q('year_min=2015&resolution=4K'));
		const b = mediaSchema.parseQuery(q('resolution=4K&year_min=2015'));
		expect(stateKey(mediaSchema, { sort: 'added_desc', query: a })).toBe(
			stateKey(mediaSchema, { sort: 'added_desc', query: b })
		);
	});
});

describe('S1–S3 sort lists', () => {
	const lists = { MEDIA_SORTS, ENTITY_SORTS, TAG_SORTS, FILM_SORTS };

	it('S1 every label fits the 375px toolbar budget (≤ 20 characters)', () => {
		for (const [name, list] of Object.entries(lists)) {
			for (const s of list) expect(s.label.length, `${name}: ${s.label}`).toBeLessThanOrEqual(20);
		}
	});
	it('S2 completeness is owner-only everywhere it appears, and absent from Tags', () => {
		for (const list of [MEDIA_SORTS, ENTITY_SORTS]) {
			const c = list.filter((s) => s.value.startsWith('completeness'));
			expect(c).toHaveLength(2);
			expect(c.every((s) => s.ownerOnly)).toBe(true);
		}
		expect(TAG_SORTS.some((s) => s.ownerOnly || s.value.startsWith('completeness'))).toBe(false);
	});
	it('S3 a page state has exactly one sort field', () => {
		expectTypeOf<keyof ListState<'a' | 'b', object>>().toEqualTypeOf<'sort' | 'query'>();
	});
});
