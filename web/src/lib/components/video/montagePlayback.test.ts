// Hover-montage playback (F83, testing-strategy §35.5 F1–F4).
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { vi } from 'vitest';
import { ARM_DELAY_MS, CardMontage, FADE_MS, MontageController } from './montagePlayback.svelte';

const yes = () => true;

describe('CardMontage', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('F1: only one card plays at a time', () => {
		const c = new MontageController();
		const a = new CardMontage(() => '/m/1', c, yes);
		const b = new CardMontage(() => '/m/2', c, yes);
		a.focus();
		a.onplaying();
		expect(a.visible).toBe(true);
		b.focus();
		expect(a.visible).toBe(false);
		expect(a.play).toBe(false);
		expect(b.play).toBe(true);
	});

	it('F2: sets src on enter, shows motion only after 400 ms and a playing event', () => {
		const m = new CardMontage(() => '/m/1', new MontageController(), yes);
		m.pointerenter('mouse');
		expect(m.src).toBe('/m/1');
		expect(m.play).toBe(false);
		m.onplaying(); // the element may report playing before the arm delay
		expect(m.visible).toBe(false);
		vi.advanceTimersByTime(ARM_DELAY_MS - 1);
		expect(m.play).toBe(false);
		vi.advanceTimersByTime(1);
		expect(m.play).toBe(true);
		expect(m.visible).toBe(false); // the cue is set; motion waits for the element
		m.onplaying();
		expect(m.visible).toBe(true);
	});

	it('F2: leaving before the delay cancels the play and drops the src after the fade', () => {
		const m = new CardMontage(() => '/m/1', new MontageController(), yes);
		m.pointerenter('mouse');
		vi.advanceTimersByTime(ARM_DELAY_MS / 2);
		m.release();
		vi.advanceTimersByTime(FADE_MS - 1);
		expect(m.src).toBe('/m/1'); // still mounted while fading
		vi.advanceTimersByTime(ARM_DELAY_MS);
		expect(m.play).toBe(false); // the cancelled arm never fires
		expect(m.src).toBeUndefined();
	});

	it('F3: focus plays at once, and release (blur or Escape) stops it', () => {
		const m = new CardMontage(() => '/m/1', new MontageController(), yes);
		m.focus();
		expect(m.play).toBe(true);
		m.onplaying();
		expect(m.visible).toBe(true);
		m.release();
		expect(m.visible).toBe(false);
	});

	it('F4: never mounts without a montage, when not allowed, on touch, or after a failure', () => {
		const c = new MontageController();
		const none = new CardMontage(() => undefined, c, yes);
		none.focus();
		expect(none.src).toBeUndefined();

		const reduced = new CardMontage(() => '/m/1', c, () => false);
		reduced.focus();
		reduced.pointerenter('mouse');
		expect(reduced.src).toBeUndefined();

		const touch = new CardMontage(() => '/m/1', c, yes);
		touch.pointerenter('touch');
		expect(touch.src).toBeUndefined();

		const broken = new CardMontage(() => '/m/1', c, yes);
		broken.focus();
		broken.onerror();
		vi.advanceTimersByTime(FADE_MS);
		expect(broken.src).toBeUndefined();
		broken.focus();
		expect(broken.src).toBeUndefined();
	});
});
