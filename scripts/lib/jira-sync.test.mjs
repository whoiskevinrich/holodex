import { test } from "node:test";
import assert from "node:assert/strict";
import { extractKeys, selectTransitionId, syncKeys } from "./jira-sync.mjs";

test("extractKeys pulls the key from a branch name", () => {
  assert.deepEqual(extractKeys("HOLODEX-132-jira-transitions-rest-api"), ["HOLODEX-132"]);
});

test("extractKeys dedups and upper-cases", () => {
  assert.deepEqual(extractKeys("fix holodex-4, HOLODEX-4 and HOLODEX-56"), [
    "HOLODEX-4",
    "HOLODEX-56",
  ]);
});

test("extractKeys returns [] when no key is present", () => {
  assert.deepEqual(extractKeys("chore: tidy up"), []);
});

test("extractKeys tolerates null/undefined", () => {
  assert.deepEqual(extractKeys(null), []);
  assert.deepEqual(extractKeys(undefined), []);
});

test("extractKeys honours a custom prefix and ignores others", () => {
  assert.deepEqual(extractKeys("HOLODEX-128 and BOOKSHELF-1", "BOOKSHELF"), ["BOOKSHELF-1"]);
  assert.deepEqual(extractKeys("BOOKSHELF-1"), []); // default prefix is HOLODEX
});

test("selectTransitionId matches on destination status, case-insensitive", () => {
  const transitions = [
    { id: "11", to: { name: "To Do" } },
    { id: "31", to: { name: "In Review" } },
  ];
  assert.equal(selectTransitionId(transitions, "in review"), "31");
});

test("selectTransitionId returns null when no transition reaches the target", () => {
  assert.equal(selectTransitionId([{ id: "11", to: { name: "To Do" } }], "Done"), null);
});

test("selectTransitionId tolerates missing/empty input", () => {
  assert.equal(selectTransitionId(undefined, "Done"), null);
  assert.equal(selectTransitionId([], "Done"), null);
});

// HOLODEX-185: an Epic must never be auto-transitioned (e.g. Done -> Released
// on a tagged release) — its status is a manual/reviewed step, since CI has no
// way to confirm all child issues/gates are actually satisfied.
test("syncKeys skips an Epic without transitioning it", async () => {
  const calls = [];
  const client = {
    currentStatus: async (key) => ({ status: "Done", issueType: "Epic" }),
    findTransitionId: async () => {
      calls.push("findTransitionId");
      return "2";
    },
    transition: async () => calls.push("transition"),
  };
  const log = { info: () => {}, warn: () => {} };

  const failures = await syncKeys({
    keys: ["HOLODEX-182"],
    targetStatus: "Released",
    client,
    log,
  });

  assert.equal(failures, 0);
  assert.deepEqual(calls, []);
});

test("syncKeys still transitions a non-Epic issue", async () => {
  const calls = [];
  const client = {
    currentStatus: async () => ({ status: "Done", issueType: "Story" }),
    findTransitionId: async () => "2",
    transition: async (key, id) => calls.push([key, id]),
  };
  const log = { info: () => {}, warn: () => {} };

  const failures = await syncKeys({
    keys: ["HOLODEX-183"],
    targetStatus: "Released",
    client,
    log,
  });

  assert.equal(failures, 0);
  assert.deepEqual(calls, [["HOLODEX-183", "2"]]);
});

// HOLODEX-173/220: a standalone gate-artifact PR (spec/ADR/design-handoff/worklog)
// merging non-draft fired a premature Done, which then cascaded to Released on the
// next deploy. docsOnly=true guards Done specifically for a PR that touched only docs/**.
test("syncKeys skips Done for a docs-only PR", async () => {
  const calls = [];
  const client = {
    currentStatus: async () => ({ status: "In Progress", issueType: "Story" }),
    findTransitionId: async () => {
      calls.push("findTransitionId");
      return "41";
    },
    transition: async () => calls.push("transition"),
  };
  const log = { info: () => {}, warn: () => {} };

  const failures = await syncKeys({
    keys: ["HOLODEX-173"],
    targetStatus: "Done",
    client,
    log,
    docsOnly: true,
  });

  assert.equal(failures, 0);
  assert.deepEqual(calls, []);
});

test("syncKeys does not guard In Review for a docs-only PR", async () => {
  const calls = [];
  const client = {
    currentStatus: async () => ({ status: "In Progress", issueType: "Story" }),
    findTransitionId: async () => "31",
    transition: async (key, id) => calls.push([key, id]),
  };
  const log = { info: () => {}, warn: () => {} };

  const failures = await syncKeys({
    keys: ["HOLODEX-173"],
    targetStatus: "In Review",
    client,
    log,
    docsOnly: true,
  });

  assert.equal(failures, 0);
  assert.deepEqual(calls, [["HOLODEX-173", "31"]]);
});

test("syncKeys still transitions to Done when docsOnly is false", async () => {
  const calls = [];
  const client = {
    currentStatus: async () => ({ status: "In Review", issueType: "Story" }),
    findTransitionId: async () => "41",
    transition: async (key, id) => calls.push([key, id]),
  };
  const log = { info: () => {}, warn: () => {} };

  const failures = await syncKeys({
    keys: ["HOLODEX-186"],
    targetStatus: "Done",
    client,
    log,
    docsOnly: false,
  });

  assert.equal(failures, 0);
  assert.deepEqual(calls, [["HOLODEX-186", "41"]]);
});
