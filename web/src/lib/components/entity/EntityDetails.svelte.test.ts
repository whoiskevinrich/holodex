import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { ResolvedField } from '#lib/types.ts';
import EntityDetails from './EntityDetails.svelte';

// HOLODEX-582: the film and studio pages' shared Details section. The page tests cover the
// visitor/owner gate on each page; these cover the two rules the extraction fixed.

function props(fields: ResolvedField[], isOwner: boolean) {
	return {
		fields,
		isOwner,
		sourcesLoaded: true,
		providers: [],
		linked: () => false,
		onenrich: vi.fn(),
		onrefresh: vi.fn(),
		onclear: vi.fn(),
		onrefreshall: vi.fn(),
		decide: vi.fn(async () => {})
	};
}

describe('EntityDetails', () => {
	it('gives a visitor the custom pill on a custom url row', async () => {
		const website = {
			canonical: 'website',
			label: 'Website',
			display: 'url',
			values: ['https://studio.example/'],
			winning_source: 'manual:website'
		} as ResolvedField;
		const screen = await render(EntityDetails, props([website], false));

		await expect.element(screen.getByRole('link', { name: /studio\.example/ })).toBeInTheDocument();
		await expect.element(screen.getByText('custom')).toBeInTheDocument();
	});

	it('shows the owner a kept-blank row with no candidates', async () => {
		const founded = {
			canonical: 'founded',
			label: 'Founded',
			values: [],
			candidates: [],
			winning_source: 'manual:founded',
			decision: { source: 'manual', standing: true, manual_value: '' }
		} as unknown as ResolvedField;

		const owner = await render(EntityDetails, props([founded], true));
		await expect.element(owner.getByText('Founded:')).toBeInTheDocument();
		owner.unmount();

		// A visitor has nothing to see in it.
		const visitor = await render(EntityDetails, props([founded], false));
		expect(visitor.container.querySelector('#field-founded')).toBeNull();
	});
});
