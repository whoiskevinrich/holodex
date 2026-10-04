#!/usr/bin/env node
// doc-type-guard.mjs — keeps specs, architecture docs and design docs from comingling
// (HOLODEX-523; rules in docs/reference/doc-types.md). A Claude Code PreToolUse hook
// (.claude/settings.json) acting at the two moments a doc type is chosen:
//
//   * Skill — when /architecture, /write-spec or /design-handoff (any plugin prefix) is
//             invoked, that doc type's boundary is injected as context. The skills are
//             plugin-owned and their templates know nothing of this repo's rules (the
//             /architecture template scaffolds a numbered ADR), so the boundary has to
//             arrive alongside them.
//   * Write — creating a NEW numbered docs/architecture/ADR-NNN-*.md is BLOCKED (exit 2):
//             architecture lives in living topic docs now. Editing an existing ADR passes,
//             because the transition rules still allow it until the ADR files are retired.
//
// Never re-enters Claude, and never blocks on its own failure. Input is the hook JSON on
// stdin; the pure parts are exported for scripts/hooks/doc-type-guard.test.mjs.

import { existsSync } from "node:fs";
import { isAbsolute, join } from "node:path";
import { pathToFileURL } from "node:url";

const RULES = "docs/reference/doc-types.md";

const BOUNDARIES = {
  architecture:
    "Holodex doc types (" + RULES + "): you are writing ARCHITECTURE. Edit or start a living topic " +
    "doc at docs/architecture/<topic>.md (index: docs/architecture/README.md) — never a new numbered " +
    "ADR-NNN file, whatever this skill's template says; the repo blocks creating one. Write only a " +
    "real technology fork (stack, data model, storage, service boundary, security perimeter, " +
    "deployment, cross-cutting pattern) with the credible alternative it rejected. Never include " +
    "user-visible rules, thresholds the owner sees, or UI. State current truth (no history), end each " +
    "section with `Decided in [`<sha>`](…/commit/<sha>)`. No fork → write nothing and record " +
    "`[~] architecture — no technology fork` in the worklog.",
  "write-spec":
    "Holodex doc types (" + RULES + "): you are writing a SPEC — what the feature does as the owner " +
    "experiences it. Never include table names, endpoints, or component/function names; those are " +
    "architecture or design. A changed product rule is an in-place edit of the existing spec, never a " +
    "new ADR. Link architecture and design docs instead of restating them.",
  "design-handoff":
    "Holodex doc types (" + RULES + "): you are writing a DESIGN doc — components, flows, states and " +
    "interactions the owner sees. Never state rules the backend enforces or schema; link the spec for " +
    "behaviour and the architecture topic doc for mechanism. The doc is living: edit it in place.",
};

// "engineering:architecture", "architecture", "design:design-handoff" … -> the boundary key, or null.
export function docTypeOfSkill(toolInput) {
  const m = /(^|:)(architecture|write-spec|design-handoff)$/.exec(String(toolInput?.skill ?? ""));
  return m ? m[2] : null;
}

export function contextFor(docType) {
  return BOUNDARIES[docType] ?? null;
}

const NUMBERED_ADR = /(^|[\\/])docs[\\/]architecture[\\/]ADR-\d+[^\\/]*\.md$/;

// Block only the creation of a new numbered ADR; `exists` says whether the target is on disk.
export function decideWrite(toolName, filePath, exists) {
  if (toolName !== "Write" || !NUMBERED_ADR.test(String(filePath ?? "")) || exists) return { block: false };
  return {
    block: true,
    message:
      "new numbered ADRs are retired (" + RULES + "). Put the decision in the living topic doc that " +
      "owns it (index: docs/architecture/README.md), or start docs/architecture/<topic>.md. If there is " +
      "no technology fork, write no architecture doc and record `[~] architecture — no technology fork`.",
  };
}

async function readStdin() {
  let raw = "";
  for await (const chunk of process.stdin) raw += chunk;
  return raw ? JSON.parse(raw) : {};
}

async function main() {
  let input;
  try {
    input = await readStdin();
  } catch {
    return; // not hook JSON — nothing to judge
  }
  const toolName = input.tool_name ?? "";
  const toolInput = input.tool_input ?? {};

  if (toolName === "Skill") {
    const ctx = contextFor(docTypeOfSkill(toolInput));
    if (!ctx) return;
    process.stdout.write(
      JSON.stringify({ hookSpecificOutput: { hookEventName: "PreToolUse", additionalContext: ctx } }),
    );
    return;
  }

  if (toolName === "Write") {
    const filePath = String(toolInput.file_path ?? "");
    let exists = true; // fail open: an unreadable path is never blocked
    try {
      exists = existsSync(isAbsolute(filePath) ? filePath : join(process.cwd(), filePath));
    } catch {}
    const verdict = decideWrite(toolName, filePath, exists);
    if (verdict.block) {
      process.stderr.write(`doc-type-guard: ${verdict.message}\n`);
      process.exitCode = 2;
    }
  }
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) main();
