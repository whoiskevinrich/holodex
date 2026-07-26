import { test } from "node:test";
import assert from "node:assert/strict";
import {
  remainingHops,
  ladderLine,
  section,
  frontmatter,
  parseGates,
  parseUpNext,
  findWorklog,
} from "./whats-left.mjs";

const WORKLOG = `---
key: HOLODEX-123
status: In Progress
depends-on: [HOLODEX-117]
release_note: "Unified merge across entities."
---

## Gates
- [x] spec          docs/specs/x.md
- [x] architecture  ADR-051
- [/] backend       internal/resolver
- [ ] frontend
- [~] testing       deferred until: backend merged
- [ ] security

## Up next
1. Wire decision-chips [frontend]
2. Facet-switch [frontend] -> HOLODEX-120

## Session log
S4 · /write-spec
`;

test("remainingHops lists the hops after the current status", () => {
  assert.deepEqual(remainingHops("Done"), ["Released"]);
  assert.deepEqual(remainingHops("To Do"), ["In Progress", "In Review", "Done", "Released"]);
  assert.deepEqual(remainingHops("Released"), []);
});

test("remainingHops is case-insensitive and flags off-ladder statuses", () => {
  assert.deepEqual(remainingHops("in review"), ["Done", "Released"]);
  assert.equal(remainingHops("Blocked"), null);
});

test("ladderLine brackets the current rung", () => {
  assert.match(ladderLine("In Progress"), /\[In Progress\]/);
  assert.ok(!ladderLine("In Progress").includes("[Done]"));
});

test("section returns lines up to the next heading", () => {
  const lines = section(WORKLOG, "Up next").filter((l) => l.trim());
  assert.equal(lines.length, 2);
  assert.ok(!lines.some((l) => l.includes("Session log")));
});

test("parseGates reads every checkbox state", () => {
  const gates = parseGates(WORKLOG);
  assert.equal(gates.length, 6);
  assert.equal(gates.filter((g) => g.state !== "x").length, 4);
  assert.deepEqual(
    gates.map((g) => g.state),
    ["x", "x", "/", " ", "~", " "],
  );
  assert.match(gates[4].label, /deferred until: backend merged/);
});

test("parseUpNext keeps only the ordered items, in order", () => {
  const up = parseUpNext(WORKLOG);
  assert.equal(up.length, 2);
  assert.match(up[0], /^1\. Wire decision-chips/);
});

test("frontmatter pulls declared fields", () => {
  assert.equal(frontmatter(WORKLOG, "depends-on"), "[HOLODEX-117]");
  assert.match(frontmatter(WORKLOG, "release_note"), /Unified merge/);
  assert.equal(frontmatter(WORKLOG, "nope"), null);
});

// The scaffolded template documents every field with a trailing comment; before this was handled,
// `depends-on: []  # …` read as a real dependency and an unset `release_note:` reported as set.
test("frontmatter drops the scaffolded inline comments", () => {
  const scaffold = `---
key: HOLODEX-123                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff
---
`;
  assert.equal(frontmatter(scaffold, "key"), "HOLODEX-123");
  assert.equal(frontmatter(scaffold, "status"), "in-progress");
  assert.equal(frontmatter(scaffold, "depends-on"), "[]"); // → not blocked
  assert.equal(frontmatter(scaffold, "release_note"), null); // → UNSET, as it should be
});

test("frontmatter keeps a `#` that is part of the value", () => {
  const fm = `---
release_note: "Fixes #123 and #124."
status: in-progress#not-a-comment
---
`;
  // A quoted scalar is taken whole — `#` inside quotes is literal, not a comment.
  assert.equal(frontmatter(fm, "release_note"), '"Fixes #123 and #124."');
  // YAML only opens a comment after whitespace, so a bare `#` mid-token stays in the value.
  assert.equal(frontmatter(fm, "status"), "in-progress#not-a-comment");
});

test("findWorklog matches by filename prefix", () => {
  const fs = {
    readdirSync: () => ["HOLODEX-123.md", "other-plan.md"],
    readFileSync: (p) => (p.endsWith("HOLODEX-123.md") ? WORKLOG : "unrelated"),
  };
  const wl = findWorklog("HOLODEX-123", "/plans", fs);
  assert.ok(wl && wl.path.endsWith("HOLODEX-123.md"));
});

test("findWorklog falls back to a frontmatter key match on any filename", () => {
  const fs = {
    readdirSync: () => ["studio-entity-implementation.md"],
    readFileSync: () => WORKLOG,
  };
  const wl = findWorklog("HOLODEX-123", "/plans", fs);
  assert.ok(wl, "should find worklog via `key:` frontmatter regardless of filename");
});

test("findWorklog returns null when nothing matches", () => {
  const fs = { readdirSync: () => ["a.md"], readFileSync: () => "no key here" };
  assert.equal(findWorklog("HOLODEX-999", "/plans", fs), null);
});
