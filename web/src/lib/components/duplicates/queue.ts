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
import type { DuplicateLane, DuplicateLaneCounts, DuplicatePair, EntityKind } from '#lib/types.ts';

/** One stable key per pair. The `{#each}` key, the panel's id and the disclosure's id
 *  all derive from it, so focus can be moved by id without threading refs up the tree. */
export function pairKey(p: DuplicatePair): string {
	return `${p.entity_type}-${p.a.id}-${p.b.id}`;
}

export const disclosureId = (p: DuplicatePair): string => `dup-disclosure-${pairKey(p)}`;
export const panelId = (p: DuplicatePair): string => `dup-panel-${pairKey(p)}`;
export const laneTabId = (lane: DuplicateLane): string => `dup-tab-${lane}`;
/** The lane's tabpanel: the focus ladder's middle rung, now the group heading is gone. */
export const lanePanelId = (lane: DuplicateLane): string => `dup-lane-${lane}`;
/** The queue container — the one landing spot that outlives every lane (P0-6). */
export const QUEUE_ID = 'dup-queue';

/** The lanes in their fixed tab order (P0-8, RD13). The order never changes and a lane
 *  with no pairs keeps its place, so the tabs never shift. */
export const LANES: readonly DuplicateLane[] = ['video', 'person', 'studio', 'tag', 'film'];

export const LANE_LABEL: Record<DuplicateLane, string> = {
	video: 'Videos',
	person: 'People',
	studio: 'Studios',
	tag: 'Tags',
	film: 'Films'
};

/** `?type=` → a lane, or null when absent or not a lane. */
export function parseLane(value: string | null): DuplicateLane | null {
	return (LANES as readonly string[]).includes(value ?? '') ? (value as DuplicateLane) : null;
}

/** The lane the page opens on when the URL names none: the first, in tab order, with pairs;
 *  Videos when every lane is empty. A lane the URL does name opens even at 0 — the
 *  Duplicates banner's link must land where it says. */
export function defaultLane(counts: DuplicateLaneCounts): DuplicateLane {
	return LANES.find((l) => counts[l] > 0) ?? 'video';
}

/** The compare panel is person and studio (RD10, 598): a tag or film pair has no evidence
 *  worth two columns, and a control that cannot change what you see is one the reader
 *  learns to distrust. */
export function showsComparePanel(entityType: EntityKind): boolean {
	return entityType === 'person' || entityType === 'studio';
}

/** The entity's own page, which a pair row and its panel open in a new tab (P0-9). Only
 *  the comparable kinds link; tag and film rows keep plain names. */
export function profileHref(entityType: EntityKind, id: number): string {
	return entityType === 'person' ? `/people/${id}` : `/studios/${id}`;
}

/** A studio evidence namespace as the panel names it. 'file' is the file tags; 'manual' is
 *  a value the owner set; anything else is a provider, named by its own label where the
 *  side's links carry one. '' (nothing resolved) is absent, never a placeholder. */
export function sourceLabel(source: string, labels: Record<string, string> = {}): string {
	if (source === 'file') return 'File tags';
	if (source === 'manual') return 'you';
	return labels[source] ?? source;
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

/** The noun a citation uses for each kind, so the chip reads as a sentence. `tag` is here
 *  for exhaustiveness only — a tag is not enrichable, so no tag pair carries this variation
 *  (ADR-107 D5, asserted at both producers). */
const KIND_NOUN: Record<EntityKind, string> = {
	person: 'person',
	studio: 'studio',
	film: 'film',
	tag: 'tag'
};

/** The chip for a `shared-external-id` row, or null for every other variation — a SIBLING
 *  of MATCH_KIND_LABEL, never an entry in it, because that map is keyed on the derived
 *  `match_kind`, which ListReviewPairs leaves '' for every non-fuzzy row.
 *
 *  It replaces the muted variation slug rather than joining it, so the row costs nothing and
 *  stays one line — which is why, unlike `labelPlacement`, this does not move to the panel
 *  for a person.
 *
 *  It names the ASSERTER, not the verdict: "tmdb says one person", never "same person". F70's
 *  rule holds — the surface reports who asserted what and never adjudicates, so the chip is a
 *  citation. `detail` carries the provider; a row that somehow has none says so generically
 *  rather than inventing a name. Styled accent, never `text-warn`: warn already means *weak*
 *  on this surface and this is the strongest evidence the queue will ever carry. */
export function sharedIdChip(pair: DuplicatePair): { text: string } | null {
	if (pair.variation !== 'shared-external-id') return null;
	const noun = KIND_NOUN[pair.entity_type] ?? 'record';
	// `?? ''` guards a payload without the field rather than throwing: `detail` is new to
	// this response (F71), and a row that cannot name its asserter must still render.
	const asserter = (pair.detail ?? '').trim();
	return { text: asserter ? `${asserter} says one ${noun}` : `a provider says one ${noun}` };
}

/** Where the match-kind label renders (OQ3, resolved 2026-09-23). A PERSON or STUDIO
 *  pair carries it in the panel — measured at real type sizes it needs ~836 px in the
 *  row, and 100 % of the live queue carries one, so leaving it there made the two-line
 *  row the default on a surface built for scanning. Tag and film keep it in the row:
 *  they have no panel to move it to, and without it two unalike names are paired with
 *  nothing explaining why. Gate on entity type, never on "the panel exists". */
export function labelPlacement(pair: DuplicatePair): 'row' | 'panel' | 'none' {
	if (!matchKindLabel(pair.match_kind).text) return 'none';
	return showsComparePanel(pair.entity_type) ? 'panel' : 'row';
}

/** Where focus may land, in order, when `pair` is removed from `siblings` (its own
 *  entity group, in render order). The removal is instant and unanimated, so without
 *  this focus falls to `<body>` (P0-6).
 *
 *  Three spots: the next row's disclosure, the lane's tabpanel (which outlives its last
 *  row — it then holds "No possible duplicate …"), and the queue container, for a lane
 *  that was switched away mid-resolve. A next row is only offered when it HAS a
 *  disclosure, so a tag or film lane always lands on its tabpanel rather than on an id
 *  nothing rendered.
 *
 *  The caller focuses the first of these that is actually in the DOM. */
export function focusLandingIds(pair: DuplicatePair, siblings: DuplicatePair[]): string[] {
	const next = siblings[siblings.indexOf(pair) + 1];
	const ladder = next && showsComparePanel(next.entity_type) ? [disclosureId(next)] : [];
	return [...ladder, lanePanelId(pair.entity_type), QUEUE_ID];
}
