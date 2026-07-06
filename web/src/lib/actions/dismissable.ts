import type { Action } from 'svelte/action';

export interface DismissableOptions {
	/** Only listen while the popover is open — toggles the listeners on/off. */
	enabled: boolean;
	/**
	 * CSS selector marking the "inside" region(s): a click that isn't within an element matching
	 * it dismisses. Put it on the wrapper that holds both the trigger and the menu so clicking the
	 * trigger (or, in a list, a sibling's trigger) counts as inside — its own handler toggles/
	 * switches instead of this dismissing-then-reopening.
	 */
	inside: string;
	/**
	 * Called to dismiss. The argument is `true` when triggered by Escape and `false` on an
	 * outside click, so the caller can restore focus to the trigger on keyboard-close but not
	 * on a pointer-click (the pointer has already left).
	 */
	onclose: (viaEscape: boolean) => void;
}

/**
 * `use:dismissable` — the shared "open popover" dismissal idiom (HOLODEX-164): while `enabled`,
 * Escape or a click outside `inside` closes it. Escape is captured (capture-phase + stopPropagation)
 * so it dismisses only this popover, not an ancestor modal. Attach once to a stable ancestor (the
 * list/chip wrapper); the window listeners do the work, so the node itself is just a mount point.
 * Extracted from the verbatim copies in EnrichProviderChips + /tags' per-pill ⋯ menu.
 */
export const dismissable: Action<HTMLElement, DismissableOptions> = (_node, options) => {
	let opts = options;

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			opts.onclose(true);
		}
	}
	function onClick(e: MouseEvent) {
		const t = e.target;
		if (!(t instanceof Element) || !t.closest(opts.inside)) opts.onclose(false);
	}

	function activate() {
		window.addEventListener('keydown', onKey, true);
		window.addEventListener('click', onClick);
	}
	function deactivate() {
		window.removeEventListener('keydown', onKey, true);
		window.removeEventListener('click', onClick);
	}

	if (opts.enabled) activate();

	return {
		update(next: DismissableOptions) {
			const was = opts.enabled;
			opts = next;
			if (opts.enabled && !was) activate();
			else if (!opts.enabled && was) deactivate();
		},
		destroy: deactivate
	};
};
