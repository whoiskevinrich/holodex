#!/usr/bin/env node
// Jira single-issue transition (ADR-058 / ADR-064) — move ONE issue to a target status
// over the uncounted REST path. This is the entry Flightplan's SessionStart hook uses to
// fire "In Progress" at start-of-work (the one transition ADR-058 leaves to the agent/session
// rather than CI). Also usable standalone for any manual transition.
//
// Shares the idempotent + soft-fail plumbing in ./lib/jira-sync.mjs. Dependency-free
// (Node global fetch); talks only to Jira.
//
// SOFT-FAILS by design: a missing credential, an un-keyed branch, a Jira outage, or an
// unreachable transition logs a warning and exits 0 — starting a session must never be
// blocked by a Jira hiccup (ADR-064: the connector is never in the hot path).
//
// Env:
//   JIRA_ISSUE_KEY     the issue to transition (e.g. HOLODEX-182). If absent, derived from
//                      BRANCH_REF via the key regex.
//   BRANCH_REF         fallback key source — a branch name carrying the key.
//   JIRA_TARGET_STATUS optional — destination status name (default "In Progress").
//   JIRA_BASE_URL      required — the api.atlassian.com/ex/jira/<cloudId> gateway URL.
//   JIRA_USER_EMAIL    required — Atlassian account email for the API token.
//   JIRA_API_TOKEN     required — scoped Atlassian API token.
//   JIRA_KEY_PREFIX    optional — issue-key project prefix (default "HOLODEX").
//   DRY_RUN            optional — "true" to validate + log without POSTing.

import { makeLog, extractKeys, makeJiraClient, syncKeys } from "./lib/jira-sync.mjs";

const log = makeLog("jira-transition");
const { warn } = log;

const {
  JIRA_ISSUE_KEY,
  BRANCH_REF,
  JIRA_TARGET_STATUS = "In Progress",
  JIRA_BASE_URL,
  JIRA_USER_EMAIL,
  JIRA_API_TOKEN,
  JIRA_KEY_PREFIX = "HOLODEX",
  DRY_RUN,
} = process.env;

const dryRun = DRY_RUN === "true";

// Never fail the caller: any missing config is a warning + clean exit.
function bailSoft(msg) {
  warn(msg);
  process.exit(0);
}

const key =
  JIRA_ISSUE_KEY?.toUpperCase() ||
  extractKeys(BRANCH_REF ?? "", JIRA_KEY_PREFIX)[0] ||
  null;
if (!key) bailSoft(`no ${JIRA_KEY_PREFIX}-* key in JIRA_ISSUE_KEY/BRANCH_REF — nothing to do`);

const missing = Object.entries({ JIRA_BASE_URL, JIRA_USER_EMAIL, JIRA_API_TOKEN })
  .filter(([, v]) => !v)
  .map(([k]) => k);
if (missing.length) bailSoft(`missing required env: ${missing.join(", ")} — skipping transition`);

async function main() {
  log.info(`${dryRun ? "[dry-run] " : ""}transitioning ${key} → "${JIRA_TARGET_STATUS}"`);
  const client = makeJiraClient({
    baseUrl: JIRA_BASE_URL,
    email: JIRA_USER_EMAIL,
    token: JIRA_API_TOKEN,
  });
  await syncKeys({ keys: [key], targetStatus: JIRA_TARGET_STATUS, client, dryRun, log });
}

main().catch((err) => bailSoft(`unexpected error: ${err.message}`));
