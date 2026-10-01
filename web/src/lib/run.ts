// A run (F75, ADR-121 D7): Play all / Shuffle over a query-backed grid or a playlist,
// walked by the next-up strip on /media/[id]. All state is client-held and never written
// to the server. Everything here is pure except `newSeed`/`newRunId` (randomness) and the
// storage helpers, which take the Storage to use.
//
// The model is one array. `order` is the play order; the items before the current one
// are the history, played in exactly that order. A shuffle toggle or a repeat only
// rewrites what comes after the current item, so the current item never moves and
// Previous always walks back through what actually played (spec RD11).

export type RunMode = 'in-order' | 'shuffled';

/** What a run plays: a /media query (canonical string, no sort/paging) or a playlist. */
export type RunSource =
	| { kind: 'query'; query: string; sort?: string; seed?: number; label: RunLabel }
	| { kind: 'playlist'; id: number; seed?: number; label: RunLabel };

/** How the strip names the source: "Person · Ana" linking to the person page. */
export interface RunLabel {
	/** "Person", "Tag", "Studio", "Film", "Browse", or "" for a playlist (its name alone). */
	kind: string;
	name: string;
	href: string;
}

export interface Run {
	id: string;
	source: RunSource;
	mode: RunMode;
	/** The seed of the current shuffle; meaningless in order. */
	seed: number;
	/** The source order, as fetched at the press (or at the start of the current pass). */
	ids: number[];
	/** The play order. order[0..current] is the history. */
	order: number[];
	repeat: boolean;
}

// mulberry32: a small, well-mixed 32-bit PRNG. Seeded, so one run's order is stable
// across reloads (the seed is stored with the run).
export function mulberry32(seed: number): () => number {
	let a = seed >>> 0;
	return () => {
		a = (a + 0x6d2b79f5) >>> 0;
		let t = a;
		t = Math.imul(t ^ (t >>> 15), t | 1);
		t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
}

/** A seeded Fisher–Yates permutation of `ids`. Never mutates its input. */
export function shuffle(ids: readonly number[], seed: number): number[] {
	const out = ids.slice();
	const rand = mulberry32(seed);
	for (let i = out.length - 1; i > 0; i--) {
		const j = Math.floor(rand() * (i + 1));
		[out[i], out[j]] = [out[j], out[i]];
	}
	return out;
}

export function newSeed(): number {
	return Math.floor(Math.random() * 4294967296);
}

export function newRunId(): string {
	return Math.random().toString(36).slice(2, 10);
}

/** A fresh run over `ids`, in the source order or shuffled under `seed`. */
export function startRun(opts: {
	id: string;
	source: RunSource;
	ids: number[];
	mode: RunMode;
	seed: number;
	repeat?: boolean;
}): Run {
	return {
		id: opts.id,
		source: opts.source,
		mode: opts.mode,
		seed: opts.seed,
		ids: opts.ids.slice(),
		order: opts.mode === 'shuffled' ? shuffle(opts.ids, opts.seed) : opts.ids.slice(),
		repeat: opts.repeat ?? false
	};
}

export interface Neighbours {
	/** 0-based index of the current video in the play order, or -1 when it is not in it. */
	index: number;
	prev: number | null;
	next: number | null;
}

export function neighbours(order: readonly number[], videoId: number): Neighbours {
	const index = order.indexOf(videoId);
	if (index === -1) return { index, prev: null, next: null };
	return {
		index,
		prev: index > 0 ? order[index - 1] : null,
		next: index < order.length - 1 ? order[index + 1] : null
	};
}

/**
 * Switch shuffle on or off mid-run (spec RD11). The current item and everything before
 * it stay put. On: the rest is permuted under `seed`. Off: the rest goes back to its
 * source order. A current item outside the run (a stale link) reorders the whole list.
 */
export function setMode(run: Run, currentId: number, mode: RunMode, seed: number): Run {
	const cur = run.order.indexOf(currentId);
	const head = run.order.slice(0, cur + 1);
	const rest = run.order.slice(cur + 1);
	const rank = new Map(run.ids.map((id, i) => [id, i]));
	const tail =
		mode === 'shuffled'
			? shuffle(rest, seed)
			: rest.slice().sort((a, b) => (rank.get(a) ?? 0) - (rank.get(b) ?? 0));
	return { ...run, mode, seed: mode === 'shuffled' ? seed : run.seed, order: [...head, ...tail] };
}

/**
 * The next pass of a repeating run (spec RD12): over `freshIds` (the source re-read, so
 * new videos join), reshuffled under `seed` when shuffled, and never starting with
 * `justEnded`. A one-item run can't avoid it and replays that item.
 */
export function nextPass(run: Run, freshIds: number[], justEnded: number, seed: number): Run {
	const order = run.mode === 'shuffled' ? shuffle(freshIds, seed) : freshIds.slice();
	if (order.length > 1 && order[0] === justEnded) [order[0], order[1]] = [order[1], order[0]];
	return { ...run, ids: freshIds.slice(), seed: run.mode === 'shuffled' ? seed : run.seed, order };
}

// --- URL ---------------------------------------------------------------------------

/** What a /media/[id] URL says about the run it belongs to. */
export interface RunParam {
	/** The run's storage key; absent on an F69 `?playlist=` link that predates runs. */
	run?: string;
	/** A query source: the canonical /media query string. */
	from?: string;
	sort?: string;
	/** A playlist source (F69's `?playlist=`). */
	playlist?: number;
	seed?: number;
}

export function parseRunParam(url: URL): RunParam | null {
	const sp = url.searchParams;
	const out: RunParam = {};
	const run = sp.get('run');
	if (run && /^[a-z0-9]{1,16}$/.test(run)) out.run = run;
	const playlist = Number(sp.get('playlist'));
	if (Number.isInteger(playlist) && playlist > 0) out.playlist = playlist;
	else if (sp.has('from')) out.from = sp.get('from') ?? '';
	const sort = sp.get('sort');
	if (sort && out.from != null) out.sort = sort;
	const rawSeed = sp.get('seed');
	const seed = rawSeed == null ? NaN : Number(rawSeed);
	if (Number.isInteger(seed)) out.seed = seed;
	return out.playlist != null || out.from != null ? out : null;
}

/** `/media/{videoId}?run=…&from=…` (or `&playlist=…`): the link that keeps the run. */
export function runHref(videoId: number, run: Run): string {
	const p = new URLSearchParams({ run: run.id });
	const s = run.source;
	if (s.kind === 'playlist') p.set('playlist', String(s.id));
	else {
		p.set('from', s.query);
		if (s.sort) p.set('sort', s.sort);
	}
	if (s.seed != null) p.set('seed', String(s.seed));
	return `/media/${videoId}?${p}`;
}

/** True when `param` names the same source as `source` (a hop or reload within a run). */
export function sameSource(source: RunSource, param: RunParam): boolean {
	return source.kind === 'playlist' ? source.id === param.playlist : param.from != null && source.query === param.from;
}

const ENTITY_KEYS: Record<string, [kind: string, path: string]> = {
	person: ['Person', '/people/'],
	tag: ['Tag', '/tags/'],
	studio_id: ['Studio', '/studios/']
};

/**
 * How the strip names a query source. A single entity facet (plus an optional title
 * filter) is that entity's page; anything else is browse. `name` is the entity's
 * display name when the caller knows it; a run rebuilt from a URL doesn't, and the strip
 * links the kind alone.
 */
export function queryLabel(query: string, name = ''): RunLabel {
	const p = new URLSearchParams(query);
	const keys = [...new Set(p.keys())].filter((k) => k !== 'q');
	if (keys.length === 1 && ENTITY_KEYS[keys[0]] && p.getAll(keys[0]).length === 1) {
		const [kind, path] = ENTITY_KEYS[keys[0]];
		return { kind, name, href: path + encodeURIComponent(p.get(keys[0]) ?? '') };
	}
	return { kind: 'Browse', name, href: query ? `/?${query}` : '/' };
}

// --- Storage -----------------------------------------------------------------------

const KEY = 'holodex:run:';
const INDEX = 'holodex:runs';
// A tab keeps its most recent runs only: each one holds two id lists, and a long session
// of Play all presses would otherwise fill sessionStorage and make every save fail.
const KEEP = 5;

/** The stored run, or null when it's missing, unreadable or storage is unavailable. */
export function loadRun(storage: Storage | undefined, id: string): Run | null {
	try {
		const raw = storage?.getItem(KEY + id);
		if (!raw) return null;
		const r = JSON.parse(raw) as Run;
		return r && r.id === id && Array.isArray(r.order) && Array.isArray(r.ids) ? r : null;
	} catch {
		return null;
	}
}

export function saveRun(storage: Storage | undefined, run: Run): void {
	if (!storage) return;
	try {
		let index: string[] = [];
		try {
			const parsed = JSON.parse(storage.getItem(INDEX) ?? '[]');
			if (Array.isArray(parsed)) index = parsed.filter((x) => typeof x === 'string');
		} catch {
			// A corrupt index just starts over.
		}
		index = [...index.filter((id) => id !== run.id), run.id];
		for (const old of index.slice(0, -KEEP)) storage.removeItem(KEY + old);
		storage.setItem(INDEX, JSON.stringify(index.slice(-KEEP)));
		storage.setItem(KEY + run.id, JSON.stringify(run));
	} catch {
		// Storage full or blocked: the run lives on in memory for this page view, and a
		// reload rebuilds it from the URL.
	}
}
