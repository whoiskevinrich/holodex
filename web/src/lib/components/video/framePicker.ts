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

// FrameReader tracks which frame a paused <video> is presenting, from requestVideoFrameCallback.
// A paused video presents a frame only after a seek, so its loop is idle between seeks. Shared by
// the frame picker and the trim picker, which must agree on what "this frame's time" means.
export class FrameReader {
	last = 0; // the presented frame's media time
	count = 0; // frames reported; 0 means the browser isn't reporting any
	#alive = true;
	#onReport: (() => void) | null = null; // read()'s early finish

	constructor(private onFrame?: (mediaTime: number, seeking: boolean) => void) {}

	static get supported(): boolean {
		return typeof HTMLVideoElement !== 'undefined' && 'requestVideoFrameCallback' in HTMLVideoElement.prototype;
	}

	watch(v: HTMLVideoElement) {
		const cb: VideoFrameRequestCallback = (_now, meta) => {
			this.last = meta.mediaTime;
			this.count++;
			this.onFrame?.(meta.mediaTime, v.seeking);
			this.#onReport?.();
			if (this.#alive) v.requestVideoFrameCallback(cb);
		};
		v.requestVideoFrameCallback(cb);
	}

	stop() {
		this.#alive = false;
	}

	// Seek and resolve with the presented frame's media time: as soon as a new frame is reported
	// after the seek, else from the record shortly after it settles (a seek inside the same frame
	// presents nothing new, so the record is already right). Null when the browser has reported no
	// frame at all (a page that isn't rendering).
	read(v: HTMLVideoElement, t: number): Promise<number | null> {
		return new Promise((resolve) => {
			const before = this.count;
			let done = false;
			const finish = () => {
				if (done) return;
				done = true;
				this.#onReport = null;
				resolve(this.count > 0 ? this.last : null);
			};
			this.#onReport = () => {
				if (this.count > before && !v.seeking) finish();
			};
			v.addEventListener('seeked', () => setTimeout(finish, 200), { once: true });
			setTimeout(finish, 1500); // a seek that never settles
			v.currentTime = t;
		});
	}

	// Spec P0-6: one frame is the real interval between two presented frames, probed in 1/120 s
	// steps from the opening frame. Leaves the video on that frame. Null when no frame is reported.
	async measure(v: HTMLVideoElement, from: number): Promise<{ first: number; interval: number } | null> {
		const first = await this.read(v, from);
		if (first === null) return null;
		let interval = FALLBACK_FRAME;
		for (let k = 1; k <= 8 && this.#alive; k++) {
			const next = await this.read(v, first + k / 120);
			if (next !== null && next > first) {
				interval = frameInterval(first, next);
				break;
			}
		}
		if (this.#alive) await this.read(v, first + interval / 2);
		return { first, interval };
	}
}

export function settled(v: HTMLVideoElement): Promise<void> {
	return v.seeking ? new Promise((r) => v.addEventListener('seeked', () => r(), { once: true })) : Promise.resolve();
}

// The decoded frame on screen as a JPEG. A capture, not a timestamp: the bytes are what was shown.
export async function captureFrame(v: HTMLVideoElement): Promise<File> {
	await settled(v);
	const canvas = document.createElement('canvas');
	canvas.width = v.videoWidth;
	canvas.height = v.videoHeight;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error("This browser can't capture the frame.");
	ctx.drawImage(v, 0, 0, canvas.width, canvas.height);
	const blob = await new Promise<Blob | null>((r) => canvas.toBlob(r, 'image/jpeg', 0.92));
	if (!blob) throw new Error("This browser can't capture the frame.");
	return new File([blob], 'frame.jpg', { type: 'image/jpeg' });
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
