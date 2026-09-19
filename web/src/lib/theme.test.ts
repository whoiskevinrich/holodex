import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ThemeState, CUSTOM_TOKENS } from './theme.svelte';
import { api } from './api';
import type { ThemeCapability, ThemeCustom } from './types';

// Node has no DOM: a minimal <html> stand-in exposing what apply() touches —
// dataset.theme and inline custom properties — plus the same localStorage stub the
// adminMode tests use.
function fakeRoot() {
	const props = new Map<string, string>();
	return {
		dataset: {} as Record<string, string>,
		style: {
			setProperty: (k: string, v: string) => void props.set(k, v),
			removeProperty: (k: string) => void props.delete(k)
		},
		props
	};
}
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

const CUSTOM: ThemeCustom = {
	name: 'Rich Archive',
	base: 'cinematheque',
	tokens: { bg: '#0b0a0c', ink: '#efe9e0', accent: '#c0483f', muted: '#9a9188', warn: '#e2603f' }
};

let root: ReturnType<typeof fakeRoot>;
beforeEach(() => {
	root = fakeRoot();
	(globalThis as { document: unknown }).document = { documentElement: root };
	(globalThis as { localStorage: Storage }).localStorage = fakeStorage();
	vi.restoreAllMocks();
});

describe('theme (F66 instance skin)', () => {
	it('applies the server skin to <html> and never keeps a preference', () => {
		const t = new ThemeState();
		t.applyServer({ active: 'broadcast', custom: null });
		expect(root.dataset.theme).toBe('broadcast');
		expect(t.current).toBe('broadcast');
		expect(t.active).toBe('broadcast');
		expect(localStorage.getItem('holodex-theme')).toBeNull();
	});

	it('custom = base data-theme + the five inline primaries; switching back leaves no residue', () => {
		const t = new ThemeState();
		t.applyServer({ active: 'custom', custom: CUSTOM });
		expect(root.dataset.theme).toBe('cinematheque');
		expect(t.current).toBe('cinematheque');
		expect(t.active).toBe('custom');
		for (const k of CUSTOM_TOKENS) expect(root.props.get(`--${k}`)).toBe(CUSTOM.tokens[k]);

		t.applyServer({ active: 'brutalist', custom: CUSTOM });
		expect(root.dataset.theme).toBe('brutalist');
		expect(root.props.size).toBe(0);
		expect(t.custom).toEqual(CUSTOM); // still configured, just not active
	});

	it('paint cache: init applies the last server value before capabilities arrive', () => {
		const first = new ThemeState();
		first.applyServer({ active: 'custom', custom: CUSTOM });

		const cold = new ThemeState();
		const freshRoot = fakeRoot();
		(globalThis as { document: unknown }).document = { documentElement: freshRoot };
		cold.init();
		expect(freshRoot.dataset.theme).toBe('cinematheque');
		expect(freshRoot.props.get('--accent')).toBe('#c0483f');

		// The server value is authoritative: a changed instance overwrites the cache.
		cold.applyServer({ active: 'broadcast', custom: null });
		expect(freshRoot.dataset.theme).toBe('broadcast');
		expect(freshRoot.props.size).toBe(0);
		expect(JSON.parse(localStorage.getItem('holodex-theme-cache')!).active).toBe('broadcast');
	});

	it('init tolerates a corrupt or blocked cache', () => {
		localStorage.setItem('holodex-theme-cache', '{not json');
		const t = new ThemeState();
		expect(() => t.init()).not.toThrow();
		expect(root.dataset.theme).toBeUndefined();

		(globalThis as { localStorage: unknown }).localStorage = {
			getItem: () => {
				throw new Error('blocked');
			}
		};
		expect(() => new ThemeState().init()).not.toThrow();
	});

	it('init ignores a cached custom palette with missing tokens', () => {
		localStorage.setItem(
			'holodex-theme-cache',
			JSON.stringify({ active: 'custom', custom: { name: 'x', base: 'cinematheque', tokens: { bg: '#000' } } })
		);
		const t = new ThemeState();
		t.init();
		expect(root.dataset.theme).toBeUndefined();
		expect(root.props.size).toBe(0);
	});

	it('select("custom") with no palette configured refuses locally without touching <html>', async () => {
		const t = new ThemeState();
		t.applyServer({ active: 'broadcast', custom: null });
		const spy = vi.spyOn(api, 'setTheme');
		await expect(t.select('custom')).resolves.toBe(false);
		expect(spy).not.toHaveBeenCalled();
		expect(root.dataset.theme).toBe('broadcast');
	});

	it('select applies optimistically, persists, and reverts when the server refuses', async () => {
		const t = new ThemeState();
		t.applyServer({ active: 'cinematheque', custom: null });

		const ok: ThemeCapability = { active: 'broadcast', custom: null };
		const spy = vi.spyOn(api, 'setTheme').mockResolvedValueOnce(ok);
		await expect(t.select('broadcast')).resolves.toBe(true);
		expect(spy).toHaveBeenCalledWith('broadcast');
		expect(root.dataset.theme).toBe('broadcast');

		vi.spyOn(api, 'setTheme').mockRejectedValueOnce(new Error('403'));
		const p = t.select('brutalist');
		expect(root.dataset.theme).toBe('brutalist'); // optimistic
		await expect(p).resolves.toBe(false);
		expect(root.dataset.theme).toBe('broadcast'); // reverted
		expect(t.active).toBe('broadcast');
	});
});
