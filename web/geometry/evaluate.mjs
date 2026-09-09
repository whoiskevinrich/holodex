// Turning measurements into verdicts (HOLODEX-349).
//
// Split out of the driver on purpose: this is the half that decides whether a page is
// wrong, and it is pure — a plain function from "what the browser measured" to "what
// the report should say". That makes the interesting logic (the vacuity guard, the
// known-open inversion) testable without a browser.

/**
 * @typedef {object} Measured One matched element as the in-page probe reports it.
 * @property {number} index
 * @property {string} tag
 * @property {number} width
 * @property {number} height
 * @property {number} overflowX
 * @property {number} overflowY
 * @property {number} fontSize
 * @property {boolean} visible
 * @property {string} text
 */

/** The metrics an assertion may bound. Anything else is a typo, and is refused. */
export const METRICS = ['width', 'height', 'overflowX', 'overflowY', 'fontSize'];

/**
 * within reports whether a value satisfies a {min, max} bound, and how it reads when
 * it does not. Both ends are inclusive: "at least 40px" is `min: 40`, and 40 passes.
 *
 * @param {number} value
 * @param {{min?: number, max?: number}} bound
 */
export function within(value, bound) {
	if (bound.min !== undefined && value < bound.min) return { ok: false, want: `>= ${bound.min}` };
	if (bound.max !== undefined && value > bound.max) return { ok: false, want: `<= ${bound.max}` };
	return { ok: true, want: describeBound(bound) };
}

/** @param {{min?: number, max?: number}} bound */
export function describeBound(bound) {
	const parts = [];
	if (bound.min !== undefined) parts.push(`>= ${bound.min}`);
	if (bound.max !== undefined) parts.push(`<= ${bound.max}`);
	return parts.join(' and ') || 'any';
}

/**
 * evaluate scores one assertion against one page in one context.
 *
 * The vacuity guard is the load-bearing part. A selector that matches nothing would
 * otherwise satisfy "every matched element is at least 40px wide" trivially — so a
 * renamed class would turn a real assertion green instead of red, which is the exact
 * failure mode this harness exists to prevent. `atLeast` (default 1) makes an empty
 * match a failure with its own status, so the report can say "measured nothing" rather
 * than "passed".
 *
 * @param {import('./assertions.mjs').Assertion} a
 * @param {{matched: number, elements: Measured[], error?: string}} probe
 * @returns {{status: 'pass'|'fail'|'vacuous'|'error'|'blocked'|'fixed', detail: string,
 *            offenders?: {index: number, value: number, tag: string, text: string}[]}}
 */
export function evaluate(a, probe) {
	const verdict = score(a, probe);
	// A known-open bug is expected to fail, so a failure is not news and must not turn
	// the run red. Whether the *marker* has gone stale is a question about the whole
	// assertion rather than about one page — most pages pass even while the bug is
	// open — so it is answered once, afterwards, by reconcileBlocked.
	if (a.blockedBy && (verdict.status === 'fail' || verdict.status === 'vacuous')) {
		return { ...verdict, status: 'blocked' };
	}
	return verdict;
}

/**
 * reconcileBlocked decides, per assertion, whether a `blockedBy` marker is now a lie.
 *
 * It is a whole-assertion question: `no-horizontal-page-overflow` covers 33 pages in
 * six cells, and while the bug is open most of those still pass. Only when *nothing*
 * is left failing is the bug actually fixed — at which point the stale marker would
 * quietly disarm the assertion against the next regression, so it is reported as news
 * and fails the run.
 *
 * Returns the results plus one synthetic entry per newly-passing assertion, rather
 * than rewriting the passes, so the report keeps saying how many checks actually ran.
 *
 * @template {{assertion: {key: string, blockedBy?: string}, status: string}} R
 * @param {R[]} results
 * @returns {(R | {assertion: any, url: string, label: string, cell: string, status: string, detail: string})[]}
 */
export function reconcileBlocked(results) {
	/** @type {Map<string, {assertion: any, statuses: string[]}>} */
	const groups = new Map();
	for (const r of results) {
		if (!r.assertion.blockedBy) continue;
		if (!groups.has(r.assertion.key)) groups.set(r.assertion.key, { assertion: r.assertion, statuses: [] });
		groups.get(r.assertion.key).statuses.push(r.status);
	}
	const extra = [];
	for (const { assertion, statuses } of groups.values()) {
		const stillBroken = statuses.some((s) => s === 'blocked' || s === 'error');
		const ranAtAll = statuses.some((s) => s === 'pass');
		if (!stillBroken && ranAtAll) {
			extra.push({
				assertion,
				url: '—',
				label: `all ${statuses.length} checks`,
				cell: '—',
				status: 'fixed',
				detail:
					`every check now passes — ${assertion.blockedBy} looks fixed. Drop \`blockedBy\` ` +
					`to arm this assertion, or the next regression goes unreported.`
			});
		}
	}
	return [...results, ...extra];
}

/**
 * @param {import('./assertions.mjs').Assertion} a
 * @param {{matched: number, elements: Measured[], error?: string}} probe
 */
function score(a, probe) {
	if (probe.error) return { status: /** @type {const} */ ('error'), detail: probe.error };

	const bound = a.expect;
	if (a.applies === 'count') {
		const check = within(probe.matched, bound);
		return check.ok
			? { status: /** @type {const} */ ('pass'), detail: `count ${probe.matched} ${check.want}` }
			: {
					status: /** @type {const} */ ('fail'),
					detail: `matched ${probe.matched} elements, want ${check.want}`
				};
	}

	const atLeast = a.atLeast ?? 1;
	if (probe.matched < atLeast) {
		return {
			status: /** @type {const} */ ('vacuous'),
			detail:
				`selector matched ${probe.matched} elements, expected at least ${atLeast} — ` +
				`the assertion measured nothing, so it proves nothing (selector stale, or the page did not render)`
		};
	}

	const metric = a.measure;
	const offenders = [];
	const unmeasurable = [];
	for (const el of probe.elements) {
		const value = el[metric];
		// An overflow metric is null on a box that cannot scroll. Treating that as 0
		// would be the worst possible answer — a silent pass on precisely the inline
		// elements whose text is most likely to be spilling — so it is its own failure.
		if (value === null || value === undefined) {
			unmeasurable.push({ index: el.index, value: NaN, tag: el.tag, text: el.text });
			continue;
		}
		if (!within(value, bound).ok) {
			offenders.push({ index: el.index, value, tag: el.tag, text: el.text });
		}
	}
	if (unmeasurable.length > 0) {
		return {
			status: /** @type {const} */ ('error'),
			detail:
				`${metric} is not measurable on ${unmeasurable.length}/${probe.matched} matched elements ` +
				`(a non-scrollable box has no overflow) — measure the block that contains them instead`,
			offenders: unmeasurable
		};
	}
	if (offenders.length === 0) {
		return {
			status: /** @type {const} */ ('pass'),
			detail: `${probe.matched}/${probe.matched} elements have ${metric} ${describeBound(bound)}`
		};
	}
	return {
		status: /** @type {const} */ ('fail'),
		detail: `${offenders.length}/${probe.matched} elements have ${metric} outside ${describeBound(bound)}`,
		offenders
	};
}

/**
 * failed reports whether a status should make the run exit non-zero. `blocked` does
 * not: it is a bug already filed and being tracked elsewhere. `fixed` does — not
 * because anything is broken, but because the assertion table now disagrees with
 * reality and a stale `blockedBy` is how a fixed bug quietly stops being guarded.
 *
 * @param {string} status
 */
export function failed(status) {
	return status === 'fail' || status === 'vacuous' || status === 'error' || status === 'fixed';
}
