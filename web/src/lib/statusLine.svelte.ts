// StatusLine is a page's transient success line (HOLODEX-86): rendered in the slot
// beside the page's inline error, under aria-live, and cleared after a few seconds.
// It is deliberately not a toast — the app has no toast system, and the metadata
// pages report outcomes inline (metadata-refresh.md). A newer show() replaces the
// text and restarts the clock; show('') clears it at once.
export class StatusLine {
	text = $state('');
	#timer: ReturnType<typeof setTimeout> | undefined;
	#ms: number;

	constructor(ms = 4000) {
		this.#ms = ms;
	}

	show(text: string) {
		clearTimeout(this.#timer);
		this.text = text;
		if (text) this.#timer = setTimeout(() => (this.text = ''), this.#ms);
	}
}
