#!/usr/bin/env node
// Flightplan · worklog gate — the server-side half of ADR-007 §4, for merges that never pass through
// a Claude session (the GitHub button, the desktop app's PR pane). Copy this file and worklog-gate.yml
// into the consuming repo; it deliberately imports nothing from the plugin, because a consumer's CI
// cannot reach it (ADR-002/003). That means `deriveStatus` exists twice — here as two regexes over
// a schema that has been stable since ADR-001 — and ADR-007 records the duplication as accepted.
//
// Fails a non-draft PR whose branch key's worklog has a gate at [ ]/[/], an empty release_note, or
// a sign-off whose artifact changed since the commit it was given at. Passes (silently) when the
// branch carries no key: un-keyed branches are untracked by design. Needs the full history
// (fetch-depth: 0) for the staleness diff. Node 20+, no dependencies.

import { readFileSync, existsSync } from "node:fs";
import { execFileSync } from "node:child_process";

const branch = process.env.GITHUB_HEAD_REF || process.env.BRANCH || "";
const cfg = existsSync(".claude/flightplan.yaml") ? readFileSync(".claude/flightplan.yaml", "utf8") : "";
const keyRe = cfg.match(/^\s*branch_key:\s*(?:'([^']*)'|"([^"]*)"|(\S+))/m);
const key = branch.match(new RegExp(keyRe ? (keyRe[1] ?? keyRe[2] ?? keyRe[3]) : "[A-Z][A-Z0-9]+-\\d+"))?.[0];
if (!key) process.exit(0);
const dir = cfg.match(/^\s*dir:\s*(\S+)/m)?.[1] ?? "docs/plans";
const path = `${dir}/${key}.md`;
if (!existsSync(path)) {
  console.log(`::error::${path} not found for ${key}`);
  process.exit(1);
}

const text = readFileSync(path, "utf8").replace(/\r\n/g, "\n").replace(/<!--[\s\S]*?--!?>/g, (m) => m.replace(/[^\n]/g, " "));
const fm = text.match(/^---\n([\s\S]*?)\n---/)?.[1] ?? "";
const gStart = text.indexOf("\n## Gates");
const gBody = gStart < 0 ? "" : text.slice(gStart + 1);
const gEnd = gBody.indexOf("\n## ", 1);
const gates = gEnd < 0 ? gBody : gBody.slice(0, gEnd);
const open = [...gates.matchAll(/^-\s*\[([ /])\]\s*(\S*)/gm)].map((m) => m[2] || "?");
const releaseNote = fm.match(/^release_note:[^\S\n]*([^#\n]\S*)/m)?.[1];
const problems = [];
if (open.length) problems.push(`gates not settled: ${open.join(", ")}`);
if (!releaseNote) problems.push("release_note is empty");

// Sign-offs: `approved:` → `<gate>:` → `at: <sha>`; stale when the gate's artifact glob changed since.
const artifacts = Object.fromEntries(
  [...cfg.matchAll(/-\s*\{([^}]*)\}/g)]
    .map((m) => [m[1].match(/\bid:\s*([\w-]+)/)?.[1], m[1].match(/\bartifact:\s*(?:'([^']*)'|"([^"]*)"|([^\s#,}]+))/)])
    .filter(([id, a]) => id && a)
    .map(([id, a]) => [id, a[1] ?? a[2] ?? a[3]]),
);
const approvedBlock = fm.match(/^approved:[^\n]*\n((?:[ \t]+.*\n?)*)/m)?.[1] ?? "";
for (const m of approvedBlock.matchAll(/^[ \t]{1,3}([\w-]+):[^\n]*\n(?:[ \t]{4,}.*\n?)*/gm)) {
  const gate = m[1];
  const at = m[0].match(/^\s+at:\s*([0-9a-f]{7,40})/m)?.[1];
  const glob = artifacts[gate];
  if (!at || !glob) continue;
  let changed;
  try {
    changed = execFileSync("git", ["diff", "--name-only", `${at}..HEAD`, "--", globToPathspec(glob)], { encoding: "utf8" }).trim();
  } catch {
    changed = "(commit not found)";
  }
  if (changed) problems.push(`sign-off on ${gate} is stale — changed since ${at}: ${changed.split("\n").join(", ")}`);
}

if (problems.length) {
  for (const p of problems) console.log(`::error::Flightplan · ${key}: ${p}`);
  process.exit(1);
}
console.log(`Flightplan · ${key}: in-review, sign-offs current`);

// git's pathspec globs are close enough to the config's: `**` is the only translation needed.
function globToPathspec(glob) {
  return `:(glob)${glob}`;
}
