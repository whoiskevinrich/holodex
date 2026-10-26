// Hover-montage playback for a grid card (F83, docs/design/moving-thumbnails-handoff.md).
// The card's still stays on screen until the montage's first frame is actually playing,
// and only one card on the page plays at a time.

export const ARM_DELAY_MS = 400; // pointer rest before playback starts (spec P0-3)
export const FADE_MS = 150; // still ↔ motion cross-fade

// Whether this viewer can get a montage at all: a fine pointer that can hover, and no
// reduced-motion preference (spec P0-5). Touch-only devices and reduced motion keep stills.
export function montageAllowed(): boolean {
	if (typeof window === 'undefined' || !window.matchMedia) return false;
	return (
		window.matchMedia('(hover: hover) and (pointer: fine)').matches &&
		!window.matchMedia('(prefers-reduced-motion: reduce)').matches
	);
}

// One active card per page: claiming a card releases the previous one (spec P0-4).
export class MontageController {
	private active: CardMontage | null = null;

	claim(card: CardMontage) {
		if (this.active && this.active !== card) this.active.release();
		this.active = card;
	}

	drop(card: CardMontage) {
		if (this.active === card) this.active = null;
	}
}

export const montageController = new MontageController();

export class CardMontage {
	// The <video> mounts only while `src` is set, so a card nobody hovers never requests one.
	src = $state<string | undefined>(undefined);
	// play is the cue for the element to start; visible flips on only once it really plays.
	play = $state(false);
	visible = $state(false);
	failed = $state(false);

	private armTimer: ReturnType<typeof setTimeout> | undefined;
	private clearTimer: ReturnType<typeof setTimeout> | undefined;
	private playing = false;

	constructor(
		private url: () => string | undefined,
		private controller: MontageController = montageController,
		private allowed: () => boolean = montageAllowed
	) {}

	get enabled(): boolean {
		return !!this.url() && !this.failed && this.allowed();
	}

	// A pointer settling on the card arms it; touch never does.
	pointerenter = (pointerType: string) => {
		if (pointerType === 'touch') return;
		this.start(ARM_DELAY_MS);
	};

	// Keyboard focus plays at once.
	focus = () => this.start(0);

	private start(delay: number) {
		if (!this.enabled) return;
		this.controller.claim(this);
		clearTimeout(this.clearTimer);
		clearTimeout(this.armTimer);
		this.src = this.url();
		if (delay === 0) {
			this.play = true;
		} else {
			this.armTimer = setTimeout(() => (this.play = true), delay);
		}
	}

	onplaying = () => {
		this.playing = true;
		this.visible = this.play;
	};

	// Leaving, blurring, Escape, scrolling away or another card starting: fade back to the
	// still, then unmount the element so the next play starts from the first clip.
	release = () => {
		if (!this.src) return; // never armed: nothing to undo
		clearTimeout(this.armTimer);
		this.play = false;
		this.playing = false;
		this.visible = false;
		this.controller.drop(this);
		clearTimeout(this.clearTimer);
		this.clearTimer = setTimeout(() => (this.src = undefined), FADE_MS);
	};

	// A montage that fails to load leaves the card on its still for this page view, with no
	// retry (spec P0-6).
	onerror = () => {
		console.debug('montage failed to load', this.src);
		this.failed = true;
		this.release();
	};
}
