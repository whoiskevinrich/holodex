import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { JobDigest } from '#lib/types.ts';
import Page from './+page.svelte';

// The job views' two shipped invariants (HOLODEX-203), first users of the browser harness
// (HOLODEX-223): the digest paints without waiting on /admin/activity (HOLODEX-205), and it
// is the default view, with the log fetched only on demand (HOLODEX-210).

const { api } = vi.hoisted(() => ({
	api: {
		activityDigest: vi.fn<() => Promise<JobDigest>>(),
		activityHistory: vi.fn()
	}
}));

vi.mock('#lib/api.ts', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.ts')>()),
	api
}));

// The activity read-model never lands: `loading` stays true and `data` null for the whole
// test, which is exactly the window HOLODEX-205 says must not hold the job views back.
vi.mock('#lib/activity.svelte.ts', () => ({
	activity: {
		data: null,
		caps: null,
		error: '',
		loading: true,
		effectiveOwner: true,
		needToken: false,
		start: () => {},
		stop: () => {}
	}
}));

vi.mock('$app/state', () => ({ page: { url: new URL('http://localhost/owner/status') } }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

beforeEach(() => {
	api.activityDigest.mockReset().mockResolvedValue({ kinds: [], failures: [] });
	api.activityHistory.mockReset().mockResolvedValue({ runs: [] });
});

describe('/owner/status job views', () => {
	it('paints the digest while activity is still pending (HOLODEX-205)', async () => {
		const screen = await render(Page);

		await expect.element(screen.getByText('No jobs recorded yet.')).toBeInTheDocument();
		await expect.element(screen.getByText('Loading activity…')).toBeInTheDocument();
	});

	it('opens on the digest and fetches the log only when asked (HOLODEX-210)', async () => {
		const screen = await render(Page);
		const summary = screen.getByRole('tab', { name: 'Summary' });
		const log = screen.getByRole('tab', { name: 'Log' });

		await expect.element(summary).toHaveAttribute('aria-selected', 'true');
		await expect.element(log).toHaveAttribute('aria-selected', 'false');
		await expect.element(screen.getByText('No jobs recorded yet.')).toBeInTheDocument();
		expect(api.activityDigest).toHaveBeenCalledOnce();
		expect(api.activityHistory).not.toHaveBeenCalled();

		await log.click();
		await expect.element(log).toHaveAttribute('aria-selected', 'true');
		expect(api.activityHistory).toHaveBeenCalledOnce();

		// Reopening the log reuses what was loaded.
		await summary.click();
		await log.click();
		await expect.element(log).toHaveAttribute('aria-selected', 'true');
		expect(api.activityHistory).toHaveBeenCalledOnce();
	});
});
