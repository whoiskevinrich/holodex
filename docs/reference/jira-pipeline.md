# Jira ↔ GitHub Pipeline Reference

How Holodex work is tracked in Jira and kept in sync with the GitHub pipeline via the
**GitHub for Jira** app. This is the source of truth for the Jira project shape; the
automation rules below are configured in the Jira UI (Project settings → Automation) and
mirrored here so they're version-controlled.

- **Site:** `https://whoiskevinrich.atlassian.net`
- **Project:** HOLODEX (team-managed / next-gen, software). Scope all work to this project.

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
| **In Progress** | Branch created, work underway | automation (branch created) |
| **In Review** | PR open | automation (PR opened) |
| **Done** | Merged to `main` (code complete) | automation (PR merged) |
| **Released** | Shipped in a tagged GHCR image | automation (`ghcr` deployment succeeded) |

`Done ≠ Released`: a merge to `main` is code-complete, but the artifact ships only when
`release.yml` builds the `v*` image and records the **`ghcr` GitHub Deployment** (ADR-034).
That deployment is the signal that drives the **Released** transition.

The released-in-version dimension is tracked with **Jira Releases (`fixVersion`)** set to the
`release-please` version (`v1.3.1`, …), giving a "what shipped in 1.3.1" report aligned with
the GitHub Release notes.

---

## Branch ↔ Jira linkage (prerequisite)

The GitHub for Jira app links branches, commits, PRs, builds, and deployments to an issue
**only when its key is present**. Holodex carries the key in the **branch name**:

- Branch / worktree name: `HOLODEX-123-short-slug`.
- Commit subjects and PR titles stay **clean Conventional Commits** — `release-please` and
  `git-cliff` parse them into the changelog, so the key must *not* appear there (it would
  pollute every CHANGELOG/Release line). The branch name alone carries the linkage.
- Transitions are driven by **automation on dev events**, not Smart Commits, so commit
  hygiene is untouched. Smart Commits (`#comment`, `#time`) remain available ad hoc.

---

## Automation rules (Jira UI: Project settings → Automation)

Each rule's trigger comes from the GitHub for Jira app's development events; the issue is
resolved from the branch-name key.

| # | Trigger | Condition | Action |
|---|---|---|---|
| 1 | Branch created | branch name matches `HOLODEX-\d+` | Transition issue → **In Progress**; assign to project lead |
| 2 | Pull request created | — | Transition issue → **In Review** |
| 3 | Pull request merged | target branch = `main` | Transition issue → **Done**; comment with PR link |
| 4 | Build status = failed | branch has issue key | Add comment "CI failed on {{branch}}" — **do not** transition |
| 5 | Deployment succeeded | environment = `ghcr` | Transition issue → **Released**; set `fixVersion` = deployment tag |
| 6 | Pull request merged | author = `release-please` (the release PR) | For every issue key in the release diff, set `fixVersion` = the new version *(optional bulk-stamp)* |

Notes:

- Rules 1–3 are the core flow and need only branch/PR events. Rule 5 depends on the `ghcr`
  Deployment already emitted by `release.yml`; nothing new is needed on the GitHub side.
- Rule 4 deliberately **comments instead of transitioning** — a red build shouldn't yank an
  issue backward out of In Review.
- Rule 6 is optional; if skipped, set `fixVersion` manually at release time or rely on Rule 5
  stamping each issue as the deployment lands.

---

## Setup checklist (one-time, Jira UI)

1. Add the **Released** status to the HOLODEX workflow, after Done.
2. Create the four `needs-*` labels.
3. Connect the `whoiskevinrich/holodex` repo in the GitHub for Jira app (Apps → Manage →
   GitHub → Configure).
4. Create automation rules 1–5 (6 optional) per the table above.
5. Adopt the `HOLODEX-###-slug` branch-naming convention (already in CLAUDE.md).
