import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { DEFAULT_CHARACTERS, FORBIDDEN_CHARACTERS, characterOrder } from '#lib/rename.ts';
import CharacterTable from './CharacterTable.svelte';

// HOLODEX-633 (handoff "Characters"): the nine have no ×; + Add character focuses a one-character
// input; × removes; Reset to defaults is withdrawn at the default; a stand-in's spaces show as ␣
// while unfocused; the example follows the stand-in; a row's error replaces its example.

const defaults = () => characterOrder(DEFAULT_CHARACTERS).map((char, i) => ({ id: i + 1, char, standIn: DEFAULT_CHARACTERS[char] }));
const rowIds = (c: HTMLElement) => [...c.querySelectorAll('[data-row]')].map((r) => r.getAttribute('data-row'));

describe('CharacterTable', () => {
	it('lists the nine first with no ×, then the others with one', async () => {
		const screen = await render(CharacterTable, { rows: defaults() });
		for (const c of FORBIDDEN_CHARACTERS) {
			await expect.element(screen.getByRole('textbox', { name: `Stand-in for ${c}` })).toBeInTheDocument();
			await expect.element(screen.getByRole('button', { name: `Remove ${c}` })).not.toBeInTheDocument();
		}
		await expect.element(screen.getByRole('button', { name: 'Remove ;' })).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Remove !' })).toBeInTheDocument();
		await expect.element(screen.getByText('Star Wars: Episode IV → Star Wars- Episode IV')).toBeInTheDocument();
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for ?' })).toHaveAttribute('placeholder', 'removed');
	});

	it('removes a row with ×', async () => {
		const onedit = vi.fn();
		const screen = await render(CharacterTable, { rows: defaults(), onedit });
		await screen.getByRole('button', { name: 'Remove ;' }).click();
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for ;' })).not.toBeInTheDocument();
		expect(rowIds(screen.container)).toHaveLength(10);
		expect(onedit).toHaveBeenCalledWith(null);
	});

	it('adds a draft row, focuses its character input, and shows its example and ␣ spaces', async () => {
		const screen = await render(CharacterTable, { rows: defaults() });
		await screen.getByRole('button', { name: '+ Add character' }).click();
		const input = screen.getByRole('textbox', { name: 'Character' });
		await expect.element(input).toHaveFocus();
		await userEvent.keyboard('&');
		const standIn = screen.getByRole('textbox', { name: 'Stand-in for &' });
		await standIn.fill(' and ');
		await expect.element(screen.getByText('Fast & Furious → Fast and Furious')).toBeInTheDocument();
		await expect.element(standIn).toHaveValue(' and ');
		await userEvent.tab();
		await expect.element(standIn).toHaveValue('␣and␣');
		await expect.element(screen.getByRole('button', { name: 'Remove &' })).toBeInTheDocument();
	});

	it('withdraws Reset to defaults at the default and puts the default back otherwise', async () => {
		const screen = await render(CharacterTable, { rows: defaults() });
		const reset = screen.getByRole('button', { name: 'Reset to defaults' });
		await expect.element(reset).toHaveAttribute('aria-disabled', 'true');
		await screen.getByRole('textbox', { name: 'Stand-in for :' }).fill('_');
		await screen.getByRole('button', { name: 'Remove !' }).click();
		await expect.element(reset).toHaveAttribute('aria-disabled', 'false');
		await reset.click();
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for :' })).toHaveValue('-');
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for !' })).toBeInTheDocument();
		await expect.element(reset).toHaveAttribute('aria-disabled', 'true');
	});

	it('shows a row error in place of its example, on the input it belongs to', async () => {
		const rows = [...defaults(), { id: 99, char: '@', standIn: 'a/t', added: true }];
		const screen = await render(CharacterTable, {
			rows,
			errors: { 99: { message: "A stand-in can't contain /.", on: 'stand-in' as const } }
		});
		await expect.element(screen.getByText("A stand-in can't contain /.")).toBeInTheDocument();
		await expect.element(screen.getByText('a@b → ab')).not.toBeInTheDocument();
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for @' })).toHaveAttribute('aria-invalid', 'true');
		await expect.element(screen.getByRole('textbox', { name: 'Character' })).not.toHaveAttribute('aria-invalid');
	});
});
