import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';

vi.mock('$app/navigation', () => ({ beforeNavigate: () => {} }));
const { default: VirtualPeople } = await import('./__fixtures__/VirtualPeople.svelte');

// The whole-list scroll's grid body (HOLODEX-601, testing-strategy §32.1): bounded drawing
// anywhere in the list, the row-height contract between placeholders and entries, list
// positions, one Tab stop with arrow keys past what's drawn, focus that survives a
// placeholder turning into its entry, and the failed page's retry row.

const TOTAL = 1512;
type Row = { id: number; name: string };
const page = (offset: number, limit: number) => ({
	items: Array.from({ length: Math.max(Math.min(limit, TOTAL - offset), 0) }, (_, k) => ({
		id: offset + k,
		// Every 7th name is long enough to wrap if a row ever let it.
		name: (offset + k) % 7 ? `person ${offset + k}` : `a very long name that would wrap over two lines ${offset + k}`.repeat(3)
	})),
	total: TOTAL
});

/** A fetcher whose replies the test releases page by page, or fails. */
function server() {
	const held = new Map<number, { resolve: () => void; reject: (e: Error) => void }>();
	let hold = false;
	const fetch = vi.fn(async (offset: number, limit: number) => {
		if (hold && offset > 0) {
			await new Promise<void>((resolve, reject) => held.set(offset, { resolve, reject }));
		}
		return page(offset, limit) as { items: Row[]; total: number };
	});
	return {
		fetch,
		hold: () => (hold = true),
		release: (offset: number) => held.get(offset)?.resolve(),
		fail: (offset: number) => held.get(offset)?.reject(new Error('offline')),
		held
	};
}

const cells = () => [...document.querySelectorAll<HTMLElement>('[role="list"] [data-index]')].filter((c) => !c.closest('.sr-only'));
const cell = (i: number) => document.querySelector<HTMLElement>(`[role="list"] [data-index="${i}"]`);
const frames = async (n = 4) => {
	for (let k = 0; k < n; k++) await new Promise(requestAnimationFrame);
};
async function scrollTo(y: number) {
	window.scrollTo(0, y);
	window.dispatchEvent(new Event('scroll'));
	await frames();
	await new Promise((r) => setTimeout(r, 200)); // the core's scrolling flag settles
	await frames();
}

afterEach(() => window.scrollTo(0, 0));

describe('VirtualRows', () => {
	it('draws a bounded number of rows at the top, the middle and the end, each with its list position', async () => {
		const s = server();
		render(VirtualPeople, { fetch: s.fetch, lanes: 2 });
		await expect.poll(() => cells().length).toBeGreaterThan(0);
		const screens = Math.ceil(window.innerHeight / 78) * 2 * 3; // three screens of two lanes
		expect(cells().length).toBeLessThanOrEqual(screens);
		for (const y of [document.documentElement.scrollHeight / 2, document.documentElement.scrollHeight]) {
			await scrollTo(y);
			await expect.poll(() => cells().some((c) => c.getAttribute('aria-busy') !== 'true')).toBe(true);
			expect(cells().length).toBeLessThanOrEqual(screens);
		}
		const last = cell(TOTAL - 1)!;
		expect(last.getAttribute('aria-posinset')).toBe(String(TOTAL));
		expect(last.getAttribute('aria-setsize')).toBe(String(TOTAL));
		expect(document.querySelector('[role="list"]')!.getAttribute('aria-label')).toBe('People');
	});

	it('keeps every row, placeholder or loaded, at one height (the row-height contract)', async () => {
		const s = server();
		s.hold();
		render(VirtualPeople, { fetch: s.fetch, lanes: 1 });
		await expect.poll(() => cells().length).toBeGreaterThan(0);
		await scrollTo(50 * 78 - 200); // straddle the end of page 0 and the start of page 1
		await expect.poll(() => cells().some((c) => c.getAttribute('aria-busy') === 'true')).toBe(true);
		const rows = [...document.querySelectorAll<HTMLElement>('[role="list"] [data-row]')];
		const busy = rows.filter((r) => r.querySelector('[aria-busy="true"]'));
		const loaded = rows.filter((r) => !r.querySelector('[aria-busy="true"]'));
		expect(busy.length).toBeGreaterThan(0);
		expect(loaded.length).toBeGreaterThan(0); // page 0's rows, with long names among them
		const heights = new Set(rows.map((r) => r.getBoundingClientRect().height));
		expect(heights.size).toBe(1);
	});

	it('is one Tab stop; arrows move past what is drawn and End reaches the last entry', async () => {
		const s = server();
		render(VirtualPeople, { fetch: s.fetch, lanes: 2 });
		await expect.poll(() => cell(0)).not.toBeNull();
		expect(document.querySelectorAll('[role="list"] [tabindex="0"]').length).toBe(2); // entry 0 and its ring
		expect(document.querySelectorAll('[role="list"] a[tabindex="0"]').length).toBe(0);
		cell(0)!.focus();
		await userEvent.keyboard('{ArrowDown}');
		await expect.poll(() => document.activeElement?.getAttribute('data-index')).toBe('2');
		await userEvent.keyboard('{ArrowRight}');
		await expect.poll(() => document.activeElement?.getAttribute('data-index')).toBe('3');
		await userEvent.keyboard('{End}');
		await expect.poll(() => document.activeElement?.getAttribute('data-index'), { timeout: 3000 }).toBe(String(TOTAL - 1));
		await userEvent.keyboard('{Home}');
		await expect.poll(() => document.activeElement?.getAttribute('data-index'), { timeout: 3000 }).toBe('0');
	});

	it('keeps focus on the same element when its placeholder turns into the entry', async () => {
		const s = server();
		s.hold();
		render(VirtualPeople, { fetch: s.fetch, lanes: 1 });
		await expect.poll(() => cell(0)).not.toBeNull();
		await scrollTo(60 * 78);
		await expect.poll(() => s.held.has(50)).toBe(true);
		const target = cells().find((c) => c.getAttribute('aria-busy') === 'true')!;
		target.focus();
		s.release(50);
		await expect.poll(() => target.getAttribute('aria-busy')).toBeNull();
		expect(document.activeElement).toBe(target);
		expect(target.textContent).toContain('person');
	});

	it('gives a failed page one retry row without covering its neighbours, and Retry fills it', async () => {
		const s = server();
		s.hold();
		render(VirtualPeople, { fetch: s.fetch, lanes: 3 });
		await expect.poll(() => cell(0)).not.toBeNull();
		// Page 1 (50–99) starts mid-row 16 (48–50) at three lanes: its retry row is row 17.
		await scrollTo(10 * 78);
		await expect.poll(() => s.held.has(50)).toBe(true);
		s.fail(50);
		const retry = () => [...document.querySelectorAll<HTMLButtonElement>('[role="list"] button')].find((b) => b.textContent?.trim() === 'Retry');
		await expect.poll(() => retry()).toBeDefined();
		expect(retry()!.closest('.grid')!.textContent).toContain('Couldn’t load these people.');
		expect(cell(48)!.textContent).toContain('person 48'); // page 0's entries in the shared row stay
		expect(cell(50)!.className).toContain('opacity-55');
		expect(cell(50)!.getAttribute('aria-busy')).toBeNull();
		s.fetch.mockImplementation(async (offset: number, limit: number) => page(offset, limit) as { items: Row[]; total: number });
		retry()!.click();
		await expect.poll(() => retry()).toBeUndefined();
		await expect.poll(() => cell(50)?.textContent ?? '').toContain('person 50');
	});
});
