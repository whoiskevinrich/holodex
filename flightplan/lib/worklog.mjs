// Flightplan · the canonical worklog parser (ADR-064). One implementation of the
// docs/plans/<KEY>.md schema, shared by the hooks (flightplan/hooks/) and the repo's reporting CLI
// (scripts/whats-left.mjs). Pure and dependency-free: every function is string → value, with no
// filesystem access, so it can be unit-tested without loading any hook machinery.
//
// This module exists because the two consumers previously carried their own copies of
// section()/frontmatter()/parseGates()/parseUpNext(). Three bugs in a row (trailing YAML comments in
// frontmatter, commented-out example gates counted as real, a session-log entry written inside an
// open HTML comment) each had to be found and fixed twice. HOLODEX-182 item 7 tracked the collapse.

// ---- comment handling ---------------------------------------------------------------------------

// Blank out HTML-comment spans. The scaffolded worklog carries commented-out *examples* — a deferred
// gate row, a session-log entry — that are textually indistinguishable from real content to a line
// regex. Every non-newline character inside `<!-- … -->` becomes a space, so line count and column
// positions are preserved and any index computed against the masked copy stays valid against the
// original. That property is what lets the write helpers match on a masked view and mutate the real
// lines, leaving the emitted file byte-identical apart from the intended edit.
export function maskComments(text) {
  return text.replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, " "));
}

// Drop a trailing YAML comment from a frontmatter scalar. The scaffold documents every field inline
// (`status: in-progress    # todo | in-progress | …`), so a bare `(.+)$` capture spills the comment
// into the value. A quoted scalar is taken whole (a `#` inside quotes is literal); otherwise
// whitespace-then-`#`, or a `#` in the value's first column (a field left unset under its comment),
// opens the comment. Quotes are left on for the caller to strip.
export function stripComment(raw) {
  const s = (raw ?? "").trim();
  const quoted = s.match(/^(['"]).*?\1/);
  if (quoted) return quoted[0];
  if (s.startsWith("#")) return null;
  return s.split(/\s+#/)[0].trimEnd() || null;
}

// ---- structure ----------------------------------------------------------------------------------

// A frontmatter scalar, comment-stripped. null when absent or set to nothing but a comment.
export function frontmatter(text, field) {
  const m = text.match(new RegExp(`^${field}:\\s*(.+)$`, "im"));
  return m ? stripComment(m[1]) : null;
}

// Start/end line indices of a `## <name>` section body ([start, end) excludes the heading; end is the
// next `## ` heading or EOF). null when the section is absent. Takes lines and does NOT mask, so the
// write helpers can pass a masked view and use the returned indices against the original array.
export function sectionBounds(lines, name) {
  const want = `## ${name.toLowerCase()}`;
  const start = lines.findIndex((l) => l.trim().toLowerCase().startsWith(want));
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

// Lines of a `## <name>` section, comment-masked (empty when the section is absent).
export function section(text, name) {
  const lines = maskComments(text).split(/\r?\n/);
  const b = sectionBounds(lines, name);
  return b ? lines.slice(b.start, b.end).map((l) => l.trimEnd()) : [];
}

// The `## Gates` checklist as { state, label }. state ∈ { ' ', '/', '~', 'x' }. The label is optional
// so a bare `- [ ]` still counts as a gate.
export function parseGates(text) {
  return section(text, "Gates")
    .map((l) => l.match(/^-\s*\[([ x/~])\]\s*(.*)$/i))
    .filter(Boolean)
    .map((m) => ({ state: m[1].toLowerCase(), label: m[2].trim() }));
}

// Numbered items under `## Up next`, as written. The checkbox is optional — the template uses
// `1. [ ] [gate] …` but older worklogs use a bare `1. …`, and both are the queue.
export function parseUpNext(text) {
  return section(text, "Up next")
    .map((l) => l.trim())
    .filter((l) => /^\d+\.\s/.test(l));
}

// Flatten inline markdown (code, links, emphasis) to plain text for the one-line banner.
export function stripMd(s) {
  return s
    .replace(/`([^`]*)`/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/\*+/g, "")
    .trim();
}

// ---- banner distillation ------------------------------------------------------------------------

export function emptyWorklog() {
  return {
    title: null,
    status: null,
    gatesDone: 0,
    gatesTotal: 0,
    next: null,
    lastHandoff: null,
    handoffPending: false,
    blockers: [],
  };
}

// Distill a worklog into the handful of fields the orientation banner needs. Never throws.
export function parseWorklogText(text) {
  const out = emptyWorklog();
  const norm = text.replace(/\r\n/g, "\n"); // CRLF working trees

  const fm = norm.match(/^---\n([\s\S]*?)\n---\n?/);
  const body = fm ? norm.slice(fm[0].length) : norm;
  out.status = fm ? frontmatter(fm[1], "status") : null;

  for (const l of maskComments(body).split("\n")) {
    const h1 = l.match(/^#\s+(.+)$/); // first body H1 wins
    if (h1) {
      const t = h1[1].trim();
      out.title = t.includes(" · ") ? t.slice(t.indexOf(" · ") + 3).trim() : t;
      break;
    }
  }

  for (const g of parseGates(body)) {
    out.gatesTotal++;
    if (g.state === "x") out.gatesDone++;
  }

  // One pass over Up next yields both the top actionable item and the blocker list.
  for (const raw of parseUpNext(body)) {
    const m = raw.match(/^\d+\.\s+(?:\[([ x/~])\]\s+)?(.+)$/i);
    if (!m) continue;
    const state = (m[1] ?? " ").toLowerCase();
    const label = stripMd(m[2]);
    if (!out.next && state !== "x" && state !== "~") out.next = label; // first not-done, not-deferred
    if (m[2].includes("⛔")) out.blockers.push(label.replace(/\s*⛔.*$/, "").trim());
  }

  const log = section(body, "Session log");
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

// ---- write helpers — pure string → string, CRLF-preserving ---------------------------------------

// Record a skill run under today's session-log entry: append to its `- skills:` line (deduped),
// creating a fresh `### <date> · session` entry at the top of the log when the newest one isn't
// today's. Returns the (possibly unchanged) text — unchanged when the skill is already logged today.
export function logSkillRun(text, skill, date) {
  const nl = /\r\n/.test(text) ? "\r\n" : "\n";
  const lines = text.split(/\r?\n/);
  const view = maskComments(text).split(/\r?\n/); // match against this, mutate `lines`
  const b = sectionBounds(view, "Session log");
  if (!b) return text;

  const firstEntry = view.slice(b.start, b.end).findIndex((l) => /^###\s/.test(l));
  const entryIdx = firstEntry < 0 ? -1 : b.start + firstEntry;

  if (entryIdx >= 0 && view[entryIdx].includes(date)) {
    // Newest entry is today's — merge the skill into its `- skills:` line (or add that line).
    for (let i = entryIdx + 1; i < b.end && !/^#{2,3}\s/.test(view[i]); i++) {
      const m = view[i].match(/^-\s*skills:\s*(.*)$/i);
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
  const view = maskComments(text).split(/\r?\n/); // match against this, mutate `lines`
  const b = sectionBounds(view, "Gates");
  if (!b) return text;
  for (let i = b.start; i < b.end; i++) {
    const m = view[i].match(/^-\s*\[ \]\s+(\S+)/); // only an untouched `[ ]` gate
    if (m && m[1] === gateId) {
      lines[i] = lines[i].replace(/^(-\s*\[) (\])/, "$1/$2");
      return lines.join(nl);
    }
  }
  return text;
}
