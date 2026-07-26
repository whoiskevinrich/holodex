// Flightplan · the hooks' file-reading adapter over the canonical parser (ADR-064).
//
// Everything here used to be a second copy of the docs/plans/<KEY>.md schema; it now lives in
// ../lib/worklog.mjs, shared with scripts/whats-left.mjs. This module keeps only the impure part —
// reading the file — so the parser itself stays testable without touching a filesystem.

import { readFileSync } from "node:fs";
import { emptyWorklog, parseWorklogText } from "../lib/worklog.mjs";

// Re-exported so the hooks (and the previous import sites) keep working unchanged.
export {
  emptyWorklog,
  flipGate,
  frontmatter,
  logSkillRun,
  maskComments,
  parseGates,
  parseUpNext,
  parseWorklogText,
  section,
  sectionBounds,
  stripComment,
  stripMd,
} from "../lib/worklog.mjs";

// Distill a worklog file into the fields the orientation banner needs. Never throws; a missing or
// unreadable file yields the zero value.
export function parseWorklog(worklogPath) {
  let text;
  try {
    text = readFileSync(worklogPath, "utf8");
  } catch {
    return emptyWorklog();
  }
  return parseWorklogText(text);
}
