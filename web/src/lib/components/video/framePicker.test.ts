import { describe, expect, it } from 'vitest';
import { FALLBACK_FRAME, WheelStepper, frameInterval, stepTarget, timecode } from './framePicker.ts';

// Frame stepping for the thumbnail frame picker (F84, testing-strategy §36.1).

describe('stepTarget', () => {
	const d = 1 / 25;
	it('seeks into the middle of the neighbouring frame (F1)', () => {
		expect(stepTarget(10, d, 1, 100)).toBeCloseTo(10 + 1.5 * d, 9);
		expect(stepTarget(10, d, -1, 100)).toBeCloseTo(10 - 0.5 * d, 9);
	});
	it('clamps to the video', () => {
		expect(stepTarget(0, d, -1, 100)).toBe(0);
		expect(stepTarget(99.99, d, 1, 100)).toBe(100);
	});
});

describe('frameInterval', () => {
	it('is the gap between two presented frames (F2)', () => {
		expect(frameInterval(1, 1 + 1 / 24)).toBeCloseTo(1 / 24, 9);
	});
	it('falls back to 1/30 on a failed measurement', () => {
		expect(frameInterval(2, 2)).toBe(FALLBACK_FRAME);
		expect(frameInterval(2, 1)).toBe(FALLBACK_FRAME);
		expect(frameInterval(2, 3)).toBe(FALLBACK_FRAME);
	});
});

describe('WheelStepper', () => {
	it('gives one step per mouse notch (F3)', () => {
		const w = new WheelStepper();
		expect(w.push({ deltaY: 100, deltaMode: 0 })).toBe(1);
		expect(w.push({ deltaY: -100, deltaMode: 0 })).toBe(-1);
	});
	it('accumulates trackpad deltas', () => {
		const w = new WheelStepper();
		expect(w.push({ deltaY: 20, deltaMode: 0 })).toBe(0);
		expect(w.push({ deltaY: 20, deltaMode: 0 })).toBe(0);
		expect(w.push({ deltaY: 20, deltaMode: 0 })).toBe(1);
	});
	it('counts a line-mode notch as one step', () => {
		expect(new WheelStepper().push({ deltaY: 3, deltaMode: 1 })).toBe(1);
	});
	it('drops the remainder on a direction change', () => {
		const w = new WheelStepper();
		expect(w.push({ deltaY: 40, deltaMode: 0 })).toBe(0);
		expect(w.push({ deltaY: -20, deltaMode: 0 })).toBe(0);
		expect(w.push({ deltaY: -30, deltaMode: 0 })).toBe(-1);
	});
});

describe('timecode', () => {
	it('gives the second and the frame within it (F4)', () => {
		expect(timecode(784.4, 1 / 25)).toBe('13:04 · frame 10');
		expect(timecode(3600 + 5, 1 / 25)).toBe('1:00:05 · frame 0');
	});
	it('never reaches the frame rate', () => {
		expect(timecode(1.9999, 1 / 25)).toBe('0:01 · frame 24');
	});
});
