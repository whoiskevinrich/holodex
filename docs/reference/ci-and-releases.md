# CI and releases

What runs on GitHub Actions, what each workflow guards, and how a merge on `main` becomes a
versioned release. How the release *image* is produced (promote by retag) is in
[`docs/architecture/deployment.md`](../architecture/deployment.md); how to canary a candidate
before shipping it is in [`canary-releases.md`](canary-releases.md); the Jira transitions CI
fires are in [`jira-pipeline.md`](jira-pipeline.md).

---

## Workflows

| Workflow | Trigger | Does |
|---|---|---|
| `ci.yml` | `pull_request`, push to `main`, `workflow_call` | **backend** (`make vet`, `make test-go`) · **frontend** (`npm ci`, `npm run check`, Vitest, `npm run build`) · **scripts** (`make test-scripts`) · **theming** (grep guard: no hardcoded palette/radius in `*.svelte`) · **secrets** (gitleaks over full history) |
| `codeql.yml` | PR and push to `main`, weekly cron | CodeQL static analysis for Go and JS/TS |
| `commit-type-scope-check.yml` | PR `opened` / `synchronize` / `edited` / `ready_for_review` | Advisory comment when a `docs`/`chore` title carries real code (below) |
| `worklog-gate.yml` | PR `opened` / `ready_for_review` / `synchronize` / `reopened` | Flightplan's server-side worklog gate (holds a non-draft PR to its epic's definition of done) |
| `jira-sync.yml` | PR `opened` / `ready_for_review` / `closed` | `In Review` / `Done` transitions — see [`jira-pipeline.md`](jira-pipeline.md) |
| `image.yml` | push to `main` (path-scoped), `workflow_dispatch` | Multi-arch core image → `edge` + `sha-<short>`, then a Trivy scan into the Security tab |
| `provider-tmdb.yml` | push to `main` (path-scoped), `workflow_dispatch` | Same, for the TMDB provider sidecar image |
| `release-please.yml` | push to `main` | Maintains the rolling release PR; merging it cuts the `v*` tag |
| `release-candidate.yml` | completion of either image build or Release Please, `workflow_dispatch` | Advisory digest/freshness comment on the release PR — see [`canary-releases.md`](canary-releases.md) |
| `release.yml` | push of a `v*` tag | `ci` → `promote` (retag the canaried digests) → `github-release` (notes, `prod` deployment, Jira `Released`) |
| `scan.yml` | weekly cron, `workflow_dispatch` | Trivy against the published `:latest` and `:edge` of both images with a fresh CVE database |
| `pages.yml` | push to `main` touching `site/` | Deploys the static landing page |
| `dependabot.yml` | weekly | Update PRs for `gomod`, `npm`, `github-actions`, `docker` |

The check-style workflows (`ci`, `codeql`, the two image builds, `commit-type-scope-check`,
`release-candidate`) carry a `concurrency` group with `cancel-in-progress`, so a superseded run on
a rapid push is cancelled. Workflows with side effects that must finish — `release-please`,
`jira-sync`, `pages`, `scan` — queue instead of cancelling.

### Permissions

Workflows use the built-in `GITHUB_TOKEN` with per-job least privilege: `contents: read` by
default, `packages: write` only to push or retag images, `security-events: write` only to upload
SARIF, `contents: write` only to create a Release, `pull-requests: write` only to post or update
a comment. PR-triggered workflows use plain `pull_request`, **never** `pull_request_target`, so
fork PRs get no secrets and untrusted branch or title text never reaches a privileged context.

The stored credentials are exactly: `RELEASE_PLEASE_TOKEN` (below) and the Jira trio
(`JIRA_BASE_URL` variable, `JIRA_USER_EMAIL` / `JIRA_API_TOKEN` secrets — see
[`jira-pipeline.md`](jira-pipeline.md)).

---

## The merge gate

`ci.yml` and the worklog gate are the checks a PR must pass to merge. The other PR workflows
are advisory: they comment or annotate, they don't block.

The repository ruleset `main` enforces this on `refs/heads/main` and `refs/heads/release/**`
(HOLODEX-546):

| Rule | Setting |
|---|---|
| Required status checks | `backend`, `frontend`, `theming`, `scripts`, `secrets` (`ci.yml`), `gate` (worklog gate) |
| Deletion, force-push | Blocked |
| Bypass | Repository admin, for an emergency hotfix |

- **Required names are job names, not workflow names.** The worklog gate reports as `gate`. A
  required `worklog gate` would never report, and every PR would wait on it forever. Renaming a
  `ci.yml` job means updating the ruleset in the same change.
- **A required check must run on every PR event that changes the head.** That is why
  `transition` (`jira-sync.yml`) is not required: it only runs on `opened`, `ready_for_review` and
  `closed`, so any later push would leave it unreported.
- **CodeQL's `analyze` jobs are not required.** Their findings land in the Security tab, not the
  merge decision. If the ruleset ever requires them, list them in the table above, and keep
  `codeql.yml` free of a `paths:` filter, or docs-only PRs will wait forever.
- **Drafts skip the worklog gate**, and a skipped check counts as passing. Nothing slips through,
  because a draft can't be merged, and marking it ready re-runs the gate.
- The release path still re-runs CI on the tag, so a red tree can't be promoted even through
  the bypass.

**CI does not path-filter.** It is cheap enough that a `paths:` filter isn't worth it, and on a
required check a filter would leave docs-only PRs with a status that never reports.

**`secrets` scans the whole history, not the diff** (`fetch-depth: 0`). A secret added in one
commit and "removed" in the next still fails, because it is still in the objects. That makes it
the one CI failure that can't be fixed forward: once a value is pushed, rotate it. The local-dev
side of this rule is in [`dev-credentials.md`](dev-credentials.md).

**No CI on design-phase pushes.** `ci.yml` runs on `pull_request` and on push to `main`, never on
a plain branch push. A keyed branch in its design phase has no PR yet (see
[`workflow-idea-to-merge.md`](workflow-idea-to-merge.md)), so its pushes get no checks until the
PR opens. Run `make test-scripts` locally for tooling that rides a design phase.

### Image builds are path-scoped

`image.yml` and `provider-tmdb.yml` rebuild on a push to `main` only when something that lands in
that image changes. Each lists its inputs explicitly rather than ignoring docs, so a new
top-level source directory fails closed — it doesn't match, so nothing silently ships unbuilt,
but it does mean **a new image-relevant path must be added to the list**.

| Image | `paths:` |
|---|---|
| core | `cmd/**`, `internal/**`, `web/**`, `go.mod`, `go.sum`, `Dockerfile`, `.dockerignore`, the workflow itself |
| provider-tmdb | `providers/tmdb/**`, `Dockerfile.provider-tmdb`, `go.mod`, `go.sum`, the workflow itself |

Docs-, spec- and `site/`-only merges build nothing, which leaves `edge` behind `main` while
still being correct. `workflow_dispatch` always builds; use it to force a rebuild. The same lists
are what the release-candidate freshness check measures against, so widening one widens that
check too.

Trivy runs after the push (a multi-arch manifest can't be loaded locally), so a finding lands in
the Security tab rather than blocking the publish. `scan.yml` re-scans the deployed tags weekly
so a quiet week with no build still gets a scan.

### Advisory: commit type vs. diff scope

The repo is **squash-only**, so the PR title becomes the merge commit, and the merge commit is
what release-please and git-cliff parse. `docs` and `chore` are hidden from the changelog, so a
PR titled `docs(...)` that carries real code ships unannounced.

`commit-type-scope-check.yml` compares the title's type with the changed files and posts one
sticky comment (marker `<!-- holodex-commit-type-scope -->`) when a `docs`/`chore` title meets
real code. It flags when:

- **more than one** changed file is under `internal/**/*.go`, `cmd/**/*.go`,
  `providers/**/*.go`, `web/src/**/*.svelte` or `web/src/**/*.ts`; or
- **any** file under `internal/db/migrations/` changed.

One stray file is tolerated (a comment fix in a `.go` file while writing a spec). The globs are
an allowlist of product code, so `.claude/`, `.github/`, `scripts/` and root config stay silent;
**a new product-code directory needs a glob added** in `scripts/commit-type-scope-check.mjs`.
The workflow always exits 0. Retitle the PR or trim the diff and the next run updates the same
comment to say so.

---

## Versioning and release notes

### Conventional Commits drive both the version and the notes

Commit subjects — in practice, PR titles — are `type(scope): summary`. Both tools read them:

| Type | Changelog section | Shown? |
|---|---|---|
| `feat` | Features | yes |
| `fix` | Bug Fixes | yes |
| `perf` | Performance | yes |
| `refactor` | Refactor | yes |
| `docs` | Documentation | yes |
| `test` | Testing | yes |
| `ci`, `build` | CI / Build | yes |
| `revert` | Revert | yes |
| `chore`, `style` | — | **hidden** |

The sections are kept identical in `cliff.toml` and `release-please-config.json`. Agent and
dev-tooling changes (`.claude/`, hooks, skills, Flightplan plumbing) use `chore(...)` so they
stay out of release notes; neither tool can filter by scope. The Jira key never goes in the
subject — the branch name carries it.

### Release Please computes the version

`release-please.yml` keeps one open **release PR** on `main` that accumulates the next version
and `CHANGELOG.md` from the commits since the last tag. **Merging the release PR is the decision
to ship.** It creates the `vX.Y.Z` tag and a GitHub Release.

Config (`release-please-config.json` + `.release-please-manifest.json`): release type `go`,
`v`-prefixed tags, no component in the tag, `bump-minor-pre-major`. The git tag is the version
of record; no version file is bumped, and `web/package.json`'s version is unrelated. The
release PR's commit touches only the manifest and `CHANGELOG.md`.

**`RELEASE_PLEASE_TOKEN` is load-bearing.** A tag pushed with `GITHUB_TOKEN` does not trigger
other workflows, so the release PR would still work but `release.yml` would never fire — no
images promoted, no published Release. The secret is a fine-grained PAT (or GitHub App token)
with Contents and Pull requests read/write on this repo. It expires; rotate it before it does.
The workflow falls back to `GITHUB_TOKEN` silently, so an expired token shows up as a tag with no
release run.

**Break-glass:** pushing a `v*` tag by hand (`git tag vX.Y.Z && git push origin vX.Y.Z`) fires
`release.yml` directly.

### `release.yml` publishes

On a `v*` tag:

1. **`ci`** — reuses `ci.yml` via `workflow_call`, so the release re-runs exactly the merge
   checks. `promote` `needs: ci`, so a failing check stops the release.
2. **`promote`** — retags the already-built, canaried digests of both images as the exact
   version plus whichever of `X.Y`, `X` and `latest` it is the highest release for
   (`scripts/release-tags.mjs`). No rebuild; the ancestry checks that make this safe are in
   [`canary-releases.md`](canary-releases.md#what-protects-the-release).
3. **`github-release`** —
   - renders the release body with **git-cliff** `--latest` from `cliff.toml`. That body
     overwrites the one Release Please wrote, so **git-cliff is the published release note** and
     Release Please's `CHANGELOG.md` is the staging copy;
   - appends `docker pull` lines and GHCR package links for both images;
   - declares `environment: prod` (URL: the package page), which records a GitHub **Deployment**
     natively and links Release ↔ Deployment ↔ Environment. It is named `prod`, not `ghcr`, so
     GitHub-for-Jira maps it to Production. The environment has no protection rules;
   - marks the GitHub Release "Latest" only when the image promotion took `latest`;
   - moves every Jira issue in `Done` to `Released` (`scripts/jira-release-sync.mjs`, soft-fail),
     only when the tag is reachable from `origin/main`.

`environment:` sits on the release job rather than the image jobs so a deployment is recorded
once per release, not on every build.

### Hotfix releases

For a fix to the shipped line while `main` is not ready to release. The rationale is in
[deployment.md](../architecture/deployment.md#hotfixes-promote-from-a-releasevxy-branch).

1. Cut the branch from the shipped tag, if it doesn't exist yet:
   `git switch -c release/v1.16 v1.16.1 && git push -u origin release/v1.16`.
2. Commit the fix to `release/v1.16` and push. If `release.yml`, `image.yml` or
   `scripts/release-tags.mjs` on the branch predate HOLODEX-545, carry that change too. The tag runs
   the branch's copies, not `main`'s.
3. Wait for **Build image** on the branch to publish `sha-<short>`. Optionally pull and run it.
4. Tag and push: `git tag v1.16.2 && git push origin v1.16.2`. Release Please is not involved,
   because its PR lives on `main`.
5. Move the hotfix's own Jira issues to `Released` by hand. The automatic sync is skipped off `main`.
6. If `main` needs the same fix, land it there through a normal PR.
