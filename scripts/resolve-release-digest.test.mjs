import { test } from "node:test";
import assert from "node:assert/strict";
import { REVISION_LABEL } from "./lib/imagetools.mjs";
import { resolveReleaseDigest, shortSha } from "./resolve-release-digest.mjs";

const A = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"; // tagged commit (version bump)
const B = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"; // last real change — has an image
const DIGEST = "sha256:1111111111111111111111111111111111111111111111111111111111111111";

const config = (rev) => JSON.stringify({ config: { Labels: { [REVISION_LABEL]: rev } } });

const ok = (stdout = "") => ({ ok: true, stdout, stderr: "" });
const fail = (stderr) => ({ ok: false, stdout: "", stderr });

/**
 * Fake exec. `images` maps a `sha-<short>` tag suffix to its revision label; anything
 * absent reports the registry's genuine "not there" error.
 */
function fakeExec({ ancestors = [A, B], images = {}, ancestryOk = true } = {}) {
  return async (cmd, args) => {
    if (cmd === "git" && args[0] === "rev-list") {
      return ancestors.length ? ok(ancestors.join("\n")) : fail("unknown revision");
    }
    if (cmd === "git" && args[0] === "merge-base") return ancestryOk ? ok() : fail("not an ancestor");
    if (cmd === "docker") {
      const short = args[3].split(":sha-")[1];
      if (!(short in images)) return fail("manifest unknown");
      return ok(args.includes("{{.Manifest.Digest}}") ? DIGEST : config(images[short]));
    }
    throw new Error(`unexpected call: ${cmd} ${args.join(" ")}`);
  };
}

const resolve = (over = {}) => resolveReleaseDigest({ image: "ghcr.io/o/r", taggedCommit: A, ...over });

test("shortSha truncates and lowercases", () => {
  assert.equal(shortSha("ABCDEF1234567890"), "abcdef1");
});

test("shortSha rejects anything that isn't a sha", () => {
  assert.throws(() => shortSha("main"), /not a commit sha/);
  assert.throws(() => shortSha(""), /not a commit sha/);
  assert.throws(() => shortSha(null), /not a commit sha/);
});

test("walks past the version-bump commit to the newest ancestor that has an image", async () => {
  const got = await resolve({ exec: fakeExec({ images: { [shortSha(B)]: B } }) });
  assert.equal(got.commit, B);
  assert.equal(got.digest, DIGEST);
  assert.equal(got.tag, `ghcr.io/o/r:sha-${shortSha(B)}`);
});

test("promotes the tagged commit itself when it does have an image", async () => {
  const got = await resolve({ exec: fakeExec({ images: { [shortSha(A)]: A, [shortSha(B)]: B } }) });
  assert.equal(got.commit, A);
});

// The negative cases below are the reason this module exists — each is a path by which an
// image that was never canaried could be published under a release version.

test("refuses to promote when the image's revision label names a different commit", async () => {
  await assert.rejects(
    resolve({ exec: fakeExec({ images: { [shortSha(B)]: "c".repeat(40) } }) }),
    /claims revision .* refusing to promote/,
  );
});

test("refuses to promote when the commit is not an ancestor of the tag", async () => {
  await assert.rejects(
    resolve({ exec: fakeExec({ images: { [shortSha(B)]: B }, ancestryOk: false }) }),
    /not an ancestor .* refusing to promote/,
  );
});

test("fails when no ancestor has a published image", async () => {
  await assert.rejects(resolve({ exec: fakeExec({ images: {} }) }), /no published sha-\* image found/);
});

test("fails when the manifest exists but its config cannot be read", async () => {
  const base = fakeExec({ images: { [shortSha(B)]: B } });
  const exec = async (cmd, args) =>
    args.includes("{{json .Image}}") ? { ok: false, stdout: "", stderr: "boom" } : base(cmd, args);
  await assert.rejects(resolve({ exec }), /image config could not be read/);
});

// A registry error is NOT "this commit has no image". Treating it as one would make the
// walk continue and quietly promote an older ancestor under the new version.
test("aborts rather than walking past a commit whose lookup errored", async () => {
  const base = fakeExec({ images: { [shortSha(A)]: A, [shortSha(B)]: B } });
  const exec = async (cmd, args) =>
    cmd === "docker" && args[3].includes(shortSha(A))
      ? { ok: false, stdout: "", stderr: "unauthorized: authentication required" }
      : base(cmd, args);
  await assert.rejects(resolve({ exec }), /cannot determine whether .* exists/);
});

test("fails when the ancestor list cannot be read", async () => {
  await assert.rejects(resolve({ exec: fakeExec({ ancestors: [] }) }), /cannot list ancestors/);
});

test("requires an image", async () => {
  await assert.rejects(resolve({ image: undefined, exec: fakeExec() }), /image is required/);
});
