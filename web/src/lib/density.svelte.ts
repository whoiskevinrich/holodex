// Shared, persisted grid-density preference: how many columns a VideoGrid targets
// at its widest viewport tier. One site-wide value (not per-page, unlike sort/SP1)
// so density stays consistent as you navigate between the media list, entity pages,
// and related shelves.

const KEY = 'holodex:media-density';
export const DENSITY_MIN = 2;
export const DENSITY_MAX = 6;
const DEFAULT_DENSITY = 4;

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

// The density slider's native range input increases left-to-right (standard/accessible),
// but the UI wants dragging right to mean "bigger cards" (fewer columns) — this self-inverse
// map translates between the two, applied both when displaying the slider and when reading
// its input back into the stored preference.
export function invertDensity(n: number): number {
	return DENSITY_MIN + DENSITY_MAX - n;
}

// Column ceiling per viewport tier, mirroring the breakpoints VideoGrid used to express as
// Tailwind utility classes (lg/xl/2xl + a custom 480px step). A shared singleton (one resize
// listener) rather than per-VideoGrid-instance state, since several grids can be mounted at
// once (the media list, an entity page's own grid, its "related" shelves).
const TIERS: { min: number; cap: number }[] = [
	{ min: 1536, cap: 6 },
	{ min: 1280, cap: 4 },
	{ min: 1024, cap: 3 },
	{ min: 480, cap: 2 }
];

function capForWidth(width: number): number {
	return TIERS.find((t) => width >= t.min)?.cap ?? 1;
}

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
