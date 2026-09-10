import { describe, it, expect } from 'vitest';
import { within, describeBound, evaluate, failed, reconcileBlocked } from './evaluate.mjs';

/** @param {Partial<import('./evaluate.mjs').Measured>[]} els */
const probeOf = (els) => ({
	matched: els.length,
	elements: els.map((e, index) => ({
		index,
		tag: 'div',
		width: 0,
		height: 0,
		overflowX: 0,
		overflowY: 0,
		fontSize: 16,
		visible: true,
		text: '',
		...e
	}))
});

const base = { key: 'k', finds: 'f', selector: '.x', measure: 'width', expect: { min: 40 } };

describe('within', () => {
	it('treats both bounds as inclusive', () => {
		expect(within(40, { min: 40 }).ok).toBe(true);
		expect(within(40, { max: 40 }).ok).toBe(true);
		expect(within(39.9, { min: 40 }).ok).toBe(false);
	});

	it('applies both ends when both are given', () => {
		expect(within(50, { min: 40, max: 60 }).ok).toBe(true);
		expect(within(70, { min: 40, max: 60 }).ok).toBe(false);
		expect(describeBound({ min: 40, max: 60 })).toBe('>= 40 and <= 60');
	});
});

describe('evaluate — each', () => {
	it('passes when every element is inside the bound', () => {
		const v = evaluate(base, probeOf([{ width: 80 }, { width: 41 }]));
		expect(v.status).toBe('pass');
	});

	it('names every offender, and only the offenders', () => {
		const v = evaluate(base, probeOf([{ width: 80 }, { width: 31.5 }, { width: 12 }]));
		expect(v.status).toBe('fail');
		expect(v.offenders.map((o) => o.value)).toEqual([31.5, 12]);
		expect(v.detail).toContain('2/3');
	});

	// The whole point of the guard: "every matched element is >= 40px" is trivially
	// true of no elements, so a stale selector would read as a pass and the assertion
	// would stop guarding anything without ever going red.
	it('refuses to pass an assertion that matched nothing', () => {
		const v = evaluate(base, probeOf([]));
		expect(v.status).toBe('vacuous');
		expect(v.detail).toContain('measured nothing');
	});

	it('honours an explicit atLeast', () => {
		expect(evaluate({ ...base, atLeast: 3 }, probeOf([{ width: 80 }, { width: 80 }])).status).toBe('vacuous');
		expect(evaluate({ ...base, atLeast: 0 }, probeOf([])).status).toBe('pass');
	});

	// A non-scrollable box (an inline <a> or <span>) has scrollWidth === clientWidth === 0,
	// so a naive subtraction reports a confident "no overflow" for exactly the elements
	// whose text is most likely to be spilling. The probe reports null; this must not pass.
	it('refuses to score an overflow metric the element cannot carry', () => {
		const v = evaluate({ ...base, measure: 'overflowX', expect: { max: 0 } }, probeOf([{ overflowX: null }]));
		expect(v.status).toBe('error');
		expect(v.detail).toContain('not measurable');
	});

	it('reports a probe error as an error, not a failure', () => {
		const v = evaluate(base, { matched: 0, elements: [], error: 'boom' });
		expect(v.status).toBe('error');
		expect(v.detail).toBe('boom');
	});
});

describe('evaluate — count', () => {
	const counting = { ...base, applies: 'count', expect: { max: 200 } };

	it('bounds the number of matches instead of each match', () => {
		expect(evaluate(counting, probeOf(Array(10).fill({}))).status).toBe('pass');
		const v = evaluate(counting, probeOf(Array(201).fill({})));
		expect(v.status).toBe('fail');
		expect(v.detail).toContain('201');
	});

	// A count bound is usually a ceiling, and zero satisfies a ceiling — so this is the
	// one shape where a stale selector reads as a confident pass rather than as a
	// suspiciously green `each`. The guard has to run before the count branch, not after.
	it('applies the vacuity guard, so a stale selector is not a passing count', () => {
		const v = evaluate(counting, probeOf([]));
		expect(v.status).toBe('vacuous');
		expect(failed(v.status)).toBe(true);
	});

	// The opt-out, for a count assertion that is genuinely asserting absence.
	it('honours atLeast: 0 for a count that may legitimately be empty', () => {
		expect(evaluate({ ...counting, atLeast: 0 }, probeOf([])).status).toBe('pass');
	});
});

describe('evaluate — known-open bugs', () => {
	const blocked = { ...base, blockedBy: 'HOLODEX-354' };

	it('does not let a filed, unfixed bug turn the run red', () => {
		const v = evaluate(blocked, probeOf([{ width: 10 }]));
		expect(v.status).toBe('blocked');
		expect(failed(v.status)).toBe(false);
	});

	// A page passing while the bug is open is the ordinary case, not news — most pages
	// pass even when one is broken. Staleness is decided over the whole assertion.
	it('leaves an individual pass alone', () => {
		expect(evaluate(blocked, probeOf([{ width: 80 }])).status).toBe('pass');
	});

	// A marker mutes a *failure*. It does not mute vacuity: "the selector matched
	// nothing" is a statement about the harness, and an open ticket is no reason to
	// believe the assertion should have measured zero elements. Muting it would let a
	// renamed class hide behind the ticket and exit 0.
	it('still reports a stale selector under a marker, and still fails the run', () => {
		const v = evaluate(blocked, probeOf([]));
		expect(v.status).toBe('vacuous');
		expect(failed(v.status)).toBe(true);
	});
});

describe('reconcileBlocked', () => {
	const marked = { key: 'k', blockedBy: 'HOLODEX-354' };
	const plain = { key: 'p' };
	const r = (assertion, status) => ({ assertion, status });

	// The inversion that matters: a marker left behind after the fix would silently
	// disarm the assertion, so a fully-passing one is reported as news and fails the run.
	it('reports a marker whose assertion now passes everywhere', () => {
		const out = reconcileBlocked([r(marked, 'pass'), r(marked, 'pass')]);
		const news = out.filter((x) => x.status === 'fixed');
		expect(news).toHaveLength(1);
		expect(news[0].detail).toContain('HOLODEX-354');
		expect(news[0].label).toBe('all 2 checks');
	});

	// The bug this replaced: scoring staleness per page turned 101 ordinary passes into
	// 101 false alarms on an assertion whose bug was still open on 97 other pages.
	it('stays quiet while any check is still blocked', () => {
		const out = reconcileBlocked([r(marked, 'pass'), r(marked, 'pass'), r(marked, 'blocked')]);
		expect(out.some((x) => x.status === 'fixed')).toBe(false);
		expect(out).toHaveLength(3);
	});

	it('stays quiet when an error means nothing was actually proven', () => {
		expect(reconcileBlocked([r(marked, 'pass'), r(marked, 'error')]).some((x) => x.status === 'fixed')).toBe(false);
	});

	// The regression that came with un-muting `vacuous`: once it stopped being remapped
	// to `blocked`, a group of some passes and some stale selectors slipped past a
	// `stillBroken` that only enumerated blocked/error — and reported "every check now
	// passes" for an assertion that measured nothing on half its pages.
	it('stays quiet when some pages measured nothing', () => {
		const out = reconcileBlocked([r(marked, 'pass'), r(marked, 'vacuous')]);
		expect(out.some((x) => x.status === 'fixed')).toBe(false);
		expect(out).toHaveLength(2);
	});

	// Stated as an exclusion rather than a list, so a status added later suppresses the
	// verdict by default instead of silently counting as evidence the bug is gone.
	it('stays quiet on a status it has never heard of', () => {
		expect(reconcileBlocked([r(marked, 'pass'), r(marked, 'weird')]).some((x) => x.status === 'fixed')).toBe(false);
	});

	it('says nothing about an assertion carrying no marker', () => {
		expect(reconcileBlocked([r(plain, 'pass')])).toHaveLength(1);
	});

	// A --skin/--width slice is not evidence a bug is fixed: it may simply not have
	// measured the cell the bug lives in. Retiring a marker on that would disarm the
	// assertion against a bug that is still open, so a narrowed run declines to answer.
	it('refuses to retire a marker on a narrowed run', () => {
		const out = reconcileBlocked([r(marked, 'pass'), r(marked, 'pass')], { complete: false });
		expect(out.some((x) => x.status === 'fixed')).toBe(false);
		expect(out).toHaveLength(2);
	});
});

describe('failed', () => {
	it('counts every way of not knowing as a failure', () => {
		expect(['fail', 'vacuous', 'error', 'fixed'].every(failed)).toBe(true);
		expect(['pass', 'blocked', 'skipped'].some(failed)).toBe(false);
	});
});
