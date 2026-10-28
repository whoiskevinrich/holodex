import { afterAll, afterEach, beforeAll, describe, expect, it, onTestFinished, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import ThumbnailFramePicker from './ThumbnailFramePicker.svelte';

// The thumbnail frame picker (F84, testing-strategy §36.2): silent, one frame per arrow key,
// a captured JPEG handed to onuse, and failures kept in the dialog. No real video is decoded
// here, so the element is faked to a 100 s, 1280×720 video, and requestVideoFrameCallback is
// hidden so the picker takes its 1/30 s fallback instead of measuring.

const play = vi.spyOn(HTMLMediaElement.prototype, 'play');
const proto = HTMLVideoElement.prototype as Partial<HTMLVideoElement>;
const rvfc = proto.requestVideoFrameCallback;
beforeAll(() => delete proto.requestVideoFrameCallback);
afterAll(() => {
	proto.requestVideoFrameCallback = rvfc;
});

function fakeVideo(v: HTMLVideoElement) {
	const sets: number[] = [];
	let t = 0;
	// The fake src 404s; keep that from reading as a codec failure.
	v.addEventListener('error', (e) => e.stopImmediatePropagation(), true);
	Object.defineProperties(v, {
		duration: { get: () => 100 },
		seeking: { get: () => false },
		videoWidth: { get: () => 1280 },
		videoHeight: { get: () => 720 },
		currentTime: {
			get: () => t,
			set: (x: number) => {
				t = x;
				sets.push(x);
			}
		}
	});
	v.dispatchEvent(new Event('loadedmetadata'));
	return sets;
}

async function mount() {
	const onuse = vi.fn(async (_f: File, _at: number) => {});
	const onreset = vi.fn();
	const onclose = vi.fn();
	const screen = await render(ThumbnailFramePicker, { src: '/no-video.mp4', startAt: 10, onuse, onreset, onclose });
	const video = document.querySelector('video')!;
	const sets = fakeVideo(video);
	await expect.element(screen.getByRole('button', { name: 'Use this frame' })).toBeEnabled();
	return { screen, video, sets, onuse, onreset, onclose };
}

afterEach(() => {
	play.mockClear();
});

describe('ThumbnailFramePicker', () => {
	it('is silent: muted, no controls, never played (C1)', async () => {
		const { video, screen } = await mount();
		expect(video.muted).toBe(true);
		expect(video.volume).toBe(0);
		expect(video.controls).toBe(false);
		expect(video.autoplay).toBe(false);
		screen.getByRole('slider').element().focus();
		await userEvent.keyboard('{ArrowRight}{ArrowLeft}');
		expect(play).not.toHaveBeenCalled();
	});

	it('steps one frame per arrow key, into the middle of the next frame (C2)', async () => {
		const { screen, sets } = await mount();
		sets.length = 0;
		screen.getByRole('slider').element().focus();
		await userEvent.keyboard('{ArrowRight}');
		expect(sets).toHaveLength(1);
		expect(sets[0]).toBeCloseTo(10 + 1.5 / 30, 6);
		await userEvent.keyboard('{ArrowLeft}');
		expect(sets[1]).toBeCloseTo(10 + 0.5 / 30, 6);
	});

	it('still steps with focus on the panel, as after clicking the frame', async () => {
		const { screen, sets } = await mount();
		sets.length = 0;
		screen.getByRole('dialog').element().focus();
		await userEvent.keyboard('{ArrowRight}');
		expect(sets).toHaveLength(1);
	});

	it('hands onuse a JPEG of the frame, and keeps the dialog open on failure (C3)', async () => {
		const draw = vi.spyOn(CanvasRenderingContext2D.prototype, 'drawImage').mockImplementation(() => {});
		onTestFinished(() => draw.mockRestore());
		const { screen, onuse } = await mount();
		onuse.mockRejectedValueOnce(new Error('upload failed: 400'));
		await screen.getByRole('button', { name: 'Use this frame' }).click();
		await expect.element(screen.getByRole('alert')).toHaveTextContent('upload failed: 400');
		const [file, at] = onuse.mock.calls[0];
		expect(file.type).toBe('image/jpeg');
		expect(at).toBeCloseTo(10, 0); // the picked time, for the ·frame tile's title (F85 P1-1)
		expect(screen.getByRole('dialog').element()).toBeTruthy();
	});

	it('cancels and resets without saving (C4)', async () => {
		const { screen, onuse, onreset, onclose } = await mount();
		await screen.getByRole('button', { name: 'Reset to automatic' }).click();
		await screen.getByRole('button', { name: 'Cancel' }).click();
		expect(onreset).toHaveBeenCalledOnce();
		expect(onclose).toHaveBeenCalledOnce();
		expect(onuse).not.toHaveBeenCalled();
	});
});
