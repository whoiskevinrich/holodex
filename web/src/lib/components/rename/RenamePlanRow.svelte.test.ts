import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { RenamePlanRow as Row } from '#lib/rename.ts';

// W3 (testing-strategy §39.6): a row that can't rename has no checkbox but keeps its slot;
// refusals carry the warn border and fine states the muted one; the old name is struck; names
// are never truncated; → reads "becomes"; an id-settled clash is selectable and names the holder.

vi.mock('#lib/api.ts', () => ({ api: { setNameLock: vi.fn() } }));
const { default: RenamePlanRow } = await import('./RenamePlanRow.svelte');

const row = (over: Partial<Row>): Row => ({
	video_id: 7,
	title: 'Ada Lovelace',
	current: 'ada.lovelace.2023.1080p.WEB-DL.x264-GRP.mkv',
	name: '[Acme] Ada Lovelace (2023).mkv',
	template: 0,
	reason: 'will-rename',
	...over
});

describe('RenamePlanRow', () => {
	it('strikes the old name, labels the arrow and never truncates', async () => {
		const long = 'x'.repeat(240) + '.mkv';
		const screen = await render(RenamePlanRow, { row: row({ name: long }), templates: [], selectable: true });
		const old = screen.getByText('ada.lovelace.2023.1080p.WEB-DL.x264-GRP.mkv');
		await expect.element(old).toHaveClass('line-through');
		await expect.element(old).toHaveClass('decoration-warn');
		await expect.element(screen.getByLabelText('becomes')).toBeInTheDocument();
		const name = screen.getByText(long, { exact: false }).element() as HTMLElement;
		expect(getComputedStyle(name).textOverflow).not.toBe('ellipsis');
		expect(name.closest('p')!.className).toContain('wrap-anywhere');
		await expect.element(screen.getByRole('checkbox')).toBeInTheDocument();
	});

	it('gives a refusal no checkbox, keeps the slot, and a warn chip', async () => {
		const screen = await render(
			RenamePlanRow,
			{ row: row({ reason: 'clash', current: 'harbour.mkv', name: 'Harbour Lights (2021).mkv' }), templates: [] },

		);
		await expect.element(screen.getByRole('checkbox')).not.toBeInTheDocument();
		const slot = screen.container.querySelector('li > span')!;
		expect(slot.className).toContain('w-4');
		await expect.element(screen.getByText('clash', { exact: true })).toHaveClass('border-warn');
		await expect.element(screen.getByText(/is already in this folder and isn't in your library/)).toBeInTheDocument();
	});

	it('gives a fine state the muted border and says why', async () => {
		const screen = await render(
			RenamePlanRow,
			{ row: row({ reason: 'locked', lock_source: 'owner', locked_at: '2026-10-12T00:00:00Z' }), templates: [] },

		);
		await expect.element(screen.getByText('locked', { exact: true })).toHaveClass('border-rule');
		await expect.element(screen.getByText('Locked by you.')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Unlock name for Ada Lovelace' })).toBeInTheDocument();
	});

	it('keeps an id-settled clash selectable and names the holder', async () => {
		const screen = await render(
			RenamePlanRow,
			{ row: row({ name: 'Ada Lovelace (2023) {hx-412}.mkv', holder_id: 398 }), templates: [], selectable: true },

		);
		await expect.element(screen.getByRole('checkbox')).toBeInTheDocument();
		await expect.element(screen.getByText(/Video 398 already has/)).toBeInTheDocument();
		await expect.element(screen.getByText('Ada Lovelace (2023).mkv')).toBeInTheDocument();
	});

	it('names the field an out-of-date name changed on', async () => {
		const screen = await render(
			RenamePlanRow,
			{
				row: row({
					reason: 'out-of-date',
					changes: [{ token: 'title', recorded: ['Nothern Lights'], current: ['Northern Lights'] }]
				}),
				templates: [],
				selectable: true
			},

		);
		await expect.element(screen.getByText(/Since Holodex named it, title changed/)).toBeInTheDocument();
		await expect.element(screen.getByText('Nothern Lights')).toBeInTheDocument();
	});

	it('replaces the checkbox with the job state and offers Refresh plan for a stale name', async () => {
		const onrefresh = vi.fn();
		const screen = await render(
			RenamePlanRow,
			{
				row: row({}),
				templates: [],
				selectable: true,
				job: { state: 'failed', error: 'the name changed since the plan; refresh the plan' },
				onrefresh
			},

		);
		await expect.element(screen.getByRole('checkbox')).not.toBeInTheDocument();
		await expect.element(screen.getByText('The name changed since you looked.', { exact: false })).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Refresh plan' }).click();
		expect(onrefresh).toHaveBeenCalledOnce();
	});
});
