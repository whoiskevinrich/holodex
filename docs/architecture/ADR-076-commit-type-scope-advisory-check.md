# ADR-076: Advisory CI check — `docs`/`chore`-typed PRs that touch non-doc code

**Status**: Proposed
**Date**: 2026-07-30
**Deciders**: Project owner
**Relates to**: [ADR-058](ADR-058-jira-transitions-via-rest-api.md) (the Jira REST transitions this check sits alongside), [ADR-069](ADR-069-draft-prs-for-pre-implementation-gates.md) (Draft-PR gate-artifact convention this incident broke), [ADR-034](ADR-034-release-notes-and-deployments.md) (git-cliff changelog this check protects); HOLODEX-238

---

## Context

PR [#187](https://github.com/whoiskevinrich/holodex/pull/187) was titled
`docs(tags): F50 spec + ADR-075`, correctly reflecting its first commits (the spec and the
ADR). Over the life of the PR it also accumulated the **full F50 implementation** — 3 DB
migrations, ~15 Go files, 5 Svelte files — and shipped live in `v1.14.1`, still carrying
the `docs(...)` title. Because this repo squash-merges (`allow_squash_merge` only), the
squash commit's message defaults to the PR title, so that `docs(...)` type is exactly what
`release-please`/`git-cliff` parsed for the merge commit.

`cliff.toml` and `release-please-config.json` deliberately hide `docs`/`chore` from the
user-facing CHANGELOG (see CLAUDE.md, "Commit type for Claude/agent-tooling changes") — by
design, so agent-tooling churn (`.claude/`, hooks, `flightplan/`) doesn't pollute release
notes. That design assumes `docs`/`chore` commits never carry product-facing code, an
assumption this PR broke. Two concrete failures followed:

1. **The shipped feature never surfaced for its required operator-facing documentation
   updates** — a `docs(...)`-typed merge reads as already-documented, so nothing flagged
   the gap (fixed retroactively in PR #189 / HOLODEX-237).
2. **Epic HOLODEX-224 and its 9 child stories stayed stuck at `In Progress`** instead of
   reaching `Done`/`Released`. `jira-sync.yml`'s existing changed-paths guard
   (`scripts/lib/jira-sync.mjs`, HOLODEX-173/220) only checks whether a merged PR touched
   **only** `docs/**`, to stop a *docs-scoped* PR from firing a premature `Done`. PR #187's
   diff was not docs-only (it touched `internal/**`, `web/src/**`), so that guard's
   `docs_only` flag was correctly `false` and `Done` fired — but nothing checked the
   inverse signal: a PR **typed** `docs`/`chore` while its **diff** carries real
   product/infra code. (Manually reconciled to `Released` in the same session that filed
   this ADR.)

Both failures share one root cause: **nothing compares a PR's declared commit type against
what its diff actually touches.** This ADR adds that check.

### Non-goal: the one-branch-to-many-issues gap

Investigation surfaced a second, structurally different cause behind symptom 2:
`scripts/jira-branch-sync.mjs` transitions the **single** issue key parsed from a PR's
branch name. An epic with 9 child stories implemented inside one PR only ever transitions
the one key the branch carries — the other 8 never get their own branch/PR to transition
on. That gap exists independent of commit typing (it would strand the same 8 children even
with a perfectly-typed `feat(...)` PR) and has no clean fix without a rollup mechanism Jira
doesn't offer today (mirroring why epics are already excluded from auto-transition,
HOLODEX-185). **Out of scope here** — worth its own ADR if it recurs.

## Decision

**A new, advisory-only GitHub Actions workflow inspects the PR title's Conventional-Commit
type against the PR's changed files, and posts a sticky PR comment when a `docs`/`chore`
type coincides with a diff that looks like real implementation.**

### 1. Type source: PR title, not individual commits

The repo squash-merges (verified: `allow_squash_merge: true`, `allow_merge_commit: false`,
`allow_rebase_merge: false`), so the **PR title** is what actually lands as the merge
commit message and what `release-please`/`git-cliff` parse — not any individual commit on
the branch. The check reads `github.event.pull_request.title`, mirroring the same
squash-merge assumption `docs/reference/jira-pipeline.md` already documents for the Jira
side (branch name carries the key; PR title carries the Conventional Commit).

### 2. Scope signal: changed-file globs, with a threshold

Reuse the changed-files approach `jira-sync.yml` already established (`gh api
.../pulls/{n}/files`), but invert the question: instead of "is everything under `docs/**`,"
ask "does anything fall under a non-doc/tooling glob":

- `internal/**/*.go` (Go backend, tests included — a test-only diff is still real signal
  that a `docs`/`chore` type is wrong)
- `cmd/**/*.go`, `providers/**/*.go` (the entrypoint and the TMDB provider sidecar — real
  Go product surfaces alongside `internal/`; omitting them would leave the exact PR #187
  failure mode uncaught for either directory)
- `web/src/**/*.svelte`, `web/src/**/*.ts` (SvelteKit frontend)
- `internal/db/migrations/**` (schema changes — see threshold below)

**Threshold, to keep this advisory rather than noisy:** flag when either (a) **more than
one** file matches a non-doc/tooling glob, or (b) **any** migration file is touched,
regardless of count. (a) tolerates the legitimate false positive named in the HOLODEX-238
scope note — a `docs(...)` commit that touches one non-doc line (e.g. fixing a comment in a
`.go` file while writing a spec) — without tolerating a multi-file implementation diff.
(b) has no equivalent false positive: a schema migration is never docs-only, so any count
above zero is real signal.

### 3. Advisory, not blocking

The workflow always exits 0. It never sets a failing/red status check — it posts (or
updates) a sticky PR comment, marked with an HTML comment (`<!-- holodex-commit-type-scope
-->`) so re-runs edit the same comment instead of accumulating one per push, following the
existing `release-candidate-comment.mjs` upsert pattern. When a later push resolves the
mismatch (title retyped, or the non-doc files dropped from the diff), the comment is
updated to say so rather than left stale. This mirrors the advisory posture already
established for `release-candidate-comment.mjs` (ADR-070) and the soft-fail posture of the
Jira sync scripts (ADR-058) — CI here informs, it does not gate.

### 4. New workflow, not a job in `jira-sync.yml`

A new workflow file (`.github/workflows/commit-type-scope-check.yml`) rather than a job
appended to `jira-sync.yml`:

- **Different trigger shape.** `jira-sync.yml` only needs to fire once per state
  transition (`opened`, `ready_for_review`, `closed`) — the transition is a one-time event.
  This check needs to re-evaluate on every **content change** (`opened`, `synchronize`,
  `edited` — title edits included — `ready_for_review`), since both the title and the diff
  can change independently across a PR's lifetime.
- **Different concern.** `jira-sync.yml` is Jira-state plumbing (soft-fails on missing Jira
  secrets by design); this check has nothing to do with Jira and needs no Jira credentials
  — coupling them would make an unrelated Jira outage swallow a check that has nothing to
  do with Jira, and vice versa.
- Matches the existing precedent of one narrowly-scoped workflow per concern
  (`release-candidate.yml` alongside `release.yml`, rather than folding into it).

### 5. Script shape

Follows the existing dependency-free `scripts/*.mjs` convention (`release-candidate-comment.mjs`
is the direct template): a pure `classify({ title, files })` function returning
`{ flagged, type, matched, migrationTouched }`, unit-tested with `node --test`
(`scripts/commit-type-scope-check.test.mjs`, picked up automatically by the existing
`make test-scripts` / `node --test "scripts/**/*.test.mjs"` glob — no CI wiring needed for
the tests themselves), plus a thin `main()` that reads `GITHUB_*` env, calls `gh api` for
the PR's changed files, and upserts the comment via a shared `scripts/lib/gh-comment.mjs`
helper (`findCommentId` + `upsertIssueComment`) — extracted here rather than copy-pasted a
second time, since `release-candidate-comment.mjs` already implements the identical
marker-based upsert. Both scripts also share `run`/`runStrict` from `scripts/lib/imagetools.mjs`
rather than each redefining their own `execFile` wrapper.

### 6. Allowlist, not the `docs/**`-denylist `jira-sync.yml` uses

`jira-sync.yml`'s `docs_only` guard asks a *symmetric* question — "is every changed file
under `docs/**`" — which trivially covers any current or future non-doc directory. This
check instead **allowlists** specific glob patterns (§2). That's a real trade-off, not an
oversight: a denylist ("anything not under `docs/**`") would also have to enumerate every
legitimately docs/tooling-scoped path to avoid false-positiving on them — `.claude/`,
`.github/`, `scripts/`, `flightplan/`, and root config files are all `chore`-typed by
convention (CLAUDE.md, "Commit type for Claude/agent-tooling changes") and would need
their own exclusion list, which is the same maintenance burden in the opposite direction.
The allowlist is scoped to what this ADR can actually justify as "real product/infra code
that must never hide behind `docs`/`chore`" — `internal/`, `cmd/`, `providers/`, and
`web/src/` — and stays silent (correctly permissive) on anything else, including this very
check's own `scripts/`/`.github/` changes. **Accepted trade-off:** a new product-code
directory added later needs a matching glob added here, the same way `jira-sync.yml`'s
`docs_only` guard needs no such addition. Revisit if a real PR slips through an
uncovered product directory — evidence, not speculation, should drive widening this list.

## Options Considered

### Where to source the commit type

| Option | Assessment |
|---|---|
| **PR title (chosen)** | Matches exactly what squash-merge writes to `main` and what `release-please`/`git-cliff` parse. One string, no ambiguity. |
| Every commit on the branch | Wrong signal for a squash-merge repo — individual commits never reach `main`'s history; a branch can have any mix of WIP commit messages that squash-merge discards entirely. Would flag or clear based on messages nobody will ever see in the changelog. |
| The squash commit body GitHub *would* produce | Not observable before merge (only computed at merge time), and defaults to the PR title anyway when the repo has one commit or the title is used — no information gain over reading the title directly, at the cost of needing a merge simulation. |

### Enforcement level

| Option | Assessment |
|---|---|
| **Advisory comment (chosen)** | Correct default for a solo-maintainer repo: false positives (single-line comment fixes) are real and shouldn't block a merge. Matches the existing advisory posture of `release-candidate-comment.mjs`. |
| Hard-blocking status check | Rejected for now — the threshold (§2) is a heuristic, not a certainty, and a false-positive block on a trivial PR is worse than an occasionally-late comment. Revisit if the advisory comment is repeatedly ignored (evidence-driven, same posture as ADR-069's rejected `gate-only` label alternative). |
| GitHub `check_run` with `neutral` conclusion | More GitHub-native than a plain comment (shows in the Checks tab, dismissible), but adds a second API surface (Checks API vs. Issues API) for marginal benefit over a comment on a repo this size; the sticky-comment pattern is already proven here. Revisit if comments prove easy to miss. |

### Folding into `jira-sync.yml`

| Option | Assessment |
|---|---|
| **New workflow (chosen)** | Different trigger set, different concern, no shared credentials — see §4. |
| New job in `jira-sync.yml` | Rejected — would force `jira-sync.yml`'s narrower `types:` list wider for an unrelated concern, and couples a Jira-credential-dependent workflow to a check that needs none. |

## Consequences

**Easier**
- A mistyped `docs`/`chore` PR that ships real code gets caught at review time, before it's
  merged and hidden from the changelog.
- The comment is self-correcting — retype the title or trim the diff, and the next push
  updates the same comment rather than leaving a stale warning.

**Harder / newer**
- One more advisory comment a reviewer can learn to ignore if it fires too often; the
  threshold (§2) exists specifically to keep the false-positive rate low enough that
  doesn't happen. Revisit the threshold on evidence, not speculation.
- Doesn't catch the case a commit type is *right* but the PR bundles unrelated docs and
  code changes that individually would each look fine — this check only compares type vs.
  diff-scope, not diff-cohesion.

**Unchanged / out of scope**
- The one-branch-to-many-issues gap (see "Non-goal" above) — a separate problem, not
  addressed here.
- `jira-sync.yml`'s existing `docs_only` guard (HOLODEX-173/220) is unchanged; this ADR
  adds a second, independent signal rather than modifying that one.
- Security review runs against the implementation diff (not this ADR): the workflow reads
  untrusted PR title/file-list data on plain `pull_request` (never `pull_request_target`),
  so fork PRs get no secrets and the values it handles reach only `gh api` calls, never a
  shell interpolation — worth explicit confirmation in review given the branch/PR-derived
  input.

## Action Items

1. [x] Add `scripts/commit-type-scope-check.mjs` (+ `.test.mjs`) — pure `classify()` +
   comment-upsert `main()`, following `release-candidate-comment.mjs`.
2. [x] Extract `scripts/lib/gh-comment.mjs` (`findCommentId` + `upsertIssueComment`) so the
   marker-based sticky-comment pattern isn't copy-pasted a second time.
3. [x] Add `.github/workflows/commit-type-scope-check.yml` — `pull_request: [opened,
   synchronize, edited, ready_for_review]`, `permissions: pull-requests: write`.
4. [ ] Run `/security-review` on the implementation diff before merge.
4. [ ] Update the ADR index (README).
