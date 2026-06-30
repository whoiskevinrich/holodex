// F36 — Per-field source-of-truth decisions (ADR-051). Pure view-model helpers for the
// SourceSelect control: derive the segments/candidates from a ResolvedField's frozen
// `decision` / `candidates` / `in_sync` payload. No I/O, no Svelte — unit-tested in isolation
// (f36.test.ts), reused by SourceSelect.svelte and the media detail page.
import type { DecisionSource, FieldCandidate, ResolvedField } from './types';

const PROVIDER_PREFIX = 'provider:';

// providerOf extracts the provider name from a `provider:<name>` source key; '' for file/manual.
export function providerOf(source: DecisionSource | string): string {
	return source.startsWith(PROVIDER_PREFIX) ? source.slice(PROVIDER_PREFIX.length) : '';
}

// isProviderSource is the source-kind guard used to split file/manual from a provider pick.
export function isProviderSource(source: DecisionSource | string): boolean {
	return source.startsWith(PROVIDER_PREFIX);
}

// A replace (scalar) field carries a source decision; merge (multi/set) fields do not — they
// keep F30 per-value curation, so the SourceSelect control is replace-only (RD1).
export function isReplaceField(field: ResolvedField): boolean {
	return !field.multi;
}

// decidedSource is the field's currently-pinned source. An absent decision is the implicit
// file default (file-first, RD4), so an undecided field reports 'file'.
export function decidedSource(field: ResolvedField): DecisionSource {
	return field.decision?.source ?? 'file';
}

// fileCandidateValue is the file baseline value (the `Keep file` segment's value), or '' when
// the file has no value for the field.
export function fileCandidateValue(field: ResolvedField): string {
	return (field.candidates ?? []).find((c) => c.source === 'file')?.value ?? '';
}

// providerCandidates are the matched providers that actually supply a value — an empty provider
// value yields no `Adopt` segment (handoff edge case), so it is filtered out here.
export function providerCandidates(field: ResolvedField): FieldCandidate[] {
	return (field.candidates ?? []).filter(
		(c) => isProviderSource(c.source) && c.value.trim() !== ''
	);
}

// providersDiffer is true when ≥2 provider candidates disagree on the value — the trigger for
// the muted "providers differ" hint on the candidates line. Informational only, never warn
// (Open-Q3): disagreement is not an error.
export function providersDiffer(field: ResolvedField): boolean {
	const values = new Set(providerCandidates(field).map((c) => c.value.trim()));
	return values.size >= 2;
}

// outOfSync is true when the field's decided value differs from the value embedded in the file
// (`in_sync === false`). This is the single per-field `text-warn` signal (RD2). An undecided
// (file-default) field is in sync by construction, so only a decision can read out of sync.
export function outOfSync(field: ResolvedField): boolean {
	return field.in_sync === false;
}

// outOfSyncCount is the aggregate the header surfaces as "Write decisions to file · {n} out of
// sync" (RD2). Replace-only by contract (RD1) — merge fields never carry a decision, so the
// filter makes that explicit rather than relying on the backend never setting `in_sync` on them.
export function outOfSyncCount(fields: ResolvedField[]): number {
	return fields.filter((f) => isReplaceField(f) && outOfSync(f)).length;
}
