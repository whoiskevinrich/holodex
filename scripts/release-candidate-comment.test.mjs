import { test } from "node:test";
import assert from "node:assert/strict";
import { REVISION_LABEL } from "./lib/imagetools.mjs";
import { COMMENT_MARKER, commitsBehind, freshness, inspectEdge, renderComment } from "./release-candidate-comment.mjs";

const MAIN = "9beaed3aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const OLD = "ca6185baaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const D1 = "sha256:1111";
const D2 = "sha256:2222";
const PATHS = ["providers/tmdb", "go.mod"];

const fresh = { name: "holodex", ref: "ghcr.io/o/holodex", digest: D1, revision: MAIN, behind: 0 };
const stale = { name: "holodex-provider-tmdb", ref: "ghcr.io/o/holodex-provider-tmdb", digest: D2, revision: OLD, behind: 2 };

const render = (over = {}) => renderComment({ images: [fresh], commits: [], ...over });

test("freshness: nothing behind is current", () => {
  assert.deepEqual(freshness({ revision: MAIN, behind: 0 }), { ok: true, text: "current" });
});

// The bug this check exists for (HOLODEX-208): each image only rebuilds when its own
// `paths:` match, so an image whose source is untouched is current however far its
// revision trails main HEAD. Measuring against main reported it stale every release.
test("freshness: an image whose source is untouched is current, however old its revision", () => {
  assert.deepEqual(freshness({ revision: OLD, behind: 0 }), { ok: true, text: "current" });
});

// An image can be NEWER than required — a workflow_dispatch rebuild is exactly what a
// warning tells you to do. Comparing against an expected commit called that stale and
// rendered the self-contradicting "0 source commits behind".
test("freshness: an image newer than its latest source is current, not a warning", () => {
  const v = freshness({ revision: "dispatch-build-at-main", behind: 0 });
  assert.deepEqual(v, { ok: true, text: "current" });
});

test("freshness: behind its own source reports the distance and pluralises", () => {
  assert.equal(freshness({ revision: OLD, behind: 2 }).text, "2 source commits behind");
  assert.equal(freshness({ revision: OLD, behind: 1 }).text, "1 source commit behind");
});

test("freshness: a missing image is not ok", () => {
  assert.deepEqual(freshness({ revision: null, behind: 0 }), { ok: false, text: "no image published" });
});

// Never silently pass: unknown means unknown, not fine.
test("freshness: an unknown distance is not ok", () => {
  assert.deepEqual(freshness({ revision: MAIN, behind: null }), {
    ok: false,
    text: "freshness could not be determined",
  });
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
  assert.match(body, /⚠️ \*\*1 image is not built from the latest source/);
  assert.match(body, /2 source commits behind/);
});

// A rarely-touched sidecar is the common case, and warning on it every release is what
// would train the reader to stop reading the comment.
test("renderComment confirms, and shows no warning, when every image is current", () => {
  const body = render({ images: [fresh, { ...stale, behind: 0 }] });
  assert.match(body, /✅ Every image is built from its latest source/);
  assert.doesNotMatch(body, /⚠️/);
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
  assert.equal(await commitsBehind(MAIN, MAIN, PATHS, exec), 0);
  assert.equal(await commitsBehind(null, MAIN, PATHS, exec), 0);
});

test("commitsBehind counts, and reports unknown as null", async () => {
  assert.equal(await commitsBehind(OLD, MAIN, PATHS, async () => "2"), 2);
  assert.equal(await commitsBehind(OLD, MAIN, PATHS, async () => null), null);
});

// An unscoped count measures against all of main — the original bug. Without paths the
// only safe answer is "unknown"; counting anyway would resurrect the false staleness.
test("commitsBehind reports unknown rather than counting unscoped", async () => {
  assert.equal(await commitsBehind(OLD, MAIN, null, async () => assert.fail("should not shell out")), null);
});

test("commitsBehind scopes the count to the image's own paths", async () => {
  let args;
  await commitsBehind(OLD, MAIN, PATHS, async (_cmd, a) => ((args = a), "1"));
  assert.deepEqual(args.slice(args.indexOf("--")), ["--", ...PATHS]);
});
