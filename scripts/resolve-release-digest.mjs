// Resolve the image digests a v* tag should promote, and prove each belongs to that tag
// (ADR-070). Promotion retags an already-built digest instead of rebuilding, so this is
// the only thing standing between "the bits the canary validated" and "an arbitrary image
// published under a release version". Treat every check here as load-bearing.
//
// Strategy: walk the tagged commit's ancestors newest-first and take the first one that
// has a published `sha-<short>` image. Ancestry is therefore structural — a commit that
// isn't reachable from the tag is never even considered. Two cross-checks back that up:
// the image's revision label must name the same commit (so a hand-pushed or mismatched
// `sha-*` tag can't slip through), and `merge-base --is-ancestor` is asserted explicitly.
//
// The walk normally lands one commit back: release-please's version-bump commit touches
// only version.txt / CHANGELOG.md / the manifest, none of which appear in image.yml's
// `paths:` filter (ADR-035), so no image is built for it.

import { pathToFileURL } from "node:url";
import { execDetailed, inspectDigest, inspectImageConfig, parseRevision, releaseImages } from "./lib/imagetools.mjs";

// How far back to look for a built image. One is the expected answer (the version-bump
// commit); more than a handful means the image workflow has been failing for a while.
const MAX_DEPTH = 20;

/** Short form used by docker/metadata-action's `type=sha,prefix=sha-`. */
export function shortSha(sha) {
  if (typeof sha !== "string" || !/^[0-9a-f]{7,40}$/i.test(sha)) {
    throw new Error(`not a commit sha: ${JSON.stringify(sha)}`);
  }
  return sha.slice(0, 7).toLowerCase();
}

/**
 * @returns {Promise<{digest: string, commit: string, tag: string}>}
 * @throws when no ancestor has an image, or when a check fails — never returns a digest
 *         it could not tie back to the tagged commit.
 */
export async function resolveReleaseDigest({ image, taggedCommit, exec = execDetailed }) {
  if (!image) throw new Error("image is required");

  const revList = await exec("git", ["rev-list", `--max-count=${MAX_DEPTH}`, taggedCommit]);
  if (!revList.ok || !revList.stdout) throw new Error(`cannot list ancestors of ${taggedCommit}`);
  const ancestors = revList.stdout.split("\n").map((l) => l.trim()).filter(Boolean);

  for (const commit of ancestors) {
    const tag = `${image}:sha-${shortSha(commit)}`;

    const digest = await inspectDigest(tag, exec);
    if (!digest) continue; // no image for this commit — keep walking back

    const raw = await inspectImageConfig(tag, exec);
    if (!raw) throw new Error(`${tag} exists but its image config could not be read`);

    const revision = parseRevision(raw);
    if (revision !== commit.toLowerCase()) {
      throw new Error(`${tag} claims revision ${revision}, expected ${commit} — refusing to promote`);
    }

    if (!(await exec("git", ["merge-base", "--is-ancestor", commit, taggedCommit])).ok) {
      throw new Error(`${commit} is not an ancestor of ${taggedCommit} — refusing to promote`);
    }

    return { digest: digest.trim(), commit, tag };
  }

  throw new Error(
    `no published sha-* image found in the ${ancestors.length} commits behind ${taggedCommit}`,
  );
}

// CLI: node scripts/resolve-release-digest.mjs <tagged-commit>
// Emits a single `images=<json>` line for $GITHUB_OUTPUT, keyed by image id. Resolving
// every image before the caller retags any is deliberate — a sidecar that can't be
// resolved must not leave a half-published release.
if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  const taggedCommit = process.argv[2] ?? "HEAD";
  const images = releaseImages(process.env.GITHUB_REPOSITORY, process.env.REGISTRY);

  Promise.all(
    images.map(async (img) => ({ ...img, ...(await resolveReleaseDigest({ image: img.ref, taggedCommit })) })),
  )
    .then((resolved) => {
      process.stdout.write(`images=${JSON.stringify(resolved)}\n`);
    })
    .catch((err) => {
      process.stderr.write(`${err.message}\n`);
      process.exit(1);
    });
}
