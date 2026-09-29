import { beforeEach, describe, expect, it } from 'vitest';
import { createNavSnapshotRegistry, type Keyed } from './navSnapshot.svelte';

interface Snap extends Keyed {
	scrollY: number;
}

// Node has no DOM: the same Storage stub the filterPreference/adminMode tests use.
function fakeStorage(): Storage {
	const m = new Map<string, string>();
	return {
		getItem: (k) => m.get(k) ?? null,
		setItem: (k, v) => void m.set(k, v),
		removeItem: (k) => void m.delete(k),
		clear: () => m.clear(),
		key: () => null,
		length: 0
	};
}

function throwingStorage(): Storage {
	const boom = () => {
		throw new Error('SecurityError');
	};
	return { getItem: boom, setItem: boom, removeItem: boom, clear: boom, key: boom, length: 0 };
}

function setStorage(s: Storage | undefined) {
	(globalThis as { sessionStorage?: Storage }).sessionStorage = s;
}

beforeEach(() => setStorage(fakeStorage()));

describe('createNavSnapshotRegistry (ADR-032, ADR-118)', () => {
	it('restores after a full reload — a fresh registry over the same sessionStorage', () => {
		createNavSnapshotRegistry<Snap>('ns').save('people', { key: 'sort=name', scrollY: 1200 });
		const afterReload = createNavSnapshotRegistry<Snap>('ns');
		expect(afterReload.take('people', 'sort=name')).toEqual({ key: 'sort=name', scrollY: 1200 });
	});

	it('is one-shot across a reload: take clears the stored slot', () => {
		createNavSnapshotRegistry<Snap>('ns').save('people', { key: 'k', scrollY: 5 });
		const reg = createNavSnapshotRegistry<Snap>('ns');
		expect(reg.take('people', 'k')).not.toBeNull();
		expect(reg.take('people', 'k')).toBeNull();
		expect(sessionStorage.getItem('ns:people')).toBeNull();
	});

	it('is stale-on-mismatch across a reload, and the mismatch still clears it', () => {
		createNavSnapshotRegistry<Snap>('ns').save('people', { key: 'sort=name', scrollY: 5 });
		const reg = createNavSnapshotRegistry<Snap>('ns');
		expect(reg.take('people', 'sort=added')).toBeNull();
		expect(reg.take('people', 'sort=name')).toBeNull();
	});

	it('keeps ids isolated', () => {
		const reg = createNavSnapshotRegistry<Snap>('ns');
		reg.save('people', { key: 'k', scrollY: 1 });
		reg.save('studios', { key: 'k', scrollY: 2 });
		expect(reg.take('people', 'nope')).toBeNull();
		expect(createNavSnapshotRegistry<Snap>('ns').take('studios', 'k')?.scrollY).toBe(2);
	});

	it('clear removes the stored slot too', () => {
		const reg = createNavSnapshotRegistry<Snap>('ns');
		reg.save('people', { key: 'k', scrollY: 1 });
		reg.clear('people');
		expect(createNavSnapshotRegistry<Snap>('ns').take('people', 'k')).toBeNull();
	});

	it('ignores a corrupt stored value', () => {
		sessionStorage.setItem('ns:people', '{not json');
		expect(createNavSnapshotRegistry<Snap>('ns').take('people', 'k')).toBeNull();
	});

	it('falls back to memory when sessionStorage throws or is missing', () => {
		for (const s of [throwingStorage(), undefined]) {
			setStorage(s);
			const reg = createNavSnapshotRegistry<Snap>('ns');
			reg.save('people', { key: 'k', scrollY: 7 });
			expect(reg.take('people', 'k')?.scrollY).toBe(7);
			expect(reg.take('people', 'k')).toBeNull();
		}
	});
});
