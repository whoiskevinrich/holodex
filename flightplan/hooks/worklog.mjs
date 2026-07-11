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

  out.status = (fmText.match(/^status:\s*(.+)$/m) ?? [])[1]?.trim() ?? null;

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

  const hand = section(lines, "Session log").find((l) => /^- handoff:/.test(l)); // newest is on top
  if (hand) out.lastHandoff = hand.replace(/^- handoff:\s*/, "").trim();

  return out;
}

// Lines of a "## <name>" section up to the next "## " heading (JS regex has no \Z, so scan lines).
export function section(lines, name) {
  const head = new RegExp(`^##\\s+${name}`);
  let start = -1;
  for (let i = 0; i < lines.length; i++) {
    if (start === -1) {
      if (head.test(lines[i])) start = i + 1;
      continue;
    }
    if (/^##\s/.test(lines[i])) return lines.slice(start, i);
  }
  return start === -1 ? [] : lines.slice(start);
}

// Flatten inline markdown (code, links, emphasis) to plain text for the one-line banner.
export function stripMd(s) {
  return s
    .replace(/`([^`]*)`/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/\*+/g, "")
    .trim();
}
