import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { MontageStatus as Status } from '#lib/types.ts';

// The owner's Hover montage row (F83, testing-strategy §35.5 F5): hidden in off mode,
// polls only while waiting or making, and stops on ready and on unmount.

const statuses: Status[] = [];
const montageStatus = vi.fn(async (): Promise<Status> => statuses.shift() ?? { mode: 'manual', state: 'ready', size_bytes: 1024 });
const requestMontage = vi.fn(async () => ({ status: 'queued' }));

vi.mock('#lib/api.ts', () => ({
	api: {
		montageStatus: () => montageStatus(),
		requestMontage: () => requestMontage(),
		montageURL: (id: number) => `/m/${id}`
	}
}));
vi.mock('./montageStatus', async (orig) => ({ ...(await orig<typeof import('./montageStatus')>()), MONTAGE_POLL_MS: 20 }));

const { default: MontageStatus } = await import('./MontageStatus.svelte');
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

afterEach(() => {
	statuses.length = 0;
	montageStatus.mockClear();
	requestMontage.mockClear();
});

describe('MontageStatus', () => {
	it('renders nothing in off mode', async () => {
		statuses.push({ mode: 'off', state: 'none' });
		const screen = await render(MontageStatus, { videoId: 1 });
		await sleep(50);
		expect(screen.container.textContent?.trim()).toBe('');
	});

	it('makes a montage, polls while waiting and making, and stops on ready', async () => {
		statuses.push(
			{ mode: 'manual', state: 'none' },
			{ mode: 'manual', state: 'waiting' },
			{ mode: 'manual', state: 'making' },
			{ mode: 'manual', state: 'ready', size_bytes: 2048 }
		);
		const screen = await render(MontageStatus, { videoId: 1 });
		await expect.element(screen.getByText('None yet')).toBeInTheDocument();
		await screen.getByRole('button', { name: 'Make montage' }).click();
		expect(requestMontage).toHaveBeenCalledOnce();
		await expect.element(screen.getByText(/^Ready · 2\.0 KB$/)).toBeInTheDocument();
		await expect.element(screen.getByRole('button', { name: 'Remake' })).toBeInTheDocument();
		const calls = montageStatus.mock.calls.length;
		await sleep(100);
		expect(montageStatus.mock.calls.length).toBe(calls); // no polling once ready
	});

	it('stops polling when unmounted', async () => {
		for (let i = 0; i < 50; i++) statuses.push({ mode: 'manual', state: 'making' });
		const screen = await render(MontageStatus, { videoId: 1 });
		await expect.element(screen.getByText('Making…')).toBeInTheDocument();
		screen.unmount();
		const calls = montageStatus.mock.calls.length;
		await sleep(100);
		expect(montageStatus.mock.calls.length).toBe(calls);
	});
});
