# Jira ↔ GitHub Pipeline Reference

How Holodex work is tracked in Jira and kept in sync with the GitHub pipeline. Status
**transitions** are driven by **direct Jira REST API calls** from CI and the agent
([ADR-058](../architecture/ADR-058-jira-transitions-via-rest-api.md)); the **GitHub for
Jira** app is still installed, but only for the **development panel** (branch/commit/PR/
build/deployment links on each issue) — it no longer drives transitions.

- **Site:** `https://whoiskevinrich.atlassian.net`
- **Project:** HOLODEX (team-managed / next-gen, software). Scope all work to this project.
- **Gateway base URL:** `https://api.atlassian.com/ex/jira/e7c03552-8036-43fa-bb8b-b415de46f9f6`

---

## Work item types

Types map to the Conventional Commit taxonomy already used in commits, `release-please`,
and `cliff.toml` — no extra types beyond the five below.

| Commit type(s) | Jira type | Notes |
|---|---|---|
| (feature cluster `F##`, phase) | **Epic** | Parent for a related batch of stories/tasks |
| `feat` | **Story** | User-facing behavior |
| `fix` | **Bug** | |
| `refactor` `perf` `docs` `test` `ci` `build` `chore` | **Task** | All technical/non-user-facing work |
| — | **Sub-task** | Breakdown of a Story/Task/Bug |

Team-managed project: link a child to its parent with the **`parent`** field (there is no
"Epic Link" field).

The lockstep artifact gates from the CLAUDE.md change-routing table are **labels**, not
types — they describe what a work item still owes, independent of its kind:

| Label | Applied when the change touches… | Cleared when… |
|---|---|---|
| `needs-spec` | functionality / behavior | the `docs/specs/` spec lands |
| `needs-adr` | infrastructure / architecture | the ADR lands |
| `needs-design` | UX / user-facing surface | the design handoff lands |
| `needs-security-review` | auth / access / infrastructure | `/security-review` signs off |

---

## Statuses

```
To Do → In Progress → In Review → Done → Released
```

| Status | Meaning | Set by |
|---|---|---|
| **To Do** | Backlog / triaged | manual |
| **In Progress** | Branch created, work underway | **agent/session** (MCP `transitionJiraIssue` at branch-rename) |
| **In Review** | PR open | **CI** — `jira-sync.yml` on `pull_request: opened` |
| **Done** | Merged to `main` (code complete) | **CI** — `jira-sync.yml` on `pull_request` merged |
| **Released** | Shipped in a tagged GHCR image | **CI** — `release.yml` on the `ghcr` deploy (batch) |

`Done ≠ Released`: a merge to `main` is code-complete, but the artifact ships only when
`release.yml` builds the `v*` image and records the **`ghcr` GitHub Deployment** (ADR-034).
Because releases are batched behind a Release-Please PR (ADR-044), a real *"merged but not
yet shipped"* window exists — that's exactly what the `Done`→`Released` split captures.

The released-in-version dimension can additionally be tracked with **Jira Releases
(`fixVersion`)** set to the `release-please` version, but the GitHub Releases (git-cliff
changelog per `v*` tag) already provide the authoritative "what shipped in v1.3.1" report.

---

## Branch ↔ Jira linkage (prerequisite)

Both the GitHub-for-Jira dev panel *and* the CI transitions key off the issue key in the
**branch name** — Holodex carries it there and **nowhere else**:

- Branch / worktree name: `HOLODEX-123-short-slug`.
- Commit subjects and PR titles stay **clean Conventional Commits** — `release-please` and
  `git-cliff` parse them into the changelog, so the key must *not* appear there (it would
  pollute every CHANGELOG/Release line). **The branch name alone carries the linkage**, which
  is why the CI transitions read `github.head_ref` (the branch), not the commit message.
- A branch with no `HOLODEX-<n>` key simply doesn't transition (the CI script no-ops).

---

## CI transitions (REST API)

The transitions are direct `POST /rest/api/3/issue/{key}/transitions` calls against the
**gateway** base URL. Reference: [ADR-058](../architecture/ADR-058-jira-transitions-via-rest-api.md);
scripts in [`scripts/`](../../scripts).

| Transition | Trigger | Fired by | Key source |
|---|---|---|---|
| → **In Progress** | start-of-work (branch rename to key) | agent/session (MCP `transitionJiraIssue`) | current branch |
| → **In Review** | `pull_request: opened` | `jira-sync.yml` → `scripts/jira-branch-sync.mjs` | `github.head_ref` |
| → **Done** | `pull_request` merged | `jira-sync.yml` → `scripts/jira-branch-sync.mjs` | `github.head_ref` |
| → **Released** | `ghcr` deploy (Release-Please tag) | `release.yml` → `scripts/jira-release-sync.mjs` | JQL `status = Done` (batch) |

Design notes:

- **`In Review` / `Done` are single-issue** — one key parsed from the PR branch.
- **`Released` is a batch** — Holodex commit subjects are keyless, so the shipping set is
  read from **Jira state**, not the diff: every merged PR was already moved to `Done` by
  the branch sync, so a release cut from `main` ships exactly the current
  `project = HOLODEX AND status = Done` set. The release sync transitions them all.
- **`In Progress` is agent-only** — it's the one transition with no server-side event, so
  the session fires it (see CLAUDE.md → *Branch ↔ Jira linkage*). It won't fire on a rare
  session-less start; the CI trio covers everything server-visible.
- **Idempotent + soft-fail** — each script skips an issue already at the target, matches the
  transition by destination status **name**, and on any failure (missing key/secret, Jira
  outage, unreachable transition) logs a GitHub `::warning::` and **exits 0**. A Jira hiccup
  never red-builds a deploy or blocks a merge.
- **Security** — the PR workflow uses plain `pull_request` (**never** `pull_request_target`),
  so secrets are withheld from fork PRs (they no-op) and untrusted branch names can't reach a
  privileged context; the key is matched by an anchored `\bHOLODEX-\d+\b` regex; the token is
  never logged. Verified by `/security-review` on the implementing PR.

### CI credentials (one-time)

A single **scoped** Atlassian API token, shared with the Bookshelf repo (the site-wide
scopes cover every project). Scopes: `read:jira-work`, `write:jira-work`, `read:jira-user`.

> **Trap:** a scoped token **401s** against `https://whoiskevinrich.atlassian.net/…`. It
> only authenticates through the **gateway** host `https://api.atlassian.com/ex/jira/<cloudId>`.
> Always set `JIRA_BASE_URL` to the gateway URL. Verify with the `/myself` probe (expect 200):
>
> ```bash
> curl.exe -s -o /dev/null -w "%{http_code}\n" \
>   -u "<account-email>:<token>" -H "Accept: application/json" \
>   "https://api.atlassian.com/ex/jira/e7c03552-8036-43fa-bb8b-b415de46f9f6/rest/api/3/myself"
> ```

Set the config on the repo (`JIRA_BASE_URL` is a non-secret variable; email + token are secrets):

```powershell
gh variable set JIRA_BASE_URL   --repo whoiskevinrich/holodex --body "https://api.atlassian.com/ex/jira/e7c03552-8036-43fa-bb8b-b415de46f9f6"
gh secret   set JIRA_USER_EMAIL --repo whoiskevinrich/holodex --body "<account-email>"
gh secret   set JIRA_API_TOKEN  --repo whoiskevinrich/holodex   # paste the scoped token
```

**Rotation (annual — scoped tokens expire).** One password-manager entry is the source of
truth; a single command fans it out to both repos that share the token:

```powershell
$t = Read-Host -AsSecureString "New Jira token"
$plain = [System.Net.NetworkCredential]::new('', $t).Password
'holodex','bookshelf' | ForEach-Object {
  gh secret set JIRA_API_TOKEN --repo "whoiskevinrich/$_" --body $plain
}
```

---

## One-time setup

### Phase 0 — Connect the repo to GitHub for Jira (dev panel only)

Still worth doing: it populates each issue's **Development** panel (branches, commits, PRs,
builds, deployments). It no longer drives transitions.

`holodex` is a repo under the **personal `whoiskevinrich` account**, not a GitHub org — the
flow still works; you install the app on your user account.

1. **Settings (gear) → Apps → GitHub** (the GitHub for Jira config) → **Configure**.
2. **Connect GitHub organization** — on the GitHub screen, install the GitHub for Jira app on
   your personal `whoiskevinrich` account and choose **Only select repositories → `holodex`**.
3. Back in Jira, verify an issue's **Development** panel populates once a branch with the key
   exists (make a throwaway `HOLODEX-1-test` branch with one commit to confirm, then delete).

### Phase 1 — Add the **Released** status (if not already present)

1. **Project settings → Issue types → Story** → workflow editor → **+ Add status** →
   **`Released`**, category **Done** (green). Add a **Done → Released** transition
   (or "Any status → Released"). **Save.** Repeat for **Task** and **Bug**.

### Phase 2 — CI credentials

Create the scoped token and set the three config values (see *CI credentials* above).

### Phase 3 — Create the four `needs-*` labels

Open any issue → **Labels** → type `needs-spec` ↵, then `needs-adr`, `needs-design`,
`needs-security-review`. Optionally save a filter to keep the gates visible:

```
project = HOLODEX AND labels in (needs-spec, needs-adr, needs-design, needs-security-review) AND statusCategory != Done
```

### Phase 4 — Retire the Jira Automation rules (cutover)

**Only after** the REST path is verified end-to-end (open a smoke PR, watch it walk To Do →
In Progress → In Review → Done, cut a release, watch Released), delete the old transition
rules so nothing double-fires and the quota stops metering: **Project settings → Automation**
→ delete the branch-created, PR-created, PR-merged, and deployment-succeeded rules. Keep any
non-transition rules you still want (e.g. a build-failed comment).

---

## Quota note (why this migration exists)

On the **Free** plan the site gets **100 automation flow runs per month, shared across every
project** on the instance — and **every rule counts, single-project ones included**. (The
"single-project rules are free" carve-out applies only to **paid** plans; an earlier version
of this doc claimed it applied on Free, which is wrong and is what let the quota creep up
unnoticed.) A rule firing on every branch/PR/merge/deploy silently exhausts the budget
mid-month and then stops transitioning. The **REST API is not metered**, and GitHub Actions
is unlimited on this public repo — so the CI transitions cost nothing against either budget.

---

## Gotchas

- **The branch key is the linchpin.** Dev-panel links *and* CI transitions resolve the issue
  from the `HOLODEX-<n>` in the branch name. No key → no link and no transition.
- **An empty Development panel ≠ a broken connection.** The panel is populated per issue from
  refs that name the key, so it stays blank until some branch/commit/PR mentions that issue.
- **A commitless branch may not surface** in the dev panel. Push at least one commit on the
  branch, then hard-refresh the issue.
- **`Released` moves the whole `Done` set.** If you deliberately parked an issue in `Done`
  that you *don't* want released, move it elsewhere before cutting the release — the batch
  sync treats every `Done` issue as shipping.
- **Fork PRs no-op.** Secrets are withheld from `pull_request` runs on forks, so the branch
  sync soft-fails there. For this solo repo, PRs come from same-repo branches (secrets present).
