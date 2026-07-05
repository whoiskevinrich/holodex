// Shared, dependency-free helpers for CI-side Jira status sync (ADR-058).
//
// Two entry points build on this:
//   - jira-branch-sync.mjs  → In Review (PR opened) / Done (PR merged), keyed off
//                             the PR *branch name* (Holodex carries the key there,
//                             never in the commit subject — see docs/reference/jira-pipeline.md)
//   - jira-release-sync.mjs → Released, on the `ghcr` deploy: every HOLODEX issue
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
export function selectTransitionId(transitions, targetStatus) {
  const target = targetStatus.toLowerCase();
  const t = (transitions ?? []).find((tr) => tr.to?.name?.toLowerCase() === target);
  return t?.id ?? null;
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
      const res = await fetch(`${base}/rest/api/3/issue/${key}?fields=status`, { headers });
      if (res.status === 404) return { missing: true };
      if (!res.ok) throw new Error(`GET issue ${key} failed: ${res.status} ${res.statusText}`);
      const json = await res.json();
      return { status: json.fields?.status?.name ?? null };
    },
    async findTransitionId(key, targetStatus) {
      const res = await fetch(`${base}/rest/api/3/issue/${key}/transitions`, { headers });
      if (!res.ok)
        throw new Error(`GET transitions for ${key} failed: ${res.status} ${res.statusText}`);
      const json = await res.json();
      return selectTransitionId(json.transitions, targetStatus);
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

async function syncOne({ key, targetStatus, client, dryRun, log, context }) {
  const cur = await client.currentStatus(key);
  if (cur.missing) return log.warn(`${key}: not found in Jira — skipping`);
  if (cur.status?.toLowerCase() === targetStatus.toLowerCase()) {
    return log.info(`${key}: already "${targetStatus}" — no-op`);
  }
  const id = await client.findTransitionId(key, targetStatus);
  if (!id) {
    return log.warn(
      `${key}: no transition to "${targetStatus}" available from "${cur.status}" — skipping`,
    );
  }
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
export async function syncKeys({ keys, targetStatus, client, dryRun, log, context }) {
  let failures = 0;
  for (const key of keys) {
    try {
      await syncOne({ key, targetStatus, client, dryRun, log, context });
    } catch (err) {
      failures++;
      log.warn(`${key}: ${err.message}`);
    }
  }
  if (failures) log.warn(`${failures} ticket(s) could not be synced — see warnings above`);
  return failures;
}
