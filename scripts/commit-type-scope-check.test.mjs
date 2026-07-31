import { test } from "node:test";
import assert from "node:assert/strict";
import { classify, parseCommitType, renderComment, renderResolvedComment } from "./commit-type-scope-check.mjs";

test("parseCommitType reads a scoped Conventional Commit type", () => {
  assert.equal(parseCommitType("docs(tags): F50 spec + ADR-075"), "docs");
  assert.equal(parseCommitType("chore(flightplan): tidy worklog"), "chore");
  assert.equal(parseCommitType("feat(tags): add hierarchy"), "feat");
});

test("parseCommitType reads an unscoped or breaking-change type", () => {
  assert.equal(parseCommitType("chore: tidy up"), "chore");
  assert.equal(parseCommitType("feat!: breaking change"), "feat");
});

test("parseCommitType returns null for a non-conventional title", () => {
  assert.equal(parseCommitType("Fix the thing"), null);
  assert.equal(parseCommitType(""), null);
  assert.equal(parseCommitType(undefined), null);
});

test("classify ignores non-advisory types entirely", () => {
  const result = classify({
    title: "feat(tags): add hierarchy",
    files: ["internal/repo/tag_hierarchy.go", "internal/api/tag_materialize.go"],
  });
  assert.deepEqual(result, { flagged: false, type: "feat", matched: [], migrationTouched: false });
});

// This is the PR #187 scenario that motivated ADR-076: a docs(...)-typed PR that actually
// shipped the full F50 implementation across Go, Svelte, and migrations.
test("classify flags a docs(...) PR whose diff is a real implementation", () => {
  const result = classify({
    title: "docs(tags): F50 spec + ADR-075",
    files: [
      "docs/specs/tag-governance-and-video-enrichment.md",
      "docs/architecture/ADR-075-tag-governance-and-video-enrichment.md",
      "internal/api/tag_materialize.go",
      "internal/repo/tag_hierarchy.go",
      "internal/db/migrations/0030_tag_hierarchy.up.sql",
      "web/src/routes/owner/tags/+page.svelte",
    ],
  });
  assert.equal(result.flagged, true);
  assert.equal(result.type, "docs");
  assert.equal(result.migrationTouched, true);
  assert.deepEqual(result.matched.sort(), [
    "internal/api/tag_materialize.go",
    "internal/repo/tag_hierarchy.go",
    "web/src/routes/owner/tags/+page.svelte",
  ].sort());
});

test("classify tolerates a single non-doc file touched by a docs(...) PR", () => {
  const result = classify({
    title: "docs(tags): fix ADR cross-reference",
    files: ["docs/architecture/ADR-004-metadata-extraction.md", "internal/api/handler.go"],
  });
  assert.equal(result.flagged, false);
  assert.deepEqual(result.matched, ["internal/api/handler.go"]);
});

test("classify flags two or more non-doc files touched by a chore(...) PR", () => {
  const result = classify({
    title: "chore(deps): bump go modules",
    files: ["internal/api/handler.go", "internal/repo/video.go", "go.mod"],
  });
  assert.equal(result.flagged, true);
  assert.equal(result.matched.length, 2);
});

// cmd/ (entrypoint) and providers/ (the TMDB sidecar) are real Go product surfaces
// alongside internal/ — a mistyped PR shipping code there must be caught the same way.
test("classify flags a docs(...) PR touching the sidecar or the entrypoint", () => {
  const result = classify({
    title: "docs(provider): notes",
    files: ["providers/tmdb/client.go", "cmd/holodex/main.go"],
  });
  assert.equal(result.flagged, true);
  assert.deepEqual(result.matched.sort(), ["cmd/holodex/main.go", "providers/tmdb/client.go"]);
});

test("classify flags any migration file regardless of count", () => {
  const result = classify({
    title: "chore(db): tidy schema",
    files: ["internal/db/migrations/0031_denied_tags.up.sql"],
  });
  assert.equal(result.flagged, true);
  assert.equal(result.migrationTouched, true);
  assert.deepEqual(result.matched, []);
});

test("classify tolerates missing/empty file list", () => {
  assert.deepEqual(classify({ title: "docs: update readme", files: [] }), {
    flagged: false,
    type: "docs",
    matched: [],
    migrationTouched: false,
  });
  assert.deepEqual(classify({ title: "docs: update readme", files: undefined }), {
    flagged: false,
    type: "docs",
    matched: [],
    migrationTouched: false,
  });
});

test("renderComment includes the marker, type, and matched files", () => {
  const body = renderComment({
    type: "docs",
    matched: ["internal/api/tag_materialize.go"],
    migrationTouched: true,
  });
  assert.match(body, /<!-- holodex-commit-type-scope -->/);
  assert.match(body, /`docs\(\.\.\.\)`/);
  assert.match(body, /internal\/api\/tag_materialize\.go/);
  assert.match(body, /DB migration is never legitimately docs-only/);
});

test("renderResolvedComment carries the same marker for the upsert match", () => {
  assert.match(renderResolvedComment(), /<!-- holodex-commit-type-scope -->/);
});
