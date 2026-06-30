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

## One-time setup (Jira UI walkthrough)

All of this is project-level in the team-managed HOLODEX project — no Jira admin rights
needed. Do the phases in order; the dev-event triggers don't exist until the repo is
connected.

### Phase 0 — Connect the repo (first; nothing links without it)

1. **Settings (gear, top-right) → Apps → GitHub** (the GitHub for Jira config), or
   **Apps → Manage your apps → GitHub → Configure**.
2. **Connect a GitHub organization** → the org that owns `whoiskevinrich/holodex`.
3. **Only select repositories → `holodex`** (add the TMDB provider repo too if you want its
   CI linked).
4. Verify: open any issue → a **Development** panel appears once a branch with the key
   exists. Make a throwaway `HOLODEX-1-test` branch to confirm, then delete it.

### Phase 1 — Enable Releases (so `fixVersion` works)

1. **Project settings → Features** → toggle **Releases** on (a **Releases** item appears in
   the left sidebar).
2. **Releases → Create version** for the current line (e.g. `v1.3.1`); pre-create the next
   one or add them at tag time.

### Phase 2 — Add the **Released** status

1. **Project settings → Issue types → Story.**
2. In the workflow editor, **+ Add status** → name **`Released`**, category **Done** (green)
   — keeps `statusCategory = Done` JQL counting it as complete.
3. Add a transition **Done → Released** (and optionally "Any status → Released"). **Save.**
4. Repeat for **Task** and **Bug**. The status is shared by name, so you're only adding the
   transition. The board shows it as a new right-most column.

### Phase 3 — Create the four `needs-*` labels

Labels are created by use, not an admin screen.

1. Open any issue → **Labels** → type `needs-spec` ↵, then `needs-adr`, `needs-design`,
   `needs-security-review`. They now autocomplete everywhere.
2. Save a filter to keep the gates visible — **Filters → Create filter**:
   ```
   project = HOLODEX AND labels in (needs-spec, needs-adr, needs-design, needs-security-review) AND statusCategory != Done
   ```
   Star it or add it as a board quick-filter.

### Phase 4 — Automation rules

**Project settings → Automation → Create rule.** Each rule is Trigger → (Condition) →
Action; the issue is auto-resolved from the branch/PR key. Build them per the
[Automation rules](#automation-rules-jira-ui-project-settings--automation) table above:

| Rule | Trigger | Key condition | Action |
|---|---|---|---|
| 1 | Branch created | Status **is** To Do | Transition → In Progress *(opt. assign)* |
| 2 | Pull request created | Status one of To Do, In Progress | Transition → In Review |
| 3 | Pull request merged | `{{pullRequest.destinationBranch}}` = `main` | Transition → Done; comment PR link |
| 4 | Build status changed | status = failed | Comment only — **no transition** |
| 5 | Deployment status changed | env = `ghcr` AND status = successful | Transition → Released; set Fix version |
| 6 *(opt.)* | Pull request merged | title matches `chore\(main\): release` | Bulk-set Fix version on linked issues |

Turn each rule on, then smoke-test: create `HOLODEX-###`, branch `HOLODEX-###-smoke`, open
and merge a trivial PR, and watch it walk To Do → … → Done.

### Gotchas

- **The branch key is the linchpin.** Builds and deployments inherit their issue link from
  the commit/branch they ran on, so Rules 4–5 only fire on issues whose branch was named
  `HOLODEX-###-…`.
- **Rules 4 & 5 need build/deployment data flowing.** If the Development panel shows
  branches/PRs but no builds/deployments, the GitHub for Jira app isn't forwarding
  Actions/Deployment data — recheck Phase 0.
- **Branch convention** `HOLODEX-###-slug` is already documented in CLAUDE.md.
