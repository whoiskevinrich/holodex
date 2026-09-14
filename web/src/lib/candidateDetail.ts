// Per-candidate `detail` reveal state for the Enrich picker (F61, HOLODEX-380).
// Pure: EnrichPicker.svelte hands it the last response's candidates and seeds its
// open/closed map from what comes back. Kept out of the component, like
// searchedCaption.ts, so the label-collision rule in the design handoff is
// unit-testable without a DOM (this repo has no component-test harness).

/** The slice of a candidate the collision rule reads. */
export interface DetailCandidate {
	external_id: string;
	label: string;
	detail?: string[];
}

/**
 * The comparison key for "same label": case-folded, internal whitespace collapsed,
 * trimmed — so `harbor  lights` and `Harbor Lights` collide, as the handoff's
 * "Label-collision rule" specifies.
 */
export function normalizeLabel(label: string): string {
	return label.trim().replace(/\s+/g, ' ').toLocaleLowerCase();
}

/**
 * The initial open/closed map for a response, keyed by `external_id`: every
 * candidate that carries `detail` AND shares its normalized label with at least one
 * other candidate starts open; everything else starts closed (absent from the map).
 * A candidate without `detail` never appears — it has no toggle to be open.
 */
export function collisionOpen(candidates: readonly DetailCandidate[]): Record<string, boolean> {
	const count = new Map<string, number>();
	for (const c of candidates) {
		const k = normalizeLabel(c.label);
		count.set(k, (count.get(k) ?? 0) + 1);
	}
	const open: Record<string, boolean> = {};
	for (const c of candidates) {
		if (hasDetail(c) && (count.get(normalizeLabel(c.label)) ?? 0) >= 2) open[c.external_id] = true;
	}
	return open;
}

/** Whether a candidate renders a toggle at all — `[]` and a missing key both mean no. */
export function hasDetail(c: DetailCandidate): boolean {
	return (c.detail?.length ?? 0) > 0;
}

/** Toggle copy: `details` closed, `hide details` open (design handoff, Content spec). */
export function detailLabel(open: boolean): string {
	return open ? 'hide details' : 'details';
}
