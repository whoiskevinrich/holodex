# ADR-024: CI/CD Pipeline — PR Gate, Supply-Chain Scanning, Tag-Driven Releases

**Status**: Accepted
**Date**: 2026-06-11
**Deciders**: Project owner

## Context

[testing-strategy.md §8](../testing-strategy.md) specifies the intended CI pipeline (lint /
unit / integration / e2e / scanning), but the repo had no CI — only the image-publish workflow
from [ADR-023](ADR-023-image-distribution.md). Nothing verified a PR before merge, and
dependency CVEs (e.g. the `go-chi` advisory `GHSA-vrw8-fxc6-2r93`) were caught only by
GitHub's ambient Dependabot *alerting*, then fixed by hand. We want merge-gating quality
checks, automated dependency updates, static + image security scanning, and a coherent
tag-driven release — implemented incrementally, starting with what exists today.

## Decision

GitHub Actions, four workflows plus Dependabot config under `.github/`:

| Workflow | Trigger | Does |
|---|---|---|
| `ci.yml` | `pull_request`, push to `main` | **backend** (`go vet`, `go test ./...`), **frontend** (`npm ci`, `svelte-check`, Vitest, `vite build`), **theming** (grep guard: no hardcoded palette/radius in `*.svelte`, per ADR-021). |
| `codeql.yml` | PR, push to `main`, weekly cron | CodeQL static analysis for `go` + `javascript-typescript`. |
| `image.yml` | push to `main`, `workflow_dispatch`, `workflow_call` | Reusable multi-arch build/push (ADR-023) + Trivy image scan → Security tab. |
| `release.yml` | push tag `v*` | Calls `image.yml` (semver + `latest`), then cuts a GitHub Release with auto-notes. |
| `dependabot.yml` | weekly | Update PRs for `gomod`, `npm`, `github-actions`, `docker`. |

### Rationale for the shape

- **Reusable image workflow.** `image.yml` is `workflow_call`-able so `release.yml` reuses the
  exact build path instead of duplicating it; the `v*` trigger lives only in `release.yml`, so
  a tag builds the image **and** the Release together with no double-build.
- **Branch protection** on `main` requires the `ci.yml` checks (`backend`, `frontend`,
  `theming`) before merge — workflows that don't gate merge are advisory only.
- **Latest action majors**, per standing preference (checkout v6, setup-go/node v6, docker
  actions v4–v7, codeql v4, gh-release v3, trivy 0.36.0).
- **Start minimal, grow.** Linters named in the testing strategy (`golangci-lint`, `eslint`,
  `prettier`) and Vitest specs are **not yet present**; CI runs `go vet` + `go test` +
  `svelte-check` + build + the theming grep today, with `--passWithNoTests` on Vitest and a
  commented integration step ready for when `testdata/gen.sh` + golden fixtures land.

## Consequences

- Every PR is gated on real checks; the `go-chi`-style fix is now automated by Dependabot.
- Trivy scans run **after** push (multi-arch manifests can't be loaded locally), so a finding
  surfaces in the Security tab rather than blocking publish — acceptable for the current
  cadence; can move pre-push with single-arch scanning later if needed.
- **CodeQL uses `build-mode: manual` for Go** (explicit `go build ./...`), *not* autobuild:
  autobuild runs `make` with no target, which hits the Makefile's default `run` target
  (`go run ./cmd/holodex`) and hangs booting the server. The frontend uses `build-mode: none`.
  (The init input is `languages`, plural — `language` is silently ignored.)
- **Code scanning requires the repo to be public** (or GHAS on a private repo): CodeQL and the
  Trivy SARIF upload both publish to GitHub code scanning, which is unavailable on a private
  repo without Advanced Security. The repo was made public (2026-06-11) to enable this; the
  full git history was secret/PII-scanned clean first.
- This is an **infrastructure/access** change: workflows use only the built-in `GITHUB_TOKEN`
  with per-job least-privilege scopes (`contents: read` by default; `packages: write` only to
  push; `security-events: write` only to upload SARIF; `contents: write` only on the release
  job). No stored secrets. Ran through `/security-review`.
- Standalone release binaries are **out of scope** for now (the image is the primary artifact);
  the embed build makes them more involved — a later addition if demand appears.
