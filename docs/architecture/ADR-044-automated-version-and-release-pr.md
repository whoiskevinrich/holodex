# ADR-044: Automated version computation + release-PR gate (Release Please)

**Status**: Accepted
**Date**: 2026-06-27
**Deciders**: Project owner
**Relates to**: ADR-024 (CI/CD — tag-driven releases), ADR-034 (release notes + GHCR deployment linkage), ADR-035 (release test-gate). Extends all three; **revisits ADR-034's "no release-please" rationale**.

---

## Context

Releases are cut by hand: the maintainer picks the next version and runs
`git tag v0.1.0 && git push origin v0.1.0`, which fires `release.yml` (ADR-024/034/035:
CI gate → semver + `latest` images for both the runtime and TMDB-provider → git-cliff
release body + `ghcr` Deployment link). ADR-034 §Rationale deliberately **declined**
release-please/semantic-release as "overkill for a single-maintainer repo that tags by
hand," leaving the seam open "if that changes."

Two frictions made it change:

1. **Version is computed in the maintainer's head.** Nothing maps the accumulated
   `feat`/`fix`/`breaking` commits to the correct semver bump. It's easy to tag a
   patch when a `feat` landed, or to forget what's shipped since the last tag.
2. **No staging view of "what the next release contains."** The changelog only
   materialises *after* the tag (git-cliff `--latest` in `release.yml`), so there's no
   pre-tag artifact to review and no record of the pending version.

We still want to keep the existing release machinery — the git-cliff changelog (ADR-034),
the dual-image build, and the `ghcr` deployment linkage are all working and proven. We
only want to *automate the version decision and stage it for approval*, not rebuild the
release.

## Decision

Adopt **Release Please** (`googleapis/release-please-action@v4`) in **manifest mode** as
a thin version-computation + release-PR layer **in front of** the unchanged `release.yml`.

### 1. Rolling release PR on `main`
A new workflow `.github/workflows/release-please.yml` runs on `push: main`. Release
Please maintains a single open "release PR" that accumulates the next version (from
Conventional-Commit history) and a `CHANGELOG.md`. Merging that PR **is** the explicit
"ship it" gate — the manual decision moves from "which version do I type" to "merge the
release PR when ready."

Config: `release-please-config.json` (release-type **`go`**, `v`-prefixed tags, pre-1.0
minor bumps via `bump-minor-pre-major`) + `.release-please-manifest.json` (seeded
`"."`: `0.0.0`). Changelog sections mirror `cliff.toml`'s categories.

### 2. Tag still drives the existing pipeline — via a non-`GITHUB_TOKEN` identity
Merging the release PR makes Release Please create the `vX.Y.Z` tag and a GitHub Release.
GitHub's recursion guard means a tag pushed by the default `GITHUB_TOKEN` **does not
trigger another workflow** — so `release.yml` would never fire. Release Please is
therefore given a **fine-grained PAT** (`RELEASE_PLEASE_TOKEN`, Contents + Pull-requests
read/write); the PAT-created tag triggers `release.yml` exactly as a hand-pushed tag did.

### 3. git-cliff remains the published changelog (ADR-034 preserved)
`release.yml` is **unchanged**. Its `softprops/action-gh-release` step is create-**or-update**
by tag: it updates the release Release Please just created, overwriting the body with the
git-cliff `--latest` render. So the **published** notes and the `ghcr` Deployment link are
still authored by `release.yml` per ADR-034. Release Please's auto-generated release body
and committed `CHANGELOG.md` are the staging/review artifact; git-cliff is the
release-of-record. (`image.yml`, `provider-tmdb.yml`, `cliff.toml` are untouched.)

## Rationale

- **Smallest blast radius.** The proven build/scan/release pipeline (ADR-024/034/035) is
  not modified. Release Please only computes the version and stages the PR; the tag it
  pushes flows through the identical path a manual tag did.
- **Removes the one manual judgement that was error-prone** — the semver bump — while
  keeping a human gate (merging the release PR), which suits a single-maintainer repo
  better than fully-automatic release-on-every-merge.
- **Honors ADR-034.** git-cliff stays the published changelog author; this ADR adds a
  version-decision layer rather than replacing the notes mechanism. The redundant
  release-please `CHANGELOG.md` is a useful committed side-artifact, not the source of
  the GitHub Release body.
- **`go` release-type** matches the primary module and avoids churning a version file —
  the git tag remains the source of truth (the web `package.json` version is unrelated
  and intentionally not bumped).

## Consequences

- New committed files: `release-please-config.json`, `.release-please-manifest.json`,
  `.github/workflows/release-please.yml`, and (on first release-PR merge) `CHANGELOG.md`.
- **New required secret `RELEASE_PLEASE_TOKEN`** (a PAT/GitHub-App token). This is
  load-bearing: with only `GITHUB_TOKEN`, the release PR still works but the tag will
  **not** trigger `release.yml` — no images, no published release. The PAT is a new
  rotation/secret-hygiene responsibility (a GitHub App is the no-expiry alternative).
- **New supply-chain dependency**: `googleapis/release-please-action` (pinned `@v4`,
  github-actions dependabot ecosystem). `/security-review` of the new workflow is the
  sign-off gate before merge (CLAUDE.md routing: infra change).
- The `ghcr` Environment is created (no protection rules — single maintainer; matches
  ADR-034's stance).
- **First-run note:** with the manifest seeded at `0.0.0`, the first release PR
  enumerates the full commit history into `CHANGELOG.md` (cosmetic). A one-time
  `bootstrap-sha` in the config trims it if desired. First computed release is **v0.1.0**
  (pre-1.0, `bump-minor-pre-major`).
- Changelog quality still depends on **Conventional-Commit discipline** (unchanged from
  ADR-034) — Release Please now *also* relies on it for the version bump itself.
- The manual `git tag v*` path still works as a break-glass fallback (it triggers
  `release.yml` directly), so adopting Release Please is reversible: delete the workflow
  + config to return to ADR-034's hand-tag flow.
