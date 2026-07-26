// Flightplan · pure worklog parsers (ADR-064). No side effects beyond reading the given file —
// kept in its own module so unit tests (batch 2) and the SessionStart hook share one implementation
// and tests never load the hook's start-of-work machinery. Dependency-free.
//
// NOTE (follow-up): scripts/whats-left.mjs parses the SAME docs/plans/<KEY>.md schema with its own
// section()/parseGates()/parseUpNext()/frontmatter(). When the PostToolUse hook lands (batch 1) and
// a shared flightplan/lib/ is extracted, these two should collapse onto one canonical parser.

import { readFileSync } from "node:fs";

// Distill a worklog into the handful of fields the orientation banner needs. Never throws;
// a missing/garbled file yields the zero value.
export function parseWorklog(worklogPath) {
  const out = {
    title: null,
    status: null,
    gatesDone: 0,
    gatesTotal: 0,
    next: null,
    lastHandoff: null,
    handoffPending: false,
    blockers: [],
  };
  let text;
  try {
    text = readFileSync(worklogPath, "utf8").replace(/\r\n/g, "\n"); // normalize CRLF working trees
  } catch {
    return out;
  }

  // Split frontmatter from body once, and the body into lines once — both are scanned repeatedly.
  const fm = text.match(/^---\n([\s\S]*?)\n---\n?/);
  const fmText = fm ? fm[1] : "";
  const lines = (fm ? text.slice(fm[0].length) : text).split("\n");

  out.status = stripComment((fmText.match(/^status:\s*(.+)$/m) ?? [])[1]);

  for (const l of lines) {
    const h1 = l.match(/^#\s+(.+)$/); // first body H1 wins
    if (h1) {
      const t = h1[1].trim();
      out.title = t.includes(" · ") ? t.slice(t.indexOf(" · ") + 3).trim() : t;
      break;
    }
  }

  for (const l of section(lines, "Gates")) {
    const m = l.match(/^- \[([ xX/~])\]/);
    if (!m) continue;
    out.gatesTotal++;
    if (m[1].toLowerCase() === "x") out.gatesDone++;
  }

  // One pass over Up next yields both the top actionable item and the blocker list.
  for (const l of section(lines, "Up next")) {
    const m = l.match(/^\d+\.\s+\[([ xX/~])\]\s+(.+)$/);
    if (!m) continue;
    const state = m[1].toLowerCase();
    const label = stripMd(m[2]);
    if (!out.next && state !== "x" && state !== "~") out.next = label; // first not-done, not-deferred
    if (m[2].includes("⛔")) out.blockers.push(label.replace(/\s*⛔.*$/, "").trim());
  }

  const log = section(lines, "Session log");
  const hand = log.find((l) => /^- handoff:/.test(l)); // newest handoff anywhere (entries are top-down)
  if (hand) out.lastHandoff = hand.replace(/^- handoff:\s*/, "").trim();

  // Did the NEWEST session entry leave a handoff? If it exists but has none, the last session ended
  // without orienting the next — surface that distinctly (a stale older handoff would otherwise mask it).
  const firstEntry = log.findIndex((l) => /^###\s/.test(l));
  if (firstEntry >= 0) {
    let hasHandoff = false;
    for (let i = firstEntry + 1; i < log.length && !/^###\s/.test(log[i]); i++) {
      if (/^- handoff:/.test(log[i])) {
        hasHandoff = true;
        break;
      }
    }
    out.handoffPending = !hasHandoff;
  }

  return out;
}

// Lines of a "## <name>" section, up to the next "## " heading (empty when the section is absent).
export function section(lines, name) {
  const b = sectionBounds(lines, name);
  return b ? lines.slice(b.start, b.end) : [];
}

// Drop a trailing YAML comment from a frontmatter scalar. The scaffolded worklog documents every
// field inline (`status: in-progress    # todo | in-progress | …`), so a bare `(.+)$` capture would
// spill the whole comment into the orientation banner. A quoted scalar is taken whole (a `#` inside
// quotes is literal); otherwise whitespace-then-`#`, or a `#` in the value's first column (a field
// left unset under its comment), opens the comment. Mirrors `frontmatter()` in scripts/whats-left.mjs
// — see the NOTE at the top of this file about collapsing the two parsers.
export function stripComment(raw) {
  const s = (raw ?? "").trim();
  const quoted = s.match(/^(['"]).*?\1/);
  if (quoted) return quoted[0];
  if (s.startsWith("#")) return null;
  return s.split(/\s+#/)[0].trimEnd() || null;
}

// Flatten inline markdown (code, links, emphasis) to plain text for the one-line banner.
export function stripMd(s) {
  return s
    .replace(/`([^`]*)`/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/\*+/g, "")
    .trim();
}

// ---- write helpers (batch-1 PostToolUse) — pure string → string, CRLF-preserving ---------------

// Record a skill run under today's session-log entry: append to its `- skills:` line (deduped),
// creating a fresh `### <date> · session` entry at the top of the log when the newest one isn't
// today's. Returns the (possibly unchanged) text — unchanged when the skill is already logged today.
export function logSkillRun(text, skill, date) {
  const nl = /\r\n/.test(text) ? "\r\n" : "\n";
  const lines = text.split(/\r?\n/);
  const b = sectionBounds(lines, "Session log");
  if (!b) return text;

  const firstEntry = lines.slice(b.start, b.end).findIndex((l) => /^###\s/.test(l));
  const entryIdx = firstEntry < 0 ? -1 : b.start + firstEntry;

  if (entryIdx >= 0 && lines[entryIdx].includes(date)) {
    // Newest entry is today's — merge the skill into its `- skills:` line (or add that line).
    for (let i = entryIdx + 1; i < b.end && !/^#{2,3}\s/.test(lines[i]); i++) {
      const m = lines[i].match(/^-\s*skills:\s*(.*)$/i);
      if (!m) continue;
      const skills = m[1].split(",").map((s) => s.trim()).filter(Boolean);
      if (skills.includes(skill)) return text; // already logged today → no-op
      skills.push(skill);
      lines[i] = `- skills: ${skills.join(", ")}`;
      return lines.join(nl);
    }
    lines.splice(entryIdx + 1, 0, `- skills: ${skill}`);
    return lines.join(nl);
  }

  // No entry for today — insert one at the top of the log body (before the first existing entry, or
  // at the end of the section when there are none yet).
  const at = entryIdx >= 0 ? entryIdx : b.end;
  lines.splice(at, 0, `### ${date} · session`, `- skills: ${skill}`, "");
  return lines.join(nl);
}

// Flip an untouched `- [ ] <gateId>` gate to `[/]` (in progress). No-op for a gate already
// `[/]`/`[x]`/`[~]`, or when the id isn't found. Returns the (possibly unchanged) text.
export function flipGate(text, gateId) {
  const nl = /\r\n/.test(text) ? "\r\n" : "\n";
  const lines = text.split(/\r?\n/);
  const b = sectionBounds(lines, "Gates");
  if (!b) return text;
  for (let i = b.start; i < b.end; i++) {
    const m = lines[i].match(/^-\s*\[ \]\s+(\S+)/); // only an untouched `[ ]` gate
    if (m && m[1] === gateId) {
      lines[i] = lines[i].replace(/^(-\s*\[) (\])/, "$1/$2");
      return lines.join(nl);
    }
  }
  return text;
}

// Start/end line indices of a "## <name>" section body ([start, end) excludes the heading; end is the
// next "## " heading or EOF). null when the section is absent.
function sectionBounds(lines, name) {
  const head = new RegExp(`^##\\s+${name}`);
  const start = lines.findIndex((l) => head.test(l));
  if (start < 0) return null;
  let end = lines.length;
  for (let i = start + 1; i < lines.length; i++) {
    if (/^##\s/.test(lines[i])) {
      end = i;
      break;
    }
  }
  return { start: start + 1, end };
}
