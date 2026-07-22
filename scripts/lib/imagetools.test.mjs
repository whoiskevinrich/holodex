import { test } from "node:test";
import assert from "node:assert/strict";
import { REVISION_LABEL, inspectDigest, isAbsent, parseRevision, releaseImages } from "./imagetools.mjs";

const REV = "b".repeat(40);

const singlePlatform = (rev) => JSON.stringify({ config: { Labels: { [REVISION_LABEL]: rev } } });
const multiPlatform = (...revs) =>
  JSON.stringify(
    Object.fromEntries(revs.map((r, i) => [`linux/${i}`, { config: { Labels: { [REVISION_LABEL]: r } } }])),
  );

test("parseRevision reads a single-platform config", () => {
  assert.equal(parseRevision(singlePlatform(REV)), REV);
});

test("parseRevision reads a multi-arch index when platforms agree", () => {
  assert.equal(parseRevision(multiPlatform(REV, REV)), REV);
});

test("parseRevision rejects a multi-arch index whose platforms disagree", () => {
  assert.throws(() => parseRevision(multiPlatform("a".repeat(40), REV)), /disagree/);
});

test("parseRevision rejects a config with no revision label", () => {
  assert.throws(() => parseRevision(JSON.stringify({ config: { Labels: {} } })), /missing the/);
});

test("parseRevision rejects non-JSON and empty input", () => {
  assert.throws(() => parseRevision("not json"), /did not return JSON/);
  assert.throws(() => parseRevision("null"), /no image config/);
});

test("parseRevision lower-cases so label casing can't fake a mismatch", () => {
  assert.equal(parseRevision(singlePlatform(REV.toUpperCase())), REV);
});

test("releaseImages names every image this repo publishes", () => {
  assert.deepEqual(releaseImages("whoiskevinrich/holodex"), [
    { id: "core", name: "holodex", ref: "ghcr.io/whoiskevinrich/holodex" },
    { id: "provider_tmdb", name: "holodex-provider-tmdb", ref: "ghcr.io/whoiskevinrich/holodex-provider-tmdb" },
  ]);
});

test("releaseImages honours a custom registry", () => {
  assert.equal(releaseImages("o/r", "example.test")[0].ref, "example.test/o/r");
});

test("releaseImages requires a repo", () => {
  assert.throws(() => releaseImages(""), /repo is required/);
});

test("isAbsent recognises a genuinely missing tag", () => {
  assert.ok(isAbsent("ghcr.io/o/r:sha-abc1234: manifest unknown"));
  assert.ok(isAbsent("MANIFEST_UNKNOWN: manifest unknown"));
  assert.ok(isAbsent("Error response from daemon: not found"));
});

// Anything that isn't "it isn't there" must NOT read as absent — that's what stops a
// registry blip from being mistaken for "this commit has no image".
test("isAbsent rejects auth and transport failures", () => {
  assert.equal(isAbsent("unauthorized: authentication required"), false);
  assert.equal(isAbsent("denied: permission_denied"), false);
  assert.equal(isAbsent("dial tcp: i/o timeout"), false);
  assert.equal(isAbsent("received unexpected HTTP status: 503 Service Unavailable"), false);
});

test("inspectDigest returns the digest on success and null when absent", async () => {
  assert.equal(await inspectDigest("r", async () => ({ ok: true, stdout: "sha256:abc", stderr: "" })), "sha256:abc");
  assert.equal(await inspectDigest("r", async () => ({ ok: false, stdout: "", stderr: "manifest unknown" })), null);
});

test("inspectDigest throws on a failure that isn't a missing tag", async () => {
  await assert.rejects(
    inspectDigest("r", async () => ({ ok: false, stdout: "", stderr: "unauthorized" })),
    /cannot determine whether r exists/,
  );
});
