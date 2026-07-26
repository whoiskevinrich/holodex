import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flipGate, logSkillRun, maskComments, parseGates, parseWorklogText } from "./worklog.mjs";

// The scaffold is the highest-risk input: it is what every new epic starts from, and its
// commented-out examples are what the section scanners used to mistake for real content.
const SCAFFOLD = readFileSync(fileURLToPath(new URL("../templates/worklog.md", import.meta.url)), "utf8");

// True when `marker` lands inside an unclosed HTML comment. `--!>` closes a comment just as `-->`
// does, so both end forms are counted (matching only `-->` is CodeQL's js/bad-tag-filter).
function insideComment(text, marker) {
  const before = text.slice(0, text.indexOf(marker));
  return (before.match(/<!--/g) ?? []).length > (before.match(/--!?>/g) ?? []).length;
}

test("maskComments preserves line count and column positions", () => {
  const src = "a\n<!-- one\ntwo -->\nb";
  const out = maskComments(src);
  assert.equal(out.split("\n").length, src.split("\n").length);
  assert.equal(out.length, src.length);
  assert.equal(out, `a\n${" ".repeat(8)}\n${" ".repeat(7)}\nb`); // "<!-- one" / "two -->"
  assert.equal(out.replace(/ /g, ""), "a\n\n\nb"); // nothing but whitespace survives the comment
});

// HTML accepts `--!>` as a comment end. Matching only `-->` left such a comment open, so the mask
// ran past it and blanked real content — a gate row after it would vanish from the count.
test("maskComments honours the --!> comment end", () => {
  assert.equal(maskComments("a<!-- x --!>b"), `a${" ".repeat(11)}b`);
  // Multi-line is where it bites: with the end tag unrecognised the comment never closes, so the
  // example row inside it survives the mask and gets counted as a real gate.
  const t = "## Gates\n<!-- note\n- [ ] commented example\n--!>\n- [ ] frontend\n";
  assert.deepEqual(
    parseGates(t).map((g) => g.label),
    ["frontend"],
  );
});

test("parseWorklogText counts only real gates in the scaffold", () => {
  // 7 configured gates; the deferred-gate example in the trailing HTML comment is not one of them.
  const wl = parseWorklogText(SCAFFOLD);
  assert.equal(wl.gatesTotal, 7);
  assert.equal(wl.gatesDone, 0);
});

test("parseWorklogText reads a status with a trailing YAML comment", () => {
  // The template ships `status: todo   # todo | in-progress | …`; the option list is a comment.
  assert.equal(parseWorklogText(SCAFFOLD).status, "todo");
});

test("parseWorklogText takes Up next items with or without a checkbox", () => {
  const withBox = "## Up next\n1. [ ] do the thing\n";
  const without = "## Up next\n1. do the thing\n";
  assert.equal(parseWorklogText(withBox).next, "do the thing");
  assert.equal(parseWorklogText(without).next, "do the thing");
});

test("parseWorklogText skips done and deferred items when picking next", () => {
  const t = "## Up next\n1. [x] shipped\n2. [~] deferred\n3. [ ] the real next\n";
  assert.equal(parseWorklogText(t).next, "the real next");
});

test("parseWorklogText collects blockers", () => {
  const t = "## Up next\n1. [ ] merge the thing ⛔ blocked on HOLODEX-114\n";
  assert.deepEqual(parseWorklogText(t).blockers, ["merge the thing"]);
});

test("logSkillRun writes a new entry outside the commented-out example", () => {
  const out = logSkillRun(SCAFFOLD, "design-handoff", "2026-07-26");
  assert.ok(out.includes("### 2026-07-26 · session"));
  assert.equal(insideComment(out, "### 2026-07-26"), false);
  assert.ok(out.includes("- skills: design-handoff"));
});

test("logSkillRun merges into today's entry and is idempotent", () => {
  const once = logSkillRun(SCAFFOLD, "design-handoff", "2026-07-26");
  const twice = logSkillRun(once, "write-spec", "2026-07-26");
  assert.match(twice, /- skills: design-handoff, write-spec/);
  assert.equal(logSkillRun(twice, "write-spec", "2026-07-26"), twice); // already logged → no-op
});

test("logSkillRun preserves CRLF", () => {
  const crlf = SCAFFOLD.replace(/\n/g, "\r\n");
  const out = logSkillRun(crlf, "write-spec", "2026-07-26");
  assert.ok(out.includes("\r\n"));
  assert.ok(!/[^\r]\n/.test(out), "should not introduce a bare LF");
});

test("flipGate marks only the named untouched gate", () => {
  const out = flipGate(SCAFFOLD, "design");
  assert.match(out, /- \[\/\] design `design-handoff`/);
  assert.match(out, /- \[ \] spec `write-spec`/); // siblings untouched
});

test("flipGate leaves an already-marked gate and an unknown id alone", () => {
  const done = "## Gates\n- [x] spec\n- [~] security\n";
  assert.equal(flipGate(done, "spec"), done);
  assert.equal(flipGate(done, "security"), done);
  assert.equal(flipGate(SCAFFOLD, "nope"), SCAFFOLD);
});

test("flipGate will not edit a gate inside a comment", () => {
  const t = "## Gates\n<!-- - [ ] spec example -->\n- [ ] frontend\n";
  assert.equal(flipGate(t, "spec"), t);
  assert.match(flipGate(t, "frontend"), /- \[\/\] frontend/);
});
