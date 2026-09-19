import { test } from "node:test";
import assert from "node:assert/strict";
import { nextFree, parseReservations, pruneReservations } from "./adr-claims.mjs";
import {
  CLAIMS_FILENAME,
  collapseClaims,
  collisions,
  parseGrep,
  parseHeading,
  renderFile,
  specSlug,
} from "./feature-claims.mjs";

test("parseHeading pulls the feature id out of a spec H1", () => {
  assert.deepEqual(parseHeading("# Spec: Candidate thumbnail in the resolve picker (F64)"), { num: 64, id: "64" });
  assert.deepEqual(parseHeading("# Unified name-edit mechanism (F56.2, HOLODEX-269)"), { num: 56, id: "56.2" });
  assert.deepEqual(parseHeading("# Spec: Job History — Digest, Pagination (F21.3b)"), { num: 21, id: "21.3b" });
  assert.deepEqual(parseHeading("# Spec: Queryable fields substrate — Phase 1 (F46 Phase 1)"), {
    num: 46,
    id: "46-phase1",
  });
  assert.deepEqual(parseHeading('# Spec: Video Credits — cast/crew (F32, "Populate")'), { num: 32, id: "32" });
});

test("parseHeading ignores QA companions, unnumbered specs and non-H1 lines", () => {
  assert.equal(parseHeading("# QA: Metadata Writeback (F28)"), null);
  assert.equal(parseHeading("# Spec: Sticky sort preferences + Random sort"), null);
  assert.equal(parseHeading("## Related: F64"), null);
  assert.equal(parseHeading("FROM golang:1.22 # F99"), null);
});

test("specSlug is the spec's basename", () => {
  assert.equal(specSlug("docs/specs/candidates-image.md"), "candidates-image");
});

test("parseGrep takes the first '# ' line per file and drops non-claims", () => {
  const out = [
    "abc123:docs/specs/candidates-image.md:# Spec: Candidate thumbnail (F64)",
    "abc123:docs/specs/candidates-image.md:# not the H1 — a code block line",
    "abc123:docs/specs/qa-writeback.md:# QA: Metadata Writeback (F28)",
    "abc123:docs/specs/hotkeys.md:# Spec: Hotkeys (F62)",
    "",
  ].join("\n");
  assert.deepEqual(parseGrep(out, "origin/main"), [
    { num: 64, id: "64", slug: "candidates-image", ref: "origin/main" },
    { num: 62, id: "62", slug: "hotkeys", ref: "origin/main" },
  ]);
});

const row = (id, slug, ref) => ({ num: Number.parseInt(id, 10), id, slug, ref });

test("collapseClaims: the same spec on many branches is one claim, main wins", () => {
  const claims = collapseClaims([
    row("62", "hotkeys", "HOLODEX-405-hotkeys"),
    row("62", "hotkeys", "origin/main"),
    row("62", "hotkeys", "origin/HOLODEX-405-hotkeys"),
  ]);
  assert.equal(claims.length, 1);
  assert.equal(claims[0].ref, "origin/main");
  assert.deepEqual(claims[0].alsoOn, []);
  assert.equal(claims[0].collision, false);
});

test("collapseClaims: an in-flight spec taking a number main already has is a collision", () => {
  // The HOLODEX-390 / HOLODEX-406 case: both took F63, 390 merged first.
  const claims = collapseClaims([
    row("63", "provider-link-badge-coverage", "origin/main"),
    row("63", "candidates-image", "origin/HOLODEX-406-picker-candidate-thumb"),
    row("63", "candidates-image", "HOLODEX-406-picker-candidate-thumb"),
  ]);
  assert.equal(claims[0].slug, "provider-link-badge-coverage");
  assert.deepEqual(claims[0].alsoOn, [
    { slug: "candidates-image", ref: "origin/HOLODEX-406-picker-candidate-thumb", count: 2 },
  ]);
  assert.equal(collisions(claims).length, 1);
});

test("collapseClaims: two in-flight specs on different branches taking the same number collide", () => {
  const claims = collapseClaims([
    row("65", "spec-a", "origin/HOLODEX-1-a"),
    row("65", "spec-b", "origin/HOLODEX-2-b"),
  ]);
  assert.equal(collisions(claims).length, 1);
});

test("collapseClaims: two specs sharing a number on main is a fact, not a collision", () => {
  // F55 and F56 both already ship this way; retro-renumbering is out of scope.
  const claims = collapseClaims([
    row("55", "entity-completeness-score", "origin/main"),
    row("55", "people-poster-view", "origin/main"),
    row("55", "people-poster-view", "origin/HOLODEX-9-carrying-it"),
  ]);
  assert.equal(claims[0].alsoOn.length, 1);
  assert.equal(claims[0].collision, false);
  assert.equal(collisions(claims).length, 0);
});

test("collapseClaims: sub-features and phases are children, not rivals, of the parent", () => {
  const claims = collapseClaims([
    row("56", "two-tier-field-editing", "origin/main"),
    row("56.2", "unified-name-edit", "origin/main"),
    row("56.5", "people-relationship-picker", "origin/HOLODEX-8-in-flight"),
    row("46-phase1", "queryable-fields-substrate", "origin/main"),
    row("46-phase3", "cast-billing-role-queries", "origin/HOLODEX-180-cast-billing-spec"),
  ]);
  assert.deepEqual(
    claims.map((c) => c.id),
    ["46-phase1", "46-phase3", "56", "56.2", "56.5"],
  );
  assert.equal(collisions(claims).length, 0);
  assert.equal(nextFree(claims), 57);
});

test("reservations round-trip through the shared parser with the F prefix", () => {
  const claims = collapseClaims([row("64", "candidates-image", "origin/main")]);
  const text = renderFile(claims, [{ num: 65, slug: "writeback-cockpit", at: "2026-09-17" }], 66, new Date("2026-09-18"));
  assert.match(text, /^# Next free: F66$/m);
  assert.match(text, /^F64 {9}candidates-image/m);
  assert.match(text, /^F65 {9}RESERVED {2}writeback-cockpit {2}2026-09-17$/m);
  assert.deepEqual(parseReservations(text), [{ num: 65, slug: "writeback-cockpit", at: "2026-09-17" }]);
  // Once F65 shows up in git the hold is dropped.
  assert.deepEqual(pruneReservations(parseReservations(text), [{ num: 65 }]), []);
});

test("renderFile flags collisions and lists shared-on-main numbers quietly", () => {
  const claims = collapseClaims([
    row("55", "entity-completeness-score", "origin/main"),
    row("55", "people-poster-view", "origin/main"),
    row("63", "provider-link-badge-coverage", "origin/main"),
    row("63", "candidates-image", "origin/HOLODEX-406-picker-candidate-thumb"),
  ]);
  const text = renderFile(claims, [], 64, new Date("2026-09-18"));
  assert.match(text, /^# !! COLLISIONS/m);
  assert.match(text, /^# {4}F63 {2}provider-link-badge-coverage @ origin\/main {2}VS {2}candidates-image @ origin\/HOLODEX-406/m);
  assert.match(text, /^ {12}shared with people-poster-view$/m);
  assert.doesNotMatch(text, /F55.*VS/);
  assert.ok(text.startsWith(`# ${CLAIMS_FILENAME}`));
});
