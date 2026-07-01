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

// SourceChip is one selectable value chip in the unified source-of-truth control (HOLODEX-112).
// It collapses the old resolved-chip + segmented-control + candidates-line into a single row of
// source-tagged value chips: one chip per *distinct* candidate value, tagged with every source
// that supplies it. `decisionSource` is what selecting the chip pins; `sources` feeds
// CurationChip's `·provenance` suffix (and its provider-vs-baseline colouring).
export interface SourceChip {
	key: string; // stable DOM/selection key: 'file' | 'provider:<name>' | 'custom'
	value: string; // display value ('' on the file chip ⇒ UI placeholder; '' on custom ⇒ opener)
	sources: string[]; // provenance namespaces, e.g. ['file'], ['tmdb'], ['file','tmdb']
	decisionSource: DecisionSource; // the decision selecting this chip pins
	manual?: boolean; // the trailing Custom chip
}

// sourceChips builds the one-row chip model for a replace field. The file baseline is always the
// first, anchored chip (tagged `·file`) — the file-first mental model is the whole point of F36,
// so the baseline never becomes just another value in the row. A provider whose value equals the
// file value folds into that file chip (`·file + tmdb`) rather than repeating the value; providers
// that share a distinct value fold together too. Selecting a folded file chip still pins `file`.
// The Custom chip is always last: the frozen manual literal when decided, else the inline-input opener.
export function sourceChips(field: ResolvedField): SourceChip[] {
	const fileVal = fileCandidateValue(field);
	const file: SourceChip = { key: 'file', value: fileVal, sources: ['file'], decisionSource: 'file' };
	const chips: SourceChip[] = [file];

	for (const c of providerCandidates(field)) {
		const name = c.provider || providerOf(c.source);
		const v = c.value.trim();
		if (fileVal.trim() !== '' && v === fileVal.trim()) {
			if (!file.sources.includes(name)) file.sources.push(name);
			continue;
		}
		const twin = chips.find((ch) => ch.decisionSource !== 'file' && ch.value.trim() === v);
		if (twin) {
			if (!twin.sources.includes(name)) twin.sources.push(name);
		} else {
			chips.push({
				key: c.source,
				value: c.value,
				sources: [name],
				decisionSource: c.source as DecisionSource
			});
		}
	}

	const manual = field.decision?.source === 'manual';
	chips.push({
		key: 'custom',
		value: manual ? (field.decision?.manual_value ?? '') : '',
		sources: ['manual'],
		decisionSource: 'manual',
		manual: true
	});
	return chips;
}

// selectedChipKey maps the field's decided source onto the chip that should read selected. A
// provider decision resolves to whichever chip carries that provider (standalone or folded into
// the file chip); an unmatched decision falls back to the file chip (harmless — the value is gone).
export function selectedChipKey(field: ResolvedField, chips: SourceChip[]): string {
	const src = decidedSource(field);
	if (src === 'file') return 'file';
	if (src === 'manual') return 'custom';
	const name = providerOf(src);
	return chips.find((c) => c.sources.includes(name))?.key ?? 'file';
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
