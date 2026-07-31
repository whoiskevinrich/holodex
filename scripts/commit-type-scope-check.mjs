#!/usr/bin/env node
// Advisory CI check (ADR-076) — flags a PR whose title is typed `docs`/`chore` per
// Conventional Commits but whose diff touches files that look like real implementation,
// not docs/tooling.
//
// This repo squash-merges (`allow_squash_merge` only), so the PR TITLE is what actually
// lands as the merge commit message and what release-please/git-cliff parse — not any
// individual commit on the branch (docs/reference/jira-pipeline.md makes the same
// assumption for the Jira key: branch name, not commit subject). Reading commit messages
// here would be the wrong signal.
//
// Deliberately ADVISORY: posts/updates a sticky PR comment, never a failing status check.
// A `docs(...)` commit touching one non-doc line (e.g. a comment fix while writing a spec)
// is a legitimate false positive — classify() tolerates a single matched file; a touched
// migration is never tolerated (a schema change is never legitimately docs-only).

import { pathToFileURL } from "node:url";
import { run } from "./lib/imagetools.mjs";
import { findCommentId, upsertIssueComment } from "./lib/gh-comment.mjs";

export const COMMENT_MARKER = "<!-- holodex-commit-type-scope -->";
const ADVISORY_TYPES = new Set(["docs", "chore"]);
// Real Go/Svelte/TS product code. cmd/ and providers/ sit alongside internal/ as genuine
// product surfaces (the sidecar, the entrypoint) — a mistyped PR shipping code there is
// the same failure this check exists to catch, not a narrower case of it.
const NON_DOC_GLOBS = [
  /^internal\/.*\.go$/,
  /^cmd\/.*\.go$/,
  /^providers\/.*\.go$/,
  /^web\/src\/.*\.svelte$/,
  /^web\/src\/.*\.ts$/,
];
const MIGRATIONS_RE = /^internal\/db\/migrations\//;

/** Conventional-Commit type from a PR title, e.g. "docs(tags): ..." -> "docs". Null if unparseable. */
export function parseCommitType(title) {
  const m = (title ?? "").match(/^(\w+)(\([^)]*\))?!?:\s/);
  return m ? m[1].toLowerCase() : null;
}

/**
 * Pure classifier: does this (title, changed-files) pair look like a mistyped PR?
 * Threshold: flag when more than one non-doc/tooling file changed, OR any migration file
 * changed (regardless of count) — a migration is never legitimately docs-only.
 */
export function classify({ title, files }) {
  const type = parseCommitType(title);
  if (!ADVISORY_TYPES.has(type)) {
    return { flagged: false, type, matched: [], migrationTouched: false };
  }

  const matched = (files ?? []).filter((f) => NON_DOC_GLOBS.some((re) => re.test(f)));
  const migrationTouched = (files ?? []).some((f) => MIGRATIONS_RE.test(f));

  return { flagged: migrationTouched || matched.length > 1, type, matched, migrationTouched };
}

export function renderComment({ type, matched, migrationTouched }) {
  return [
    COMMENT_MARKER,
    "## ⚠️ Commit type looks narrower than the diff",
    "",
    `This PR is titled \`${type}(...)\`, which \`release-please\`/\`git-cliff\` hide from the ` +
      "user-facing CHANGELOG by design (see CLAUDE.md). But its diff touches files that look " +
      "like real implementation, not docs/tooling:",
    "",
    ...matched.map((f) => `- \`${f}\``),
    "",
    migrationTouched
      ? "A DB migration is never legitimately docs-only."
      : `${matched.length} non-doc/tooling file(s) changed.`,
    "",
    "If this PR is genuinely just docs/tooling, ignore this. If it also ships product or " +
      "infra code, retype the title (e.g. `feat(...)`/`fix(...)`) so it appears in the " +
      "release notes and the change-routing gates (spec/ADR/design/testing) apply.",
    "",
    "---",
    "<sub>Advisory only — does not block the merge (ADR-076).</sub>",
  ].join("\n");
}

export function renderResolvedComment() {
  return [COMMENT_MARKER, "✅ Commit type now matches the diff scope."].join("\n");
}

async function main() {
  const repo = process.env.GITHUB_REPOSITORY;
  const prNumber = process.env.PR_NUMBER;
  const prTitle = process.env.PR_TITLE ?? "";
  if (!repo || !prNumber) throw new Error("GITHUB_REPOSITORY and PR_NUMBER are required");

  // Independent reads — run concurrently rather than paying two sequential round-trips.
  const [filesOut, existingId] = await Promise.all([
    run("gh", ["api", `repos/${repo}/pulls/${prNumber}/files`, "--paginate", "-q", ".[].filename"]),
    findCommentId(repo, prNumber, COMMENT_MARKER),
  ]);
  const files = filesOut ? filesOut.split("\n").filter(Boolean) : [];

  const result = classify({ title: prTitle, files });

  const body = result.flagged
    ? renderComment(result)
    : existingId
      ? renderResolvedComment()
      : null;

  if (body === null) {
    process.stdout.write("No commit-type/diff-scope mismatch — nothing to comment.\n");
    return;
  }

  await upsertIssueComment({ repo, issueNumber: prNumber, id: existingId, body });
  process.stdout.write(
    `${existingId ? `Updated comment ${existingId}` : "Created comment"} on PR #${prNumber}: ` +
      `flagged=${result.flagged}, type=${result.type}, matched=${result.matched.length}, ` +
      `migrations=${result.migrationTouched}\n`,
  );
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  main().catch((err) => {
    // Soft-fail by design: this check is advisory and must never block the PR.
    process.stderr.write(`::warning::${err.message}\n`);
    process.exit(0);
  });
}
