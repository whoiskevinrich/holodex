# ADR-034: Release notes (Conventional-Commit changelog) + GHCR deployment linkage

**Status**: Accepted
**Date**: 2026-06-15
**Deciders**: Project owner
**Relates to**: ADR-023 (image distribution — GHCR), ADR-024 (CI/CD pipeline — tag-driven releases). Extends both.

---

## Context

Tag-driven releases (ADR-024) publish the GHCR image and cut a GitHub Release. The
existing release job used GitHub's `generate_release_notes: true` — a **flat,
uncategorised list of merged PR titles** — plus a `docker pull` snippet. Three gaps:

1. **No human-readable, categorised changelog.** A flat PR list doesn't separate
   features from fixes from docs, and reads as noise as the project grows.
2. **No Release ↔ Deployment linkage.** Publishing the image to GHCR isn't recorded
   as a GitHub *Deployment*, so there's no Environments view of "which version was
   published when," and the Release/commit carries no deployment badge.
3. **No direct link to the Package.** The notes told you the pull command but didn't
   link the GHCR package page.

The repo already uses **Conventional Commits** (`feat`/`fix`/`docs`/`refactor`/…),
which unlocks commit-driven changelog generation without PR-label bookkeeping. The
release flow is a **manual `git tag v*`** (single maintainer); we want to keep that —
not adopt a release-PR bot.

## Decision

Three additive changes to the release workflow (`.github/workflows/release.yml`),
no change to the manual-tag flow:

### 1. Changelog from Conventional Commits — git-cliff
A committed **`cliff.toml`** (based on the git-cliff default template, with Holodex
categories: Features / Bug Fixes / Performance / Refactor / Documentation / Testing /
CI · Build / Revert; `chore`/`style` skipped). The release job runs
`orhun/git-cliff-action` with `--latest` to render the just-tagged version's sections,
and uses that as the Release body. This replaces `generate_release_notes`.

### 2. GHCR publish as a GitHub Deployment — the `environment:` key
The `github-release` job declares `environment: { name: ghcr, url: <package-url> }`.
GitHub Actions records a **Deployment to the `ghcr` environment** on each tagged run,
giving native **Release ↔ Deployments ↔ Environments** linkage with the package page
as the environment URL. The runner records it natively (no `deployments: write`, no
extra action). The `environment:` lives on the release job (not the reusable `image`
job — reusable-workflow caller jobs can't take `environment:`, and we don't want a
deployment recorded on every PR image build).

### 3. Package link in the body
The Release body appends a **📦 Package** link to the GHCR package page alongside the
existing `docker pull` / compose-pin snippet.

## Rationale

- **Conventional Commits we already write** make git-cliff a zero-bookkeeping source of
  a categorised changelog — better signal than a flat PR list, no PR labels needed.
- **`environment:` is the lightest native deployment linkage** — two lines, no API
  token, no third-party deployment action. For a GHCR-distribution project (ADR-023)
  the "deployment" is the package publish; the env URL points at the package. If a real
  deploy target (NAS/cloud) is added later, that step can own a richer Deployment.
- **Manual-tag flow preserved.** We deliberately did *not* adopt release-please /
  semantic-release (release-by-merging-a-PR + automated version bumps) — overkill for a
  single-maintainer repo that tags by hand. The seam is open if that changes.

## Consequences

- New committed `cliff.toml`; `release.yml` gains the git-cliff step + `environment:`
  key + package link, and drops `generate_release_notes`.
- New **supply-chain dependency**: `orhun/git-cliff-action` (pinned to `@v4`, matching
  the repo's major-tag pin convention; covered by the github-actions dependabot
  ecosystem). A `/security-review` of the workflow change is the sign-off gate before
  merge (CLAUDE.md routing: infra change).
- The `ghcr` **Environment** is auto-created on first use; no protection rules needed
  for a single-maintainer repo (add reviewers later if desired).
- Changelog quality now depends on **commit-message discipline** — keep writing
  Conventional Commits (`type(scope): summary`); `chore`/`style` are intentionally
  excluded from notes.
- Validation is on the **next tag** (CI workflows can't be fully exercised pre-merge);
  the first `v*` after merge confirms the rendered notes + the Deployment entry.
