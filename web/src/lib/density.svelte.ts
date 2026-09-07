// Shared, persisted grid-density preference: how many columns a VideoGrid targets
// at its widest viewport tier. One site-wide value (not per-page, unlike sort/SP1)
// so density stays consistent as you navigate between the media list, entity pages,
// and related shelves.

const KEY = 'holodex:media-density';

// Column ceiling per viewport tier. The rungs at 1536 and below mirror the breakpoints
// VideoGrid used to express as Tailwind utility classes (lg/xl/2xl + a custom 480px step);
// 2560 and 3840 extend the ladder onto ultrawides, where the table used to dead-end — a
// 5120px screen rendered four 1252px cards, a poster row taller than the screen (HOLODEX-331).
//
// Each rung is picked to hold a card in a ~170–320px band at the widths it covers, so "max
// density" means roughly the same card size on every display rather than "however many
// columns the number happens to allow". `density.test.ts` asserts that band.
//
// INVARIANT: sorted by `min` descending — `capForWidth` returns the first match, so an
// out-of-order rung would shadow the ones below it.
//
// Tuned up to 5120 (a 49" ultrawide). Above 3840 the cap stays 16, so a hypothetical 7680px
// display would grow cards to ~462px rather than adding columns — the same shape of dead-end
// this ladder fixed, at twice the width. That is a deliberate stopping point, not an
// oversight: add a rung when such a display is actually in play.
const TIERS: { min: number; cap: number }[] = [
	{ min: 3840, cap: 16 },
	{ min: 2560, cap: 12 },
	{ min: 1536, cap: 8 },
	{ min: 1280, cap: 4 },
	{ min: 1024, cap: 3 },
	{ min: 480, cap: 2 }
];

export function capForWidth(width: number): number {
	return TIERS.find((t) => width >= t.min)?.cap ?? 1;
}

export const DENSITY_MIN = 2;
// The highest column count any viewport can reach. Taken as the max over the rungs rather than
// `TIERS[0].cap`, so it stays correct however the array is ordered — that leaves exactly one
// ordering invariant to hold (the `min`-descending one above), which the boundary tests guard.
export const DENSITY_MAX = Math.max(...TIERS.map((t) => t.cap));
const DEFAULT_DENSITY = 4;

// Clamps to the *global* range, not the current viewport's cap, so a preference set on a wide
// display survives a visit to a narrow one: `effectiveDensity` narrows it at render instead.
// Note this only protects the value while the slider is left alone — the slider's own range
// spans the current cap (see DensitySlider), so deliberately dragging it on a small screen
// does lower the stored preference. That is intended: moving the control is an explicit choice.
function clamp(n: number): number {
	return Math.min(DENSITY_MAX, Math.max(DENSITY_MIN, Math.round(n)));
}

function load(): number {
	if (typeof localStorage === 'undefined') return DEFAULT_DENSITY;
	try {
		const raw = Number(localStorage.getItem(KEY));
		return raw ? clamp(raw) : DEFAULT_DENSITY;
	} catch {
		return DEFAULT_DENSITY;
	}
}

class MediaDensity {
	#value = $state(load());

	get value(): number {
		return this.#value;
	}

	set value(n: number) {
		this.#value = clamp(n);
		if (typeof localStorage === 'undefined') return;
		try {
			localStorage.setItem(KEY, String(this.#value));
		} catch {
			// Storage full/unavailable — the preference just won't persist (non-fatal).
		}
	}
}

export const mediaDensity = new MediaDensity();

// The density slider's native range input increases left-to-right (standard/accessible), but
// the UI wants dragging right to mean "bigger cards" (fewer columns) — this map translates
// between the two, applied both when displaying the slider and when reading its input back.
//
// `max` is the *current viewport's* cap, not DENSITY_MAX: the slider spans only the columns
// this window can actually show, so no stop is inert. Inverting against a fixed DENSITY_MAX
// would flip the direction on every viewport below the widest tier — at a 3-column cap it
// would return 15, far outside the slider's own range. Self-inverse for any given `max`,
// which is what lets one call both render the slider and read it back.
export function invertDensity(n: number, max: number): number {
	return DENSITY_MIN + max - n;
}

// A shared singleton (one resize listener) rather than per-VideoGrid-instance state, since
// several grids can be mounted at once (the media list, an entity page's own grid, its
// "related" shelves).
class ViewportTierCap {
	#value = $state(typeof window === 'undefined' ? 1 : capForWidth(window.innerWidth));

	constructor() {
		if (typeof window === 'undefined') return;
		window.addEventListener('resize', () => {
			this.#value = capForWidth(window.innerWidth);
		});
	}

	get value(): number {
		return this.#value;
	}
}

export const viewportTierCap = new ViewportTierCap();

// Columns actually rendered here and now: the stored preference narrowed to what this viewport
// can hold. The distinction between "what was chosen" and "what fits" is the whole model, so it
// lives here rather than being re-derived by each grid. Reads two $state getters, so it stays
// reactive when called inside a $derived.
export function effectiveDensity(): number {
	return Math.min(mediaDensity.value, viewportTierCap.value);
}

// People posters run at twice the video grid's column count (RD8): a 2:3 poster reads fine at
// roughly half a 16:9 thumbnail's width. Kept next to the rungs it doubles so the ratio is
// tuned and tested alongside them, rather than as a bare `* 2` in a consumer.
export function posterColumns(): number {
	return effectiveDensity() * 2;
}
