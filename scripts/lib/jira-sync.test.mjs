import { test } from "node:test";
import assert from "node:assert/strict";
import { extractKeys, selectTransitionId } from "./jira-sync.mjs";

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
