#!/usr/bin/env node
// Jira branch sync (ADR-058) — transition the single issue named by a PR's branch:
//   - PR ready for review → In Review
//   - PR merged           → Done
//
// This script is deliberately unaware of PR state: whether an event deserves a transition
// (a Draft PR gets none — ADR-069) is decided by the workflow's `if:`, which already has
// the event shape in scope. Here we only ever "transition this branch's keys to this status".
//
// Holodex carries the issue key in the BRANCH NAME (never the commit subject, which
// stays a clean Conventional Commit for release-please/git-cliff — see
// docs/reference/jira-pipeline.md). So the key source is the head branch ref, not the
// commit message. On a fork PR, GitHub withholds secrets and this simply no-ops.
//
// Shares the idempotent + soft-fail plumbing in ./lib/jira-sync.mjs. Dependency-free
// (Node 22 global fetch); talks only to Jira.
//
// SOFT-FAILS by design: a Jira outage, a missing key, or an unreachable transition
// logs a GitHub `::warning::` and exits 0, so it never fails the PR workflow.
//
// Env:
//   BRANCH_REF         required — the PR head branch name (e.g. GITHUB_HEAD_REF)
//   JIRA_TARGET_STATUS required — "In Review" or "Done"
//   JIRA_BASE_URL      required — the api.atlassian.com/ex/jira/<cloudId> gateway URL
//   JIRA_USER_EMAIL    required — Atlassian account email for the API token
//   JIRA_API_TOKEN     required — scoped Atlassian API token
//   JIRA_KEY_PREFIX    optional — issue-key project prefix (default "HOLODEX")
//   DRY_RUN            optional — "true" to validate + log without POSTing

import { makeLog, extractKeys, makeJiraClient, syncKeys } from "./lib/jira-sync.mjs";

const log = makeLog("jira-branch-sync");
const { warn, info } = log;

const {
  BRANCH_REF,
  JIRA_TARGET_STATUS,
  JIRA_BASE_URL,
  JIRA_USER_EMAIL,
  JIRA_API_TOKEN,
  JIRA_KEY_PREFIX = "HOLODEX",
  DRY_RUN,
} = process.env;

const dryRun = DRY_RUN === "true";

// Never fail the workflow: any missing config is a warning + clean exit.
function bailSoft(msg) {
  warn(msg);
  process.exit(0);
}

const missing = Object.entries({
  BRANCH_REF,
  JIRA_TARGET_STATUS,
  JIRA_BASE_URL,
  JIRA_USER_EMAIL,
  JIRA_API_TOKEN,
})
  .filter(([, v]) => !v)
  .map(([k]) => k);
if (missing.length) bailSoft(`missing required env: ${missing.join(", ")} — skipping Jira sync`);

async function main() {
  const keys = extractKeys(BRANCH_REF, JIRA_KEY_PREFIX);
  if (keys.length === 0) {
    info(`no ${JIRA_KEY_PREFIX}-* key in branch "${BRANCH_REF}" — nothing to sync`);
    return;
  }
  info(
    `${dryRun ? "[dry-run] " : ""}syncing ${keys.join(", ")} → "${JIRA_TARGET_STATUS}" (branch ${BRANCH_REF})`,
  );

  const client = makeJiraClient({
    baseUrl: JIRA_BASE_URL,
    email: JIRA_USER_EMAIL,
    token: JIRA_API_TOKEN,
  });
  await syncKeys({ keys, targetStatus: JIRA_TARGET_STATUS, client, dryRun, log });
  info("Jira sync complete");
}

main().catch((err) => bailSoft(`unexpected error: ${err.message}`));
