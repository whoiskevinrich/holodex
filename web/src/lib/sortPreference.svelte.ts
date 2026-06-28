// Per-page sticky sort preference + a shared session shuffle seed (spec
// sort-persistence SP1/SP2, ADR-045).
//
// SP1 — each index page (media, people, tags) remembers its last-chosen sort under
// its own localStorage key, validated against that page's allowed set so a stale or
// corrupt value safely falls back to the default. Mirrors the `holodex-theme`
// pattern (SSR-safe guard, never throws).
//
// SP2 — "Random" is stable per session: one integer seed, held in sessionStorage so
// it survives in-SPA navigation and a same-tab reload, drives both the client-side
// People/Tags shuffle and the server-side Media `holo_shuffle`. A new tab/session
// draws a fresh seed; reroll() regenerates it on demand.

function keyFor(page: string): string {
	return `holodex:sort:${page}`;
}

// readSort returns the saved sort for a page if it is still a valid option, else the
// default. `allowed` makes removing an option forward-safe — a stale value falls back.
export function readSort<T extends string>(page: string, allowed: readonly T[], def: T): T {
	if (typeof localStorage === 'undefined') return def;
	try {
		const v = localStorage.getItem(keyFor(page));
		if (v && (allowed as readonly string[]).includes(v)) return v as T;
	} catch {
		// Malformed or unavailable storage — fall through to the default.
	}
	return def;
}

export function writeSort(page: string, value: string): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(keyFor(page), value);
	} catch {
		// Storage full/unavailable — the preference just won't persist (non-fatal).
	}
}

const SEED_KEY = 'holodex:shuffle-seed';

// A positive 31-bit integer: safe to send to the server as ?seed= and to feed the
// client PRNG. Math.random is fine — this is a cosmetic shuffle, not a secret.
function newSeed(): number {
	return Math.floor(Math.random() * 0x7fffffff);
}

function loadSeed(): number {
	if (typeof sessionStorage === 'undefined') return newSeed();
	try {
		const raw = sessionStorage.getItem(SEED_KEY);
		const n = raw == null ? NaN : Number(raw);
		if (Number.isInteger(n) && n >= 0) return n;
		const s = newSeed();
		sessionStorage.setItem(SEED_KEY, String(s));
		return s;
	} catch {
		return newSeed();
	}
}

class ShuffleSeed {
	value = $state(loadSeed());

	// reroll draws a new shuffle for the rest of the session. Reading `value`
	// reactively (the People/Tags derived shuffle, the Media fetch) re-applies it.
	reroll() {
		this.value = newSeed();
		if (typeof sessionStorage !== 'undefined') {
			try {
				sessionStorage.setItem(SEED_KEY, String(this.value));
			} catch {
				// non-fatal — the seed still drives this session in memory
			}
		}
	}
}

export const shuffleSeed = new ShuffleSeed();
