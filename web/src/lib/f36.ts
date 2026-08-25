// F36 — Per-field source-of-truth decisions (ADR-051). Pure view-model helpers for the
// SourceSelect control: derive the segments/candidates from a ResolvedField's frozen
// `decision` / `candidates` / `in_sync` payload. No I/O, no Svelte — unit-tested in isolation
// (f36.test.ts), reused by SourceSelect.svelte and the media detail page.
//
// F37 generalizes the baseline: the anchored first chip's source key is the entity's
// `baselineKey` — 'file' for videos (the default, so every F36 call site is untouched),
// 'record' for persons (RD4: `·file` is factually wrong for a person). Only the key is
// parameterized; the anchor/fold/select behavior is identical across entities.
import type { DecisionSource, FieldCandidate, ResolvedField, ResolvedValue } from './types';

const PROVIDER_PREFIX = 'provider:';

// providerOf extracts the provider name from a `provider:<name>` source key; '' for the
// baseline (file/record) and manual.
export function providerOf(source: DecisionSource | string): string {
	return source.startsWith(PROVIDER_PREFIX) ? source.slice(PROVIDER_PREFIX.length) : '';
}

// isProviderSource is the source-kind guard used to split baseline/manual from a provider pick.
export function isProviderSource(source: DecisionSource | string): boolean {
	return source.startsWith(PROVIDER_PREFIX);
}

// A replace (scalar) field carries a source decision; merge (multi/set) fields do not — they
// keep F30 per-value curation, so the SourceSelect control is replace-only (RD1).
export function isReplaceField(field: ResolvedField): boolean {
	return !field.multi;
}

// decidedSource is the field's currently-pinned source. An absent decision is the implicit
// baseline default (file-first, RD4), so an undecided field reports the baseline key.
export function decidedSource(field: ResolvedField, baselineKey = 'file'): DecisionSource {
	return field.decision?.source ?? (baselineKey as DecisionSource);
}

// baselineCandidateValue is the entity's baseline value (the anchored first chip's value), or
// '' when the baseline has no value for the field.
export function baselineCandidateValue(field: ResolvedField, baselineKey = 'file'): string {
	return (field.candidates ?? []).find((c) => c.source === baselineKey)?.value ?? '';
}

// fileCandidateValue is the video-entity spelling of baselineCandidateValue, kept so F36 call
// sites and tests read unchanged.
export function fileCandidateValue(field: ResolvedField): string {
	return baselineCandidateValue(field, 'file');
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
// that supplies it. `decisionSource` is what selecting the chip pins; `sources` is what
// resolveSelection matches a standing decision's raw source key against, so it must stay
// namespace-only (e.g. "film:42", not the film's own name) — `labels` is the parallel,
// display-friendly array (e.g. "Dune") that feeds CurationChip's `·provenance` suffix and
// SourceBadge's collapsed-state label instead.
export interface SourceChip {
	key: string; // stable DOM/selection key: baselineKey | 'provider:<name>' | 'custom'
	value: string; // display value ('' on the baseline chip ⇒ UI placeholder; '' on custom ⇒ opener)
	sources: string[]; // provenance namespaces, e.g. ['file'], ['record'], ['tmdb'], ['file','tmdb']
	labels: string[]; // display names parallel to `sources` — equal to `sources` except for a film source, whose namespace ("film:42") differs from its display name ("Dune")
	decisionSource: DecisionSource; // the decision selecting this chip pins
	manual?: boolean; // the trailing Custom chip
}

// CurationChip's provenance suffix wants display labels ("Dune"), not the raw namespaces
// `sources` carries for decision-matching ("film:42") — see the SourceChip doc comment above.
export function chipToResolvedValue(chip: SourceChip): ResolvedValue {
	return { value: chip.value, sources: chip.labels, manual: chip.manual };
}

// sourceChips builds the one-row chip model for a replace field. The entity baseline is always
// the first, anchored chip (tagged `·file` / `·record`) — the baseline-first mental model is the
// whole point of F36, so the baseline never becomes just another value in the row. A provider
// whose value equals the baseline value folds into that baseline chip (`·file + tmdb`,
// `·record + tmdb`) rather than repeating the value; providers that share a distinct value fold
// together too. Selecting a folded baseline chip still pins the baseline.
// The Custom chip is always last: the frozen manual literal when decided, else the inline-input opener.
export function sourceChips(field: ResolvedField, baselineKey = 'file'): SourceChip[] {
	const baseVal = baselineCandidateValue(field, baselineKey);
	const base: SourceChip = {
		key: baselineKey,
		value: baseVal,
		sources: [baselineKey],
		labels: [baselineKey],
		decisionSource: baselineKey as DecisionSource
	};
	const chips: SourceChip[] = [base];

	for (const c of providerCandidates(field)) {
		const ns = providerOf(c.source);
		const label = c.provider || ns;
		const v = c.value.trim();
		if (baseVal.trim() !== '' && v === baseVal.trim()) {
			if (!base.sources.includes(ns)) {
				base.sources.push(ns);
				base.labels.push(label);
			}
			continue;
		}
		const twin = chips.find((ch) => ch.decisionSource !== baselineKey && ch.value.trim() === v);
		if (twin) {
			if (!twin.sources.includes(ns)) {
				twin.sources.push(ns);
				twin.labels.push(label);
			}
		} else {
			chips.push({
				key: c.source,
				value: c.value,
				sources: [ns],
				labels: [label],
				decisionSource: c.source as DecisionSource
			});
		}
	}

	const manual = field.decision?.source === 'manual';
	chips.push({
		key: 'custom',
		value: manual ? (field.decision?.manual_value ?? '') : '',
		sources: ['manual'],
		labels: ['manual'],
		decisionSource: 'manual',
		manual: true
	});
	return chips;
}

// standing is true only for a real, owner-made decision. The backend always sends a populated
// `decision` object for a replace field (replaceMarkers() never returns a nil marker) — even
// undecided fields carry one, reporting the resolver's implicit winner with `standing: false`.
// So `field.decision` truthiness is NOT a real-vs-implicit signal against the live API; only
// `.standing` is. Tests that omit `decision` entirely to mean "undecided" still read as
// non-standing here (`undefined?.standing === true` is false), so both representations agree.
function standing(field: ResolvedField): boolean {
	return field.decision?.standing === true;
}

// resolveSelection maps the field's decided source onto the chip that should read selected, and
// whether that selection is a real decision or the RD6 implicit-winner fallback (HOLODEX-245) —
// one walk of the branch, shared by selectedChipKey/isPendingSelection/SourceSelect so nothing
// re-derives when RD6 fires from the outside. A provider decision resolves to whichever chip
// carries that provider (standalone or folded into the baseline chip); an unmatched decision
// falls back to the baseline chip (harmless — the value is gone). Undecided fields default to
// the baseline chip when the baseline has a value (file-first, RD4) — but with an EMPTY baseline
// the resolver's winner is a provider value, so the provider chip reads selected, `pending: true`
// (F37 RD6: display identical to the raw enrichment list, marked as not a standing decision). An
// explicit baseline decision (the blank-pin, RD3) still selects the `—` baseline chip.
export function resolveSelection(
	field: ResolvedField,
	chips: SourceChip[],
	baselineKey = 'file'
): { key: string; pending: boolean } {
	const src = decidedSource(field, baselineKey);
	if (src === 'manual') return { key: 'custom', pending: false };
	if (src === baselineKey) {
		// decidedSource only reports baselineKey here when nothing overrode it — either a real
		// baseline pin (standing) or the resolver's own default when the baseline has a value.
		// The RD6 case (empty baseline, provider wins) never reaches this branch against the
		// real API, since decidedSource already returns the provider source directly then — but
		// test fixtures that omit `decision` to mean "undecided" fall back to baselineKey via
		// decidedSource's `??`, so this still needs to catch that shape too.
		if (!standing(field) && baselineCandidateValue(field, baselineKey).trim() === '') {
			const resolved = (field.values?.[0] ?? '').trim();
			const winner =
				chips.find(
					(c) => !c.manual && c.key !== baselineKey && resolved !== '' && c.value.trim() === resolved
				) ?? chips.find((c) => !c.manual && c.key !== baselineKey);
			if (winner) return { key: winner.key, pending: true };
		}
		return { key: baselineKey, pending: false };
	}
	const name = providerOf(src);
	const key = chips.find((c) => c.sources.includes(name))?.key ?? baselineKey;
	// The resolved key is a non-baseline (provider/folded) chip with no standing decision behind
	// it — the resolver's own precedence order picked it, most commonly RD6's empty-baseline case.
	return { key, pending: key !== baselineKey && !standing(field) };
}

// selectedChipKey / isPendingSelection are the two projections of resolveSelection's result,
// kept as separate exports for callers that only need one (f36.test.ts pins both independently).
export function selectedChipKey(field: ResolvedField, chips: SourceChip[], baselineKey = 'file'): string {
	return resolveSelection(field, chips, baselineKey).key;
}

export function isPendingSelection(field: ResolvedField, chips: SourceChip[], baselineKey = 'file'): boolean {
	return resolveSelection(field, chips, baselineKey).pending;
}

// outOfSync is true when the field's decided value differs from the value embedded in the file
// (`in_sync === false`). This is the single per-field `text-warn` signal (RD2). An undecided
// (file-default) field is in sync by construction, so only a decision can read out of sync.
// Person fields never carry `in_sync` (F37 — a person has no file), so this is never true there.
export function outOfSync(field: ResolvedField): boolean {
	return field.in_sync === false;
}

// isWritable is true when the field has a destination file tag for the video's
// current container (HOLODEX-216) — the predicate behind the writeback dialog's
// row treatment: an unwritable field is shown (so the operator can see it exists
// and why it's excluded) but can never be checked, so it can no longer be
// offered and then silently dropped on write. A field the backend never stamped
// `write_target` on (older payload shape, or a non-video entity) reads as
// unwritable rather than writable-by-default — this predicate only turns TRUE on
// an explicit target name.
export function isWritable(field: ResolvedField): boolean {
	return !!field.write_target;
}

// needsWriteback is the single predicate behind BOTH writeback surfaces: the header's "{n} out
// of sync" count and the batch dialog's initial checked state (HOLODEX-213). Deriving them from
// one function is what makes them unable to disagree — a dialog opened on N reported out-of-sync
// fields pre-checks exactly those N. Replace-only by contract (RD1) — merge fields never carry a
// decision, so the filter makes that explicit rather than relying on the backend never setting
// `in_sync` on them. A provider value that merely wins by mapping precedence is UNDECIDED and so
// in sync by construction (see outOfSync): it stays listed and checkable in the dialog, just not
// pre-checked, because the button writes *decisions*. Also requires isWritable — a decided value
// with no file-tag mapping for the container must never auto-check into a write that can only
// silently drop it (HOLODEX-216); it still lists, just unchecked and disabled, like any other
// unwritable row.
export function needsWriteback(field: ResolvedField): boolean {
	return isReplaceField(field) && outOfSync(field) && isWritable(field);
}

// outOfSyncCount is the aggregate the header surfaces as "Write decisions to file · {n} out of
// sync" (RD2).
export function outOfSyncCount(fields: ResolvedField[]): number {
	return fields.filter(needsWriteback).length;
}
