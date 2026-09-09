// Reading the stress fixture's manifest back (HOLODEX-349).
//
// `testdata/stressseed` writes `data/stress/manifest.json` on every run. That file
// is the whole reason this harness can assert anything durable: it turns "media 902"
// into a coordinate — people=2, tags=3, namespaces=5 — so an assertion can be written
// against the *property* that makes a page interesting rather than against its id.
// See spec D4 and D6 (`docs/specs/stress-fixture.md`).
//
// Everything here is pure so it can be unit-tested without a browser or a server.

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));

/** Where the seeder puts the manifest, relative to this file. */
export const DEFAULT_MANIFEST = resolve(here, '../../data/stress/manifest.json');

/**
 * @typedef {object} Entry One addressed fixture entity, as the seeder wrote it.
 * @property {number} id
 * @property {'video'|'film'|'person'|'studio'|'tag'} entity
 * @property {string} dimension
 * @property {string} variant
 * @property {unknown} value
 * @property {string} name
 * @property {string} url
 * @property {object} axes
 */

/**
 * load reads and shallow-validates the manifest.
 *
 * The validation is not defensive noise: a stale or half-written manifest produces
 * assertions that select nothing, and an assertion that selects nothing silently
 * passes. Failing here — loudly, once — is the difference between "green" and
 * "green because it measured nothing".
 *
 * @param {string} [path]
 * @returns {{generated_at: string, seed: number, count: number, pool_base: number,
 *            dimensions: {key: string, entity: string, block: number, finds: string, rungs: string[]}[],
 *            entities: Record<string, Entry>, by_dimension: Record<string, number[]>,
 *            breadth?: object}}
 */
export function load(path = DEFAULT_MANIFEST) {
	let raw;
	try {
		raw = readFileSync(path, 'utf8');
	} catch (err) {
		throw new Error(
			`no fixture manifest at ${path}\n` +
				`  Seed one first:  go run ./testdata/stressseed\n` +
				`  (cause: ${/** @type {Error} */ (err).message})`
		);
	}
	const m = JSON.parse(raw);
	if (!m || typeof m !== 'object') throw new Error(`${path}: not a JSON object`);
	for (const key of ['entities', 'by_dimension', 'dimensions']) {
		if (!m[key]) throw new Error(`${path}: missing "${key}" — regenerate with 'go run ./testdata/stressseed'`);
	}
	if (Object.keys(m.entities).length === 0) {
		throw new Error(`${path}: manifest has no entities — the seed did not complete`);
	}
	return m;
}

/**
 * entries returns every addressed entity, ordered by id so a run's output is stable
 * between invocations. Map iteration order would already be insertion order, but the
 * seeder's insertion order is a function of the ladder table, and sorting means adding
 * a dimension does not reshuffle an unrelated part of the report.
 *
 * @param {ReturnType<typeof load>} m
 * @returns {Entry[]}
 */
export function entries(m) {
	return Object.values(m.entities).sort((a, b) => a.id - b.id);
}

/**
 * select resolves an assertion's `when` predicate to the pages it applies to.
 *
 * This is the AC's "assertions generalise by dimension, not by single page". The
 * predicate reads the entity's *axes*, so "every page with ten or more people" picks
 * up the `people` dimension's 10/25/50 rungs today and picks up an 80 rung the day
 * somebody adds one — without the assertion being touched.
 *
 * @param {ReturnType<typeof load>} m
 * @param {(e: Entry) => boolean} when
 * @returns {Entry[]}
 */
export function select(m, when) {
	return entries(m).filter((e) => {
		try {
			return Boolean(when(e));
		} catch {
			// A predicate that reaches into the wrong half of `axes` — e.axes.video.people
			// on a film — throws rather than returning false. Treat it as "does not apply":
			// axes is deliberately kind-keyed (only one half is populated) precisely so a
			// predicate can be written for one kind without enumerating the others.
			return false;
		}
	});
}

/**
 * describe renders an entry as the one-line coordinate a failure message quotes. The
 * seeded `name` already encodes the full coordinate, but it is long; this is the short
 * form — dimension, rung, and the address to open.
 *
 * @param {Entry} e
 */
export function describe(e) {
	return `${e.dimension}/${e.variant} (${e.entity} ${e.id}, ${e.url})`;
}
