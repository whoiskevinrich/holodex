import { describe, it, expect, beforeEach } from 'vitest';
import { OwnerViewState } from './ownerView.svelte';

// Minimal in-memory localStorage stub (tests run in node, no DOM). The store
// guards on `typeof localStorage`, so assigning the global is enough.
function fakeStorage() {
	const m = new Map<string, string>();
	return {
		getItem: (k: string) => (m.has(k) ? (m.get(k) as string) : null),
		setItem: (k: string, v: string) => void m.set(k, v),
		removeItem: (k: string) => void m.delete(k),
		clear: () => m.clear(),
		key: () => null,
		length: 0
	} as unknown as Storage;
}

const KEY = 'holodex-owner-view';
const LEGACY_KEY = 'holodex-admin-mode';

describe('ownerView store (F29)', () => {
	let store: OwnerViewState;

	beforeEach(() => {
		(globalThis as { localStorage: Storage }).localStorage = fakeStorage();
		store = new OwnerViewState();
	});

	it('defaults ON', () => {
		expect(store.enabled).toBe(true);
	});

	it('set persists the boolean to localStorage', () => {
		store.set(false);
		expect(store.enabled).toBe(false);
		expect(localStorage.getItem(KEY)).toBe('false');
		store.set(true);
		expect(store.enabled).toBe(true);
		expect(localStorage.getItem(KEY)).toBe('true');
	});

	it('toggle flips the current value', () => {
		store.toggle();
		expect(store.enabled).toBe(false);
		store.toggle();
		expect(store.enabled).toBe(true);
	});

	it('init restores an explicit stored "false"', () => {
		localStorage.setItem(KEY, 'false');
		store.init();
		expect(store.enabled).toBe(false);
	});

	it('init keeps the default ON for an absent or garbage value', () => {
		store.init(); // absent
		expect(store.enabled).toBe(true);
		localStorage.setItem(KEY, 'nope');
		store.init(); // garbage
		expect(store.enabled).toBe(true);
	});

	it('init does not throw without localStorage', () => {
		delete (globalThis as { localStorage?: Storage }).localStorage;
		const s = new OwnerViewState();
		expect(() => s.init()).not.toThrow();
		expect(s.enabled).toBe(true);
	});

	it('reveal forces ON and announces (auto-reveal, P0-6)', () => {
		store.set(false);
		store.reveal();
		expect(store.enabled).toBe(true);
		expect(store.announcement).toBe('Owner view on.');
	});

	it('reveal is a no-op (no announcement) when already ON', () => {
		store.reveal();
		expect(store.enabled).toBe(true);
		expect(store.announcement).toBe('');
	});

	it('init migrates a legacy-key value to the new key and drops the legacy key', () => {
		localStorage.setItem(LEGACY_KEY, 'false');
		store.init();
		expect(store.enabled).toBe(false);
		expect(localStorage.getItem(KEY)).toBe('false');
		expect(localStorage.getItem(LEGACY_KEY)).toBeNull();
	});

	it('init prefers an existing new-key value over a stale legacy one', () => {
		localStorage.setItem(KEY, 'true');
		localStorage.setItem(LEGACY_KEY, 'false');
		store.init();
		expect(store.enabled).toBe(true);
		expect(localStorage.getItem(LEGACY_KEY)).toBeNull();
	});
});
