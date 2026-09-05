import { test } from "node:test";
import assert from "node:assert/strict";
import {
  CLAIMS_FILENAME,
  STALE_DAYS,
  collapseClaims,
  collisions,
  daysSince,
  describeRivals,
  nextFree,
  parseAdrPath,
  parseReservations,
  pruneReservations,
  rankRef,
  renderFile,
  shortRef,
} from "./adr-claims.mjs";

test("parseAdrPath pulls number and slug out of a path", () => {
  assert.deepEqual(parseAdrPath("docs/architecture/ADR-004-metadata-extraction.md"), {
    num: 4,
    slug: "metadata-extraction",
  });
  assert.deepEqual(parseAdrPath("ADR-090-two-layer-entity-metadata-management.md"), {
    num: 90,
    slug: "two-layer-entity-metadata-management",
  });
});

test("parseAdrPath ignores non-ADR files", () => {
  assert.equal(parseAdrPath("docs/architecture/README.md"), null);
  assert.equal(parseAdrPath("docs/specs/ADR-ish-notes.md"), null);
  assert.equal(parseAdrPath("ADR-090.md"), null); // no slug
});

test("nextFree goes above the high-water mark, not into gaps", () => {
  // A gap is far more likely to be a deleted/renamed ADR than a free slot.
  assert.equal(nextFree([{ num: 1 }, { num: 2 }, { num: 5 }]), 6);
  assert.equal(nextFree([]), 1);
});

test("shortRef strips the refs/ prefixes", () => {
  assert.equal(shortRef("refs/remotes/origin/HOLODEX-194-x"), "origin/HOLODEX-194-x");
  assert.equal(shortRef("refs/heads/main"), "main");
});

test("rankRef puts main above origin branches above local branches", () => {
  assert.ok(rankRef("origin/main") < rankRef("origin/feature"));
  assert.ok(rankRef("origin/feature") < rankRef("feature"));
});

test("collapseClaims keeps the best-ranked ref per number", () => {
  const claims = collapseClaims([
    { num: 88, slug: "provider-alias-collapse", ref: "origin/some-branch" },
    { num: 88, slug: "provider-alias-collapse", ref: "origin/main" },
  ]);
  assert.equal(claims.length, 1);
  assert.equal(claims[0].ref, "origin/main");
});

test("collapseClaims sorts by number", () => {
  const claims = collapseClaims([
    { num: 90, slug: "c", ref: "origin/main" },
    { num: 12, slug: "a", ref: "origin/main" },
    { num: 51, slug: "b", ref: "origin/main" },
  ]);
  assert.deepEqual(
    claims.map((c) => c.num),
    [12, 51, 90],
  );
});

test("collisions flags one number carrying two different slugs", () => {
  // The real PR #257 case: main shipped 088-provider-alias-collapse while a branch
  // carried 088-frontend-component-reuse-discipline.
  const claims = collapseClaims([
    { num: 88, slug: "provider-alias-collapse", ref: "origin/main" },
    { num: 88, slug: "frontend-component-reuse-discipline", ref: "origin/HOLODEX-287" },
  ]);
  const found = collisions(claims);
  assert.equal(found.length, 1);
  assert.equal(found[0].num, 88);
  assert.equal(found[0].ref, "origin/main");
  assert.equal(found[0].alsoOn[0].slug, "frontend-component-reuse-discipline");
});

test("the winning slug on many refs is not itself a rival", () => {
  // Regression: the first cut treated every differing *ref* as a rival, so ADR-059 —
  // one ADR carried by ~40 branches — rendered a 90-line collision line. Only a
  // differing slug counts.
  const rows = [{ num: 59, slug: "provider-brand-icon", ref: "main" }];
  for (let i = 0; i < 40; i++) {
    rows.push({ num: 59, slug: "provider-brand-icon", ref: `claude/branch-${i}` });
  }
  const claims = collapseClaims(rows);
  assert.deepEqual(collisions(claims), []);
  assert.equal(claims[0].ref, "main");
});

test("rivals are grouped by slug with a count, not one entry per ref", () => {
  const rows = [
    { num: 68, slug: "extraction-resolve-entity-materialization", ref: "main" },
    { num: 68, slug: "optional-token-groups", ref: "docs/adr-068-a" },
    { num: 68, slug: "optional-token-groups", ref: "docs/adr-068-b" },
    { num: 68, slug: "optional-token-groups", ref: "docs/adr-068-c" },
  ];
  const claim = collapseClaims(rows)[0];
  assert.equal(claim.alsoOn.length, 1);
  assert.equal(claim.alsoOn[0].slug, "optional-token-groups");
  assert.equal(claim.alsoOn[0].count, 3);
  assert.match(describeRivals(claim), /optional-token-groups @ docs\/adr-068-a \(\+2 more refs\)/);
});

test("collisions ignores the same ADR carried on several branches", () => {
  const claims = collapseClaims([
    { num: 90, slug: "two-layer", ref: "origin/main" },
    { num: 90, slug: "two-layer", ref: "origin/HOLODEX-194" },
  ]);
  assert.deepEqual(collisions(claims), []);
});

test("parseReservations reads RESERVED rows and skips derived ones", () => {
  const text = [
    `# ${CLAIMS_FILENAME}`,
    "088  provider-alias-collapse   origin/main",
    "091  RESERVED  my-thing  2026-09-04",
    "092  RESERVED  other  2026-08-01  (stale, 34d — release it if abandoned)",
  ].join("\n");
  assert.deepEqual(parseReservations(text), [
    { num: 91, slug: "my-thing", at: "2026-09-04" },
    { num: 92, slug: "other", at: "2026-08-01" },
  ]);
});

test("pruneReservations drops a hold once git carries that number", () => {
  const reservations = [
    { num: 91, slug: "landed", at: "2026-09-01" },
    { num: 92, slug: "still-local", at: "2026-09-01" },
  ];
  const claims = [{ num: 91, slug: "landed", ref: "origin/main", alsoOn: [] }];
  assert.deepEqual(pruneReservations(reservations, claims), [
    { num: 92, slug: "still-local", at: "2026-09-01" },
  ]);
});

test("daysSince tolerates a garbage date rather than throwing", () => {
  assert.equal(daysSince("not-a-date"), 0);
  assert.equal(daysSince("2026-09-01", new Date("2026-09-11T00:00:00Z")), 10);
});

test("renderFile reports the next free number and lists claims", () => {
  const claims = collapseClaims([{ num: 88, slug: "provider-alias-collapse", ref: "origin/main" }]);
  const out = renderFile(claims, [], 89, new Date("2026-09-04T00:00:00Z"));
  assert.match(out, /# Next free: 089/);
  assert.match(out, /^088 {2}provider-alias-collapse\s+origin\/main$/m);
});

test("renderFile surfaces a collision banner", () => {
  const claims = collapseClaims([
    { num: 88, slug: "provider-alias-collapse", ref: "origin/main" },
    { num: 88, slug: "frontend-component-reuse-discipline", ref: "origin/HOLODEX-287" },
  ]);
  const out = renderFile(claims, [], 89, new Date("2026-09-04T00:00:00Z"));
  assert.match(out, /!! COLLISIONS/);
  assert.match(out, /frontend-component-reuse-discipline/);
});

test("renderFile marks a reservation stale past the threshold", () => {
  const now = new Date("2026-09-30T00:00:00Z");
  const fresh = renderFile([], [{ num: 91, slug: "a", at: "2026-09-29" }], 92, now);
  assert.doesNotMatch(fresh, /stale/);

  const old = new Date("2026-09-01T00:00:00Z");
  const oldIso = new Date(old.getTime() - STALE_DAYS * 86_400_000).toISOString().slice(0, 10);
  const stale = renderFile([], [{ num: 91, slug: "a", at: oldIso }], 92, old);
  assert.match(stale, /stale, 14d/);
});
