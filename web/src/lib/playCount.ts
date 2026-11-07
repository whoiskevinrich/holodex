// What counts as a play (F89 P0-7; owner's choice, watched time rather than position): the
// owner's watched time reaching 90% of the video's length. Only time that played counts, so a
// seek adds nothing, and each counted play starts the next from zero.

const COUNT_AT = 0.9;

export class PlayCounter {
	private watched = 0;
	private last: number | null = null;

	constructor(
		private isOwner: () => boolean,
		private post: (progress: number) => void
	) {}

	/** A different video, or the page's first. */
	reset() {
		this.watched = 0;
		this.last = null;
	}

	/** The element's `seeking`: the next position is where the seek landed, not played time. */
	seeking() {
		this.last = null;
	}

	/** The element's `timeupdate`. */
	timeupdate(position: number, duration: number) {
		const delta = this.last === null ? 0 : position - this.last;
		this.last = position;
		if (delta <= 0 || !this.isOwner() || !(duration > 0)) return;
		this.watched += delta;
		if (this.watched >= duration * COUNT_AT) {
			this.watched = 0;
			this.post(Math.min(position / duration, 1));
		}
	}
}
