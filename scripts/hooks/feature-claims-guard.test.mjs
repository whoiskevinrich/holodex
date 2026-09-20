import { test } from "node:test";
import assert from "node:assert/strict";
import { contextFor, decide, headingOf, isWriteSpecSkill, specSlugOf } from "./feature-claims-guard.mjs";

// A claim set shaped like collapseClaims() output: F66 owned by two specs (the
// HOLODEX-425 / HOLODEX-421 collision), F65 owned by one, F56 with a sub-feature.
const claims = [
  { num: 66, id: "66", slug: "entity-refresh-sweep", ref: "main", alsoOn: [{ slug: "instance-skin", ref: "origin/HOLODEX-425", count: 1 }], collision: true },
  { num: 65, id: "65", slug: "candidates-image", ref: "main", alsoOn: [], collision: false },
  { num: 56, id: "56", slug: "films-entity", ref: "main", alsoOn: [], collision: false },
  { num: 56, id: "56.2", slug: "unified-name-edit", ref: "main", alsoOn: [], collision: false },
];
const reservations = [{ num: 67, slug: "hotkeys-v2", at: "2026-09-18" }];

test("isWriteSpecSkill matches the skill with or without a plugin prefix, nothing else", () => {
  assert.equal(isWriteSpecSkill({ skill: "product-management:write-spec" }), true);
  assert.equal(isWriteSpecSkill({ skill: "write-spec" }), true);
  assert.equal(isWriteSpecSkill({ skill: "product-management:write-specs" }), false);
  assert.equal(isWriteSpecSkill({ skill: "architecture" }), false);
  assert.equal(isWriteSpecSkill({}), false);
});

test("specSlugOf recognises a spec path with either separator and rejects the rest", () => {
  assert.equal(specSlugOf("docs/specs/instance-skin.md"), "instance-skin");
  assert.equal(specSlugOf("G:\\source\\holodex\\docs\\specs\\instance-skin.md"), "instance-skin");
  assert.equal(specSlugOf("/repo/docs/specs/x.md"), "x");
  assert.equal(specSlugOf("docs/specs/nested/x.md"), null);
  assert.equal(specSlugOf("docs/architecture/ADR-102-x.md"), null);
  assert.equal(specSlugOf("docs/specs/README.txt"), null);
  assert.equal(specSlugOf(undefined), null);
});

test("headingOf finds the H1 in a Write, an Edit's new_string, or any MultiEdit edit", () => {
  assert.equal(headingOf("Write", { content: "# Spec: Foo (F70)\n\n**Status**: Draft" }), "# Spec: Foo (F70)");
  assert.equal(headingOf("Edit", { old_string: "x", new_string: "# Spec: Foo (F70)" }), "# Spec: Foo (F70)");
  assert.equal(headingOf("MultiEdit", { edits: [{ new_string: "body" }, { new_string: "# Spec: Bar (F71)\n" }] }), "# Spec: Bar (F71)");
  assert.equal(headingOf("Edit", { new_string: "## Goals\n- one" }), null, "an H2 is not an H1");
  assert.equal(headingOf("Edit", { new_string: "just prose" }), null);
  assert.equal(headingOf("Write", {}), null);
});

test("decide blocks a new spec that takes a number another spec owns, naming every holder", () => {
  const v = decide({ slug: "brand-new", h1: "# Spec: Brand new (F66)", claims, reservations });
  assert.equal(v.block, true);
  assert.match(v.message, /F66 is already claimed by entity-refresh-sweep @ main; instance-skin @ origin\/HOLODEX-425/);
  assert.match(v.message, /Next free feature number is F68/, "reservation F67 counts as taken");
  assert.match(v.message, /--reserve brand-new/);
});

test("decide lets a spec keep its own number and ignores sub-features of it", () => {
  assert.equal(decide({ slug: "instance-skin", h1: "# Spec: Instance skin (F66)", claims: [claims[0]], reservations: [] }).block, true, "the other holder still rivals it");
  assert.equal(decide({ slug: "candidates-image", h1: "# Spec: Candidate thumbnail (F65)", claims, reservations }).block, false);
  assert.equal(decide({ slug: "films-entity", h1: "# Spec: Films (F56)", claims, reservations }).block, false, "F56.2 is a child, not a rival of F56");
  assert.equal(decide({ slug: "new-child", h1: "# Spec: Films part 3 (F56.3)", claims, reservations }).block, false, "an unclaimed sub-id is free");
});

test("decide blocks a number someone else reserved, and passes unnumbered or QA headings", () => {
  const v = decide({ slug: "other", h1: "# Spec: Other (F67)", claims, reservations });
  assert.equal(v.block, true);
  assert.match(v.message, /hotkeys-v2 \(reserved 2026-09-18\)/);
  assert.equal(decide({ slug: "hotkeys-v2", h1: "# Spec: Hotkeys (F67)", claims, reservations }).block, false, "the reserver may use it");
  assert.equal(decide({ slug: "x", h1: "# Spec: Sticky sort", claims, reservations }).block, false);
  assert.equal(decide({ slug: "qa-x", h1: "# QA: Writeback (F66)", claims, reservations }).block, false);
  assert.equal(decide({ slug: "x", h1: null, claims, reservations }).block, false);
});

test("contextFor names the next free number and the in-flight collisions", () => {
  const ctx = contextFor(claims, reservations);
  assert.match(ctx, /next free number is F68/);
  assert.match(ctx, /# Spec: … \(F68\)/);
  assert.match(ctx, /F66: entity-refresh-sweep @ main vs instance-skin @ origin\/HOLODEX-425/);
  assert.doesNotMatch(contextFor([claims[1]], []), /collisions/, "no collision line when there are none");
});
