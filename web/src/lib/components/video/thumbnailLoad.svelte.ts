// A video thumbnail's load state, shared by VideoCard and the Media Table's VideoThumb.
// Thumbnails are generated in the background (ADR-009): while one is still generating
// the server returns 404, so the image is retried with backoff until it lands, then the
// caller gives up to its placeholder.
import { api } from '#lib/api.ts';

export const MAX_RETRIES = 5;

// retryDelay is the wait before retry `attempt + 1`: exponential, capped at 30s, with
// ±50% jitter (`rand` in [0, 1)) so a freshly scanned grid doesn't retry every card in
// lockstep against the API.
export function retryDelay(attempt: number, rand: number): number {
	return Math.min(2000 * 2 ** attempt, 30000) * (0.5 + rand);
}

export class ThumbnailLoad {
	attempt = $state(0);
	loaded = $state(false);
	gaveUp = $state(false);

	// `url` reads the video's thumbnail_url, falling back to the bare route while it is
	// still generating, so the retries have something to poll.
	constructor(private url: () => string) {}

	get src(): string {
		return api.thumbnailReload(this.url(), this.attempt);
	}

	onload = () => {
		this.loaded = true;
	};

	onerror = () => {
		if (this.attempt < MAX_RETRIES) {
			const next = this.attempt + 1;
			setTimeout(() => (this.attempt = next), retryDelay(this.attempt, Math.random()));
		} else {
			this.gaveUp = true;
		}
	};
}
