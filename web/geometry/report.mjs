// Turning verdicts into something worth reading (HOLODEX-349).
//
// A failure has to name four things or it costs more time than it saves: the page (so
// it can be opened), the coordinate that makes that page interesting (so the reader
// knows *why* it was measured), the selector (so the reader knows what was measured),
// and measured-vs-expected (so the reader knows how bad it is). All four appear on
// every failing line below.
//
// Pure string formatting, so the output shape is unit-tested rather than eyeballed.

import { describeBound, failed } from './evaluate.mjs';

/**
 * @typedef {object} Result
 * @property {import('./assertions.mjs').Assertion} assertion
 * @property {string} url
 * @property {string} label  The manifest coordinate, or the literal URL for a list page.
 * @property {string} cell   e.g. "brutalist/narrow".
 * @property {string} status
 * @property {string} detail
 * @property {{index: number, value: number, tag: string, text: string}[]} [offenders]
 */

const ICON = {
	pass: 'ok  ',
	fail: 'FAIL',
	vacuous: 'VOID',
	error: 'ERR ',
	blocked: 'open',
	fixed: 'NEWS',
	skipped: 'skip'
};

/**
 * render produces the whole report. Passing assertions collapse to one line each —
 * the interesting output is the failures, and burying them under 400 green lines is
 * how a harness stops being read.
 *
 * @param {Result[]} results
 * @param {{cells: number, pages: number, elapsedMs: number, reconciled?: boolean}} stats
 */
export function render(results, stats) {
	const lines = [];
	const byAssertion = new Map();
	for (const r of results) {
		if (!byAssertion.has(r.assertion.key)) byAssertion.set(r.assertion.key, []);
		byAssertion.get(r.assertion.key).push(r);
	}

	for (const [key, group] of byAssertion) {
		const bad = group.filter((r) => r.status !== 'pass' && r.status !== 'skipped');
		const a = group[0].assertion;
		const bound = `${a.applies === 'count' ? 'count' : a.measure} ${describeBound(a.expect)}`;

		if (bad.length === 0) {
			const skipped = group.filter((r) => r.status === 'skipped');
			if (skipped.length > 0) {
				lines.push(`${ICON.skipped}  ${key} — ${skipped[0].detail}`);
			} else {
				lines.push(`${ICON.pass}  ${key} — ${group.length} checks, ${bound}`);
			}
			continue;
		}

		const worst = bad.some((r) => failed(r.status)) ? bad.find((r) => failed(r.status)).status : bad[0].status;
		lines.push('');
		lines.push(`${ICON[worst] ?? worst}  ${key}${a.blockedBy ? `  [${a.blockedBy}]` : ''}`);
		lines.push(`      finds: ${wrap(a.finds, 6)}`);
		lines.push(`      where: ${a.selector}   (${bound})`);
		for (const r of bad) {
			lines.push(`      · ${r.cell}  ${r.label}`);
			lines.push(`          ${r.detail}`);
			for (const o of (r.offenders ?? []).slice(0, 5)) {
				const text = o.text ? `  "${o.text}"` : '';
				lines.push(`          [${o.index}] ${o.tag}  = ${o.value}  want ${describeBound(a.expect)}${text}`);
			}
			if ((r.offenders ?? []).length > 5) {
				lines.push(`          … and ${r.offenders.length - 5} more`);
			}
		}
		lines.push('');
	}

	lines.push('');
	lines.push(summary(results, stats));
	return lines.join('\n');
}

/**
 * summary is the last line, and the one that decides the exit code. `open` and `skip`
 * are counted but excluded from the verdict: a filed-and-unfixed bug is not a
 * regression, and an assertion that could not apply is not a result.
 *
 * @param {Result[]} results
 * @param {{cells: number, pages: number, elapsedMs: number, reconciled?: boolean}} stats
 */
export function summary(results, stats) {
	const tally = { pass: 0, fail: 0, vacuous: 0, error: 0, blocked: 0, fixed: 0, skipped: 0 };
	for (const r of results) tally[r.status] = (tally[r.status] ?? 0) + 1;
	const parts = [
		`${tally.pass} passed`,
		tally.fail ? `${tally.fail} failed` : '',
		tally.vacuous ? `${tally.vacuous} measured nothing` : '',
		tally.error ? `${tally.error} errored` : '',
		tally.blocked ? `${tally.blocked} known-open` : '',
		tally.fixed ? `${tally.fixed} newly passing` : '',
		tally.skipped ? `${tally.skipped} skipped` : ''
	].filter(Boolean);
	const secs = (stats.elapsedMs / 1000).toFixed(1);
	const line = `${parts.join(', ')}  —  ${stats.pages} page loads across ${stats.cells} skin/width cells in ${secs}s`;
	// A narrowed run cannot retire a `blockedBy` marker (see reconcileBlocked), and its
	// tally is indistinguishable from a full run that found none stale. Say which it was,
	// rather than letting the absence of a NEWS line read as evidence.
	return stats.reconciled === false
		? `${line}\n  known-open markers not checked: this run measured part of the matrix`
		: line;
}

/** @param {Result[]} results */
export function exitCode(results) {
	return results.some((r) => failed(r.status)) ? 1 : 0;
}

/**
 * wrap re-flows a long prose field to the terminal, indented under its label. The
 * `finds` text is the assertion's reason for existing and is worth reading, but it is
 * written as a paragraph.
 *
 * @param {string} text
 * @param {number} indent
 */
export function wrap(text, indent, width = 92) {
	const pad = ' '.repeat(indent + 7);
	const out = [];
	let line = '';
	for (const word of text.split(/\s+/)) {
		if (line && (line + ' ' + word).length > width - indent) {
			out.push(line);
			line = word;
		} else {
			line = line ? `${line} ${word}` : word;
		}
	}
	if (line) out.push(line);
	return out.join('\n' + pad);
}
