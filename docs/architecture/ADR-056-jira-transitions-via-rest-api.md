# ADR-056: Jira issue transitions via REST API (retire the Automation rules)

**Status**: Proposed
**Date**: 2026-07-04
**Deciders**: Project owner
**Relates to**: ADR-024/034/035/044 (CI/CD + release pipeline — the workflows the CI transitions hang off). Mirrors the shipped **Bookshelf** migration (BOOKSHELF-84); its runbook is the reference implementation.

---

## Context

Holodex's Jira issues move through `To Do → In Progress → In Review → Done → Released`
automatically on GitHub dev events. Today those transitions are driven by **Jira
Automation** rules (branch → In Progress, PR open → In Review, merge → Done, `ghcr`
deploy → Released), fired via the GitHub-for-Jira app's key detection.

**The forcing function: Jira Automation is metered, and we're bleeding the meter.** On the
**Free** plan the site gets **100 automation flow runs per month, shared across every
project** on `whoiskevinrich.atlassian.net` (Holodex + Bookshelf + anything else). As of
**July 4** the account was already at **~50%** utilization — on pace for ~300% of the
monthly cap. A rule that fires on every branch/PR/merge/deploy silently exhausts the
budget mid-month and then just stops transitioning, leaving issues stranded in the wrong
column with no error.

The quota is the whole problem, and it dictates the shape of the fix: **every transition
that currently burns an Automation execution must move onto a trigger that doesn't.**
Direct **Jira REST API** calls are **not** counted against the Automation quota — that
meter only tracks the Automation engine. Holodex is also a **public** repo, so GitHub
Actions minutes are unlimited: there's a free, deterministic, in-git execution surface
already running at three of the four trigger moments.

This is not greenfield design. **Bookshelf already migrated (BOOKSHELF-84)** and paid for
both non-obvious traps:

1. **Automation is metered** — don't use it for CI-frequency work; use the REST API (uncounted).
2. **Scoped API tokens 401 against the site URL.** Atlassian's current "API token *with
   scopes*" only authenticates through the gateway host
   `https://api.atlassian.com/ex/jira/{cloudId}/…` — hitting `<site>.atlassian.net`
   returns `401 "Client must be authenticated to access this resource."` even with a
   correct token/email/scopes. (Legacy unscoped tokens still work against the site URL;
   Atlassian now defaults to scoped.)

Because the mechanism is **site-scoped, not project-scoped**, ~90% of Bookshelf's solution
ports verbatim. This ADR records the Holodex-specific decisions; it does not re-derive the
mechanism.

### Grounded facts (verified against the live instance)

- **Site cloudId**: `e7c03552-8036-43fa-bb8b-b415de46f9f6`
- **Gateway base URL**: `https://api.atlassian.com/ex/jira/e7c03552-8036-43fa-bb8b-b415de46f9f6`
- **Auth**: Basic auth, `base64("<account-email>:<scoped-token>")`, against the gateway host.
- **Holodex workflow is team-managed with every transition `isGlobal: true`** (reachable
  from *any* status). This is a materially simpler graph than Bookshelf's staged workflow:
  the "GET transitions → match on `.to.name`" lookup can never hit an "unavailable from
  current status" edge. Verified transition map (match by destination status **name** per
  the runbook; the ids are stable global ones, useful for logging only):

  | Target status | status id | transition id |
  |---|---|---|
  | In Progress | 10005 | 21 |
  | In Review | 10006 | 31 |
  | Done | 10007 | 41 |
  | Released | 10080 | 2 |
  | To Do | 10004 | 11 |

## Decision

**Retire the four Jira Automation transition rules. Fire the same four transitions via
the Jira REST API, split across two unmetered surfaces by where each event actually
originates.**

### 1. Transition-trigger split

| Transition | Trigger | Fired by | Token needed |
|---|---|---|---|
| **In Progress** | start-of-work (branch rename to `HOLODEX-<n>`) | **Agent/session** via MCP `transitionJiraIssue` | No — uses the maintainer's interactive Atlassian auth |
| **In Review** | `pull_request: opened` | **GitHub Actions** | Yes |
| **Done** | merge to `main` | **GitHub Actions** | Yes |
| **Released** | `ghcr` deploy (tag from a merged Release-Please PR) | **GitHub Actions** (step in `release.yml`) | Yes |

**In Progress is agent-only, by design.** It's the one transition with no server-side
event (branch creation is local), and the agent *already* renames the worktree branch to
the Jira key as its first action — a perfect, free hook. The trade-off (accepted): it does
not fire on a rare session-less commit. The CI trio covers every server-visible event.

The three CI transitions attach to workflows that **already run** at the right moment;
`Released` in particular is a step added to the existing `release.yml`, not a new workflow.

**`Released` is a batch transition, not a single-issue one — this distinguishes it from
the other three.** `In Review` and `Done` fire on one issue: the key parsed from the PR's
own branch (`github.head_ref`). `Released` fires on a **tag** (cut when a Release-Please PR
merges, ADR-044) that ships **every** feature PR merged since the previous release — a
whole set of issues, not one.

**Where the set comes from — Jira state, not the diff.** Bookshelf discovers the set by
parsing issue keys out of its release notes, because Bookshelf puts keys in commit
subjects. **Holodex deliberately does not** — subjects stay clean Conventional Commits so
`release-please`/`git-cliff` changelogs read well (see `docs/reference/jira-pipeline.md`),
so the changelog and commit range carry **no keys to parse**. Instead we use the invariant
that the `Done`-on-merge trigger already moved every merged PR's issue to `Done`: a release
cut from `main` therefore ships exactly the current **`project = HOLODEX AND status = Done`**
set. `Released` reads that set via a JQL search and transitions each to `Released`. This is
the same *behavior contract* as Bookshelf's `jira-release-sync.mjs` (batch, idempotent,
soft-fail) with a Holodex-appropriate *source* (Jira query vs. release-note parsing). Note
the Release-Please **release PR itself** carries no issue key, so the `Done`-on-merge
trigger naturally **no-ops** on it — the release-PR merge is claimed by `Released`, not `Done`.

### 2. Mechanism (ported from Bookshelf's `scripts/lib/jira-sync.mjs`)

A small Node script (node is available in Holodex CI via `web/`) with the same
behavior contract, called from the three workflows:

```
GET  {base}/rest/api/3/issue/{key}?fields=status   # current status
GET  {base}/rest/api/3/issue/{key}/transitions     # pick the one whose .to.name == target (case-insensitive)
POST {base}/rest/api/3/issue/{key}/transitions      # body: {"transition":{"id":"<id>"}}
```

Load-bearing behaviors (all three preserved from Bookshelf):

- **Idempotent** — skip if the issue is already at the target status.
- **Match on destination status name**, case-insensitive — never hard-code a transition
  id (robust to workflow edits and to label vs. status-name drift).
- **Soft-fail** — a missing secret, a Jira outage, or an unreachable transition logs a
  warning and **exits 0**. A Jira hiccup must never red-build a deploy or block a merge.
- **Extract the issue key from the branch/PR ref** with a regex; **no-op silently** if the
  branch carries no `HOLODEX-<n>` key (an un-keyed branch simply doesn't transition).

**Single-issue vs. batch.** `In Review` and `Done` run the three calls above against **one**
key (from the PR branch). The **`Released`** entry point instead runs a JQL search
(`project = HOLODEX AND status = Done`) to collect the shipping set, then runs the
transition for each — same batch/idempotent/soft-fail contract as Bookshelf's
`jira-release-sync.mjs`, sourced from Jira state rather than the (keyless) commit range.
Each per-issue transition stays idempotent and soft-fails independently, so one
already-`Released` (or one transient failure) never blocks the rest of the batch.

### 3. Relationship to Bookshelf's status model — converge the mechanism, not the semantics

Bookshelf (BOOKSHELF-84) drives **deployment-anchored** statuses: dev deploy → `On Dev`,
prod promote → `Done`, and it does **not** automate `In Progress`/`In Review` at all. It's
tempting — now that both repos gate releases through **Release Please** (ADR-044) — to
adopt that model wholesale for uniformity. We deliberately **don't**, because Release Please
converges the two repos at the **event** layer while their **status semantics** stay
legitimately different:

- **Release Please *strengthens* Holodex's `Done`-vs-`Released` split rather than collapsing
  it.** The release-PR gate **batches** merges: a feature PR lands on `main` (`Done`), then
  waits until the next release PR is cut and merged (`Released`). That manufactures a real
  *"merged but not yet shipped"* window — exactly the distinction CLAUDE.md encodes
  (*Done = merged; Released = shipped in a tagged image*) and the board actively populates.
  Bookshelf's original continuous dev deploy made merge ≈ deploy, so it never needed the
  split; Holodex does.
- **`On Dev` has no Holodex analog.** It's anchored to a real dev **environment** deploy.
  Holodex has no dev/prod environment ladder — it publishes a single GHCR image — so
  importing `On Dev` would invent a stage that corresponds to nothing Holodex does.
- **Convergence that *is* real, and taken:** (1) the shared `jira-sync.mjs` **core** (auth,
  gateway, match-by-name, soft-fail) — one tested lib, two repos, per-repo trigger maps as
  thin config; (2) the **`Released` batch transition**, which reuses Bookshelf's
  `jira-release-sync.mjs` pattern directly (see §1/§2).
- **The one judgment call:** automating `In Progress`/`In Review` is optional ceremony that
  Bookshelf skips. We keep them because in *this* design they're nearly free — `In Progress`
  is one agent MCP call at branch-rename (no CI, no token, no quota) and `In Review` is one
  cheap Actions step — and they make the board read true. If transitions were metered we'd
  cut them; they aren't, so the marginal cost is trivial. Preference, not principle.

### 4. Credential storage & rotation — scripted two-repo fan-out

Reuse the **single scoped token** already minted for Bookshelf (scopes `read:jira-work`,
`write:jira-work`, `read:jira-user`); the site-wide scopes cover every project.

- `JIRA_BASE_URL` — non-secret repo **variable** (the gateway URL).
- `JIRA_USER_EMAIL` — repo **secret** (the token owner's account email).
- `JIRA_API_TOKEN` — repo **secret** (the scoped token).

Stored as **repo-level** config in both `holodex` and `bookshelf`. Rotation (scoped tokens
expire up to ~1 year, so this is a **once-a-year, two-repo** action) is a single documented
PowerShell one-liner sourced from one password-manager entry:

```powershell
$t = Read-Host -AsSecureString "New Jira token"
$plain = [System.Net.NetworkCredential]::new('', $t).Password
'holodex','bookshelf' | ForEach-Object {
  gh secret set JIRA_API_TOKEN --repo "whoiskevinrich/$_" --body $plain
}
```

The password-manager entry is the single source of truth; the command fans it out. At
annual cadence the "two stores could desync" risk is negligible (a failed write is visible
and re-runnable).

## Options Considered

Two decisions carried real alternatives: the **mechanism** (already largely settled by the
Bookshelf precedent) and the **credential home** (the genuine open question, because
"single token, one rotation location" is not automatically true).

### Mechanism — REST API vs. keep Automation

| Dimension | REST API (chosen) | Jira Automation (status quo) |
|-----------|-------------------|------------------------------|
| Quota | Uncounted | **100/month, shared site-wide — the problem** |
| Where it lives | In git, code-reviewed | Jira UI, invisible, drifts |
| Complexity | Low (ported) | Low, but opaque |

**Pros (REST):** kills the quota draw at the source; logic is versioned and reviewable;
proven in Bookshelf. **Cons:** a credential to store and rotate; ~4 small wiring points.

### Credential home — where the single token lives

The constraint "**rotate in exactly one location**" is the deciding force. Personal repos
have **no GitHub org/account-level secret scope**, so pasting the token into two repo
secret stores is literally two locations.

#### Option A — Scripted two-repo fan-out (chosen)
| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — no new infra |
| Cost | $0 |
| Coupling | None; Holodex CI stays AWS-free |
| Rotation | One command, one password-manager source, annual cadence |

**Pros:** proportionate to a once-a-year rotation; zero new dependencies; keeps
Holodex's GHCR-only CI free of AWS. **Cons:** two physical secret stores (not one),
so a failed write could desync — negligible at annual cadence and visibly re-runnable.

#### Option B — AWS Secrets Manager (shared store, fetched via OIDC)
| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — adds AWS OIDC role + `configure-aws-credentials` to Holodex CI |
| Cost | ~$0 (one secret) |
| Coupling | **Couples otherwise AWS-free Holodex CI to AWS** |
| Rotation | True single location; matches the global AWS-Secrets-Manager doctrine |

**Rejected — disproportionate.** Bolting AWS OIDC onto a GHCR-only Go/Svelte pipeline
just to read one string is more machinery than an annual rotation warrants. **Reserve
for** the day the token is needed outside CI (then centralizing genuinely pays off).

#### Option C — Create a GitHub org, use one org-level secret
| Dimension | Assessment |
|-----------|------------|
| Complexity | High one-time — a repo migration |
| Cost | $0 (Free org tier includes org secrets; public-repo Actions stay unlimited) |
| Coupling | Cleanest GitHub-native answer; scales to future repos |
| Rotation | True single location |

**Rejected — disproportionate to token rotation.** Free in dollars, but transferring
both repos ripples through **GHCR image paths** (`ghcr.io/whoiskevinrich/*` →
`ghcr.io/<org>/*`), **AWS OIDC trust-policy `sub` conditions** (`repo:whoiskevinrich/*`),
and a **GitHub-for-Jira reinstall**. **Reserve for** a deliberate repo-consolidation
decision made for its own reasons, not as a rider on this fix.

## Trade-off Analysis

The dominant force is a hard, near-term quota ceiling, so the winning axis is "move off
the meter with the least new surface." REST-via-Actions does that for free on a public
repo; the agent handles the one event Actions can't see. The only genuinely contestable
sub-decision — the credential home — turns on *rotation cadence*, not on security
sophistication: because scoped tokens last ~a year, a centralized store (ASM or an org)
optimizes an operation that happens annually, at the cost of standing infrastructure or a
repo migration. Matching effort to cadence, the scripted fan-out wins now; ASM and the org
are explicitly deferred, not dismissed, each with a named trigger condition.

## Consequences

**Easier**
- Quota draw from Holodex drops to ~zero; transitions stop silently dying mid-month.
- Transition logic is in git — reviewable, diffable, and mirrors Bookshelf's contract.
- The all-`isGlobal` workflow makes the name-match lookup trivially robust.

**Harder / newer**
- A credential to hold and rotate (annually) across two repos.
- Un-keyed branches don't transition — the branch **must** carry `HOLODEX-<n>` (already a
  standing working-agreement; the agent auto-renames on start). CI extraction no-ops
  silently when absent, so this fails quiet, not loud.
- `In Progress` depends on a session driving the work; session-less starts won't transition.

**Unchanged / to revisit**
- The **GitHub-for-Jira app stays** — it still provides the dev-panel branch/PR/commit
  linkage. **Only the Automation transition rules are retired**, not the integration.
- If the token ever needs a non-CI consumer → revisit **Option B (ASM)**.
- If repos get consolidated for other reasons → revisit **Option C (org secret)**.
- **Security-review runs against the implementation diff** (not this ADR): token handling,
  secret scoping (no token in logs — soft-fail must not echo it), and untrusted branch/PR
  values feeding the REST call (issue-key regex must be anchored/validated).

## Action Items

1. [ ] Port `scripts/lib/jira-sync.mjs` (+ the per-trigger entry scripts) into Holodex from Bookshelf's reference.
2. [ ] Add the CI transition steps: `In Review` (PR-opened workflow, single key), `Done` (merge-to-`main`, single key), `Released` (step in `release.yml`, **batch over all `HOLODEX-<n>` keys in `<prev-tag>..<this-tag>`** — port Bookshelf's `jira-release-sync.mjs`).
3. [ ] Wire `In Progress` into the agent's start-of-work step (MCP `transitionJiraIssue` right after the branch rename).
4. [ ] Set `JIRA_BASE_URL` (variable), `JIRA_USER_EMAIL` + `JIRA_API_TOKEN` (secrets) on the `holodex` repo; verify with the `/myself` gateway probe (expect `200`).
5. [ ] Copy the Bookshelf runbook into `docs/reference/` (jira-transitions-from-ci).
6. [ ] **Disable/delete the four Jira Automation transition rules** — only after the REST path is verified end-to-end (avoid a gap where neither fires).
7. [ ] Run `/security-review` on the implementation diff before merge.
8. [ ] Add the annual token-rotation reminder; document the fan-out one-liner in the runbook.
9. [ ] Update the ADR index (README) and file the HOLODEX tracking issue.
