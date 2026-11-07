import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { tick } from 'svelte';
import TrimPicker from './TrimPicker.svelte';

// The trim picker (F88, testing-strategy §40.6). No real video is decoded here, so the element is
// faked to a 100 s, 1280×720 video, and requestVideoFrameCallback is hidden so the picker takes its
// 1/30 s fallback. Real decoding is checked in the browser (§40.7).

const play = vi.spyOn(HTMLMediaElement.prototype, 'play');
const proto = HTMLVideoElement.prototype as Partial<HTMLVideoElement>;
const rvfc = proto.requestVideoFrameCallback;
beforeAll(() => delete proto.requestVideoFrameCallback);
afterAll(() => {
	proto.requestVideoFrameCallback = rvfc;
});

function fakeVideo(v: HTMLVideoElement, width = 1280) {
	let t = 0;
	const sets: number[] = [];
	v.addEventListener('error', (e) => e.stopImmediatePropagation(), true);
	Object.defineProperties(v, {
		duration: { get: () => 100 },
		seeking: { get: () => false },
		videoWidth: { get: () => width },
		videoHeight: { get: () => 720 },
		currentTime: {
			get: () => t,
			set: (x: number) => {
				t = x;
				sets.push(x);
				queueMicrotask(() => v.dispatchEvent(new Event('seeked')));
			}
		}
	});
	v.dispatchEvent(new Event('loadedmetadata'));
	return sets;
}

const okCheck = { ok: true, need_bytes: 3 << 30, free_bytes: 400 * (1 << 30), expires_at: '2026-11-13T12:00:00Z' };

async function mount(opts: { preflight?: () => Promise<typeof okCheck>; onconfirm?: () => Promise<unknown> } = {}) {
	const preflight = vi.fn(opts.preflight ?? (async () => okCheck));
	const onconfirm = vi.fn(opts.onconfirm ?? (async () => {}));
	const onclose = vi.fn();
	const screen = await render(TrimPicker, { src: '/no-video.mp4', filename: 'a {edition-Old}.mkv', preflight, onconfirm, onclose } as never);
	const video = document.querySelector('video')!;
	const sets = fakeVideo(video);
	const first = screen.getByRole('slider', { name: 'First frame kept' });
	const last = screen.getByRole('slider', { name: 'Last frame kept' });
	await vi.waitFor(() => expect(first.element().getAttribute('aria-disabled')).toBe('false'));
	return { screen, video, sets, preflight, onconfirm, onclose, first, last };
}

// Step the first handle forward and wait for it to come to rest.
async function moveFirstHandle(m: Awaited<ReturnType<typeof mount>>) {
	(m.first.element() as HTMLElement).focus();
	await userEvent.keyboard('{ArrowRight}');
	await expect.element(m.screen.getByRole('button', { name: 'Review trim…' })).toBeEnabled();
}

afterEach(() => play.mockClear());

describe('TrimPicker', () => {
	it('is silent: muted, no controls, never played (P1)', async () => {
		const m = await mount();
		expect(m.video.muted).toBe(true);
		expect(m.video.volume).toBe(0);
		expect(m.video.controls).toBe(false);
		expect(m.video.autoplay).toBe(false);
		await moveFirstHandle(m);
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await m.screen.getByRole('button', { name: '‹ Back' }).click();
		expect(play).not.toHaveBeenCalled();
	});

	it('opens on the file ends, first active, with nothing to review (P2, P3)', async () => {
		const m = await mount();
		expect(Number(m.first.element().getAttribute('aria-valuenow'))).toBe(0);
		expect(Number(m.last.element().getAttribute('aria-valuenow'))).toBeGreaterThan(99.9);
		await expect.element(m.screen.getByRole('button', { name: /Work on the first frame/ })).toHaveAttribute('aria-pressed', 'true');
		await expect.element(m.screen.getByRole('button', { name: 'Review trim…' })).toBeDisabled();
		await expect.element(m.screen.getByText('Move a handle to choose what to cut.')).toBeVisible();
	});

	it('steps only the active handle, one frame per key (P2)', async () => {
		const m = await mount();
		const before = Number(m.last.element().getAttribute('aria-valuenow'));
		(m.last.element() as HTMLElement).focus();
		await userEvent.keyboard('{ArrowLeft}');
		await vi.waitFor(() => expect(Number(m.last.element().getAttribute('aria-valuenow'))).toBeCloseTo(before - 1 / 30, 2));
		expect(Number(m.first.element().getAttribute('aria-valuenow'))).toBe(0);
	});

	it('Home and End go to the active handle’s limits', async () => {
		const m = await mount();
		(m.last.element() as HTMLElement).focus();
		await userEvent.keyboard('{Home}');
		await vi.waitFor(() => {
			const v = Number(m.last.element().getAttribute('aria-valuenow'));
			expect(v).toBeGreaterThan(0);
			expect(v).toBeLessThan(0.1);
		});
	});

	it('review arrives on Replace original, and Back keeps the handles (P4)', async () => {
		const m = await mount();
		await moveFirstHandle(m);
		const now = m.first.element().getAttribute('aria-valuenow');
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await expect.element(m.screen.getByRole('radio', { name: /Replace original/ })).toBeChecked();
		await expect.element(m.screen.getByRole('button', { name: 'Replace original' })).toBeEnabled();
		await expect.element(m.screen.getByText(/Original kept until 13 Nov/)).toBeVisible();
		await m.screen.getByRole('button', { name: '‹ Back' }).click();
		expect(m.first.element().getAttribute('aria-valuenow')).toBe(now);
	});

	it('edition reveals the prefilled name and its file name (P5)', async () => {
		const m = await mount();
		await moveFirstHandle(m);
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await m.screen.getByRole('radio', { name: /Save as edition/ }).click();
		await expect.element(m.screen.getByRole('textbox', { name: 'Edition name' })).toHaveValue('Trimmed');
		await expect.element(m.screen.getByText('a {edition-Trimmed}.mkv')).toBeVisible();
		await expect.element(m.screen.getByRole('button', { name: 'Save edition' })).toBeEnabled();
		await m.screen.getByRole('textbox', { name: 'Edition name' }).fill('');
		await expect.element(m.screen.getByText('Enter a name for this edition.')).toBeVisible();
		await expect.element(m.screen.getByRole('button', { name: 'Save edition' })).toBeDisabled();
	});

	it('withdraws the primary while checking, and never shows it first', async () => {
		let answer!: (v: typeof okCheck) => void;
		const m = await mount({ preflight: () => new Promise((r) => (answer = r)) });
		await moveFirstHandle(m);
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await expect.element(m.screen.getByText('Checking…')).toBeVisible();
		await expect.element(m.screen.getByRole('button', { name: 'Replace original' })).toBeDisabled();
		await vi.waitFor(() => expect(answer).toBeDefined());
		answer(okCheck);
		await expect.element(m.screen.getByRole('button', { name: 'Replace original' })).toBeEnabled();
		// A second check, after a passed one: the old answer doesn't keep the primary available.
		const before = answer;
		await m.screen.getByRole('radio', { name: /Save as edition/ }).click();
		await vi.waitFor(() => expect(answer).not.toBe(before));
		await expect.element(m.screen.getByRole('button', { name: 'Save edition' })).toBeDisabled();
		answer(okCheck);
		await expect.element(m.screen.getByRole('button', { name: 'Save edition' })).toBeEnabled();
	});

	it('a refusal replaces the save choice', async () => {
		const m = await mount({ preflight: async () => ({ ...okCheck, ok: false, reason: 'flac_in_matroska' }) });
		await moveFirstHandle(m);
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await expect.element(m.screen.getByText(/FLAC audio can't be cut yet/)).toBeVisible();
		await expect.element(m.screen.getByRole('radio', { name: /Replace original/ })).not.toBeInTheDocument();
		await expect.element(m.screen.getByRole('button', { name: '‹ Back' })).toBeEnabled();
	});

	it('hands over the times and both captures; a failure keeps the dialog (P6)', async () => {
		const m = await mount({
			onconfirm: async () => {
				throw new Error('The queue is unavailable');
			}
		});
		await moveFirstHandle(m);
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await m.screen.getByRole('button', { name: 'Replace original' }).click();
		await expect.element(m.screen.getByText('The queue is unavailable')).toBeVisible();
		const t = m.onconfirm.mock.calls[0][0 as never] as unknown as { inTime: number; outTime: number; mode: string; captureIn: File; captureOut: File };
		expect(t.mode).toBe('replace');
		expect(t.inTime).toBeGreaterThan(0);
		expect(t.outTime).toBeGreaterThan(99.9);
		expect(t.captureIn.type).toBe('image/jpeg');
		expect(t.captureOut.type).toBe('image/jpeg');
	});

	it('shows Can\'t decode for a video track the browser can\'t draw (P3)', async () => {
		const screen = await render(TrimPicker, { src: '/no-video.mp4', filename: 'a.mkv', preflight: vi.fn(), onconfirm: vi.fn(), onclose: vi.fn() } as never);
		fakeVideo(document.querySelector('video')!, 0);
		await expect.element(screen.getByText("This browser can't decode this file's codec.")).toBeVisible();
		await expect.element(screen.getByRole('button', { name: 'Review trim…' })).toBeDisabled();
	});

	it('an end chosen mid-move waits for the moved one to rest', async () => {
		const m = await mount();
		const still = m.screen.getByRole('button', { name: /Work on the last frame/ }).element() as HTMLElement;
		// Synchronous, so both land inside the 150 ms rest: the first end is still the one in hand.
		m.first.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
		still.click();
		await tick();
		expect(still.getAttribute('aria-pressed')).toBe('false');
		await vi.waitFor(() => expect(still.getAttribute('aria-pressed')).toBe('true'));
		expect(Number(m.first.element().getAttribute('aria-valuenow'))).toBeCloseTo(1 / 30, 3);
	});

	it('Back returns focus to the handle, so the arrow keys still step', async () => {
		const m = await mount();
		await moveFirstHandle(m);
		await m.screen.getByRole('button', { name: 'Review trim…' }).click();
		await m.screen.getByRole('button', { name: '‹ Back' }).click();
		await vi.waitFor(() => expect(document.activeElement).toBe(m.first.element()));
	});

	it('gives each handle its own limits', async () => {
		const m = await mount();
		expect(Number(m.last.element().getAttribute('aria-valuemin'))).toBeCloseTo(1 / 30, 3);
		expect(Number(m.first.element().getAttribute('aria-valuemax'))).toBeLessThan(99.97);
	});

	it('Cancel closes with no trim', async () => {
		const m = await mount();
		await m.screen.getByRole('button', { name: 'Cancel' }).click();
		expect(m.onclose).toHaveBeenCalled();
		expect(m.onconfirm).not.toHaveBeenCalled();
	});
});
