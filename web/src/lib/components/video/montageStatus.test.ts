// The owner's Hover montage row (F83, testing-strategy §35.5 F5).
import { describe, expect, it } from 'vitest';
import { montageStatusView, shouldPollMontage } from './montageStatus';
import type { MontageStatus } from '#lib/types.ts';

const st = (s: Partial<MontageStatus>): MontageStatus => ({ mode: 'manual', state: 'none', ...s });

describe('montageStatusView', () => {
	it('maps each status to its value and actions per the handoff table', () => {
		expect(montageStatusView(st({ state: 'none' }))).toEqual({ value: 'None yet', actions: ['make'] });
		expect(montageStatusView(st({ state: 'waiting' }))).toEqual({ value: 'Waiting in queue…', actions: [] });
		expect(montageStatusView(st({ state: 'making' }))).toEqual({ value: 'Making…', actions: [] });
		expect(montageStatusView(st({ state: 'ready', size_bytes: 248 * 1024 }))).toEqual({
			value: 'Ready',
			detail: '248.0 KB',
			actions: ['preview', 'remake']
		});
		expect(montageStatusView(st({ state: 'failed', error: "can't read file" }))).toEqual({
			value: 'Failed',
			detail: "can't read file",
			actions: ['retry']
		});
	});
});

describe('shouldPollMontage', () => {
	it('polls only while waiting or making', () => {
		expect(shouldPollMontage(st({ state: 'waiting' }))).toBe(true);
		expect(shouldPollMontage(st({ state: 'making' }))).toBe(true);
		for (const state of ['none', 'ready', 'failed'] as const) {
			expect(shouldPollMontage(st({ state }))).toBe(false);
		}
		expect(shouldPollMontage(null)).toBe(false);
	});
});
