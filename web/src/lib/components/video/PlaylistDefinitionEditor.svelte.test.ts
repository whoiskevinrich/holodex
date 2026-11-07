import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Playlist } from '#lib/types.ts';

// W4 (testing-strategy §40.6): the flows that need the mounted editor. The rules themselves
// are in playlistDefinition.test.ts; `#edit` and Back are the nav harness's.

type Nav = { willUnload: boolean; to: { url: URL } | null; cancel: () => void };

const h = vi.hoisted(() => ({
	navHooks: [] as ((nav: unknown) => void)[],
	goto: vi.fn(async () => {}),
	checkPlaylist: vi.fn(async () => ({ ok: true })),
	updatePlaylist: vi.fn(async (_id: number, body: Record<string, unknown>) => ({ playlist: { id: 7, ...body } })),
	parseQuery: vi.fn(async (expr: string) => ({
		ok: true,
		canonical: 'tag:1', // as the server: the field shows the name, the draft holds the id
		tree: { kind: 'cond', expr: 'tag:1', field: 'tag', op: '=', value: '1', id: 1, label: 'noir' }
	}))
}));

vi.mock('$app/navigation', () => ({
	beforeNavigate: (f: (nav: unknown) => void) => h.navHooks.push(f),
	afterNavigate: () => {},
	onNavigate: () => {},
	pushState: () => {},
	replaceState: () => {},
	invalidate: async () => {},
	invalidateAll: async () => {},
	goto: h.goto
}));

vi.mock('#lib/api.ts', async (original) => {
	const real = await original<typeof import('#lib/api.ts')>();
	const known: Record<string, unknown> = {
		checkPlaylist: h.checkPlaylist,
		updatePlaylist: h.updatePlaylist,
		parseQuery: h.parseQuery
	};
	return {
		...real,
		api: new Proxy(known, {
			// Everything the preview and the panels read: an empty library.
			get: (t, k: string) => t[k] ?? (async () => ({ items: [], total: 0, fields: [], values: [], facets: [] }))
		})
	};
});

const { ApiError } = await import('#lib/api.ts');
const { default: Editor } = await import('./PlaylistDefinitionEditor.svelte');

const playlist = (over: Partial<Playlist> = {}) =>
	({ id: 7, name: 'Noir', query: 'expr=tag%3A1', sort: 'added_desc', cap: null, visibility: 'private', ...over }) as unknown as Playlist;

function button(name: string) {
	return [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === name);
}
const field = () => document.querySelector<HTMLInputElement>('input[aria-label="Definition query"]')!;
const capSelect = () => [...document.querySelectorAll('select')].find((s) => s.textContent?.includes('Cap:'))!;

async function setCap(value: string) {
	capSelect().value = value;
	capSelect().dispatchEvent(new Event('change', { bubbles: true }));
	await expect.poll(() => capSelect().value).toBe(value);
}

function navigate(url = 'http://localhost/media/1') {
	const nav: Nav = { willUnload: false, to: { url: new URL(url) }, cancel: vi.fn() };
	for (const hook of h.navHooks) hook(nav);
	return nav;
}

async function mount(over: Partial<Playlist> = {}) {
	const onclose = vi.fn();
	const onsaved = vi.fn();
	await render(Editor, { playlist: playlist(over), savedCount: 3, onclose, onsaved });
	await expect.poll(() => field().value).toBe('tag:noir'); // the parse has landed
	return { onclose, onsaved };
}

beforeEach(() => {
	h.navHooks.length = 0;
	h.goto.mockClear();
	h.checkPlaylist.mockClear();
	h.checkPlaylist.mockImplementation(async () => ({ ok: true }));
	h.updatePlaylist.mockClear();
	h.updatePlaylist.mockImplementation(async (_id, body) => ({ playlist: { id: 7, ...body } }));
});
afterEach(() => document.body.replaceChildren());

describe('PlaylistDefinitionEditor', () => {
	it('seeds the draft from what is stored', async () => {
		await mount({ cap: 25 });
		expect(document.body.textContent).toContain('Editing definition of');
		expect(field().value).toBe('tag:noir');
		expect(capSelect().value).toBe('25');
		expect(document.querySelector('[data-list-chips]')?.textContent).toContain('noir');
	});

	it('leaves at once while nothing has changed', async () => {
		const { onclose } = await mount();
		const nav = navigate();
		expect(nav.cancel).not.toHaveBeenCalled();
		button('Cancel')!.click();
		expect(onclose).toHaveBeenCalledOnce();
		expect(document.body.textContent).not.toContain('Discard changes');
	});

	it('asks before Cancel throws away a change, and Keep editing keeps it', async () => {
		const { onclose } = await mount();
		await setCap('25');
		button('Cancel')!.click();
		await expect.poll(() => document.body.textContent).toContain('Discard changes to Noir?');
		expect(onclose).not.toHaveBeenCalled();

		button('Keep editing')!.click();
		await expect.poll(() => button('Keep editing')).toBeUndefined();
		expect(capSelect().value).toBe('25');
		expect(h.updatePlaylist).not.toHaveBeenCalled();

		button('Cancel')!.click();
		await expect.poll(() => button('Discard')).toBeDefined();
		button('Discard')!.click();
		await expect.poll(() => onclose.mock.calls.length).toBe(1);
		expect(h.updatePlaylist).not.toHaveBeenCalled();
	});

	it('counts text typed but not applied as a change', async () => {
		await mount();
		field().value = 'tag:noir -studio:acme';
		field().dispatchEvent(new Event('input', { bubbles: true }));
		await expect.poll(() => navigate().cancel).toHaveBeenCalled();
	});

	it('holds a navigation with a changed draft, and Discard carries it on', async () => {
		await mount();
		await setCap('25');
		const nav = navigate('http://localhost/media/9');
		expect(nav.cancel).toHaveBeenCalled();
		await expect.poll(() => button('Discard')).toBeDefined();
		expect(h.goto).not.toHaveBeenCalled();
		button('Discard')!.click();
		await expect.poll(() => h.goto.mock.calls.length).toBe(1);
		expect(String((h.goto.mock.calls[0] as unknown[])[0])).toBe('http://localhost/media/9');
		// Past the discard the guard stands down, or the goto would be cancelled too.
		expect(navigate().cancel).not.toHaveBeenCalled();
	});

	it('withholds Save while the server refuses the draft, and says why', async () => {
		h.checkPlaylist.mockImplementation(async () => {
			throw new ApiError(400, '/playlists/7', 'This would make a loop: A → B → A.', {
				error: 'This would make a loop: A → B → A.',
				fix: 'Remove “in: B”.',
				stale_refs: [{ kind: 'loop', via: 2 }]
			});
		});
		await mount();
		await expect.poll(() => document.body.textContent).toContain('This would make a loop: A → B → A.');
		expect(document.body.textContent).toContain('Remove “in: B”.');
		expect(button('Save definition')!.disabled).toBe(true);
	});

	it('shows every problem of a refused draft together', async () => {
		h.checkPlaylist.mockImplementation(async () => {
			throw new ApiError(400, '/playlists/7', 'A public playlist can’t use owner-only conditions.', {
				error: 'A public playlist can’t use owner-only conditions.',
				fix: 'Remove the owner-only condition or sort, or make the playlist private.',
				owner_only: true,
				also: [{ error: 'This would make a loop: A → B → A.', fix: 'Remove “in: B”.', via: 2 }]
			});
		});
		await mount();
		await expect.poll(() => document.body.textContent).toContain('This would make a loop: A → B → A.');
		expect(document.body.textContent).toContain('A public playlist can’t use owner-only conditions.');
		expect(button('Save definition')!.disabled).toBe(true);
	});

	it('saves the draft through the one PATCH', async () => {
		const { onsaved } = await mount();
		await setCap('25');
		await expect.poll(() => h.checkPlaylist.mock.calls.length).toBeGreaterThan(0);
		button('Save definition')!.click();
		await expect.poll(() => onsaved.mock.calls.length).toBe(1);
		expect(h.updatePlaylist).toHaveBeenCalledWith(7, expect.objectContaining({ query: 'expr=tag%3A1', cap: 25, sort: 'added_desc' }));
		// Saved: leaving asks nothing.
		expect(navigate().cancel).not.toHaveBeenCalled();
	});

	it('stays open with the refusal when a save fails', async () => {
		h.updatePlaylist.mockImplementation(async () => {
			throw new ApiError(400, '/playlists/7', 'A definition can reach at most 16 playlists.', {
				error: 'A definition can reach at most 16 playlists.',
				fix: 'Remove a playlist reference.'
			});
		});
		const { onsaved, onclose } = await mount();
		await setCap('25');
		button('Save definition')!.click();
		await expect.poll(() => document.body.textContent).toContain('A definition can reach at most 16 playlists.');
		expect(onsaved).not.toHaveBeenCalled();
		expect(onclose).not.toHaveBeenCalled();
		expect(capSelect().value).toBe('25');
	});
});
