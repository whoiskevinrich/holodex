import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import EnrichSourcesSummary from './EnrichSourcesSummary.svelte';

// HOLODEX-220 Part A (media-page-restructure-handoff.md): provider status collapses to one
// summary; per-provider actions are exceptions inside its popover.

function props(over: Record<string, unknown> = {}) {
	return {
		providers: ['tmdb', 'imdb'],
		linked: (p: string) => p === 'tmdb',
		syncedAt: () => undefined,
		onenrich: vi.fn(),
		onrefresh: vi.fn(),
		onclear: vi.fn(),
		...over
	};
}

describe('EnrichSourcesSummary', () => {
	it('summarises linked and unmatched providers on one trigger', async () => {
		const screen = await render(EnrichSourcesSummary, props());
		const trigger = screen.getByRole('button', { name: /1 source linked/ });
		await expect.element(trigger).toHaveAttribute('aria-expanded', 'false');
		expect(trigger.element().textContent).toContain('1 unmatched');
	});

	it('offers Match… for an unmatched provider and Refresh behind ⋯ for a linked one', async () => {
		const p = props();
		const screen = await render(EnrichSourcesSummary, p);
		await screen.getByRole('button', { name: /source linked/ }).click();

		await screen.getByRole('button', { name: 'Match…' }).click();
		expect(p.onenrich).toHaveBeenCalledWith('imdb');

		await screen.getByRole('button', { name: /source linked/ }).click();
		await screen.getByRole('button', { name: 'More actions for tmdb' }).click();
		await screen.getByRole('menuitem', { name: 'Refresh tmdb' }).click();
		expect(p.onrefresh).toHaveBeenCalledWith('tmdb');
	});

	it('disables row actions while the page Sync runs', async () => {
		const screen = await render(EnrichSourcesSummary, props({ syncing: true }));
		await screen.getByRole('button', { name: /source linked/ }).click();
		await expect.element(screen.getByRole('button', { name: 'Match…' })).toBeDisabled();
		await expect.element(screen.getByRole('button', { name: 'More actions for tmdb' })).toBeDisabled();
	});

	it('closes on Escape and returns focus to the trigger', async () => {
		const screen = await render(EnrichSourcesSummary, props());
		const trigger = screen.getByRole('button', { name: /source linked/ });
		await trigger.click();
		await userEvent.keyboard('{Escape}');
		await expect.element(trigger).toHaveAttribute('aria-expanded', 'false');
		expect(document.activeElement).toBe(trigger.element());
	});

	it('reads "More" with no provider configured', async () => {
		const screen = await render(EnrichSourcesSummary, props({ providers: [] }));
		await expect.element(screen.getByRole('button', { name: 'More metadata actions' })).toBeInTheDocument();
	});
});
