// Shared, dependency-free helpers for reading the container images this repo publishes
// (ADR-070). Two entry points build on this: resolve-release-digest.mjs (what a v* tag
// promotes) and release-candidate-comment.mjs (what the canary should pull).
//
// The set of published images lives here rather than being re-listed per call site —
// release.yml, the candidate comment, and the release body all need the same answer.
// The image *build* workflows stay hand-written per image: their `paths:` filters can't
// be templated in GitHub Actions, so this file names each image's workflow and reads
// those filters back out rather than restating them (see readTriggerPaths).

import { execFile } from "node:child_process";
import { readFileSync } from "node:fs";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export const REVISION_LABEL = "org.opencontainers.image.revision";

/** The images published from this repo. `id` is used to key workflow outputs. */
export const IMAGES = [
  { id: "core", suffix: "", workflow: ".github/workflows/image.yml" },
  { id: "provider_tmdb", suffix: "-provider-tmdb", workflow: ".github/workflows/provider-tmdb.yml" },
];

/** @returns {{id: string, name: string, ref: string, workflow: string}[]} */
export function releaseImages(repo, registry = "ghcr.io") {
  if (!repo) throw new Error("repo is required");
  return IMAGES.map(({ id, suffix, workflow }) => ({
    id,
    name: `${repo.split("/")[1]}${suffix}`,
    ref: `${registry}/${repo}${suffix}`,
    workflow,
  }));
}

/**
 * Read a build workflow's `push.paths:` filter as git pathspecs — the only correct answer
 * to "which commits could have changed this image" (ADR-070). Read rather than restated so
 * a path added to a workflow can't silently go unwatched.
 *
 * Deliberately a narrow reader, not a YAML parser: the block is a flat list of scalars and
 * these scripts are dependency-free. Throws rather than returning [], because an empty
 * pathspec means "every commit" to git and would invert the check into always-stale.
 */
export function parseTriggerPaths(yaml) {
  const lines = yaml.split(/\r?\n/);
  const start = lines.findIndex((l) => /^\s*paths:\s*$/.test(l));
  if (start === -1) throw new Error("no `paths:` block found");

  const indent = lines[start].search(/\S/);
  const paths = [];
  for (const line of lines.slice(start + 1)) {
    if (!line.trim() || /^\s*#/.test(line)) continue;
    const item = line.match(/^(\s*)-\s*(.+?)\s*$/);
    if (!item || item[1].length <= indent) break;
    paths.push(item[2].replace(/^['"]|['"]$/g, ""));
  }
  if (paths.length === 0) throw new Error("`paths:` block is empty");

  // `foo/**` -> `foo`: git already treats a directory pathspec as everything beneath it,
  // and the bare form avoids depending on how git expands `**` outside :(glob) magic.
  return paths.map((p) => p.replace(/\/\*\*$/, ""));
}

/** @see parseTriggerPaths — same contract, reading the workflow from disk. */
export function readTriggerPaths(workflowPath) {
  try {
    return parseTriggerPaths(readFileSync(workflowPath, "utf8"));
  } catch (err) {
    throw new Error(`cannot read trigger paths from ${workflowPath}: ${err.message}`);
  }
}

/**
 * Default runner. Never throws — returns the outcome so callers can tell "the command
 * said no" apart from "the command broke". That distinction is load-bearing in
 * inspectDigest: collapsing them lets a transient registry error read as "no image for
 * this commit", which would make the release resolver walk silently past the right
 * digest and promote an older one.
 * @returns {Promise<{ok: boolean, stdout: string, stderr: string}>}
 */
export async function execDetailed(cmd, args) {
  try {
    const { stdout } = await execFileAsync(cmd, args, { maxBuffer: 16 * 1024 * 1024 });
    return { ok: true, stdout: stdout.trim(), stderr: "" };
  } catch (err) {
    return { ok: false, stdout: (err.stdout ?? "").trim(), stderr: (err.stderr ?? String(err)).trim() };
  }
}

/** Convenience for reads where any failure is genuinely "no answer". */
export async function run(cmd, args) {
  const { ok, stdout } = await execDetailed(cmd, args);
  return ok ? stdout : null;
}

/** Throws on failure — use when the command must succeed (writes, not reads). */
export async function runStrict(cmd, args) {
  const { stdout } = await execFileAsync(cmd, args, { maxBuffer: 16 * 1024 * 1024 });
  return stdout.trim();
}

/**
 * Pull the revision label out of `imagetools inspect --format '{{json .Image}}'`.
 * A single-platform image yields one config object; a multi-arch index yields a map of
 * platform → config. Every platform present must agree, so a partially-rebuilt index
 * can't be promoted on the strength of one good arch.
 */
export function parseRevision(raw) {
  let doc;
  try {
    doc = typeof raw === "string" ? JSON.parse(raw) : raw;
  } catch {
    throw new Error("imagetools inspect did not return JSON");
  }
  if (!doc || typeof doc !== "object") throw new Error("imagetools inspect returned no image config");

  const configs = doc.config ? [doc] : Object.values(doc).filter((v) => v && typeof v === "object");
  if (configs.length === 0) throw new Error("imagetools inspect returned no image config");

  const revisions = new Set();
  for (const cfg of configs) {
    const rev = cfg?.config?.Labels?.[REVISION_LABEL];
    if (!rev) throw new Error(`image config is missing the ${REVISION_LABEL} label`);
    revisions.add(rev.toLowerCase());
  }
  if (revisions.size > 1) {
    throw new Error(`image platforms disagree on ${REVISION_LABEL}: ${[...revisions].join(", ")}`);
  }
  return [...revisions][0];
}

// The digest and the config are read in two calls on purpose. A miss on the first means
// "no image for this tag"; a miss on the second means "the tag exists but its config is
// unreadable". resolve-release-digest.mjs treats those differently — collapsing them into
// one `{{json .}}` call would make an unreadable config look like an absent tag and let
// the release walk quietly past the check the resolver exists for.

/** How a registry reports a tag that simply isn't there, as opposed to a failure. */
export function isAbsent(stderr) {
  return /manifest[ _]unknown|not found|404/i.test(stderr);
}

/**
 * @returns {Promise<string|null>} the manifest digest, or null when the tag is genuinely absent.
 * @throws on any other failure — an auth error or registry blip must not be mistaken for
 *         "this commit has no image", which is what makes the caller keep walking.
 */
export async function inspectDigest(ref, exec = execDetailed) {
  const { ok, stdout, stderr } = await exec("docker", ["buildx", "imagetools", "inspect", ref, "--format", "{{.Manifest.Digest}}"]);
  if (ok) return stdout;
  if (isAbsent(stderr)) return null;
  throw new Error(`cannot determine whether ${ref} exists: ${stderr}`);
}

/** @returns {Promise<string|null>} raw `{{json .Image}}`, or null when it can't be read. */
export async function inspectImageConfig(ref, exec = execDetailed) {
  const { ok, stdout } = await exec("docker", ["buildx", "imagetools", "inspect", ref, "--format", "{{json .Image}}"]);
  return ok ? stdout : null;
}
