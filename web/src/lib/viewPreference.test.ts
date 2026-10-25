// The per-page view preference (testing-strategy §33.1, cases V1–V2).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { PAGE_VIEWS, readView, writeView } from './viewPreference.svelte';

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

beforeEach(() => {
	vi.stubGlobal('localStorage', memoryStorage());
});

describe('V1 each page reads its own key, defaulting to its first view', () => {
	it('an empty store gives the page default', () => {
		expect(readView('people')).toBe(PAGE_VIEWS.people[0]);
		expect(readView('people')).toBe('list');
	});
	it('People keeps a choice stored before F82, under the same key', () => {
		localStorage.setItem('holodex:view:people', 'poster');
		expect(readView('people')).toBe('poster');
		localStorage.setItem('holodex:view:people', 'list');
		expect(readView('people')).toBe('list');
	});
	it('a value the page does not offer falls back to the default', () => {
		localStorage.setItem('holodex:view:people', 'garbage');
		expect(readView('people')).toBe('list');
	});
	it('storage that throws falls back to the default', () => {
		vi.spyOn(localStorage, 'getItem').mockImplementation(() => {
			throw new Error('denied');
		});
		expect(readView('people')).toBe('list');
	});
	it('with no storage at all, reading gives the default', () => {
		vi.stubGlobal('localStorage', undefined);
		expect(readView('people')).toBe('list');
	});
});

describe('V2 only an explicit choice writes', () => {
	it('reading makes no setItem call', () => {
		localStorage.setItem('holodex:view:people', 'poster');
		const set = vi.spyOn(localStorage, 'setItem');
		readView('people');
		expect(set).not.toHaveBeenCalled();
	});
	it("a write sets exactly that page's key", () => {
		const set = vi.spyOn(localStorage, 'setItem');
		writeView('people', 'poster');
		expect(set).toHaveBeenCalledTimes(1);
		expect(set).toHaveBeenCalledWith('holodex:view:people', 'poster');
		expect(readView('people')).toBe('poster');
	});
	it('a write to full storage is swallowed', () => {
		vi.spyOn(localStorage, 'setItem').mockImplementation(() => {
			throw new Error('quota');
		});
		expect(() => writeView('people', 'poster')).not.toThrow();
	});
});
