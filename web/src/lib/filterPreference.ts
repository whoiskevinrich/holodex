// Per-page sticky filter preference (spec sort-persistence SP5, HOLODEX-25).
//
// Sibling of sortPreference's readSort/writeSort: each index page (media, people,
// studios, tags) remembers the filters it last showed under its own localStorage
// key, JSON-encoded, and re-validates on read so a stale or corrupt value falls back
// to the default instead of wedging a control. Same SSR-safe, never-throws posture.
//
// Owner-only filters (completeness sort, missing-facet) are persisted regardless of
// who is browsing — each page already strips them from the request for a non-owner,
// so a stale key in a non-owner browser is inert, and an owner whose capabilities
// load late self-heals into the restored filter once they resolve.

function keyFor(page: string): string {
	return `holodex:filters:${page}`;
}

// readFilters returns the saved value for a page when `validate` accepts the parsed
// JSON, else undefined. `validate` narrows unknown → T and is where a page rejects a
// shape it no longer understands (forward-safe when a filter is removed).
export function readFilters<T>(page: string, validate: (raw: unknown) => T | undefined): T | undefined {
	if (typeof localStorage === 'undefined') return undefined;
	try {
		const raw = localStorage.getItem(keyFor(page));
		if (raw == null) return undefined;
		return validate(JSON.parse(raw));
	} catch {
		// Malformed JSON or unavailable storage — fall through to the default.
		return undefined;
	}
}

export function writeFilters(page: string, value: unknown): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(keyFor(page), JSON.stringify(value));
	} catch {
		// Storage full/unavailable — the preference just won't persist (non-fatal).
	}
}

// The People/Studios pages share one filter shape: the owner-only completeness sort
// direction (CompletenessSortToggle) and the missing-facet selection (FacetFilter).
export type CompletenessDir = '' | 'asc' | 'desc';
export interface EntityFilters {
	completeness: CompletenessDir;
	missing_facet: string[];
}

const COMPLETENESS_DIRS: readonly CompletenessDir[] = ['', 'asc', 'desc'];

export function validateEntityFilters(raw: unknown): EntityFilters | undefined {
	if (typeof raw !== 'object' || raw === null) return undefined;
	const o = raw as Record<string, unknown>;
	const completeness = o.completeness;
	if (!COMPLETENESS_DIRS.includes(completeness as CompletenessDir)) return undefined;
	const missing = o.missing_facet;
	if (!Array.isArray(missing) || !missing.every((s) => typeof s === 'string')) return undefined;
	return { completeness: completeness as CompletenessDir, missing_facet: missing };
}

export function readEntityFilters(page: string): EntityFilters {
	return readFilters(page, validateEntityFilters) ?? { completeness: '', missing_facet: [] };
}

export function writeEntityFilters(page: string, value: EntityFilters): void {
	writeFilters(page, value);
}

// validateString accepts any string — the browse page stores its filter set as the
// same URL query string it already parses with paramsToFilters, so validation of the
// contents happens there, exactly as it would for a shared link.
export function validateString(raw: unknown): string | undefined {
	return typeof raw === 'string' ? raw : undefined;
}

// validateOneOf builds a validator for a single enumerated value (the Tags type filter).
export function validateOneOf<T extends string>(allowed: readonly T[]): (raw: unknown) => T | undefined {
	return (raw) => ((allowed as readonly unknown[]).includes(raw) ? (raw as T) : undefined);
}
