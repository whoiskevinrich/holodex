#!/usr/bin/env node
// Which tags a release promotion may write (HOLODEX-545, docs/architecture/deployment.md).
//
// A release always gets its exact version. The moving tags — `X.Y`, `X` and `latest` — go only to
// the highest release in their range, so a hotfix cut from `release/v1.16` after 2.0.0 shipped
// moves `1.16` and `1` but leaves `latest` on 2.x. A prerelease never gets a moving tag, matching
// docker/metadata-action's `latest=auto` behaviour this replaces.
//
// Usage: node scripts/release-tags.mjs 1.16.2   → prints the tags, space-separated
// Reads existing `v*` git tags, so the checkout needs them (fetch-depth: 0).
//
// Dependency-free. The pure half is tested in scripts/release-tags.test.mjs.

import { execFileSync } from "node:child_process";
import { pathToFileURL } from "node:url";

const RELEASE = /^v?(\d+)\.(\d+)\.(\d+)$/;

/** [major, minor, patch] for a plain release version, or null (prerelease, junk). */
export function parseRelease(v) {
  const m = RELEASE.exec(v);
  return m ? m.slice(1, 4).map(Number) : null;
}

function cmp(a, b) {
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i] - b[i];
  return 0;
}

/**
 * @param {string} version  the version being released, without the `v` (e.g. "1.16.2")
 * @param {string[]} existing  existing git tags; non-release tags are ignored
 * @returns {string[]} tags to write, exact version first
 */
export function promotionTags(version, existing) {
  const v = parseRelease(version);
  if (!v) return [version];
  const others = existing.map(parseRelease).filter(Boolean);
  const isTop = (inRange) => others.filter(inRange).every((o) => cmp(v, o) >= 0);
  const tags = [version];
  if (isTop((o) => o[0] === v[0] && o[1] === v[1])) tags.push(`${v[0]}.${v[1]}`);
  if (isTop((o) => o[0] === v[0])) tags.push(`${v[0]}`);
  if (isTop(() => true)) tags.push("latest");
  return tags;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  const version = process.argv[2];
  if (!version) {
    console.error("usage: release-tags.mjs <version>");
    process.exit(2);
  }
  const existing = execFileSync("git", ["tag", "--list", "v*"], { encoding: "utf8" }).split("\n").filter(Boolean);
  console.log(promotionTags(version, existing).join(" "));
}
