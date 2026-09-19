// The Enrich picker's RD1 auto-apply decision (F47, ADR-066), pulled out of
// EnrichPicker.svelte so the rule is unit-testable without a DOM (this repo has no
// component-test harness) — same split as candidateDetail.ts / candidateImage.ts.

/** The slice of a candidate the decision reads. */
export interface AutoApplyCandidate {
	auto_apply: boolean;
}

/**
 * The candidate the picker applies without showing the list, or undefined when the
 * owner must pick. Exactly one server-verdict `auto_apply` candidate qualifies (the
 * backend's SingleStrongMatch rule); weaker candidates alongside it don't block it,
 * while zero or two-plus strong ones always fall through to the list.
 *
 * `allowed` is the caller's say: true only for the initial, entity-seeded search of a
 * first match (or Refresh-all's needs-review hand-off). A search the owner typed, and
 * every ⋯ "Re-match…" open (HOLODEX-418 — the owner is overriding the current link,
 * so the previous match is suspect by definition), pass false and always see the list.
 */
export function autoApplyPick<C extends AutoApplyCandidate>(
	candidates: C[],
	allowed: boolean
): C | undefined {
	if (!allowed) return undefined;
	const strong = candidates.filter((c) => c.auto_apply);
	return strong.length === 1 ? strong[0] : undefined;
}
