// F36 dev mock (ADR-051) — lets the SourceSelect control be built and exercised before the
// S1 backend lands (rollout plan: S2 runs parallel to S1 against the *frozen* typed payload).
// It synthesizes the `decision` / `candidates` / `in_sync` fields the resolver will add, and
// keeps decisions in an in-memory store so set/clear round-trips through a refetch.
//
// PRODUCTION-SAFE: every call site is guarded by `f36MockEnabled` (= import.meta.env.DEV), a
// compile-time-false constant in a production build — Rollup inlines it, the guarded branches
// die, and this whole module is tree-shaken out. No mock code ships.
import type {
	DecisionSource,
	FieldCandidate,
	MediaDetailResponse,
	ResolvedField
} from './types';
import { isProviderSource, providerOf } from './f36';

// Active only in dev. The real server payload (S1) carries `decision`/`candidates`, so once
// the backend lands this mock can be deleted and the page reads the live fields directly.
export const f36MockEnabled = import.meta.env.DEV;

interface StoredDecision {
	source: DecisionSource;
	manual_value?: string;
}

// In-memory decisions, keyed `${videoId}:${canonical}`. Survives refetches within a session
// (module-level) so a set/clear is visible after the page reloads the detail — mirroring how
// the real DB-backed decision persists. Absence ⇒ the file default (undecided).
const store = new Map<string, StoredDecision>();
const keyOf = (videoId: number, canonical: string) => `${videoId}:${canonical}`;

export function mockSetDecision(
	videoId: number,
	canonical: string,
	source: DecisionSource,
	manualValue?: string
): void {
	store.set(keyOf(videoId, canonical), { source, manual_value: manualValue });
}

export function mockClearDecision(videoId: number, canonical: string): void {
	store.delete(keyOf(videoId, canonical)); // back to the file default
}

// A replace field eligible for the source control: scalar (not a merge set) and a plain text
// field (the display specials — image_url / long_text / url — are not SourceSelect surfaces).
function isSourceField(f: ResolvedField): boolean {
	return !f.multi && !f.display;
}

// The file baseline value for a field. Prefer an explicit file-sourced item; fall back to the
// filename-derived video title for `title`, else treat the current resolved value as the file
// value so Keep-file always has something to pin.
function fileValueFor(f: ResolvedField, videoTitle: string): string {
	const fileItem = f.items?.find((it) => it.sources.includes('file'));
	if (fileItem) return fileItem.value;
	if (f.canonical === 'title' && videoTitle) return videoTitle;
	return f.values[0] ?? '';
}

// Provider candidate values for a field, drawn from the enriched per-provider payload.
function providerCandidatesFor(
	f: ResolvedField,
	enriched: MediaDetailResponse['enriched']
): FieldCandidate[] {
	return (enriched ?? [])
		.filter((e) => e.canonical === f.canonical && (e.values[0] ?? '').trim() !== '')
		.map((e) => ({
			source: `provider:${e.provider}` as DecisionSource,
			provider: e.provider,
			value: e.values[0]
		}));
}

// Augment a media detail's resolved fields with the F36 payload the resolver will eventually
// supply: candidates (file + providers), the standing decision from the store (default = file,
// file-first), the decided display value, and the in-sync flag (decided vs. the file's tag —
// modeled here as the file baseline value). Returns a fresh array; the input is not mutated.
export function applyMockDecisions(detail: MediaDetailResponse, videoId: number): ResolvedField[] {
	const resolved = detail.resolved ?? [];
	const videoTitle = detail.video?.title ?? '';

	return resolved.map((f) => {
		if (!isSourceField(f)) return f;

		const fileVal = fileValueFor(f, videoTitle);
		const provCands = providerCandidatesFor(f, detail.enriched);
		const candidates: FieldCandidate[] = [
			...(fileVal ? [{ source: 'file' as DecisionSource, value: fileVal }] : []),
			...provCands
		];

		const stored = store.get(keyOf(videoId, f.canonical));
		const source: DecisionSource = stored?.source ?? 'file';

		// The decided display value follows the pinned source's *current* value (source-pin,
		// not value-pin) — except manual, which is a frozen literal.
		let value = fileVal;
		if (source === 'manual') value = stored?.manual_value ?? '';
		else if (isProviderSource(source)) {
			value = provCands.find((c) => c.source === source)?.value ?? fileVal;
		}

		// In sync ⇔ the decided value equals the file's embedded tag (the file baseline here).
		const inSync = value.trim() === fileVal.trim();

		return {
			...f,
			values: [value],
			winning_source:
				source === 'manual'
					? `manual:${f.canonical}`
					: source === 'file'
						? `file:${f.canonical}`
						: `${providerOf(source)}:${f.canonical}`,
			decision: {
				source,
				standing: stored !== undefined,
				...(source === 'manual' ? { manual_value: stored?.manual_value ?? '' } : {})
			},
			in_sync: inSync,
			candidates
		} satisfies ResolvedField;
	});
}
