import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import TemplateList from './TemplateList.svelte';

// W1 (testing-strategy §39.6): ↑/↓ keep focus on the moved row's button and are withdrawn at the
// ends; Alt+↑/↓ move a row from its input; × is withdrawn on the last template; a token chip
// inserts at the last-focused cursor, else into the last row (handoff calls 10, 12, 15).

const items = (...texts: string[]) => texts.map((text, i) => ({ id: i + 1, text }));
const values = (c: HTMLElement) => [...c.querySelectorAll<HTMLInputElement>('input')].map((i) => i.value);

describe('TemplateList', () => {
	it('moves a row with ↑/↓ and keeps focus on the same button; the ends are withdrawn', async () => {
		const screen = await render(TemplateList, { items: items('a', 'b', 'c') });
		const up1 = screen.getByRole('button', { name: 'Move template 1 up' });
		await expect.element(up1).toHaveAttribute('aria-disabled', 'true');
		await expect.element(screen.getByRole('button', { name: 'Move template 3 down' })).toHaveAttribute('aria-disabled', 'true');
		await up1.click({ force: true });
		expect(values(screen.container)).toEqual(['a', 'b', 'c']);

		await screen.getByRole('button', { name: 'Move template 2 down' }).click();
		expect(values(screen.container)).toEqual(['a', 'c', 'b']);
		// 'b' is now row 3; focus followed it to its own ↓.
		const focused = document.activeElement as HTMLElement;
		expect(focused.dataset.act).toBe('down');
		expect(focused.closest('[data-row]')?.getAttribute('data-row')).toBe('2');
	});

	it('moves a row with Alt+↑/↓ inside its input, keeping focus in the input', async () => {
		const screen = await render(TemplateList, { items: items('a', 'b') });
		await screen.getByRole('textbox', { name: 'Template 2' }).click();
		await userEvent.keyboard('{Alt>}{ArrowUp}{/Alt}');
		expect(values(screen.container)).toEqual(['b', 'a']);
		expect((document.activeElement as HTMLInputElement).value).toBe('b');
	});

	it('withdraws × on the last template while the mode is not Off', async () => {
		const screen = await render(TemplateList, { items: items('a'), lockLast: true });
		const remove = screen.getByRole('button', { name: 'Remove template 1' });
		await expect.element(remove).toHaveAttribute('aria-disabled', 'true');
		await remove.click({ force: true });
		expect(values(screen.container)).toEqual(['a']);
	});

	it('inserts a token at the last-focused cursor, else into the last row', async () => {
		const screen = await render(TemplateList, { items: items('x', 'ab'), providers: ['provider-1'] });
		await screen.getByRole('button', { name: '{provider-1-id}' }).click();
		expect(values(screen.container)).toEqual(['x', 'ab{provider-1-id}']);

		const first = screen.getByRole('textbox', { name: 'Template 1' });
		await first.click();
		(first.element() as HTMLInputElement).setSelectionRange(0, 0);
		await screen.getByRole('button', { name: '{title}' }).click();
		expect(values(screen.container)[0]).toBe('{title}x');
	});

	it('appends an empty row and focuses it on + Add template', async () => {
		const screen = await render(TemplateList, { items: items('a') });
		await screen.getByRole('button', { name: '+ Add template' }).click();
		expect(values(screen.container)).toEqual(['a', '']);
		expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Template 2' }).element());
	});
});
