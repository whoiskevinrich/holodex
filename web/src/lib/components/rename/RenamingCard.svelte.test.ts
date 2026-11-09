import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';

// W2 (testing-strategy §39.6): folded when saved and valid, open with a draft or an error; Save
// withdrawn on error; a server refusal marks each row and keeps the draft; Preview hands the
// draft to the plan; the settle window refuses 0 and 366.

class ApiError extends Error {
	constructor(
		readonly status: number,
		path: string,
		message?: string,
		readonly body?: Record<string, unknown>
	) {
		super(message ?? path);
	}
}

const putSetting = vi.fn(async (key: string, value: string) => ({ key, value }));
vi.mock('#lib/api.ts', () => ({ ApiError, api: { putSetting: (k: string, v: string) => putSetting(k, v) } }));

const { default: RenamingCard } = await import('./RenamingCard.svelte');
const { DEFAULT_CHARACTERS } = await import('#lib/rename.ts');

const base = () => ({
	mode: 'manual' as const,
	settleDays: 7,
	saved: ['[{studio}] {title}', '{title}'],
	savedCharacters: { ...DEFAULT_CHARACTERS },
	onmode: vi.fn(),
	onpreview: vi.fn(async () => {}),
	onsaved: vi.fn()
});

beforeEach(() => localStorage.clear());
afterEach(() => putSetting.mockReset().mockImplementation(async (key, value) => ({ key, value })));

describe('RenamingCard', () => {
	it('folds to one line once saved, and opens on Edit templates', async () => {
		const screen = await render(RenamingCard, base());
		await expect.element(screen.getByText(/Manual · 2 templates · 11 characters · first:/)).toBeInTheDocument();
		await expect.element(screen.getByRole('radiogroup')).not.toBeInTheDocument();
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await expect.element(screen.getByRole('radiogroup', { name: 'Renaming mode' })).toBeInTheDocument();
	});

	it('stays open with a draft, previews it, and keeps it on reload', async () => {
		const props = base();
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await screen.getByRole('textbox', { name: 'Template 2' }).fill('{title} ({year})');
		await expect.element(screen.getByText('Unsaved changes. The plan shows these templates after Preview.')).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Preview' }).click();
		// Characters unchanged: the preview carries the templates alone.
		expect(props.onpreview).toHaveBeenCalledWith({ templates: ['[{studio}] {title}', '{title} ({year})'] });
		screen.unmount();

		const again = await render(RenamingCard, base());
		await expect.element(again.getByRole('textbox', { name: 'Template 2' })).toHaveValue('{title} ({year})');
	});

	it('marks each refused row, withdraws Save and keeps the draft', async () => {
		putSetting.mockRejectedValueOnce(
			new ApiError(422, '/admin/settings/rename.templates', 'the template list has errors', {
				rows: ['', 'add {title} or an identity token: without one, unrelated videos could get the same name']
			})
		);
		const props = base();
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await screen.getByRole('textbox', { name: 'Template 2' }).fill('{studio} {year}');
		await screen.getByRole('button', { name: 'Save templates' }).click();
		await expect
			.element(screen.getByText('Add {title} or an identity token. Without one, unrelated videos could get the same name.'))
			.toBeInTheDocument();
		await expect.element(screen.getByText('Fix template 2 to save.')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Save templates' })).toBeDisabled();
		await expect.element(screen.getByRole('textbox', { name: 'Template 2' })).toHaveValue('{studio} {year}');
		expect(props.onsaved).not.toHaveBeenCalled();
	});

	it('previews and saves a character change as Save characters', async () => {
		const props = base();
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await expect.element(screen.getByRole('heading', { name: 'Characters' })).toBeInTheDocument();
		await screen.getByRole('textbox', { name: 'Stand-in for :' }).fill(' -');
		await expect.element(screen.getByText('Unsaved changes. The plan shows these characters after Preview.')).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Preview' }).click();
		const characters = { ...DEFAULT_CHARACTERS, ':': ' -' };
		expect(props.onpreview).toHaveBeenCalledWith({ templates: props.saved, characters });
		await screen.getByRole('button', { name: 'Save characters' }).click();
		expect(putSetting).toHaveBeenCalledTimes(1);
		expect(putSetting).toHaveBeenCalledWith('rename.characters', JSON.stringify(characters));
		expect(props.onsaved).toHaveBeenCalledWith({ characters });
	});

	it('labels Save changes when both changed and writes both', async () => {
		const props = base();
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await screen.getByRole('textbox', { name: 'Template 2' }).fill('{title} ({year})');
		await expect.element(screen.getByRole('button', { name: 'Save templates' })).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Remove !' }).click();
		await expect.element(screen.getByText('Unsaved changes. The plan shows these changes after Preview.')).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Save changes' }).click();
		const characters = Object.fromEntries(Object.entries(DEFAULT_CHARACTERS).filter(([c]) => c !== '!'));
		expect(putSetting).toHaveBeenCalledWith('rename.templates', JSON.stringify(['[{studio}] {title}', '{title} ({year})']));
		expect(putSetting).toHaveBeenCalledWith('rename.characters', JSON.stringify(characters));
		expect(props.onsaved).toHaveBeenCalledWith({ templates: ['[{studio}] {title}', '{title} ({year})'], characters });
	});

	it('refuses an empty or repeated character before sending', async () => {
		const props = base();
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await screen.getByRole('button', { name: '+ Add character' }).click();
		await screen.getByRole('button', { name: 'Preview' }).click();
		await expect.element(screen.getByText('Add one character.')).toBeInTheDocument();
		await expect.element(screen.getByText('Fix the new row to save.')).toBeInTheDocument();
		await screen.getByRole('textbox', { name: 'Character' }).fill(';');
		await screen.getByRole('button', { name: 'Save characters' }).click();
		await expect.element(screen.getByText('; is already in the table.')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Save characters' })).toBeDisabled();
		expect(props.onpreview).not.toHaveBeenCalled();
		expect(putSetting).not.toHaveBeenCalled();
	});

	it("maps the server's character refusals back to their rows", async () => {
		putSetting.mockRejectedValueOnce(
			new ApiError(422, '/admin/settings/rename.characters', 'the character table has errors', {
				characters: { '@': "A stand-in can't contain /." }
			})
		);
		const props = base();
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await screen.getByRole('button', { name: '+ Add character' }).click();
		await screen.getByRole('textbox', { name: 'Character' }).fill('@');
		await screen.getByRole('textbox', { name: 'Stand-in for @' }).fill('a/t');
		await screen.getByRole('button', { name: 'Save characters' }).click();
		await expect.element(screen.getByText("A stand-in can't contain /.")).toBeInTheDocument();
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for @' })).toHaveAttribute('aria-invalid', 'true');
		await expect.element(screen.getByText('Fix the @ row to save.')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Save characters' })).toBeDisabled();
		expect(props.onsaved).not.toHaveBeenCalled();
	});

	it('discards the character draft with the templates', async () => {
		const props = { ...base(), ondiscard: vi.fn() };
		const screen = await render(RenamingCard, props);
		await screen.getByRole('button', { name: 'Edit templates ▾' }).click();
		await screen.getByRole('textbox', { name: 'Stand-in for :' }).fill('_');
		await screen.getByRole('button', { name: 'Discard' }).click();
		await expect.element(screen.getByRole('textbox', { name: 'Stand-in for :' })).toHaveValue('-');
		expect(props.ondiscard).toHaveBeenCalled();
	});

	it('saves the mode at once, and shows the settle window only for Job', async () => {
		const props = { ...base(), saved: [] };
		const screen = await render(RenamingCard, props);
		await expect.element(screen.getByLabelText('Settle window')).not.toBeInTheDocument();
		await screen.getByRole('radio', { name: 'Job' }).click();
		expect(putSetting).toHaveBeenCalledWith('rename.mode', 'job');
		expect(props.onmode).toHaveBeenCalledWith('job');
	});

	it('refuses a settle window of 0 and of 366', async () => {
		const screen = await render(RenamingCard, { ...base(), mode: 'job' as const, saved: [] });
		const input = screen.getByRole('textbox', { name: /Settle window/ });
		for (const bad of ['0', '366']) {
			await input.fill(bad);
			await userEvent.keyboard('{Enter}');
			await expect.element(screen.getByText('Enter 1 to 365 days.')).toBeInTheDocument();
		}
		expect(putSetting).not.toHaveBeenCalled();
		await input.fill('14');
		await userEvent.keyboard('{Enter}');
		expect(putSetting).toHaveBeenCalledWith('rename.settle_days', '14');
	});

	it('moves the mode with the arrow keys and chooses with Enter', async () => {
		const props = { ...base(), saved: [] };
		const screen = await render(RenamingCard, props);
		await screen.getByRole('radio', { name: 'Manual' }).element().focus();
		putSetting.mockClear();
		await userEvent.keyboard('{ArrowRight}');
		expect(document.activeElement?.textContent).toBe('On writeback');
		expect(putSetting).not.toHaveBeenCalled();
		await userEvent.keyboard('{Enter}');
		expect(putSetting).toHaveBeenCalledWith('rename.mode', 'writeback');
	});
});
