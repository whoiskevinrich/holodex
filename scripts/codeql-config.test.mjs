// Guards the CodeQL scan scope (HOLODEX-449).
//
// `paths-ignore` is the kind of config that fails silently in the worst direction: broaden it by
// one entry — `web/**`, `scripts/**`, a stray `*.ts` — and the Security tab goes quiet because
// nothing is being scanned, which reads exactly like "no vulnerabilities". There is no signal to
// notice. So the invariant is asserted here instead: **every file the config excludes must be a
// test file, and the app code must survive.**
//
// Deliberately not a YAML parser (the repo root has no node_modules). The block is a short list
// of quoted strings; the parse asserts its own shape and the list's non-emptiness, so a format
// change surfaces as a failure rather than a vacuous pass.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const CONFIG = ".github/codeql/codeql-config.yml";
const WORKFLOW = ".github/workflows/codeql.yml";

const config = readFileSync(CONFIG, "utf8");

/** The `paths-ignore:` list, as written. */
function pathsIgnore() {
  const block = config.match(/^paths-ignore:\n((?:[ \t]*(?:#.*)?\n|[ \t]*-[ \t]*.*\n)*)/m);
  assert.ok(block, `${CONFIG}: no paths-ignore: block found — did the key move or get renamed?`);
  return [...block[1].matchAll(/^[ \t]*-[ \t]*['"]?([^'"#\n]+?)['"]?[ \t]*$/gm)].map((m) => m[1]);
}

/** Only the basename-anchored `**\/*.x` shape this repo uses; anything else must fail loudly. */
function toRegExp(pattern) {
  const m = pattern.match(/^\*\*\/\*(\.[\w.]+)$/);
  assert.ok(m, `unhandled paths-ignore shape ${pattern}: extend this test before using it`);
  return new RegExp(`(^|/)[^/]+${m[1].replace(/\./g, "\\.")}$`);
}

const tracked = execFileSync("git", ["ls-files"], { encoding: "utf8" }).trim().split("\n");

test("the config excludes something, and the parse is not silently empty", () => {
  const patterns = pathsIgnore();
  assert.ok(patterns.length > 0, "paths-ignore parsed as empty — the regex above has drifted");
});

test("every excluded file is a test file", () => {
  const res = pathsIgnore().map(toRegExp);
  const excluded = tracked.filter((f) => res.some((re) => re.test(f)));

  assert.ok(excluded.length > 0, "paths-ignore matched no tracked file at all");
  const notTests = excluded.filter((f) => !/\.(test|spec)\.[\w]+$/.test(f));
  assert.deepEqual(notTests, [], `paths-ignore would drop non-test files from the scan: ${notTests}`);
});

test("application code stays in scope", () => {
  const res = pathsIgnore().map(toRegExp);
  const kept = tracked.filter((f) => !res.some((re) => re.test(f)));

  // Floors, not exact counts — they must not track every file added or removed. They exist to
  // catch a pattern that swallows a whole tree.
  const inScope = (prefix, ext) =>
    kept.filter((f) => f.startsWith(prefix) && f.endsWith(ext)).length;

  assert.ok(inScope("web/src/", ".ts") > 20, "web/src TypeScript fell out of the scan");
  assert.ok(inScope("web/src/", ".svelte") > 20, "web/src components fell out of the scan");
  // scripts/ tooling runs in CI with repo write access — only its *.test.mjs siblings are skipped.
  assert.ok(inScope("scripts/", ".mjs") > 5, "scripts/ tooling fell out of the scan");
});

test("no paths: allowlist and no queries: override crept in", () => {
  // A `paths:` allowlist silently drops any new top-level directory; a `queries:` block would
  // change which rules run, which is not what this config is for.
  assert.equal(/^paths:/m.test(config), false, "a paths: allowlist would drop new directories");
  assert.equal(/^queries:/m.test(config), false, "query selection must not change here");
});

test("the workflow actually references the config", () => {
  // A config file nothing points at is worse than none: it reads as coverage that is not applied.
  const workflow = readFileSync(WORKFLOW, "utf8");
  assert.match(workflow, /config-file:\s*\.\/\.github\/codeql\/codeql-config\.yml/);
});
