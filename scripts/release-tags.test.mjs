// Guards which moving tags a release promotion writes (HOLODEX-545). The failure this prevents is
// silent: a hotfix on an old line that also takes `latest` downgrades every operator pulling it.

import { test } from "node:test";
import assert from "node:assert/strict";
import { promotionTags, parseRelease } from "./release-tags.mjs";

test("the newest release takes every moving tag", () => {
  assert.deepEqual(promotionTags("1.16.2", ["v1.16.0", "v1.16.1", "v1.15.0"]), ["1.16.2", "1.16", "1", "latest"]);
});

test("the release's own tag already existing doesn't demote it", () => {
  assert.deepEqual(promotionTags("1.16.2", ["v1.16.1", "v1.16.2"]), ["1.16.2", "1.16", "1", "latest"]);
});

test("a hotfix after a newer major keeps its minor and major but not latest", () => {
  assert.deepEqual(promotionTags("1.16.3", ["v1.16.2", "v2.0.0"]), ["1.16.3", "1.16", "1"]);
});

test("a hotfix on an older minor keeps only its minor", () => {
  assert.deepEqual(promotionTags("1.15.1", ["v1.15.0", "v1.16.2"]), ["1.15.1", "1.15"]);
});

test("a re-run of an older patch takes no moving tag", () => {
  assert.deepEqual(promotionTags("1.16.1", ["v1.16.1", "v1.16.2"]), ["1.16.1"]);
});

test("versions compare numerically, not as strings", () => {
  assert.deepEqual(promotionTags("1.10.0", ["v1.9.9"]), ["1.10.0", "1.10", "1", "latest"]);
});

test("a prerelease gets only its exact version", () => {
  assert.deepEqual(promotionTags("2.0.0-rc.1", ["v1.16.2"]), ["2.0.0-rc.1"]);
});

test("non-release tags are ignored", () => {
  assert.deepEqual(promotionTags("1.16.2", ["v2.0.0-rc.1", "vnext", "v1.16.1"]), ["1.16.2", "1.16", "1", "latest"]);
});

test("parseRelease accepts with or without the v", () => {
  assert.deepEqual(parseRelease("v1.2.3"), [1, 2, 3]);
  assert.deepEqual(parseRelease("1.2.3"), [1, 2, 3]);
  assert.equal(parseRelease("1.2.3-rc.1"), null);
});
