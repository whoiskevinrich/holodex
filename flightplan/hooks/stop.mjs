#!/usr/bin/env node
// Flightplan · Stop hook (ADR-064, batch 1) — the worklog-staleness nag.
//
// Mechanical only — NEVER invokes `claude` (fork-bomb rule, CLAUDE.md) and NEVER writes the handoff
// note itself (that judgment is /handoff's, batch 2). Its whole job: if this session moved code but
// the worklog didn't move with it, make that omission loud so the next session isn't left blind.
//
// The signal is artifact-derived and self-clearing, matching the stateless discipline of the other
// hooks: nag when a changed file (git status) is NEWER than the worklog. The moment the worklog is
// touched — by /handoff, a PostToolUse skill-log, or a hand edit — it moves ahead of the code and the
// nag goes quiet on its own, with no stored state to drift. It stays "loud until addressed" on
// purpose; if that proves too noisy in practice, a dampener is batch-4 tuning (ADR-064), not batch 1.
//
// Surfaced via `systemMessage` (+ exit 0): shown to the USER, never blocks the stop, never fed back
// to Claude — so it can't loop or re-enter the agent (and never costs Claude's context budget). Any
// failure is swallowed; a Stop hook must never wedge the session. Dependency-free, cross-platform.

import { statSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";
import { loadConfig, resolveKey } from "./config.mjs";
import { readStdin, safeParse } from "./stdin.mjs";
import { emitJson, relPath } from "./hookout.mjs";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

// Hard ceiling — a Stop hook must not stall the turn.
const guard = setTimeout(() => process.exit(0), 5000);
guard.unref();

async function main() {
  const input = safeParse(await readStdin());
  if (input?.stop_hook_active) return; // already inside a stop-driven continuation → never pile on

  const cfg = loadConfig(ROOT);
  const key = resolveKey(ROOT, cfg.branchKey);
  if (!key) return; // un-keyed branch → nothing to track

  const worklogPath = join(ROOT, cfg.worklogDir, `${key}.md`);
  const worklogMtime = mtime(worklogPath);
  if (worklogMtime == null) return; // no worklog yet → SessionStart owns scaffolding; nothing to nag about
  if (!hasChangeNewerThan(worklogPath, worklogMtime)) return; // worklog is current with the work → quiet

  emitJson({
    systemMessage:
      `⚠ Flightplan · ${key}: your worklog is behind your latest code changes. ` +
      `Add a handoff line to ${relPath(ROOT, worklogPath)} (or run /handoff) so the next session isn't blind.`,
    suppressOutput: true,
  });
}

// ---- signal --------------------------------------------------------------

// mtimeMs of a path, or null if it can't be stat'd.
function mtime(path) {
  try {
    return statSync(path).mtimeMs;
  } catch {
    return null;
  }
}

// Is any changed file (staged, unstaged, or untracked — per `git status --porcelain`), other than the
// worklog itself, newer than the worklog? Short-circuits on the first one found. false when the tree
// is clean / all committed, or git is unavailable.
function hasChangeNewerThan(worklogPath, worklogMtime) {
  let out;
  try {
    out = execFileSync("git", ["status", "--porcelain"], { cwd: ROOT, encoding: "utf8", timeout: 3000 });
  } catch {
    return false;
  }
  const wl = norm(worklogPath);
  for (const line of out.split("\n")) {
    if (!line.trim()) continue;
    const rel = changedPath(line);
    if (!rel) continue;
    const abs = resolve(ROOT, rel);
    if (norm(abs) === wl) continue; // the worklog moving is the opposite of the problem
    const m = mtime(abs);
    if (m != null && m > worklogMtime) return true; // first change newer than the worklog → stale
  }
  return false;
}

// The path out of one `git status --porcelain` line. Columns 0-1 are status, the path starts at 3;
// a rename shows `orig -> new` (take new). Best-effort unquoting of a path git wrapped in quotes.
function changedPath(line) {
  let p = line.slice(3).trim();
  const arrow = p.indexOf(" -> ");
  if (arrow >= 0) p = p.slice(arrow + 4);
  if (p.startsWith('"') && p.endsWith('"')) p = p.slice(1, -1);
  return p || null;
}

function norm(p) {
  return p.replaceAll("\\", "/").toLowerCase();
}

// Only ever run as a hook. Never let a failure block the session; the keyed-nag path exits from
// emitJson()'s drain callback, every other path falls through to a natural exit (guarded by the
// unref'd timer). We deliberately do NOT process.exit() at the top level — that would race the flush.
main().catch((e) => {
  if (process.env.FP_DEBUG) console.error("FP_DEBUG", e);
});
