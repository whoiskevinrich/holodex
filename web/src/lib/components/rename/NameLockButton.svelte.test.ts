import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

// W5 (testing-strategy §39.6): a labelled button's glyph shows what the click does; the icon-only
// toggle shows the current state with aria-pressed. The three lock-reason phrasings are W5's
// other half, in rename.test.ts (lockLine).

const setNameLock = vi.fn(async (_id: number, locked: boolean) => ({ locked }));
vi.mock('#lib/api.ts', () => ({ api: { setNameLock: (id: number, l: boolean) => setNameLock(id, l) } }));
const { default: NameLockButton } = await import('./NameLockButton.svelte');

const glyph = (el: Element) => el.querySelector('svg')!.getAttribute('data-lock');

describe('NameLockButton', () => {
	it('labelled: Lock name shows a closed padlock, Unlock an open one', async () => {
		const lock = await render(NameLockButton, { videoId: 1, title: 'Ada', locked: false, labelled: true });
		const b = lock.getByRole('button', { name: 'Lock name for Ada' });
		await expect.element(b).toHaveTextContent('Lock name');
		expect(glyph(b.element())).toBe('closed');
		lock.unmount();

		const unlock = await render(NameLockButton, { videoId: 1, title: 'Ada', locked: true, labelled: true });
		const u = unlock.getByRole('button', { name: 'Unlock name for Ada' });
		await expect.element(u).toHaveTextContent('Unlock');
		expect(glyph(u.element())).toBe('open');
	});

	it('icon-only: shows the state with aria-pressed and toggles at once', async () => {
		const onchange = vi.fn();
		const screen = await render(NameLockButton, { videoId: 9, title: 'Ada', locked: false, onchange });
		const b = screen.getByRole('button', { name: 'Lock name for Ada' });
		await expect.element(b).toHaveAttribute('aria-pressed', 'false');
		expect(glyph(b.element())).toBe('open');
		await b.click();
		expect(setNameLock).toHaveBeenCalledWith(9, true);
		expect(onchange).toHaveBeenCalledWith(true);
	});
});
