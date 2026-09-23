// The Duplicates queue's own rules (F43 S5, ADR-061; F70, HOLODEX-451), held in one
// place because three surfaces need them and two of them cannot be unit-tested where
// they live. The page, the row and the compare panel all derive the same per-pair id
// strings; re-deriving them at each site is how the disclosure and the panel drift
// apart and `aria-controls` starts pointing at nothing.
//
// The rules the panel turned on — which entity kinds get one, where the match-kind
// label renders, and where focus lands when a row is removed — are the ones the
// 2026-09-23 QA pass found by hand. They live here so `queue.test.ts` can pin them
// without a component harness (there is none; see docs/testing-strategy.md §5).
import type { DuplicatePair, EntityKind } from '$lib/types';

/** One stable key per pair. The `{#each}` key, the panel's id and the disclosure's id
 *  all derive from it, so focus can be moved by id without threading refs up the tree. */
export function pairKey(p: DuplicatePair): string {
	return `${p.entity_type}-${p.a.id}-${p.b.id}`;
}

export const disclosureId = (p: DuplicatePair): string => `dup-disclosure-${pairKey(p)}`;
export const panelId = (p: DuplicatePair): string => `dup-panel-${pairKey(p)}`;
export const groupId = (type: EntityKind): string => `dup-group-${type}`;
/** The queue container — the one landing spot that outlives every group (P0-6). */
export const QUEUE_ID = 'dup-queue';

/** The compare panel is person-only (RD10): a studio/tag/film pair has no evidence
 *  worth two columns, and a control that cannot change what you see is one the reader
 *  learns to distrust. */
export function showsComparePanel(entityType: EntityKind): boolean {
	return entityType === 'person';
}

// Why the DETECTOR fired. The two canonical names can look nothing alike when the
// match came through an alias, which read as unexplained/wrong before this existed.
// 'canonical' needs no label — that is the two names visibly matching.
const MATCH_KIND_LABEL: Record<string, string> = {
	mixed: 'via alias',
	alias: 'alias match only — weak signal'
};

/** `alias` is the weak signal and is styled `text-warn`; `mixed` is merely
 *  explanatory and is `text-muted`. */
export function matchKindLabel(matchKind: string): { text: string; weak: boolean } {
	return { text: MATCH_KIND_LABEL[matchKind] ?? '', weak: matchKind === 'alias' };
}

/** Where the match-kind label renders (OQ3, resolved 2026-09-23). A PERSON pair
 *  carries it in the panel — measured at real type sizes it needs ~836 px in the row,
 *  and 100 % of the live queue carries one, so leaving it there made the two-line row
 *  the default on a surface built for scanning. Every other kind keeps it in the row:
 *  it has no panel to move it to, and without it two unalike names are paired with
 *  nothing explaining why. Gate on entity type, never on "the panel exists". */
export function labelPlacement(pair: DuplicatePair): 'row' | 'panel' | 'none' {
	if (!matchKindLabel(pair.match_kind).text) return 'none';
	return showsComparePanel(pair.entity_type) ? 'panel' : 'row';
}

/** Where focus may land, in order, when `pair` is removed from `siblings` (its own
 *  entity group, in render order). The removal is instant and unanimated, so without
 *  this focus falls to `<body>` (P0-6).
 *
 *  Three spots, and the third is not belt-and-braces: a group with no rows left stops
 *  rendering and takes its own heading with it, so the last pair in a group has no
 *  heading to land on. The queue container always survives — it is what holds
 *  "No possible duplicates." A next row is only offered when it HAS a disclosure, so a
 *  non-person group always lands on its heading rather than on an id nothing rendered.
 *
 *  The caller focuses the first of these that is actually in the DOM. */
export function focusLandingIds(pair: DuplicatePair, siblings: DuplicatePair[]): string[] {
	const next = siblings[siblings.indexOf(pair) + 1];
	const ladder = next && showsComparePanel(next.entity_type) ? [disclosureId(next)] : [];
	return [...ladder, groupId(pair.entity_type), QUEUE_ID];
}
