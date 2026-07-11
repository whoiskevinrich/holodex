// Flightplan · shared config + tracker-key resolution (ADR-064). Used by every hook so the
// portability seam (.claude/flightplan.yaml) is read one way, not scraped per-hook. Dependency-free.
//
// A full structured YAML parse is intentionally avoided — flightplan.yaml is small and stable, and a
// vendored parser is more surface than these few line-scans. If the config grows, this is the one
// place to swap in a real parser.

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { execFileSync } from "node:child_process";

export const DEFAULTS = {
  branchKey: "[A-Z][A-Z0-9]+-\\d+",
  worklogDir: "docs/plans",
  transitionScript: "scripts/jira-transition.mjs",
};

// Read the fields the hooks need from <root>/.claude/flightplan.yaml. Missing file / fields → defaults.
export function loadConfig(root) {
  const out = { ...DEFAULTS, gates: [] };
  let text;
  try {
    text = readFileSync(join(root, ".claude", "flightplan.yaml"), "utf8").replace(/\r\n/g, "\n");
  } catch {
    return out; // no config → defaults
  }

  // A single/double-quoted or bare scalar for a block-style `key:`, stopping before an inline comment.
  const block = (k) => {
    const m = text.match(new RegExp(`^\\s*${k}:\\s*(?:'([^']*)'|"([^"]*)"|([^\\s#]+))`, "m"));
    return m ? (m[1] ?? m[2] ?? m[3]) : null;
  };
  out.branchKey = block("branch_key") ?? out.branchKey;
  out.worklogDir = block("dir") ?? out.worklogDir;

  // `script:` lives inside the inline flow map `in_progress: { via: rest, script: … }`, so it is not
  // line-anchored; stop the bare scalar before a comment, comma, or the closing brace.
  const scr = text.match(/\bscript:\s*(?:'([^']*)'|"([^"]*)"|([^\s#,}]+))/);
  if (scr) out.transitionScript = scr[1] ?? scr[2] ?? scr[3];

  // gates: each `- { id: <id>, skill: <skill>, … }` that declares a skill (id-less/skill-less gates —
  // e.g. backend/frontend — carry no skill mapping and are simply skipped here).
  for (const m of text.matchAll(/-\s*\{[^}]*\bid:\s*([\w-]+)[^}]*\bskill:\s*([\w-]+)/g)) {
    out.gates.push({ id: m[1], skill: m[2] });
  }
  return out;
}

// The tracker key on the current branch, or null. Never throws (a malformed operator regex, or git
// being unavailable, degrades to null → the caller no-ops).
export function resolveKey(root, branchKey) {
  let branch;
  try {
    branch = execFileSync("git", ["branch", "--show-current"], { cwd: root, encoding: "utf8" }).trim();
  } catch {
    return null;
  }
  if (!branch) return null;
  try {
    return (branch.match(new RegExp(branchKey)) ?? [])[0] ?? null;
  } catch {
    return null;
  }
}

// Normalize a skill name to its bare form (drop any `namespace:` prefix), for matching against the
// gate list. e.g. "engineering:architecture" → "architecture", "security-review" → "security-review".
export function bareSkill(name) {
  return (name ?? "").split(":").pop().trim();
}
