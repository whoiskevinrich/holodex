import { describe, expect, it } from 'vitest';
import type { DuplicatePair, EntityKind } from '#lib/types.ts';
import {
	defaultLane,
	disclosureId,
	focusLandingIds,
	LANE_LABEL,
	lanePanelId,
	LANES,
	laneTabId,
	labelPlacement,
	matchKindLabel,
	pairKey,
	panelId,
	parseLane,
	profileHref,
	QUEUE_ID,
	sharedIdChip,
	showsComparePanel,
	sourceLabel
} from './queue';

const pair = (
	entity_type: EntityKind,
	aId: number,
	bId: number,
	match_kind: DuplicatePair['match_kind'] = 'alias'
): DuplicatePair => ({
	entity_type,
	a: { id: aId, name: `a${aId}` },
	b: { id: bId, name: `b${bId}` },
	variation: 'provider-alias',
	match_kind,
	detail: ''
});

/** A shared-external-id pair as the server sends one: `detail` names the asserting
 *  provider and `match_kind` is '' (ListReviewPairs leaves it empty for every non-fuzzy
 *  variation), which is exactly why the chip cannot live in the MATCH_KIND_LABEL map. */
const sharedIdPair = (entity_type: EntityKind, provider = 'tmdb'): DuplicatePair => ({
	...pair(entity_type, 1, 2),
	variation: 'shared-external-id',
	match_kind: '' as DuplicatePair['match_kind'],
	detail: provider
});

// The disclosure, the panel and the `{#each}` key are produced by three different
// files; if they stop agreeing, `aria-controls` points at a panel id that never
// renders and the focus ladder below silently misses every landing spot.
describe('pair ids', () => {
	it('keys a pair by kind and both ids, so two kinds at the same ids stay distinct', () => {
		expect(pairKey(pair('person', 3, 9))).toBe('person-3-9');
		expect(pairKey(pair('studio', 3, 9))).toBe('studio-3-9');
	});

	it('derives the disclosure and panel ids from the same key', () => {
		const p = pair('person', 3, 9);
		expect(disclosureId(p)).toBe(`dup-disclosure-${pairKey(p)}`);
		expect(panelId(p)).toBe(`dup-panel-${pairKey(p)}`);
	});
});

// RD10 (598). The panel is person and studio, and the label placement below is gated on
// THIS — not on "a panel happens to be open".
describe('showsComparePanel', () => {
	it('is true for person and studio pairs and false for tag and film', () => {
		expect(showsComparePanel('person')).toBe(true);
		expect(showsComparePanel('studio')).toBe(true);
		for (const kind of ['tag', 'film'] as EntityKind[]) {
			expect(showsComparePanel(kind)).toBe(false);
		}
	});

	it('links a comparable name to its own page', () => {
		expect(profileHref('person', 7)).toBe('/people/7');
		expect(profileHref('studio', 7)).toBe('/studios/7');
	});
});

// P0-8. The tabs are fixed in order, never hidden at 0, and the page opens on the first
// lane with pairs unless the URL names one.
describe('lanes', () => {
	const zero = { video: 0, person: 0, studio: 0, tag: 0, film: 0 };

	it('keeps Videos, People, Studios, Tags, Films as the fixed tab order', () => {
		expect(LANES.map((l) => LANE_LABEL[l])).toEqual(['Videos', 'People', 'Studios', 'Tags', 'Films']);
	});

	it('opens the first lane in tab order that has pairs', () => {
		expect(defaultLane({ ...zero, studio: 3, tag: 9 })).toBe('studio');
		expect(defaultLane({ ...zero, video: 1, person: 31 })).toBe('video');
	});

	it('opens Videos when every lane is empty', () => {
		expect(defaultLane(zero)).toBe('video');
	});

	it('reads a lane from ?type=, and nothing else', () => {
		for (const l of LANES) expect(parseLane(l)).toBe(l);
		expect(parseLane(null)).toBeNull();
		expect(parseLane('')).toBeNull();
		expect(parseLane('people')).toBeNull();
	});

	it('gives each lane its own tab and tabpanel id', () => {
		expect(laneTabId('studio')).toBe('dup-tab-studio');
		expect(lanePanelId('studio')).toBe('dup-lane-studio');
	});
});

// P0-10. The studio panel names where a spelling came from; it never invents a provider.
describe('sourceLabel', () => {
	it('names the file tags and the owner in words', () => {
		expect(sourceLabel('file')).toBe('File tags');
		expect(sourceLabel('manual')).toBe('you');
	});

	it("uses a provider's own label where the side carries one, else its namespace", () => {
		expect(sourceLabel('provider-1', { 'provider-1': 'Provider One' })).toBe('Provider One');
		expect(sourceLabel('provider-2', { 'provider-1': 'Provider One' })).toBe('provider-2');
	});
});

describe('matchKindLabel', () => {
	it('marks alias as the weak signal', () => {
		expect(matchKindLabel('alias')).toEqual({
			text: 'alias match only — weak signal',
			weak: true
		});
	});

	it('explains mixed without calling it weak', () => {
		expect(matchKindLabel('mixed')).toEqual({ text: 'via alias', weak: false });
	});

	it('says nothing for canonical — the two names visibly match', () => {
		expect(matchKindLabel('canonical').text).toBe('');
	});

	it('says nothing for a match kind the frontend has never heard of', () => {
		expect(matchKindLabel('something-new').text).toBe('');
	});
});

// OQ3, reversed 2026-09-23. The trap this pins: moving the label into the panel
// without gating on entity type deletes it from studio/tag/film rows, which have no
// panel to move it to — and then two unalike names are paired with nothing saying why.
describe('labelPlacement', () => {
	it("puts a person or studio pair's label in the panel, keeping the row one line", () => {
		for (const kind of ['person', 'studio'] as EntityKind[]) {
			expect(labelPlacement(pair(kind, 1, 2, 'alias'))).toBe('panel');
			expect(labelPlacement(pair(kind, 1, 2, 'mixed'))).toBe('panel');
		}
	});

	it("keeps a tag or film pair's label in the row", () => {
		for (const kind of ['tag', 'film'] as EntityKind[]) {
			expect(labelPlacement(pair(kind, 1, 2, 'alias'))).toBe('row');
			expect(labelPlacement(pair(kind, 1, 2, 'mixed'))).toBe('row');
		}
	});

	it('renders nothing anywhere for a canonical pair, whatever the kind', () => {
		expect(labelPlacement(pair('person', 1, 2, 'canonical'))).toBe('none');
		expect(labelPlacement(pair('tag', 1, 2, 'canonical'))).toBe('none');
	});
});

// P0-6. The row is removed instantly and unanimated (`pairs.filter()`), so focus
// falls to <body> unless it is moved. The lane's tabpanel replaced the group heading as
// the middle rung (598): it outlives its last row, which leaves it holding the empty line.
describe('focusLandingIds', () => {
	const a = pair('person', 1, 2);
	const b = pair('person', 3, 4);
	const c = pair('person', 5, 6);

	it("offers the next row's disclosure first", () => {
		expect(focusLandingIds(a, [a, b, c])).toEqual([disclosureId(b), lanePanelId('person'), QUEUE_ID]);
	});

	it("falls to the lane's tabpanel for the last row in a lane", () => {
		expect(focusLandingIds(c, [a, b, c])).toEqual([lanePanelId('person'), QUEUE_ID]);
		expect(focusLandingIds(a, [a])).toEqual([lanePanelId('person'), QUEUE_ID]);
	});

	it("offers a studio row's disclosure now that studios compare too", () => {
		const s1 = pair('studio', 1, 2);
		const s2 = pair('studio', 3, 4);
		expect(focusLandingIds(s1, [s1, s2])).toEqual([disclosureId(s2), lanePanelId('studio'), QUEUE_ID]);
	});

	it('never offers a disclosure in a tag lane — those rows have none', () => {
		const t1 = pair('tag', 1, 2);
		const t2 = pair('tag', 3, 4);
		expect(focusLandingIds(t1, [t1, t2])).toEqual([lanePanelId('tag'), QUEUE_ID]);
	});
});

// F71 P0-6. The chip is the only place the queue names WHO asserted a pair, and it is the
// only row label keyed on `variation` rather than the derived `match_kind` — a
// shared-external-id row's match_kind is '', so anything driven off that map renders nothing.
describe('shared-external-id chip', () => {
	it('cites the asserting provider and the entity kind, never a verdict', () => {
		expect(sharedIdChip(sharedIdPair('person'))?.text).toBe('tmdb says one person');
		expect(sharedIdChip(sharedIdPair('studio'))?.text).toBe('tmdb says one studio');
		expect(sharedIdChip(sharedIdPair('film'))?.text).toBe('tmdb says one film');
	});

	it('stays generic rather than inventing a name when the provider is missing', () => {
		expect(sharedIdChip(sharedIdPair('person', ''))?.text).toBe('a provider says one person');
		expect(sharedIdChip(sharedIdPair('person', '   '))?.text).toBe('a provider says one person');
		// `detail` is new to this response; a payload without it must render, not throw.
		const noField = { ...sharedIdPair('person'), detail: undefined as unknown as string };
		expect(sharedIdChip(noField)?.text).toBe('a provider says one person');
	});

	it('is null for every other variation, so no other row changes', () => {
		for (const variation of ['punctuation', 'internal-whitespace', 'provider-alias', 'same-title']) {
			expect(sharedIdChip({ ...pair('person', 1, 2), variation })).toBeNull();
		}
	});

	it('does not depend on match_kind — the map keyed on that one cannot carry it', () => {
		const p = sharedIdPair('person');
		expect(matchKindLabel(p.match_kind).text).toBe('');
		expect(sharedIdChip(p)).not.toBeNull();
	});
});
