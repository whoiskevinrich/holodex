import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { RenameHistoryEntry, RenameMode, RenamePlanRow as Row, RenameStatus } from '#lib/rename.ts';

// W6 (testing-strategy §39.6): every state in the handoff's video-page table, including undo
// refused and History; hidden when the mode is Off; Rename absent in Plan mode.

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

let mode: RenameMode = 'manual';
let row: Row;
let status: RenameStatus = { queued: false, failed: false };
let history: RenameHistoryEntry[] = [];
const renameUndo = vi.fn(async () => ({ queued: 1 }));
const renameApply = vi.fn(async () => ({ batch_id: 'b', queued: 1, not_renamed: {} }));

vi.mock('#lib/api.ts', () => ({
	ApiError,
	api: {
		adminSettings: async () => ({ 'rename.mode': mode }),
		renamePlan: async () => ({ mode, templates: ['[{studio}] {title}'], rows: [row] }),
		renameStatus: async () => status,
		renameHistory: async () => history,
		renameUndo: () => renameUndo(),
		renameApply: () => renameApply(),
		renameRetry: vi.fn(async () => ({ retried: 1 })),
		renameDismiss: vi.fn(async () => ({ dismissed: 1 })),
		setNameLock: vi.fn()
	}
}));
const { default: FileNameRow } = await import('./FileNameRow.svelte');

const base = (over: Partial<Row>): Row => ({
	video_id: 3,
	title: 'Northern Lights',
	current: '[Acme] Nothern Lights (2019).mkv',
	name: '[Acme] Northern Lights (2019).mkv',
	template: 0,
	reason: 'already-named',
	...over
});

afterEach(() => {
	mode = 'manual';
	status = { queued: false, failed: false };
	history = [];
	renameUndo.mockReset().mockImplementation(async () => ({ queued: 1 }));
	renameApply.mockClear();
});

describe('FileNameRow', () => {
	it('is hidden when renaming is Off', async () => {
		mode = 'off';
		row = base({});
		const screen = await render(FileNameRow, { videoId: 3 });
		await new Promise((r) => setTimeout(r, 50));
		expect(screen.container.textContent?.trim()).toBe('');
	});

	it('up to date: matches its template, with Lock name', async () => {
		row = base({ current: 'a.mkv', name: 'a.mkv' });
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText('Matches template 1.')).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Lock name for Northern Lights' })).toBeInTheDocument();
	});

	it('out of date: the new name, the changed field and Rename (no confirm)', async () => {
		row = base({ reason: 'out-of-date', changes: [{ token: 'title', recorded: ['a'], current: ['b'] }] });
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText('[Acme] Northern Lights (2019).mkv')).toBeInTheDocument();
		await expect.element(screen.getByText('Title changed since Holodex named it.')).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Rename' }).click();
		expect(renameApply).toHaveBeenCalledOnce();
		await expect.element(screen.getByText(/You can leave this page\./)).toBeInTheDocument();
	});

	it('Plan mode shows the same lines without Rename', async () => {
		mode = 'plan';
		row = base({ reason: 'will-rename' });
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText(/^Would be/)).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Rename' })).not.toBeInTheDocument();
	});

	it('renamed: the box, Undo rename and History oldest to newest', async () => {
		row = base({ current: 'B.mkv', name: 'B.mkv' });
		history = [
			{ old_name: 'arrived.mkv', new_name: 'A.mkv', trigger: 'job', created_at: '2026-10-02T10:00:00Z' },
			{ old_name: 'A.mkv', new_name: 'B.mkv', trigger: 'manual', created_at: '2026-10-30T10:00:00Z' }
		];
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText(/^Renamed .+ from/)).toBeInTheDocument();
		await screen.getByRole('button', { name: /History \(2\)/ }).click();
		const items = [...screen.container.querySelectorAll('ol li')].map((li) => li.textContent?.replace(/\s+/g, ' ').trim());
		expect(items[0]).toContain('Arrived');
		expect(items[0]).toContain('arrived.mkv');
		expect(items[1]).toContain('job');
		expect(items[2]).toContain('manual');
	});

	it('undo refused: names the taken name, with Dismiss', async () => {
		row = base({ current: 'B.mkv', name: 'B.mkv' });
		history = [{ old_name: 'A.mkv', new_name: 'B.mkv', trigger: 'manual', created_at: '2026-10-30T10:00:00Z' }];
		renameUndo.mockRejectedValueOnce(new ApiError(409, '/undo', 'the previous name is taken', { reason: 'taken', name: 'A.mkv' }));
		const screen = await render(FileNameRow, { videoId: 3 });
		await screen.getByRole('button', { name: 'Undo rename' }).click();
		await expect.element(screen.getByText(/Can't undo:/)).toBeInTheDocument();
		await expect.element(screen.getByText('A.mkv')).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Dismiss' }).click();
		await expect.element(screen.getByRole('button', { name: 'Undo rename' })).toBeInTheDocument();
	});

	it('locked: the reason line, a padlock after the name and Unlock', async () => {
		row = base({ reason: 'locked', lock_source: 'auto', locked_at: '2026-10-12T10:00:00Z' });
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText(/Locked when you renamed it outside Holodex on/)).toBeInTheDocument();
		await expect.element(screen.getByRole('img', { name: 'Locked' })).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Unlock name for Northern Lights' })).toBeInTheDocument();
	});

	it('held: blocked until the held original goes', async () => {
		row = base({ reason: 'held' });
		const screen = await render(FileNameRow, { videoId: 3, heldUntil: '2026-11-04T00:00:00Z' });
		await expect.element(screen.getByText(/Can't rename while the original is held \(until .+\)\./)).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: /Lock name/ })).not.toBeInTheDocument();
	});

	it('refused: the warn reason, then where to fix it', async () => {
		row = base({
			reason: 'wont-read-back',
			written: { studio: ['Acme [EU]'], title: ['T'] },
			parsed: { studio: ['Acme [EU'], title: ['T'] }
		});
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText(/Won't rename: studio/)).toBeInTheDocument();
		await expect.element(screen.getByText('Edit the template or the studio name in File names.')).toBeInTheDocument();
	});

	it('no template fits', async () => {
		row = base({ reason: 'no-template', name: undefined, template: null });
		const screen = await render(FileNameRow, { videoId: 3 });
		await expect.element(screen.getByText(/^No template fits/)).toBeInTheDocument();
	});

	it('renaming: the accent pill; failed: Retry and Dismiss', async () => {
		row = base({});
		status = { queued: true, failed: false };
		const renaming = await render(FileNameRow, { videoId: 3 });
		await expect.element(renaming.getByText('renaming')).toBeInTheDocument();
		renaming.unmount();

		status = { queued: false, failed: true, error: "the folder wasn't reachable" };
		const failed = await render(FileNameRow, { videoId: 3 });
		await expect.element(failed.getByText("Rename failed: the folder wasn't reachable.")).toBeInTheDocument();
		await expect.element(failed.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
		await expect.element(failed.getByRole('button', { name: 'Dismiss' })).toBeInTheDocument();
	});
});
