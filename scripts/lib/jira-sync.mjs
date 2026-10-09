// Shared, dependency-free helpers for CI-side Jira status sync (ADR-058).
//
// Two entry points build on this:
//   - jira-branch-sync.mjs  → In Review (PR ready for review) / Done (PR merged), keyed off
//                             the PR *branch name* (Holodex carries the key there,
//                             never in the commit subject — see docs/reference/jira-pipeline.md)
//   - jira-release-sync.mjs → Released, on the `prod` deploy: every HOLODEX issue
//                             currently in "Done" is what this tag ships (batch)
//
// All transitions are idempotent (skip a ticket already at the target) and
// SOFT-FAIL by design: a Jira outage, a missing key, or an unreachable transition
// logs a GitHub `::warning::` and the run continues — a Jira hiccup must never
// red-build a deploy or block a merge that already passed. Node 22 global
// `fetch`/`Buffer`; no dependencies.
//
// NOTE on baseUrl: it MUST be the Atlassian *gateway* host
// `https://api.atlassian.com/ex/jira/<cloudId>` — a scoped API token 401s against
// the `<site>.atlassian.net` URL (ADR-058 trap #2).

import { frontmatter, maskComments } from "./worklog.mjs";

export function makeLog(label = "jira-sync") {
  return {
    warn: (msg) => console.log(`::warning::[${label}] ${msg}`),
    info: (msg) => console.log(`[${label}] ${msg}`),
  };
}

// All unique, upper-cased issue keys in `text` for the given project prefix.
export function extractKeys(text, prefix = "HOLODEX") {
  const re = new RegExp(`\\b${prefix}-\\d+\\b`, "gi");
  const keys = new Set();
  for (const m of (text ?? "").matchAll(re)) keys.add(m[0].toUpperCase());
  return [...keys];
}

// Pure: pick the transition whose DESTINATION status matches (case-insensitive) —
// robust to transition naming like "Done" vs "Mark as Done".
export function selectTransition(transitions, targetStatus) {
  const target = targetStatus.toLowerCase();
  return (transitions ?? []).find((tr) => tr.to?.name?.toLowerCase() === target) ?? null;
}

export function selectTransitionId(transitions, targetStatus) {
  return selectTransition(transitions, targetStatus)?.id ?? null;
}

// Jira's fixed statusCategory order. A category not listed here (e.g. the
// `undefined` one Jira uses for unmapped statuses) is unranked.
const CATEGORY_RANK = { new: 0, indeterminate: 1, done: 2 };

// Pure: true when moving from `fromCategory` to `toCategory` goes to an earlier
// statusCategory. Unranked on either side → not backwards (let it through).
export function isBackwards(fromCategory, toCategory) {
  const from = CATEGORY_RANK[fromCategory];
  const to = CATEGORY_RANK[toCategory];
  return from !== undefined && to !== undefined && to < from;
}

export function makeJiraClient({ baseUrl, email, token }) {
  const base = baseUrl.replace(/\/+$/, "");
  const headers = {
    Authorization: "Basic " + Buffer.from(`${email}:${token}`).toString("base64"),
    Accept: "application/json",
    "Content-Type": "application/json",
  };
  return {
    async currentStatus(key) {
      const res = await fetch(`${base}/rest/api/3/issue/${key}?fields=status,issuetype`, {
        headers,
      });
      if (res.status === 404) return { missing: true };
      if (!res.ok) throw new Error(`GET issue ${key} failed: ${res.status} ${res.statusText}`);
      const json = await res.json();
      return {
        status: json.fields?.status?.name ?? null,
        issueType: json.fields?.issuetype?.name ?? null,
        category: json.fields?.status?.statusCategory?.key ?? null,
      };
    },
    // The transition reaching `targetStatus` as { id, toCategory }, or null.
    async findTransition(key, targetStatus) {
      const res = await fetch(`${base}/rest/api/3/issue/${key}/transitions`, { headers });
      if (!res.ok)
        throw new Error(`GET transitions for ${key} failed: ${res.status} ${res.statusText}`);
      const json = await res.json();
      const t = selectTransition(json.transitions, targetStatus);
      return t ? { id: t.id, toCategory: t.to?.statusCategory?.key ?? null } : null;
    },
    async transition(key, transitionId) {
      const res = await fetch(`${base}/rest/api/3/issue/${key}/transitions`, {
        method: "POST",
        headers,
        body: JSON.stringify({ transition: { id: transitionId } }),
      });
      if (!res.ok)
        throw new Error(
          `POST transition ${transitionId} on ${key} failed: ${res.status} ${res.statusText}`,
        );
    },
    // Every issue key matching `jql`, following pagination. Used by the release
    // sync to find "what's shipping" as `status = Done` — Holodex's clean commit
    // subjects don't carry keys, so we read the set from Jira state, not the diff.
    async searchIssueKeys(jql) {
      const keys = [];
      let nextPageToken;
      do {
        const params = new URLSearchParams({ jql, fields: "key", maxResults: "100" });
        if (nextPageToken) params.set("nextPageToken", nextPageToken);
        const res = await fetch(`${base}/rest/api/3/search/jql?${params}`, { headers });
        if (!res.ok)
          throw new Error(`JQL search failed: ${res.status} ${res.statusText}`);
        const json = await res.json();
        for (const issue of json.issues ?? []) keys.push(issue.key);
        nextPageToken = json.nextPageToken;
      } while (nextPageToken);
      return keys;
    },
  };
}

// A `name: [a, b]` flow list under a top-level `block:` of flightplan.yaml, or null. Enough YAML
// for the two lists the docs-only exemption reads; a shape it doesn't recognise reads as absent.
function flowList(yaml, block, name) {
  if (!/^[\w-]+$/.test(name)) return null; // a posture name, never a pattern
  const body = yaml.match(new RegExp(`^${block}:[^\\n]*\\n((?:[ \\t]+[^\\n]*\\n?|[ \\t]*\\n)*)`, "m"))?.[1];
  const items = body?.match(new RegExp(`^[ \\t]+${name}:[ \\t]*\\[([^\\]]*)\\]`, "m"))?.[1];
  return items === undefined ? null : items.split(",").map((s) => s.trim()).filter(Boolean);
}

// Whether a docs-only merge may fire Done for this key (HOLODEX-550). The guard below exists
// for a gate artifact (spec/design/worklog) merging ahead of its implementation, and that
// can only happen to an epic whose posture has a design-phase gate. A worklog whose
// `profile:` names a posture with none (e.g. `chore`) is a ticket whose whole job is the
// docs, so its merge is the finish. Anything unreadable — no worklog, an unset or unknown
// profile, no `phases.design` — keeps the guard.
export function docsOnlyExempt(worklogText, flightplanText) {
  if (!worklogText || !flightplanText) return false;
  const profile = frontmatter(maskComments(worklogText), "profile")?.replace(/^(['"])(.*)\1$/, "$2");
  if (!profile) return false;
  const gates = flowList(flightplanText, "postures", profile);
  const designGates = flowList(flightplanText, "phases", "design");
  if (!gates || !designGates) return false;
  return !gates.some((g) => designGates.includes(g));
}

async function syncOne({ key, targetStatus, client, dryRun, log, context, docsOnly, docsOnlyExemptKeys }) {
  const cur = await client.currentStatus(key);
  if (cur.missing) return log.warn(`${key}: not found in Jira — skipping`);
  // Epics roll up child work and their own gates (see docs/specs); CI has no
  // reliable signal that either is actually satisfied, so epic status stays a
  // manual/reviewed step — never auto-transitioned (HOLODEX-185).
  if (cur.issueType === "Epic") {
    return log.info(`${key}: is an Epic — status is reviewed manually, skipping`);
  }
  // A merged PR that touched only docs/** can't be the PR that finishes an epic's
  // implementation — it's a standalone gate artifact (spec/ADR/design-handoff/worklog)
  // that should have stayed inside the epic's one Draft PR (ADR-069). Firing Done here
  // is exactly the HOLODEX-173/220 incident: a premature Done that then cascades to
  // Released on the next deploy via jira-release-sync.mjs. Scoped to Done only — an
  // early In Review doesn't cascade, so it isn't worth blocking. A key whose worklog
  // posture has no design-phase gate is exempt (docsOnlyExempt, HOLODEX-550).
  if (docsOnly && targetStatus.toLowerCase() === "done") {
    if (!docsOnlyExemptKeys?.has(key)) {
      return log.warn(
        `${key}: merged PR only touched docs/** — skipping Done (looks like a standalone ` +
          `gate-artifact PR, not the epic's implementation; see docs/reference/jira-pipeline.md)`,
      );
    }
    log.info(`${key}: docs-only merge, but its worklog posture has no design-phase gate — firing Done`);
  }
  if (cur.status?.toLowerCase() === targetStatus.toLowerCase()) {
    return log.info(`${key}: already "${targetStatus}" — no-op`);
  }
  const tr = await client.findTransition(key, targetStatus);
  if (!tr) {
    return log.warn(
      `${key}: no transition to "${targetStatus}" available from "${cur.status}" — skipping`,
    );
  }
  // Never move an issue to an earlier statusCategory (HOLODEX-462). A closeout PR on a
  // keyed branch, marked ready after the issue was already Done, dragged HOLODEX-358
  // Done → In Review; its docs-only merge then skipped Done, stranding it. Sideways
  // moves (Done → Released, In Progress → In Review) stay allowed.
  if (isBackwards(cur.category, tr.toCategory)) {
    return log.warn(
      `${key}: refusing "${cur.status}" → "${targetStatus}" — CI never moves an issue ` +
        `backwards (${cur.category} → ${tr.toCategory}; see docs/reference/jira-pipeline.md)`,
    );
  }
  const { id } = tr;
  if (dryRun) {
    return log.info(
      `${key}: [dry-run] would transition "${cur.status}" → "${targetStatus}" (id ${id})`,
    );
  }
  await client.transition(key, id);
  log.info(`${key}: "${cur.status}" → "${targetStatus}"${context ? ` (${context})` : ""}`);
}

// Transition every key toward targetStatus. Per-key try/catch — never throws;
// returns the failure count so the caller can log a summary.
export async function syncKeys({ keys, targetStatus, client, dryRun, log, context, docsOnly, docsOnlyExemptKeys }) {
  let failures = 0;
  for (const key of keys) {
    try {
      await syncOne({ key, targetStatus, client, dryRun, log, context, docsOnly, docsOnlyExemptKeys });
    } catch (err) {
      failures++;
      log.warn(`${key}: ${err.message}`);
    }
  }
  if (failures) log.warn(`${failures} ticket(s) could not be synced — see warnings above`);
  return failures;
}
