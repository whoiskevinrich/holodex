// Frame-stepping maths for the thumbnail frame picker (F84,
// docs/design/thumbnail-frame-picker-handoff.md). Pure, so the stepping rules are pinned by
// framePicker.test.ts without a real decoder.
import { formatDuration } from '#lib/format.ts';

// One frame when the browser can't measure it (spec P0-6).
export const FALLBACK_FRAME = 1 / 30;

// The interval between two presented frames' media times. Anything non-positive or longer than
// half a second is a failed measurement (a seek that skipped ahead), not a frame rate.
export function frameInterval(a: number, b: number): number {
	const d = b - a;
	return d > 0 && d <= 0.5 ? d : FALLBACK_FRAME;
}

// Where to seek for the neighbouring frame of the one presented at `frameTime`: the middle of
// that frame, so float error in currentTime can't land on the frame either side of it.
export function stepTarget(frameTime: number, interval: number, dir: 1 | -1, duration: number): number {
	const t = frameTime + dir * interval + interval / 2;
	return Math.min(Math.max(t, 0), duration);
}

// "13:04 · frame 10": the whole second, then the frame within it.
export function timecode(t: number, interval: number): string {
	const whole = Math.floor(t + 1e-6);
	const perSecond = Math.max(1, Math.round(1 / interval));
	const frame = Math.min(Math.floor((t - whole) / interval + 1e-6), perSecond - 1);
	return `${formatDuration(whole)} · frame ${Math.max(0, frame)}`;
}

// Turns wheel deltas into whole frame steps (handoff, "Scrub bar"): one step per 50px, a
// line-mode notch counting as 100px, so a mouse notch is one step and a trackpad swipe isn't
// one step per event. A direction change drops the remainder.
export class WheelStepper {
	static readonly STEP_PX = 50;
	#acc = 0;

	push(e: Pick<WheelEvent, 'deltaY' | 'deltaMode'>): number {
		const px = e.deltaMode === 0 ? e.deltaY : Math.sign(e.deltaY) * 100;
		if (px === 0) return 0;
		if (Math.sign(px) !== Math.sign(this.#acc)) this.#acc = 0;
		this.#acc += px;
		const steps = Math.trunc(this.#acc / WheelStepper.STEP_PX);
		this.#acc -= steps * WheelStepper.STEP_PX;
		// A mouse notch (100px) is one frame, not two.
		return steps === 0 ? 0 : Math.sign(steps);
	}
}
