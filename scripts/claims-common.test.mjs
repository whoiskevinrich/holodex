import { test } from "node:test";
import assert from "node:assert/strict";
import {
  daysSince,
  describeRivals,
  nextFree,
  parseReservations,
  pruneReservations,
  rankRef,
  shortRef,
} from "./claims-common.mjs";

test("nextFree goes above the high-water mark, not into gaps", () => {
  // A gap is far more likely to be a deleted/renamed claim than a free slot.
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

test("describeRivals groups a rival carried on many refs into one entry with a count", () => {
  const claim = { alsoOn: [{ slug: "optional-token-groups", ref: "docs/a", count: 3 }, { slug: "x", ref: "main", count: 1 }] };
  assert.equal(describeRivals(claim), "optional-token-groups @ docs/a (+2 more refs); x @ main");
});

test("parseReservations reads RESERVED rows, with or without an F prefix, and skips the rest", () => {
  const text = [
    "# .feature-claims",
    "F66  entity-refresh-sweep   origin/main",
    "F67  RESERVED  hotkeys-v2  2026-09-18",
    "91  RESERVED  other  2026-08-01  (stale, 34d — release it if abandoned)",
  ].join("\n");
  assert.deepEqual(parseReservations(text), [
    { num: 67, slug: "hotkeys-v2", at: "2026-09-18" },
    { num: 91, slug: "other", at: "2026-08-01" },
  ]);
});

test("pruneReservations drops a hold once git carries that number", () => {
  const reservations = [
    { num: 91, slug: "landed", at: "2026-09-01" },
    { num: 92, slug: "still-local", at: "2026-09-01" },
  ];
  const claims = [{ num: 91, slug: "landed", ref: "origin/main", alsoOn: [] }];
  assert.deepEqual(pruneReservations(reservations, claims), [{ num: 92, slug: "still-local", at: "2026-09-01" }]);
});

test("daysSince tolerates a garbage date rather than throwing", () => {
  assert.equal(daysSince("not-a-date"), 0);
  assert.equal(daysSince("2026-09-01", new Date("2026-09-11T00:00:00Z")), 10);
});
