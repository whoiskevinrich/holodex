#!/usr/bin/env node
// Jira release sync (ADR-058) — on the `prod` deploy of a `v*` tag, move every
// HOLODEX issue that is code-complete but not yet shipped to "Released".
//
// "What shipped" is read from JIRA STATE, not the diff: Holodex commit subjects are
// clean Conventional Commits (no keys — see docs/reference/jira-pipeline.md), so the
// changelog can't be parsed for issue keys the way Bookshelf's can. Instead we rely on
// the invariant that every merged PR was already moved to "Done" by jira-branch-sync:
// a release cut from `main` ships exactly the current `status = Done` set. So this is
// a batch transition of `project = HOLODEX AND status = Done` → "Released".
//
// Shares the idempotent + soft-fail plumbing in ./lib/jira-sync.mjs. Dependency-free
// (Node 22 global fetch); talks only to Jira.
//
// SOFT-FAILS by design: a Jira outage or a misconfigured secret logs a GitHub
// `::warning::` and exits 0, so it never red-builds a release that already shipped.
//
// Env:
//   JIRA_BASE_URL      required — the api.atlassian.com/ex/jira/<cloudId> gateway URL
//   JIRA_USER_EMAIL    required — Atlassian account email for the API token
//   JIRA_API_TOKEN     required — scoped Atlassian API token
//   JIRA_PROJECT       optional — project key (default "HOLODEX")
//   JIRA_SOURCE_STATUS optional — status that means "shipping now" (default "Done")
//   JIRA_TARGET_STATUS optional — status to move issues to (default "Released")
//   RELEASE_TAG        optional — e.g. "v1.4.0", for log context only
//   DRY_RUN            optional — "true" to validate + log without POSTing

import { makeLog, makeJiraClient, syncKeys } from "./lib/jira-sync.mjs";

const log = makeLog("jira-release-sync");
const { warn, info } = log;

const {
  JIRA_BASE_URL,
  JIRA_USER_EMAIL,
  JIRA_API_TOKEN,
  JIRA_PROJECT = "HOLODEX",
  JIRA_SOURCE_STATUS = "Done",
  JIRA_TARGET_STATUS = "Released",
  RELEASE_TAG,
  DRY_RUN,
} = process.env;

const dryRun = DRY_RUN === "true";

// Never fail the release: any missing config is a warning + clean exit.
function bailSoft(msg) {
  warn(msg);
  process.exit(0);
}

const missing = Object.entries({
  JIRA_BASE_URL,
  JIRA_USER_EMAIL,
  JIRA_API_TOKEN,
})
  .filter(([, v]) => !v)
  .map(([k]) => k);
if (missing.length) bailSoft(`missing required env: ${missing.join(", ")} — skipping Jira sync`);

async function main() {
  const client = makeJiraClient({
    baseUrl: JIRA_BASE_URL,
    email: JIRA_USER_EMAIL,
    token: JIRA_API_TOKEN,
  });

  // issuetype != Epic: epics are never auto-transitioned (HOLODEX-185, enforced
  // again in syncOne as the source of truth) — excluded here too so a parked
  // Done epic doesn't cost a wasted per-key GET on every release.
  const jql = `project = "${JIRA_PROJECT}" AND status = "${JIRA_SOURCE_STATUS}" AND issuetype != Epic`;
  let keys;
  try {
    keys = await client.searchIssueKeys(jql);
  } catch (err) {
    return bailSoft(`${err.message} — skipping Jira sync`);
  }

  if (keys.length === 0) {
    info(`no ${JIRA_PROJECT} issues in "${JIRA_SOURCE_STATUS}" — nothing to release`);
    return;
  }
  info(
    `${dryRun ? "[dry-run] " : ""}releasing ${keys.length} ticket(s)${RELEASE_TAG ? ` for ${RELEASE_TAG}` : ""} → "${JIRA_TARGET_STATUS}": ${keys.join(", ")}`,
  );

  await syncKeys({
    keys,
    targetStatus: JIRA_TARGET_STATUS,
    client,
    dryRun,
    log,
    context: RELEASE_TAG,
  });
  info("Jira sync complete");
}

main().catch((err) => bailSoft(`unexpected error: ${err.message}`));
