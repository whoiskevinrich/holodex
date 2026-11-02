import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import QuerySuggestPanel from './QuerySuggestPanel.svelte';

// testing-strategy §38.3 W3/W5: the nav box's query-mode panel. The error echo is three
// text nodes sliced at the server's offsets (never HTML from the query), and the list is a
// text combobox's: options aren't focusable and the highlight is aria-selected.
const base = {
	suggestions: [],
	highlight: -1,
	heading: 'Tags',
	listId: 'qs',
	errorId: 'qe',
	isOwner: false,
	onpick: () => {},
	onhover: () => {}
};

describe('QuerySuggestPanel', () => {
	it('echoes a refused query as text, underlining only the bad part', async () => {
		const text = 'é tag:<img src=x onerror=alert(1)> OR tag:a';
		const start = text.indexOf('<img');
		const end = start + '<img src=x onerror=alert(1)>'.length;
		const screen = await render(QuerySuggestPanel, {
			...base,
			text,
			error: { code: 'unknown_name', message: 'No Tag called "x".', start, end }
		});
		const echo = screen.container.querySelector('p.font-mono')!;
		expect(echo.querySelector('img')).toBeNull();
		expect(echo.textContent).toBe(text);
		expect(echo.querySelector('.query-bad')!.textContent).toBe('<img src=x onerror=alert(1)>');
		await expect.element(screen.getByText('No Tag called "x".')).toHaveAttribute('id', 'qe');
	});

	it('renders unfocusable options with the highlight selected, and picks on click', async () => {
		const onpick = vi.fn();
		const screen = await render(QuerySuggestPanel, {
			...base,
			text: 'tag:sci',
			error: null,
			highlight: 1,
			onpick,
			suggestions: [
				{ label: 'science', insert: 'science', start: 4, end: 7 },
				{ label: 'science-fiction', sub: 'alias: sci-fi · 214', insert: 'science-fiction', start: 4, end: 7 }
			]
		});
		const options = screen.container.querySelectorAll('[role="option"]');
		expect(options).toHaveLength(2);
		expect([...options].map((o) => o.getAttribute('aria-selected'))).toEqual(['false', 'true']);
		expect([...options].every((o) => !o.hasAttribute('tabindex'))).toBe(true);
		expect(options[1].id).toBe('qs-1');
		await screen.getByText('science-fiction').click();
		expect(onpick).toHaveBeenCalledWith(1);
	});

	it('shows the owner-only example in the syntax help to the owner only', async () => {
		const visitor = await render(QuerySuggestPanel, { ...base, text: 'tag:', error: null });
		await visitor.getByRole('button', { name: 'Syntax' }).click();
		expect(visitor.container.textContent).not.toContain('matched:any');
		visitor.unmount();
		const owner = await render(QuerySuggestPanel, { ...base, isOwner: true, text: 'tag:', error: null });
		await owner.getByRole('button', { name: 'Syntax' }).click();
		expect(owner.container.textContent).toContain('matched:any');
	});
});
