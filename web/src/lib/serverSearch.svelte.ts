// Bounded, typed-query server search for the pickers and suggestion inputs (HOLODEX-586):
// they ask the server for a few matches per query instead of fetching a whole entity
// table and filtering it in the browser. Debounced, and a slow answer to an older query
// never overwrites a newer one. An empty query clears the results without a request.
import { toMessage } from './format.ts';

export class ServerSearch<T> {
	items = $state<T[]>([]);
	loading = $state(false);
	error = $state('');
	// The trimmed query `items` answers ('' while nothing has been asked).
	answered = $state('');

	#run: (q: string) => Promise<T[]>;
	#delay: number;
	#timer: ReturnType<typeof setTimeout> | undefined;
	#seq = 0;

	constructor(run: (q: string) => Promise<T[]>, delay = 300) {
		this.#run = run;
		this.#delay = delay;
	}

	// Ask for `query`'s matches after the debounce.
	set(query: string) {
		clearTimeout(this.#timer);
		const q = query.trim();
		const id = ++this.#seq;
		if (!q) {
			this.items = [];
			this.answered = '';
			this.loading = false;
			this.error = '';
			return;
		}
		this.loading = true;
		this.#timer = setTimeout(() => void this.#fetch(q, id), this.#delay);
	}

	async #fetch(q: string, id: number) {
		try {
			const items = await this.#run(q);
			if (id !== this.#seq) return;
			this.items = items;
			this.answered = q;
			this.error = '';
		} catch (e) {
			if (id !== this.#seq) return;
			this.items = [];
			this.error = toMessage(e);
		} finally {
			if (id === this.#seq) this.loading = false;
		}
	}

	// Drop any pending or in-flight query (a closing picker).
	cancel() {
		clearTimeout(this.#timer);
		this.#seq++;
		this.loading = false;
	}
}
