import { test } from "node:test";
import assert from "node:assert/strict";
import { REVISION_LABEL } from "./lib/imagetools.mjs";
import { COMMENT_MARKER, commitsBehind, freshness, inspectEdge, renderComment } from "./release-candidate-comment.mjs";

const MAIN = "9beaed3aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const OLD = "ca6185baaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const D1 = "sha256:1111";
const D2 = "sha256:2222";

const fresh = { name: "holodex", ref: "ghcr.io/o/holodex", digest: D1, revision: MAIN, behind: 0 };
const stale = { name: "holodex-provider-tmdb", ref: "ghcr.io/o/holodex-provider-tmdb", digest: D2, revision: OLD, behind: 2 };

const render = (over = {}) => renderComment({ images: [fresh], commits: [], mainSha: MAIN, ...over });

test("freshness: matching revision is ok", () => {
  assert.deepEqual(freshness({ revision: MAIN, mainSha: MAIN, behind: 0 }), { ok: true, text: "matches main" });
});

test("freshness: behind main reports the distance and pluralises", () => {
  assert.equal(freshness({ revision: OLD, mainSha: MAIN, behind: 2 }).text, "2 commits behind main");
  assert.equal(freshness({ revision: OLD, mainSha: MAIN, behind: 1 }).text, "1 commit behind main");
});

test("freshness: unknown distance still reads as not ok", () => {
  assert.deepEqual(freshness({ revision: OLD, mainSha: MAIN, behind: null }), { ok: false, text: "does not match main" });
});

test("freshness: a missing image is not ok", () => {
  assert.deepEqual(freshness({ revision: null, mainSha: MAIN }), { ok: false, text: "no image published" });
});

test("renderComment emits the standing boilerplate", () => {
  const body = render();
  assert.ok(body.startsWith(COMMENT_MARKER), "marker drives the upsert, not a duplicate comment");
  assert.match(body, /Advisory only/);
  assert.match(body, /Pin by digest, not `:edge`/);
});

test("renderComment pins the pull command by digest, never by :edge", () => {
  const body = render({ images: [fresh, stale] });
  assert.match(body, /ghcr\.io\/o\/holodex@sha256:1111/);
  assert.match(body, /ghcr\.io\/o\/holodex-provider-tmdb@sha256:2222/);
  assert.doesNotMatch(body, /\S+:edge\n/);
});

test("renderComment warns loudly when an image is not current", () => {
  const body = render({ images: [fresh, stale] });
  assert.match(body, /⚠️ \*\*1 image is not current/);
  assert.match(body, /2 commits behind main/);
});

test("renderComment confirms when every image is current", () => {
  assert.match(render(), /✅ All images were built from current/);
});

test("renderComment lists the covered commits and counts them", () => {
  const body = render({ commits: ["fix(web): a", "feat: b"] });
  assert.match(body, /### Covers 2 commits/);
  assert.match(body, /- fix\(web\): a/);
  assert.match(body, /- feat: b/);
});

test("renderComment handles an empty commit list without claiming coverage", () => {
  assert.match(render(), /### Covers 0 commits/);
  assert.match(render(), /_No commits resolved\._/);
});

test("renderComment degrades when no digest resolved at all", () => {
  const none = { name: "holodex", ref: "ghcr.io/o/holodex", digest: null, revision: null, behind: null };
  const body = render({ images: [none] });
  assert.match(body, /no image published/);
  assert.match(body, /# no digests resolved/);
});

const ok = (stdout) => ({ ok: true, stdout, stderr: "" });
const fail = (stderr) => ({ ok: false, stdout: "", stderr });

test("inspectEdge returns nulls when no edge image is published", async () => {
  assert.deepEqual(await inspectEdge("ghcr.io/o/r", async () => fail("manifest unknown")), {
    digest: null,
    revision: null,
  });
});

// The release path aborts on a registry error; this one must not — nothing is published
// off the comment, and a blip should still produce a comment that reports the problem.
test("inspectEdge degrades instead of throwing when the registry errors", async () => {
  assert.deepEqual(await inspectEdge("ghcr.io/o/r", async () => fail("unauthorized")), {
    digest: null,
    revision: null,
  });
});

test("inspectEdge reports the digest even when the config label is unreadable", async () => {
  const exec = async (_cmd, args) => (args.includes("{{.Manifest.Digest}}") ? ok(D1) : ok("not json"));
  assert.deepEqual(await inspectEdge("ghcr.io/o/r", exec), { digest: D1, revision: null });
});

test("inspectEdge reads the revision when the config is readable", async () => {
  const exec = async (_cmd, args) =>
    args.includes("{{.Manifest.Digest}}")
      ? ok(D1)
      : ok(JSON.stringify({ config: { Labels: { [REVISION_LABEL]: MAIN } } }));
  assert.deepEqual(await inspectEdge("ghcr.io/o/r", exec), { digest: D1, revision: MAIN });
});

test("commitsBehind short-circuits without shelling out when the revision is current", async () => {
  const exec = async () => assert.fail("should not shell out");
  assert.equal(await commitsBehind(MAIN, MAIN, exec), 0);
  assert.equal(await commitsBehind(null, MAIN, exec), 0);
});

test("commitsBehind counts, and reports unknown as null", async () => {
  assert.equal(await commitsBehind(OLD, MAIN, async () => "2"), 2);
  assert.equal(await commitsBehind(OLD, MAIN, async () => null), null);
});
