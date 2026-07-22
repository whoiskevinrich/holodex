// Post/refresh the release-candidate comment on the open release-please PR (ADR-070).
//
// `:edge` already IS the content of that PR — release-please stacks the version bump and
// CHANGELOG, not the code — so this doesn't build anything. It resolves the digests that
// are already published, says whether they're actually current, and hands over a pinned
// pull command for the canary instance.
//
// Deliberately ADVISORY: it never sets a status check and never blocks the merge. A stale
// image is reported in the comment, not enforced. That means the comment has to stay worth
// reading — keep it short, and keep the freshness verdict at the top.

import { pathToFileURL } from "node:url";
import { execDetailed, inspectDigest, inspectImageConfig, parseRevision, releaseImages, run, runStrict } from "./lib/imagetools.mjs";

export const COMMENT_MARKER = "<!-- holodex-release-candidate -->";
const RELEASE_PR_LABEL = "autorelease: pending";

/** Freshness verdict for one image against main HEAD. `behind` is null when unknown. */
export function freshness({ revision, mainSha, behind }) {
  if (!revision) return { ok: false, text: "no image published" };
  if (revision === mainSha) return { ok: true, text: "matches main" };
  if (behind == null) return { ok: false, text: "does not match main" };
  return { ok: false, text: `${behind} commit${behind === 1 ? "" : "s"} behind main` };
}

/**
 * Pure renderer — the whole comment body from already-gathered facts.
 * @param {{name: string, ref: string, digest: string|null, revision: string|null, behind: number|null}[]} images
 */
export function renderComment({ images, commits, mainSha, version }) {
  const verdicts = images.map((img) => ({ img, f: freshness({ ...img, mainSha }) }));
  const stale = verdicts.filter(({ f }) => !f.ok);

  const rows = verdicts.map(({ img, f }) => {
    const digest = img.digest ? `\`${img.digest}\`` : "—";
    const from = img.revision ? `\`${img.revision.slice(0, 7)}\`` : "—";
    return `| \`${img.name}\` | ${digest} | ${from} | ${f.ok ? "✅" : "⚠️"} ${f.text} |`;
  });

  const pull = images.filter((img) => img.digest).map((img) => `${img.ref}@${img.digest}`).join("\n");

  return [
    COMMENT_MARKER,
    `## Release candidate${version ? ` — ${version}` : ""}`,
    "",
    "`:edge` is the content of this PR. Canary it before merging.",
    "",
    "| Image | Digest | Built from | Freshness |",
    "|---|---|---|---|",
    ...rows,
    "",
    stale.length
      ? `> ⚠️ **${stale.length} image${stale.length === 1 ? " is" : "s are"} not current with \`main\`.** ` +
        "Canarying this digest validates something other than what this PR will release. " +
        "Re-run the image workflow before relying on it."
      : "> ✅ All images were built from current `main`.",
    "",
    "### Pull onto the canary",
    "",
    "```",
    pull || "# no digests resolved",
    "```",
    "",
    "Pin by digest, not `:edge` — rolling back is re-running the digest from the previous comment.",
    "",
    `### Covers ${commits.length} commit${commits.length === 1 ? "" : "s"}`,
    "",
    commits.length ? commits.map((c) => `- ${c}`).join("\n") : "_No commits resolved._",
    "",
    "---",
    "<sub>Advisory only — this does not gate the merge (ADR-070).</sub>",
  ].join("\n");
}

/**
 * Resolve the published `:edge` digest + revision. Every failure degrades to a freshness
 * warning rather than throwing — unlike the release path, nothing is published off this,
 * and a registry blip should still produce a comment that says so.
 */
export async function inspectEdge(ref, exec = execDetailed) {
  let digest = null;
  try {
    digest = await inspectDigest(`${ref}:edge`, exec);
  } catch {
    return { digest: null, revision: null };
  }
  if (!digest) return { digest: null, revision: null };

  const raw = await inspectImageConfig(`${ref}:edge`, exec);
  try {
    return { digest, revision: raw ? parseRevision(raw) : null };
  } catch {
    return { digest, revision: null };
  }
}

export async function commitsBehind(revision, mainSha, exec = run) {
  if (!revision || revision === mainSha) return 0;
  const out = await exec("git", ["rev-list", "--count", `${revision}..${mainSha}`]);
  return out === null ? null : Number(out);
}

async function main() {
  const repo = process.env.GITHUB_REPOSITORY;
  const mainSha = (process.env.MAIN_SHA ?? "").toLowerCase();
  if (!repo || !mainSha) throw new Error("GITHUB_REPOSITORY and MAIN_SHA are required");

  const prJson = await run("gh", ["pr", "list", "--state", "open", "--label", RELEASE_PR_LABEL, "--json", "number,title", "--limit", "1"]);
  const prs = prJson ? JSON.parse(prJson) : [];
  if (prs.length === 0) {
    process.stdout.write("No open release PR — nothing to comment on.\n");
    return;
  }
  const pr = prs[0];

  // Both images concurrently: each is two registry round-trips, and they share nothing.
  const images = await Promise.all(
    releaseImages(repo, process.env.REGISTRY).map(async (img) => {
      const { digest, revision } = await inspectEdge(img.ref);
      return { ...img, digest, revision, behind: await commitsBehind(revision, mainSha) };
    }),
  );

  const lastTag = await run("git", ["describe", "--tags", "--abbrev=0", "--match", "v*"]);
  const log = await run("git", ["log", "--no-merges", "--format=%s", lastTag ? `${lastTag}..${mainSha}` : mainSha]);
  const commits = log ? log.split("\n").filter(Boolean) : [];

  const body = renderComment({ images, commits, mainSha, version: pr.title?.match(/\d+\.\d+\.\d+/)?.[0] });

  // Upsert on our marker rather than accumulating a comment per image build.
  const existing = await run("gh", ["api", `repos/${repo}/issues/${pr.number}/comments`, "--jq", `.[] | select(.body | contains("${COMMENT_MARKER}")) | .id`]);
  const id = existing ? existing.split("\n")[0] : null;

  const [method, path] = id
    ? ["PATCH", `repos/${repo}/issues/comments/${id}`]
    : ["POST", `repos/${repo}/issues/${pr.number}/comments`];
  await runStrict("gh", ["api", "--method", method, path, "-f", `body=${body}`]);

  process.stdout.write(`${id ? `Updated comment ${id}` : "Created comment"} on PR #${pr.number}\n`);
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  main().catch((err) => {
    process.stderr.write(`${err.message}\n`);
    process.exit(1);
  });
}
