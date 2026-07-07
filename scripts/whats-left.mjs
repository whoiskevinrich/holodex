#!/usr/bin/env node
// whats-left.mjs — "I have a piece of work; what's left to merge to prod?"
//
// The runnable form of docs/reference/workflow-idea-to-merge.md § "Answering
// what's left to merge to prod?". Given a HOLODEX key it prints:
//   - the status-ladder position and the remaining hops to Released (= in prod)
//   - for an Epic: the worklog gates + ordered Up next (the fine-grained remainder),
//     and the child issues not yet Done
//   - for a Story/Task/Bug: its remaining hops + a pointer to its parent epic's worklog
//
// "In prod" = Released (shipped in a tagged GHCR image), NOT merged. Done just means
// it's on main; there is always a release hop after merge (ADR-058 / jira-pipeline.md).
//
// Dependency-free (Node 22 global fetch; fs). Read-only — never transitions anything.
// Pure parse helpers are exported for scripts/whats-left.test.mjs; main() runs only
// when the file is executed directly.
//
// Usage:
//   node scripts/whats-left.mjs HOLODEX-18
//
// Env (same names as the CI Jira scripts):
//   JIRA_USER_EMAIL  required — Atlassian account email for the API token
//   JIRA_API_TOKEN   required — scoped Atlassian API token
//                    (create one: https://id.atlassian.com/manage-profile/security/api-tokens)
//   JIRA_BASE_URL    optional — defaults to this site's api.atlassian.com gateway.
//                    MUST be the gateway host, not <site>.atlassian.net (a scoped
//                    token 401s against the site URL — ADR-058 trap #2).
//   JIRA_KEY_PREFIX  optional — project prefix (default "HOLODEX")

import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";

export const LADDER = ["To Do", "In Progress", "In Review", "Done", "Released"];
const PLANS_DIR = fileURLToPath(new URL("../docs/plans", import.meta.url));
const KEY_PREFIX = process.env.JIRA_KEY_PREFIX ?? "HOLODEX";

function die(msg) {
  console.error(`whats-left: ${msg}`);
  process.exit(1);
}

// ---- pure helpers (exported for tests) --------------------------------------

// Remaining ladder hops after the current status; null if status is off-ladder.
export function remainingHops(status) {
  const i = LADDER.findIndex((s) => s.toLowerCase() === (status ?? "").toLowerCase());
  return i < 0 ? null : LADDER.slice(i + 1);
}

export function ladderLine(status) {
  const cur = (status ?? "").toLowerCase();
  return LADDER.map((s) => (s.toLowerCase() === cur ? `[${s}]` : s)).join(" -> ");
}

// Lines under a `## Heading`, up to the next `## `.
export function section(text, heading) {
  const lines = text.split(/\r?\n/);
  const start = lines.findIndex((l) =>
    l.trim().toLowerCase().startsWith(`## ${heading.toLowerCase()}`),
  );
  if (start < 0) return [];
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((l) => l.startsWith("## "));
  return (end < 0 ? rest : rest.slice(0, end)).map((l) => l.trimEnd());
}

export function frontmatter(text, field) {
  const m = text.match(new RegExp(`^${field}:\\s*(.+)$`, "im"));
  return m ? m[1].trim() : null;
}

// Parse the `## Gates` checklist into { state, label } rows. state ∈ { ' ', '/', '~', 'x' }.
export function parseGates(text) {
  return section(text, "Gates")
    .map((l) => l.match(/^-\s*\[([ x/~])\]\s*(.+)$/i))
    .filter(Boolean)
    .map((m) => ({ state: m[1].toLowerCase(), label: m[2].trim() }));
}

// Numbered items under `## Up next`.
export function parseUpNext(text) {
  return section(text, "Up next")
    .map((l) => l.trim())
    .filter((l) => /^\d+\.\s/.test(l));
}

// Find the worklog for KEY: filename prefix first, else any plan file whose
// frontmatter `key:` matches. Returns { path, text } or null.
export function findWorklog(k, dir = PLANS_DIR, fs = { readdirSync, readFileSync }) {
  let files;
  try {
    files = fs.readdirSync(dir);
  } catch {
    return null;
  }
  const md = files.filter((f) => f.endsWith(".md"));
  const byName = md.find(
    (f) => f.toUpperCase().startsWith(k + ".") || f.toUpperCase().startsWith(k + "-"),
  );
  for (const f of byName ? [byName] : md) {
    const path = `${dir}/${f}`;
    const text = fs.readFileSync(path, "utf8");
    // Compare via a literal frontmatter parse — never build a regex from `k`
    // (a CLI argument), which would be a regex-injection sink.
    if (byName || frontmatter(text, "key")?.toUpperCase() === k) return { path, text };
  }
  return null;
}

// ---- reporting + I/O --------------------------------------------------------

function reportWorklog(text) {
  const gates = parseGates(text);
  if (gates.length) {
    const left = gates.filter((g) => g.state !== "x");
    console.log(`\nGates (definition of done) — ${left.length} of ${gates.length} remain:`);
    const mark = { " ": "[ ]", "/": "[~ in progress]", "~": "[~ deferred]", x: "[x done]" };
    for (const g of gates) console.log(`  ${mark[g.state] ?? "[?]"} ${g.label}`);
  }
  const up = parseUpNext(text);
  if (up.length) {
    console.log(`\nUp next (ordered — top is the next action):`);
    for (const l of up) console.log(`  ${l}`);
  }
  const dep = frontmatter(text, "depends-on");
  if (dep && dep !== "[]") console.log(`\nBlocked by (must reach Released first): ${dep}`);
  const rn = frontmatter(text, "release_note");
  console.log(
    `\nrelease_note: ${rn ? `set -> "${rn.replace(/^["']|["']$/g, "")}"` : "UNSET (required before the epic can close)"}`,
  );
}

async function main() {
  const key = (process.argv[2] ?? "").toUpperCase();
  if (!key) die(`usage: node scripts/whats-left.mjs ${KEY_PREFIX}-<number>`);
  if (!new RegExp(`^${KEY_PREFIX}-\\d+$`).test(key)) die(`"${key}" is not a ${KEY_PREFIX} key`);

  const { JIRA_USER_EMAIL, JIRA_API_TOKEN } = process.env;
  const baseUrl =
    process.env.JIRA_BASE_URL ??
    "https://api.atlassian.com/ex/jira/e7c03552-8036-43fa-bb8b-b415de46f9f6";
  const missing = Object.entries({ JIRA_USER_EMAIL, JIRA_API_TOKEN })
    .filter(([, v]) => !v)
    .map(([k]) => k);
  if (missing.length)
    die(
      `missing env: ${missing.join(", ")}. Set JIRA_USER_EMAIL + JIRA_API_TOKEN ` +
        `(token: https://id.atlassian.com/manage-profile/security/api-tokens).`,
    );

  const base = baseUrl.replace(/\/+$/, "");
  const headers = {
    Authorization:
      "Basic " + Buffer.from(`${JIRA_USER_EMAIL}:${JIRA_API_TOKEN}`).toString("base64"),
    Accept: "application/json",
  };
  const jira = async (path) => {
    const res = await fetch(`${base}${path}`, { headers });
    if (res.status === 404) return null;
    if (!res.ok) die(`Jira GET ${path} failed: ${res.status} ${res.statusText}`);
    return res.json();
  };

  const issue = await jira(`/rest/api/3/issue/${key}?fields=summary,status,issuetype,parent`);
  if (!issue) die(`${key} not found in Jira.`);
  const { summary, status, issuetype, parent } = issue.fields;
  const type = issuetype?.name ?? "Issue";
  const statusName = status?.name ?? "(unknown)";

  console.log(`${key} · ${summary}  (${type})`);
  console.log(`Ladder: ${ladderLine(statusName)}`);
  const hops = remainingHops(statusName);
  if (statusName.toLowerCase() === "released") console.log(`  already in prod (Released).`);
  else if (hops === null)
    console.log(`  currently "${statusName}" (off the standard ladder) — check manually.`);
  else console.log(`  currently "${statusName}" — remaining to prod: ${hops.join(" -> ")}`);

  if (type.toLowerCase() === "epic") {
    const wl = findWorklog(key);
    if (wl) {
      console.log(`\nWorklog: ${wl.path.replace(/\\/g, "/")}`);
      reportWorklog(wl.text);
    } else {
      console.log(`\nWorklog: none found in docs/plans/ — this epic is NOT yet shaped.`);
      console.log(`  Fine-grained "what's left" isn't available; falling back to child statuses.`);
    }
    const search = await jira(
      `/rest/api/3/search/jql?jql=${encodeURIComponent(`parent = ${key}`)}&fields=status&maxResults=100`,
    );
    const kids = search?.issues ?? [];
    if (kids.length) {
      const open = kids.filter(
        (i) => !["done", "released"].includes((i.fields.status?.name ?? "").toLowerCase()),
      );
      console.log(`\nChildren: ${open.length} of ${kids.length} still open (not Done/Released).`);
      const byStatus = {};
      for (const i of open) (byStatus[i.fields.status?.name ?? "?"] ??= []).push(i.key);
      for (const [s, keys] of Object.entries(byStatus)) console.log(`  ${s}: ${keys.join(", ")}`);
    }
  } else if (parent) {
    console.log(`\nParent epic: ${parent.key} · ${parent.fields?.summary ?? ""}`);
    console.log(
      `  This ${type} inherits its epic's gates — run: node scripts/whats-left.mjs ${parent.key}`,
    );
  }
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  main().catch((err) => die(`unexpected error: ${err.message}`));
}
