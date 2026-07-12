#!/usr/bin/env node
// Flightplan · PostToolUse(Skill) hook (ADR-064, batch 1).
//
// Mechanical only — never invokes `claude`, never makes a judgment. After a Skill tool call it:
//   1. reads which skill ran (from the tool payload on stdin);
//   2. appends it to today's session-log entry in the current epic's worklog (deduped);
//   3. flips that skill's gate `[ ] → [/]` (in progress) if the config maps it to one.
// It NEVER sets `[x]` (done) — that judgment belongs to /handoff (batch 2). No worklog for the
// current branch ⇒ silent no-op (SessionStart owns scaffolding). Writes atomically, and only when
// something actually changed. Any failure is swallowed; the hook must never disrupt the session.

import { readFileSync, writeFileSync, renameSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { loadConfig, resolveKey, bareSkill } from "./config.mjs";
import { logSkillRun, flipGate } from "./worklog.mjs";
import { readStdin, safeParse } from "./stdin.mjs";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

// Hard ceiling — a PostToolUse hook must not stall the turn.
const guard = setTimeout(() => process.exit(0), 5000);
guard.unref();

async function main() {
  const input = safeParse(await readStdin());
  const skill = bareSkill(input?.tool_input?.skill);
  if (!skill) return; // not a Skill call we can attribute

  const cfg = loadConfig(ROOT);
  const key = resolveKey(ROOT, cfg.branchKey);
  if (!key) return; // un-keyed branch → nothing to track

  const worklogPath = join(ROOT, cfg.worklogDir, `${key}.md`);
  let text;
  try {
    text = readFileSync(worklogPath, "utf8");
  } catch {
    return; // no worklog yet → SessionStart scaffolds; nothing to append to
  }

  let next = logSkillRun(text, skill, today());
  const gate = cfg.gates.find((g) => bareSkill(g.skill) === skill);
  if (gate) next = flipGate(next, gate.id);

  if (next !== text) writeAtomic(worklogPath, next);
}

function today() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// Write via a temp file + rename so a crash mid-write can't leave a half-written worklog.
function writeAtomic(path, contents) {
  const tmp = `${path}.tmp`;
  writeFileSync(tmp, contents);
  renameSync(tmp, path);
}

main().catch(() => {}).finally(() => process.exit(0));
