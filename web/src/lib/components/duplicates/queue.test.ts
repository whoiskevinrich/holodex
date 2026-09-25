import { describe, expect, it } from 'vitest';
import type { DuplicatePair, EntityKind } from '$lib/types';
import {
	disclosureId,
	focusLandingIds,
	groupId,
	labelPlacement,
	matchKindLabel,
	pairKey,
	panelId,
	QUEUE_ID,
	sharedIdChip,
	showsComparePanel
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

// RD10. The panel is person-only, and the label placement below is gated on THIS —
// not on "a panel happens to be open".
describe('showsComparePanel', () => {
	it('is true for a person pair and false for every other kind', () => {
		expect(showsComparePanel('person')).toBe(true);
		for (const kind of ['studio', 'tag', 'film'] as EntityKind[]) {
			expect(showsComparePanel(kind)).toBe(false);
		}
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
	it("puts a person pair's label in the panel, keeping the row one line", () => {
		expect(labelPlacement(pair('person', 1, 2, 'alias'))).toBe('panel');
		expect(labelPlacement(pair('person', 1, 2, 'mixed'))).toBe('panel');
	});

	it("keeps every non-person pair's label in the row", () => {
		for (const kind of ['studio', 'tag', 'film'] as EntityKind[]) {
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
// falls to <body> unless it is moved. The third rung is the one hand QA found: the
// group heading dies with its own group, so the last pair in a group has no heading.
describe('focusLandingIds', () => {
	const a = pair('person', 1, 2);
	const b = pair('person', 3, 4);
	const c = pair('person', 5, 6);

	it("offers the next row's disclosure first", () => {
		expect(focusLandingIds(a, [a, b, c])).toEqual([disclosureId(b), groupId('person'), QUEUE_ID]);
	});

	it('falls to the group heading for the last row in a group', () => {
		expect(focusLandingIds(c, [a, b, c])).toEqual([groupId('person'), QUEUE_ID]);
	});

	it('ends at the queue for the only row in a group — the heading dies with it', () => {
		// The heading is still offered, but it will not be in the DOM once its group
		// stops rendering; the queue container is what survives, and it is what holds
		// "No possible duplicates."
		expect(focusLandingIds(a, [a])).toEqual([groupId('person'), QUEUE_ID]);
	});

	it('never offers a disclosure on a non-person group — those rows have none', () => {
		const t1 = pair('tag', 1, 2);
		const t2 = pair('tag', 3, 4);
		expect(focusLandingIds(t1, [t1, t2])).toEqual([groupId('tag'), QUEUE_ID]);
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
