#!/usr/bin/env node
// Flightplan · SessionStart hook (ADR-064, batch 1).
//
// Mechanical only — this hook NEVER invokes `claude` (fork-bomb rule, CLAUDE.md) and never
// makes the "is this epic done" judgment. At start-of-work it:
//   1. reads the branch, extracts the tracker key (regex from .claude/flightplan.yaml);
//   2. scaffolds the per-epic worklog from templates/worklog.md if it doesn't exist yet;
//   3. fires "In Progress" idempotently by spawning the config's transition `script:` (ADR-058's
//      soft-fail REST entry), bounded by a timeout — the flaky connector is NEVER in the hot path;
//   4. prints a compact (~150-token) orientation banner distilled from the LOCAL worklog.
//
// It is defensive to a fault: any failure is swallowed and the process still exits 0, because a
// hook that blocks or crashes session start would be worse than no hook at all. Dependency-free,
// cross-platform (invoked as `node …`, no shell assumptions).

import { readFileSync, existsSync, writeFileSync, mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";
import { parseWorklog } from "./worklog.mjs";
import { loadConfig, resolveKey } from "./config.mjs";
import { readStdin, safeParse } from "./stdin.mjs";
import { emitJson, relPath } from "./hookout.mjs";

// Hook lives at <root>/flightplan/hooks/session-start.mjs → root is two levels up. Deriving the
// root from the hook's own location (not cwd) makes it robust to being launched from a subdir.
const HOOK_DIR = dirname(fileURLToPath(import.meta.url));
const PLUGIN_DIR = resolve(HOOK_DIR, "..");
const ROOT = resolve(PLUGIN_DIR, "..");

// A hard ceiling: whatever happens, never delay session start by more than this.
const guard = setTimeout(() => process.exit(0), 8000);
guard.unref();

async function main() {
  const input = await readStdin();
  const source = safeParse(input)?.source ?? "startup"; // startup | resume | clear | compact

  const cfg = loadConfig(ROOT);
  const key = resolveKey(ROOT, cfg.branchKey);
  if (!key) return; // un-keyed branch → no-op silently (ADR-064)

  const worklogPath = join(ROOT, cfg.worklogDir, `${key}.md`);
  const scaffolded = ensureWorklog(worklogPath, key);

  // Fire In Progress on a real start; skip mid-session compaction (nothing changed).
  const transition =
    source === "compact" ? { line: null } : fireInProgress(key, join(ROOT, cfg.transitionScript));

  emit(buildBanner({ key, worklogPath, transition, scaffolded }));
}

// ---- steps ---------------------------------------------------------------

// Returns true if it created the file this run.
function ensureWorklog(worklogPath, key) {
  if (existsSync(worklogPath)) return false;
  try {
    const tpl = readFileSync(join(PLUGIN_DIR, "templates", "worklog.md"), "utf8");
    const seeded = tpl.replaceAll("KEY-NNN", key).replace(/^status:\s*todo\b/m, "status: in-progress");
    mkdirSync(dirname(worklogPath), { recursive: true });
    writeFileSync(worklogPath, seeded);
    return true;
  } catch {
    return false;
  }
}

function fireInProgress(key, scriptPath) {
  const { JIRA_BASE_URL, JIRA_USER_EMAIL, JIRA_API_TOKEN } = process.env;
  if (!JIRA_BASE_URL || !JIRA_USER_EMAIL || !JIRA_API_TOKEN) {
    return { line: "◦ In Progress not auto-fired (no local JIRA_* creds) — fire it via MCP." };
  }
  try {
    // Drive the transition through the config's `script:` seam (ADR-058's soft-fail entry), rather
    // than re-implementing it in-process. `timeout` bounds it so a slow Jira can't stall the session;
    // the script logs a `::warning::` (stdout) on an API-level soft-fail while still exiting 0.
    const out = execFileSync("node", [scriptPath], {
      cwd: ROOT,
      env: { ...process.env, JIRA_ISSUE_KEY: key, JIRA_TARGET_STATUS: "In Progress" },
      encoding: "utf8",
      timeout: 6000,
      stdio: ["ignore", "pipe", "pipe"],
    });
    return {
      line: /::warning::/.test(out)
        ? `◦ In Progress not confirmed — verify ${key} in Jira.`
        : "✓ In Progress (idempotent).",
    };
  } catch (err) {
    return { line: `◦ In Progress transition skipped (${short(err.message)}).` };
  }
}

// ---- banner --------------------------------------------------------------

function buildBanner({ key, worklogPath, transition, scaffolded }) {
  if (scaffolded) {
    return [
      `▸ ${key} · new worklog scaffolded at ${relPath(ROOT, worklogPath)}`,
      transition.line ? `  ${transition.line}` : null,
      `  Fill in its title, gates, and up-next as work begins — /handoff maintains it thereafter.`,
    ]
      .filter(Boolean)
      .join("\n");
  }

  const wl = parseWorklog(worklogPath);
  const last = wl.handoffPending
    ? "⚠ last session left no handoff — reconstruct from git/PR, then leave one."
    : (wl.lastHandoff ?? "(no prior handoff recorded)");
  const lines = [
    `▸ ${key} · ${wl.title ?? "(untitled)"} · ${wl.status ?? "?"} · gates ${wl.gatesDone}/${wl.gatesTotal}`,
    `  next: ${wl.next ?? "—"}`,
    `  last: ${last}`,
    `  ${wl.blockers.length ? "⛔ " + wl.blockers.join(" · ") : "⛔ none"}`,
  ];
  if (transition.line) lines.push(`  ${transition.line}`);
  return lines.join("\n");
}

// ---- utils ---------------------------------------------------------------

function short(msg) {
  return (msg ?? "").split("\n")[0].slice(0, 80);
}

function emit(context) {
  emitJson({ hookSpecificOutput: { hookEventName: "SessionStart", additionalContext: context } });
}

// This module is only ever run as a hook (its pure parsers live in ./worklog.mjs, which tests
// import directly). Never let a hook failure block the session: the keyed path exits from emitJson()'s
// drain callback; the no-key / error paths fall through to a natural exit (the unref'd guard is the
// only hard timeout). We deliberately do NOT process.exit() here — that would race the stdout flush.
main().catch((e) => { if (process.env.FP_DEBUG) console.error("FP_DEBUG", e); });
