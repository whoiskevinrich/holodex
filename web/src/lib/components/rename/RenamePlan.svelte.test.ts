import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import type { RenamePlan as Plan, RenamePlanRow as Row, RenameReason } from '#lib/rename.ts';

// W4 (testing-strategy §39.6): defaults to Out of date when its count is above 0; Select all
// counts the server total, not the page; the selection survives a filter change; Rename N turns
// into the inline confirm with focus on Yes, Escape cancels and focus returns; job states.

const page = { url: new URL('http://localhost/owner/file-names'), state: {} };
vi.mock('$app/state', () => ({ page }));
vi.mock('$app/navigation', () => ({ replaceState: vi.fn(), beforeNavigate: vi.fn() }));

const ROWS: Row[] = [
	...Array.from({ length: 150 }, (_, i): Row => ({
		video_id: i + 1,
		title: `Video ${i + 1}`,
		current: `v${i + 1}.mkv`,
		name: `[Acme] Video ${i + 1}.mkv`,
		template: 0,
		reason: 'will-rename'
	})),
	{ video_id: 500, title: 'Northern Lights', current: 'n.mkv', name: 'N.mkv', template: 0, reason: 'out-of-date' },
	{ video_id: 600, title: 'Harbour', current: 'h.mkv', name: 'H.mkv', template: 0, reason: 'clash' }
];

let outOfDate = true;
function planFor(q: { reason?: string; offset?: number; limit?: number }): Plan {
	const rows = ROWS.filter((r) => outOfDate || r.reason !== 'out-of-date');
	const counts = Object.fromEntries(
		['will-rename', 'out-of-date', 'already-named', 'no-template', 'wont-read-back', 'clash', 'bad-name', 'locked', 'held'].map(
			(k) => [k, rows.filter((r) => r.reason === k).length]
		)
	) as Record<RenameReason, number>;
	const reason = (q.reason ?? 'all') as Plan['reason'];
	const hits = rows.filter((r) => reason === 'all' || r.reason === reason);
	const offset = q.offset ?? 0;
	return {
		mode: 'manual',
		templates: ['[{studio}] {title}'],
		providers: [],
		total: rows.length,
		counts,
		template_counts: [{ names: rows.length, wont_read_back: 0 }],
		reason,
		filtered: hits.length,
		rows: hits.slice(offset, offset + (q.limit ?? 100))
	};
}

const renamePlan = vi.fn(async (q: { reason?: string; offset?: number; limit?: number }) => planFor(q));
const renameApply = vi.fn(async (ids: number[]) => ({ batch_id: 'rename-1', queued: ids.length, not_renamed: {} }));
const statuses = new Map<number, { queued: boolean; failed: boolean; error?: string }>();
vi.mock('#lib/api.ts', () => ({
	ApiError: class extends Error {},
	api: {
		renamePlan: (q: { reason?: string }) => renamePlan(q),
		renameApply: (ids: number[]) => renameApply(ids),
		renameStatus: async (id: number) => statuses.get(id) ?? { queued: false, failed: false },
		renameHistory: async () => [{ old_name: 'v.mkv', new_name: 'V.mkv', trigger: 'manual', created_at: new Date().toISOString() }],
		setNameLock: vi.fn()
	}
}));

const { default: RenamePlan } = await import('./RenamePlan.svelte');
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

afterEach(() => {
	outOfDate = true;
	statuses.clear();
	renamePlan.mockClear();
	renameApply.mockClear();
});

describe('RenamePlan', () => {
	it('opens on Out of date while it is not empty, else on All', async () => {
		const screen = await render(RenamePlan, { mode: 'manual' });
		await expect.element(screen.getByRole('button', { name: /^Out of date/, pressed: true })).toBeInTheDocument();
		screen.unmount();
		outOfDate = false;
		const again = await render(RenamePlan, { mode: 'manual' });
		await expect.element(again.getByRole('button', { name: /^All/, pressed: true })).toBeInTheDocument();
		await expect.element(again.getByRole('button', { name: /Load more.*51/ })).toBeInTheDocument();
	});

	it('selects every renameable video in the filter, past the loaded page', async () => {
		const screen = await render(RenamePlan, { mode: 'manual' });
		await screen.getByRole('button', { name: /^All/ }).click();
		await screen.getByRole('button', { name: 'Select all 151' }).click();
		await expect.element(screen.getByText('151 selected of the 151 you can rename')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Clear selection' })).toBeInTheDocument();
		// The selection survives a filter change.
		await screen.getByRole('button', { name: /^Clash/ }).click();
		await expect.element(screen.getByRole('button', { name: 'Rename 151' })).toBeInTheDocument();
	});

	it('confirms inline with focus on Yes; Escape cancels and focus returns', async () => {
		const screen = await render(RenamePlan, { mode: 'manual' });
		await screen.getByRole('checkbox', { name: 'Rename Northern Lights' }).click();
		await screen.getByRole('button', { name: 'Rename 1' }).click();
		await expect.element(screen.getByText('Rename 1 file? You can undo each one from its video page.')).toBeInTheDocument();
		expect(document.activeElement?.textContent).toBe('Yes, rename 1');
		await userEvent.keyboard('{Escape}');
		await expect.element(screen.getByRole('button', { name: 'Rename 1' })).toHaveFocus();
	});

	it('reports each row in place after Yes: renaming, then renamed or failed', async () => {
		statuses.set(500, { queued: true, failed: false });
		const screen = await render(RenamePlan, { mode: 'manual' });
		await screen.getByRole('checkbox', { name: 'Rename Northern Lights' }).click();
		await screen.getByRole('button', { name: 'Rename 1' }).click();
		await screen.getByRole('button', { name: 'Yes, rename 1' }).click();
		expect(renameApply).toHaveBeenCalledWith([500]);
		await expect.element(screen.getByRole('img', { name: 'Renaming' })).toBeInTheDocument();
		await expect.element(screen.getByText('0 renamed · 1 renaming · 0 failed.')).toBeInTheDocument();
		statuses.set(500, { queued: false, failed: true, error: 'the name is taken' });
		await sleep(1800);
		await expect.element(screen.getByText('0 renamed · 0 renaming · 1 failed.')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Dismiss' })).toBeInTheDocument();
		await expect.element(screen.getByRole('link', { name: 'Open System activity' })).toHaveAttribute('href', '/owner/status?batch=rename-1');
	});

	it('offers no checkbox and no Rename in Plan mode', async () => {
		const screen = await render(RenamePlan, { mode: 'plan' });
		await expect.element(screen.getByText('Plan mode: nothing on disk changes. Choose Manual above to rename.')).toBeInTheDocument();
		await expect.element(screen.getByRole('checkbox')).not.toBeInTheDocument();
	});
});
