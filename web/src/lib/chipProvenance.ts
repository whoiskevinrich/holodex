// Chip-row provenance (HOLODEX-144, docs/design/chip-row-provenance-handoff.md). Pure: which
// provenance mark a CurationFieldRow shows. A row whose values share one provenance gets one
// row badge, the same ProvenanceBadge a single value shows; a mixed row marks only its
// provider values, one 12px icon each. No I/O, no Svelte — unit-tested in chipProvenance.test.ts.
import { providerFromWinningSource } from './format';
import type { ResolvedValue } from './types';

// A value's provenance key. A provider is any source providerFromWinningSource doesn't treat
// as Holodex's own (file, record, manual, computed, tag), so the set lives in one place. `unknown` is a value with no sources at all — CurationFieldRow's
// fallback when a field carries `values` but no `items` — so nothing is claimed for it.
type Key = { kind: 'provider'; provider: string } | { kind: 'baseline' } | { kind: 'manual' } | { kind: 'unknown' };

export function provenanceKey(item: ResolvedValue): Key {
	if (item.manual) return { kind: 'manual' };
	const provider = item.sources.find((s) => providerFromWinningSource(s) !== '');
	if (provider) return { kind: 'provider', provider };
	if (item.sources.includes('manual')) return { kind: 'manual' };
	return item.sources.length ? { kind: 'baseline' } : { kind: 'unknown' };
}

function sameKey(a: Key, b: Key): boolean {
	if (a.kind !== b.kind) return false;
	return a.kind !== 'provider' || a.provider === (b as { provider: string }).provider;
}

export interface RowProvenance {
	// The row's one badge (ProvenanceBadge props), or null for a mixed row or one with nothing known.
	badge: { provider: string; manual: boolean } | null;
	// Per item, parallel to the input: the provider whose icon that chip carries, or ''.
	marks: string[];
}

export function rowProvenance(items: ResolvedValue[]): RowProvenance {
	const keys = items.map(provenanceKey);
	const first = keys[0];
	if (first && keys.every((k) => sameKey(k, first))) {
		const badge =
			first.kind === 'unknown'
				? null
				: { provider: first.kind === 'provider' ? first.provider : '', manual: first.kind === 'manual' };
		return { badge, marks: keys.map(() => '') };
	}
	return { badge: null, marks: keys.map((k) => (k.kind === 'provider' ? k.provider : '')) };
}
