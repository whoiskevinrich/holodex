import { afterEach, describe, it, expect, vi } from 'vitest';
import { activity } from './activity.svelte';
import { api, ApiError, ReauthError } from './api';
import type { Activity, Capabilities } from './types';

// HOLODEX-127 poll resilience. The System Activity store polls an owner-gated
// endpoint every few seconds; a single failed poll must not blank the surface, an
// upstream ForwardAuth expiry (ReauthError) must not flash an error before the
// top-level reload, and Holodex's own 401 owner-expiry (ADR-046) must still drop to
// read-only. refresh() is exercised directly (the interval is just its scheduler).

const GOOD: Activity = {
	scan: { state: 'idle' } as Activity['scan'],
	thumbnails: { depth: 0 } as Activity['thumbnails'],
	library: {} as Activity['library'],
	system: {} as Activity['system']
};

describe('activity poll resilience (HOLODEX-127)', () => {
	afterEach(() => vi.restoreAllMocks());

	// A leading success resets the shared singleton's failure/backoff counters so
	// each test starts from a known-good baseline.
	async function seedGood() {
		vi.spyOn(api, 'activity').mockResolvedValueOnce(GOOD);
		await activity.refresh();
		expect(activity.data).toEqual(GOOD);
		expect(activity.error).toBe('');
	}

	it('keeps last-good data and stays error-free through a brief blip', async () => {
		await seedGood();
		vi.spyOn(api, 'activity').mockRejectedValue(new TypeError('Failed to fetch'));

		await activity.refresh(); // failure 1
		await activity.refresh(); // failure 2 — still under FAIL_GRACE

		expect(activity.data).toEqual(GOOD); // last-good retained, not blanked
		expect(activity.error).toBe(''); // no hard error yet
	});

	it('surfaces a hard error only after sustained failure', async () => {
		await seedGood();
		vi.spyOn(api, 'activity').mockRejectedValue(new TypeError('Failed to fetch'));

		await activity.refresh();
		await activity.refresh();
		await activity.refresh(); // 3rd consecutive failure reaches FAIL_GRACE

		expect(activity.error).not.toBe('');
		expect(activity.data).toEqual(GOOD); // still shows last-good alongside the error
	});

	it('does not flash an error on an upstream ForwardAuth ReauthError', async () => {
		await seedGood();
		vi.spyOn(api, 'activity').mockRejectedValue(new ReauthError());

		await activity.refresh();

		// A top-level reload is underway (api.ts); the surface stays quiet.
		expect(activity.error).toBe('');
		expect(activity.data).toEqual(GOOD);
	});

	it('recovers cleanly after a failure streak once the endpoint returns', async () => {
		await seedGood();
		vi.spyOn(api, 'activity')
			.mockRejectedValueOnce(new TypeError('x'))
			.mockRejectedValueOnce(new TypeError('x'))
			.mockRejectedValueOnce(new TypeError('x'))
			.mockResolvedValueOnce(GOOD);

		await activity.refresh();
		await activity.refresh();
		await activity.refresh();
		expect(activity.error).not.toBe('');

		await activity.refresh(); // success clears the error and resets backoff
		expect(activity.error).toBe('');
		expect(activity.data).toEqual(GOOD);
	});

	it('still drops to read-only on a Holodex 401 owner-expiry (ADR-046)', async () => {
		await seedGood();
		vi.spyOn(api, 'activity').mockRejectedValue(new ApiError(401, '/admin/activity'));
		// dropIfNotOwner re-reads caps; a non-owner result discards the activity data.
		vi.spyOn(api, 'capabilities').mockResolvedValue({ owner: false } as Capabilities);

		await activity.refresh();

		expect(activity.error).not.toBe('');
		expect(activity.data).toBeNull(); // owner data cleared
	});
});
