// The assertion table (HOLODEX-349).
//
// This is the file you edit. Everything else in `web/geometry/` is machinery.
//
// An assertion is a layout invariant written against a *property* of a page rather
// than against a page. That is the whole point (spec D6): the owner says "media 902's
// headshots are unusably small", and what gets written back is not "media 902" but
// "any page with ten or more people" — which covers the rung he happened to look at,
// the two rungs above it, and the rung somebody adds next year.
//
// See README.md in this directory for the shape of one, and `docs/testing-strategy.md`
// for when adding one is the right move.

import { METRICS } from './evaluate.mjs';

/**
 * @typedef {object} Assertion
 * @property {string} key            Stable slug; names the assertion in the report.
 * @property {string} finds          What breaking this invariant looks like to a human.
 * @property {(e: import('./manifest.mjs').Entry) => boolean} [when]
 *   Selects addressed fixture pages by their manifest coordinate. Exactly one of
 *   `when` or `urls` is required.
 * @property {string[]} [urls]       Literal pages, for surfaces the manifest does not
 *   address — the list pages, whose subject is the unaddressed breadth pool.
 * @property {{why: string, met: (m: any) => boolean}} [requires]
 *   Gate on the shape of the seeded fixture. Skipped, with the reason printed, when
 *   unmet — for assertions that can only distinguish right from wrong at `-big`.
 * @property {string[]} [prepare]    Named page preparations to run before measuring.
 * @property {string} selector       CSS selector, or `:document` for the page itself.
 * @property {'each'|'count'} [applies]  Default `each`.
 * @property {'width'|'height'|'overflowX'|'overflowY'|'fontSize'} [measure]
 *   Required for `each`; ignored for `count`.
 * @property {{min?: number, max?: number}} expect
 * @property {number} [atLeast]      Minimum matches before the assertion means anything.
 *   Default 1. Set 0 only when an empty page is genuinely one of the valid states.
 * @property {string} [blockedBy]    Ticket key for a bug that is filed and not yet
 *   fixed. The assertion still runs; a failure is reported but does not fail the run,
 *   and a *pass* is reported as news — the marker has gone stale.
 */

/** @type {Assertion[]} */
export const ASSERTIONS = [
	{
		key: 'person-tiles-stay-legible',
		finds:
			'Headshot tiles collapsing as the cast grows, until a face is a smudge. This is the ' +
			'invariant the harness was built around — the owner’s own example was a media page ' +
			'with twelve people. It reads the cast count off whichever half of the coordinate ' +
			'applies, so one assertion covers the `people` rungs on media pages and the ' +
			'`filmcast` rungs on film pages.',
		when: (e) => (e.axes.video?.people ?? e.axes.film?.cast ?? 0) >= 10,
		// `.portrait-frame--2x3` inside a curation chip is the person tile specifically:
		// the film chips beside it on a media page use a plain aspect-ratio box, not a
		// portrait frame. The frame — not the <img> — is what sets the rendered size.
		selector: 'li.curation-chip .portrait-frame--2x3',
		measure: 'width',
		expect: { min: 40 }
	},

	{
		key: 'no-horizontal-page-overflow',
		finds:
			'Text that pushes the whole document wider than the viewport, so every page ' +
			'scrolls sideways. The `unbroken` rung — a 60+ character token with nowhere to ' +
			'wrap — is designed to cause exactly this, and CJK and the bidi pair each cause ' +
			'it differently. Scoped to the text dimensions because that is where a title, an ' +
			'overview or an entity name is the thing under test.',
		when: (e) => ['text', 'persontext', 'studiotext', 'tagtext'].includes(e.dimension),
		selector: ':document',
		measure: 'overflowX',
		expect: { max: 0 },
		blockedBy: 'HOLODEX-355, HOLODEX-356'
	},

	{
		key: 'tag-chips-stay-tappable',
		finds:
			'Tag chips squeezing below a comfortable touch target once a video carries enough ' +
			'of them to wrap onto several rows.',
		when: (e) => (e.axes.video?.tags ?? 0) >= 5,
		selector: '#field-genres a[href^="/tags/"]',
		measure: 'height',
		// 24px is WCAG 2.2 AA (SC 2.5.8 Target Size, Minimum). The project has not
		// explicitly adopted it, so the bound is a proposal and HOLODEX-357 is the ruling.
		expect: { min: 24 },
		blockedBy: 'HOLODEX-357'
	},

	{
		key: 'source-chips-stay-tappable',
		finds:
			'The ADR-051 chip row cramping when several providers compete for one field. The ' +
			'`enrich` dimension exists to give this something to measure: media 902 resolves ' +
			'five competing namespaces on `tagline` with no manual enrichment. The chips only ' +
			'exist once the fold and the badge are open, hence the two preparations.',
		when: (e) => (e.axes.video?.namespaces ?? 0) >= 5,
		prepare: ['metadata-fold', 'source-badge:tagline'],
		selector: '[data-source-badge="tagline"] [data-seg]',
		measure: 'height',
		expect: { min: 24 },
		blockedBy: 'HOLODEX-357'
	},

	{
		key: 'people-list-does-not-render-everything',
		finds:
			'The People list rendering every row it is given, unpaginated and unvirtualized — ' +
			'HOLODEX-354, found by this fixture. Only meaningful on a large fixture: at the ' +
			'default size the whole table fits under any sane page budget, so a small run ' +
			'cannot tell a paginated list from an unpaginated one.',
		urls: ['/people'],
		requires: {
			why: 'needs a large fixture — reseed with `go run ./testdata/stressseed -big`',
			met: (m) => (m.breadth?.tables?.people?.n ?? 0) >= 500
		},
		selector: 'main ul > li.scroll-mt-16',
		applies: 'count',
		expect: { max: 200 },
		blockedBy: 'HOLODEX-354'
	}
];

/**
 * validate refuses a malformed assertion at startup rather than at measurement time.
 *
 * A typo in `measure` would otherwise read back as `undefined` for every element and
 * fail every page with an incomprehensible message; a missing `when`/`urls` would
 * select no pages and quietly contribute nothing. Both are much cheaper to catch here.
 *
 * @param {Assertion[]} list
 * @returns {string[]} problems, empty when the table is sound
 */
export function validate(list = ASSERTIONS) {
	const problems = [];
	const seen = new Set();
	for (const [i, a] of list.entries()) {
		const at = a.key ? `assertion "${a.key}"` : `assertion #${i}`;
		if (!a.key) problems.push(`${at}: missing key`);
		else if (seen.has(a.key)) problems.push(`${at}: duplicate key`);
		seen.add(a.key);

		if (!a.finds) problems.push(`${at}: missing "finds" — say what breaking this looks like`);
		if (!a.selector) problems.push(`${at}: missing selector`);
		if (Boolean(a.when) === Boolean(a.urls)) {
			problems.push(`${at}: needs exactly one of "when" (manifest-addressed pages) or "urls" (literal pages)`);
		}
		if (!a.expect || (a.expect.min === undefined && a.expect.max === undefined)) {
			problems.push(`${at}: "expect" needs a min, a max, or both`);
		}
		const applies = a.applies ?? 'each';
		if (applies !== 'each' && applies !== 'count') {
			problems.push(`${at}: applies must be "each" or "count", got ${JSON.stringify(a.applies)}`);
		} else if (applies === 'each' && !METRICS.includes(/** @type {string} */ (a.measure))) {
			problems.push(`${at}: measure must be one of ${METRICS.join(', ')}, got ${JSON.stringify(a.measure)}`);
		}
	}
	return problems;
}
