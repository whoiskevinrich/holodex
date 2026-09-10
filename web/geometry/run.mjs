#!/usr/bin/env node
// The geometry assertion harness (HOLODEX-349).
//
//   cd web && npm run geometry
//
// Measures the running stress fixture against the invariants in `assertions.mjs`,
// across three skins and two viewport widths, and exits non-zero when one is broken.
//
// Prerequisites, all checked before anything is measured:
//   1. `npx playwright install chromium`   — once per machine; `npm ci` does not do it
//   2. `go run ./testdata/stressseed`      — seeds the fixture and writes the manifest
//   3. the `backend-stress` launch profile — serves it on :7800
//   4. the `web` launch profile            — the dev server on :5173, proxying /api
//
// See README.md for the shape of an assertion and docs/testing-strategy.md for when
// writing one is the right move.

import { parseArgs } from 'node:util';
import { ASSERTIONS, validate } from './assertions.mjs';
import { load, select, describe } from './manifest.mjs';
import { evaluate, reconcileBlocked } from './evaluate.mjs';
import { matrix, open, goto, prepare, probe, launch, SKINS, WIDTHS } from './browser.mjs';
import { render, exitCode } from './report.mjs';

const { values } = parseArgs({
	options: {
		base: { type: 'string', default: 'http://localhost:5173' },
		manifest: { type: 'string' },
		only: { type: 'string', multiple: true, default: [] },
		skin: { type: 'string', multiple: true, default: [] },
		width: { type: 'string', multiple: true, default: [] },
		headed: { type: 'boolean', default: false },
		list: { type: 'boolean', default: false },
		help: { type: 'boolean', default: false }
	}
});

if (values.help) {
	console.log(`geometry — layout invariants against the stress fixture (HOLODEX-349)

  --base <url>       app under test           (default http://localhost:5173)
  --manifest <path>  seed manifest            (default ../../data/stress/manifest.json)
  --only <key>       run one assertion; repeatable
  --skin <name>      restrict skins: ${SKINS.join(', ')}; repeatable
  --width <key>      restrict widths: ${WIDTHS.map((w) => w.key).join(', ')}; repeatable
  --headed           watch it run
  --list             print the plan and exit without measuring
`);
	process.exit(0);
}

const problems = validate();
if (problems.length > 0) {
	console.error('the assertion table is malformed:\n  ' + problems.join('\n  '));
	process.exit(2);
}

// Every filter value is validated up front, and an unknown one is fatal rather than
// ignored. A typo must never *narrow* the run: `--width narow --width wide` would
// otherwise measure half the matrix and report a clean pass over it, which is a worse
// outcome than any failure. Checking each list against its own vocabulary catches that,
// where a "did anything survive the filter?" test does not — one good value hides an
// arbitrary number of bad ones. (For --skin the alternative is also slow and opaque:
// every page would spend 15s waiting for `data-theme` to equal a value the app can
// never set.)
const known = [
	{ flag: '--only', given: values.only, vocabulary: ASSERTIONS.map((a) => a.key) },
	{ flag: '--skin', given: values.skin, vocabulary: SKINS },
	{ flag: '--width', given: values.width, vocabulary: WIDTHS.map((w) => w.key) }
];
for (const { flag, given, vocabulary } of known) {
	const unknown = given.filter((v) => !vocabulary.includes(v));
	if (unknown.length > 0) {
		console.error(`unknown ${flag}: ${unknown.join(', ')} — known values are ${vocabulary.join(', ')}`);
		process.exit(2);
	}
}

// A missing or malformed manifest is a prerequisite failure like any other, and
// manifest.mjs writes a better message for it than a bare stack trace does — but only
// if it is caught. Exit 2, the code this file reserves for "the harness could not run".
let manifest;
try {
	manifest = load(values.manifest);
} catch (err) {
	console.error(/** @type {Error} */ (err).message);
	process.exit(2);
}

const assertions = values.only.length
	? ASSERTIONS.filter((a) => values.only.includes(a.key))
	: ASSERTIONS;

const cells = matrix(
	values.skin.length ? values.skin : SKINS,
	values.width.length ? WIDTHS.filter((w) => values.width.includes(w.key)) : WIDTHS
);

// A narrowed matrix cannot retire a `blockedBy` marker: see reconcileBlocked.
const wholeMatrix = values.skin.length === 0 && values.width.length === 0;

/**
 * plan resolves each assertion to the pages it applies to.
 *
 * An assertion that resolves to no pages is reported, never skipped silently: it means
 * the fixture no longer contains the thing the assertion is about — a renamed
 * dimension, a dropped rung — and a harness that shrugs at that is a harness that goes
 * quietly green as its coverage evaporates.
 */
function plan() {
	/** @type {{assertion: any, url: string, label: string}[]} */
	const targets = [];
	/** @type {import('./report.mjs').Result[]} */
	const upfront = [];

	for (const a of assertions) {
		if (a.requires && !a.requires.met(manifest)) {
			upfront.push({
				assertion: a,
				url: '—',
				label: '—',
				cell: '—',
				status: 'skipped',
				detail: a.requires.why
			});
			continue;
		}
		const pages = a.urls
			? a.urls.map((url) => ({ url, label: url }))
			: select(manifest, a.when).map((e) => ({ url: e.url, label: describe(e) }));

		if (pages.length === 0) {
			upfront.push({
				assertion: a,
				url: '—',
				label: '—',
				cell: '—',
				status: 'vacuous',
				detail:
					'selects no pages in this fixture — the coordinate it asks for does not exist ' +
					'(a dimension renamed or a rung dropped?). Reseed, or update the `when` predicate.'
			});
			continue;
		}
		for (const p of pages) targets.push({ assertion: a, ...p });
	}
	return { targets, upfront };
}

const { targets, upfront } = plan();

if (values.list) {
	for (const t of targets) console.log(`${t.assertion.key.padEnd(40)} ${t.url.padEnd(16)} ${t.label}`);
	for (const u of upfront) console.log(`${u.assertion.key.padEnd(40)} ${u.status.padEnd(16)} ${u.detail}`);
	console.log(
		`\n${targets.length} page-assertions × ${cells.length} cells = ${targets.length * cells.length} checks`
	);
	process.exit(0);
}

/**
 * preflight refuses to measure the wrong thing.
 *
 * Both checks have bitten this repo before. A dev server started from a worktree with
 * no launch profile of its own silently serves a *different* checkout, and every
 * backend profile binds the same :7800 — so "the fixture is running" is exactly the
 * assumption worth verifying rather than trusting. Comparing a seeded title against
 * the manifest proves the server is serving *this* fixture, not merely some library.
 */
async function preflight() {
	const capsRes = await fetch(`${values.base}/api/v1/capabilities`).catch((err) => {
		throw new Error(`${values.base} is not answering (${err.message}). Is the \`web\` dev server running?`);
	});
	if (!capsRes.ok) {
		// A 502 here is the dev server proxying to a backend that is not up yet — `go run`
		// spends its first half-minute compiling. Say so, rather than failing on the empty
		// body a few lines later.
		throw new Error(
			`GET /api/v1/capabilities → ${capsRes.status}. The dev server is up but the backend ` +
				`behind it is not: start the \`backend-stress\` profile and give \`go run\` a moment to compile.`
		);
	}
	const caps = await capsRes.json();
	if (!caps.owner) {
		throw new Error(
			`${values.base} does not consider this client an owner, so the metadata list and chip ` +
				`row will not render. The backend-stress profile sets no ADMIN_TOKEN and is ` +
				`default-open (ADR-030) — is something else serving :7800?`
		);
	}
	if (!caps.films_enabled) {
		throw new Error(
			`films are disabled on ${values.base}; the film dimensions would 404. The ` +
				`backend-stress profile sets FILMS_ENABLED=true — is something else serving :7800?`
		);
	}
	const sample = Object.values(manifest.entities).find((e) => e.entity === 'video');
	if (!sample) {
		throw new Error(
			'the manifest addresses no video entity, so there is nothing to identify the fixture by. ' +
				'Reseed with `go run ./testdata/stressseed`.'
		);
	}
	const detail = await fetch(`${values.base}/api/v1/media/${sample.id}`);
	if (!detail.ok) {
		throw new Error(`GET /api/v1/media/${sample.id} → ${detail.status}; the fixture is not served here`);
	}
	if (!JSON.stringify(await detail.json()).includes(sample.name)) {
		throw new Error(
			`media ${sample.id} is not the seeded fixture entity.\n` +
				`  manifest says: ${sample.name}\n` +
				`  The server on ${values.base} is serving a different library — check which ` +
				`launch profile is running, and that this worktree has its own .claude/launch.json.`
		);
	}
}

try {
	await preflight();
} catch (err) {
	console.error(`preflight failed: ${/** @type {Error} */ (err).message}`);
	process.exit(2);
}

const started = Date.now();
/** @type {import('./report.mjs').Result[]} */
const results = [...upfront];
let pageLoads = 0;

// Group by URL so one navigation serves every assertion on that page. The two halves are
// run apart on purpose: a preparation opens a fold, which reflows everything below it, so
// an assertion expecting the resting layout must not be measured after one has run — the
// prepared ones each get a fresh load. The grouping is a function of `targets` alone, so
// it is built once rather than per cell.
/** @type {Map<string, {assertion: any, url: string, label: string}[]>} */
const byUrl = new Map();
for (const t of targets) {
	if (!byUrl.has(t.url)) byUrl.set(t.url, []);
	byUrl.get(t.url).push(t);
}

// Acquiring the browser is a prerequisite, not a measurement, so it exits 2 like the
// rest of them. `npm ci` installs no browser binaries — playwright 1.63 ships no
// install script — so "run `npx playwright install chromium`" is the single most likely
// thing to go wrong on a clean checkout, and it happens *after* preflight has already
// passed. Left uncaught it surfaces as a raw stack and exit 1, which reads as a layout
// regression.
let browser;
try {
	browser = await launch(values.headed);
} catch (err) {
	console.error(
		`could not start a browser: ${/** @type {Error} */ (err).message}\n` +
			`  \`npm ci\` does not download browser binaries. Install the one this harness uses:\n` +
			`    npx playwright install chromium`
	);
	process.exit(2);
}

try {
	for (const cell of cells) {
		const { context, page } = await open(browser, cell);
		process.stderr.write(`  ${cell.key} …`);
		try {
			for (const [url, group] of byUrl) {
				const resting = group.filter((t) => !t.assertion.prepare?.length);
				const prepared = group.filter((t) => t.assertion.prepare?.length);

				if (resting.length > 0) {
					await visit(page, url, cell, resting, []);
					pageLoads++;
				}
				for (const t of prepared) {
					await visit(page, url, cell, [t], t.assertion.prepare);
					pageLoads++;
				}
			}
		} finally {
			await context.close();
		}
		process.stderr.write(' done\n');
	}
} finally {
	await browser.close();
}

/**
 * visit loads one page in one cell and scores the assertions that apply to it.
 *
 * @param {import('playwright').Page} page
 * @param {string} url
 * @param {{key: string, skin: string}} cell
 * @param {{assertion: any, url: string, label: string}[]} group
 * @param {string[]} preparations
 */
async function visit(page, url, cell, group, preparations) {
	try {
		await goto(page, `${values.base}${url}`, cell.skin);
		await prepare(page, preparations);
	} catch (err) {
		for (const t of group) {
			results.push({
				assertion: t.assertion,
				url,
				label: t.label,
				cell: cell.key,
				status: 'error',
				detail: `could not reach a measurable state: ${/** @type {Error} */ (err).message}`
			});
		}
		return;
	}
	for (const t of group) {
		const measured = await probe(page, t.assertion.selector);
		const verdict = evaluate(t.assertion, measured);
		results.push({ assertion: t.assertion, url, label: t.label, cell: cell.key, ...verdict });
	}
}

// Whether a `blockedBy` marker has gone stale is decided once, over the whole run —
// and only when the run actually was the whole matrix.
const final = reconcileBlocked(results, { complete: wholeMatrix });
console.log(render(final, { cells: cells.length, pages: pageLoads, elapsedMs: Date.now() - started }));
// `process.exitCode`, not `process.exit()`: stdout to a pipe or a file is asynchronous,
// and exiting outright can truncate a long report mid-line. Setting the code lets node
// drain and exit on its own.
process.exitCode = exitCode(final);
