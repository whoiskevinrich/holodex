import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { contextFor, decideWrite, docTypeOfSkill } from "./doc-type-guard.mjs";

const GUARD = fileURLToPath(new URL("./doc-type-guard.mjs", import.meta.url));

test("docTypeOfSkill maps the three doc skills with or without a plugin prefix, nothing else", () => {
  assert.equal(docTypeOfSkill({ skill: "engineering:architecture" }), "architecture");
  assert.equal(docTypeOfSkill({ skill: "architecture" }), "architecture");
  assert.equal(docTypeOfSkill({ skill: "product-management:write-spec" }), "write-spec");
  assert.equal(docTypeOfSkill({ skill: "design:design-handoff" }), "design-handoff");
  assert.equal(docTypeOfSkill({ skill: "engineering:system-design" }), null);
  assert.equal(docTypeOfSkill({ skill: "architectures" }), null);
  assert.equal(docTypeOfSkill({}), null);
});

test("each doc type's context names its own boundary and the rules file", () => {
  assert.match(contextFor("architecture"), /never a new numbered/);
  assert.match(contextFor("architecture"), /no technology fork/);
  assert.match(contextFor("write-spec"), /Never include table names, endpoints/);
  assert.match(contextFor("design-handoff"), /Never state rules the backend enforces/);
  for (const t of ["architecture", "write-spec", "design-handoff"]) assert.match(contextFor(t), /doc-types\.md/);
  assert.equal(contextFor(null), null);
});

test("decideWrite blocks only creating a new numbered ADR", () => {
  assert.equal(decideWrite("Write", "docs/architecture/ADR-123-new-thing.md", false).block, true);
  assert.equal(decideWrite("Write", "G:\\repo\\docs\\architecture\\ADR-123-x.md", false).block, true);
  // editing or overwriting an existing ADR passes during the transition
  assert.equal(decideWrite("Write", "docs/architecture/ADR-091-fire-and-forget.md", true).block, false);
  assert.equal(decideWrite("Edit", "docs/architecture/ADR-123-x.md", false).block, false);
  // topic docs, the index and other trees pass
  assert.equal(decideWrite("Write", "docs/architecture/writeback.md", false).block, false);
  assert.equal(decideWrite("Write", "docs/architecture/README.md", false).block, false);
  assert.equal(decideWrite("Write", "docs/specs/ADR-1-not-an-adr.md", false).block, false);
  assert.equal(decideWrite("Write", undefined, false).block, false);
});

test("the hook exits 2 on a new ADR and injects context for a doc skill", () => {
  const block = spawnSync(process.execPath, [GUARD], {
    input: JSON.stringify({ tool_name: "Write", tool_input: { file_path: "docs/architecture/ADR-999-nope.md", content: "x" } }),
  });
  assert.equal(block.status, 2);
  assert.match(String(block.stderr), /new numbered ADRs are retired/);

  const ctx = spawnSync(process.execPath, [GUARD], {
    input: JSON.stringify({ tool_name: "Skill", tool_input: { skill: "design:design-handoff" } }),
  });
  assert.equal(ctx.status, 0);
  assert.match(JSON.parse(String(ctx.stdout)).hookSpecificOutput.additionalContext, /DESIGN doc/);

  const quiet = spawnSync(process.execPath, [GUARD], { input: "not json" });
  assert.equal(quiet.status, 0);
});
