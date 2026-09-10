// The stub's one contract obligation, guarded (HOLODEX-355 review).
//
// `testdata/enrich-stub/stub.js` is what docs/specs/metadata-provider-contract.md points
// third-party providers at as its worked example, so a violation here is copied outward
// rather than merely being wrong locally. The obligation is small and entirely mechanical:
//
//   §4.1  "candidates[].namespace — the prefix before the first `:`"
//   §4.1  "You MUST emit ids only in namespaces you advertise in /describe.id_namespaces"
//
// It was violated once already — every candidate declared `namespace: 'tmdb'` while
// emitting `alpha:608`/`flood:100`/`twins:200` — because the namespace was written by hand
// beside the id instead of derived from it. The code now derives both halves, so this test
// is what keeps the derivation from being replaced by a hand-written copy again.
//
// Requires the stub rather than starting it: stub.js only calls listen() when run
// directly, so this costs no port and no teardown.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const stub = require('./stub.js');

/** Every persona the stub serves: the stress table plus the legacy one on the bare routes. */
const everyPersona = [stub.LEGACY, ...stub.PERSONAS];

test('every candidate namespace is its own id prefix', () => {
	for (const persona of everyPersona) {
		const candidates = stub.candidatesFor(persona, 'hayao');
		assert.ok(candidates.length > 0, `${persona.name}: returned no candidates to check`);
		for (const c of candidates) {
			assert.equal(
				c.namespace,
				c.external_id.split(':')[0],
				`${persona.name}: namespace ${JSON.stringify(c.namespace)} is not the prefix of ` +
					`${JSON.stringify(c.external_id)} — §4.1 defines it as the prefix, and ADR-082 ` +
					`stores the pair as one "<namespace>:<id>" string the UI splits back apart`
			);
		}
	}
});

test('/describe advertises every namespace the persona actually emits', () => {
	for (const persona of everyPersona) {
		const advertised = stub.describeFor(persona, 'http://127.0.0.1:9100').id_namespaces;
		const emitted = new Set(stub.candidatesFor(persona, 'hayao').map((c) => c.namespace));
		for (const ns of emitted) {
			assert.ok(
				advertised.includes(ns),
				`${persona.name}: emits ids in ${JSON.stringify(ns)} but /describe advertises ` +
					`${JSON.stringify(advertised)} — §4.1: "You MUST emit ids only in namespaces you advertise"`
			);
		}
	}
});

// The stress personas answer to their own name because that is the id the seeder writes
// into entity_enrichment (testdata/stressseed/enrichment.go). If this drifts, a Refresh
// against the running stub stops re-fetching the record the fixture booted with.
test('a stress persona issues ids under its own provider name', () => {
	for (const persona of stub.PERSONAS) {
		if (persona.candidates) continue; // flood/twins deliberately use their own prefixes
		assert.equal(
			stub.idNamespaceFor(persona),
			persona.name,
			`${persona.name}: the seeder records "<name>:608" as the match id, so the stub has to answer to it`
		);
	}
});

test('namespaceOf refuses an id with no colon rather than truncating it', () => {
	// The bug this replaced: `slice(0, indexOf(':'))` returns the id minus its last
	// character when there is no colon, which is a near-miss of the real namespace and
	// far harder to notice than a throw.
	assert.throws(() => stub.namespaceOf('nocolon'), /has no ":"/);
	assert.equal(stub.namespaceOf('a:b:c'), 'a', 'splits on the FIRST colon');
});
