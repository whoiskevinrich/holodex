# ADR-035: CI/CD Refinements — Image Path-Scoping, CodeQL Concurrency, Release Test-Gate

**Status**: Accepted
**Date**: 2026-06-16
**Deciders**: Project owner
**Extends**: [ADR-024](ADR-024-ci-cd-pipeline.md) (CI/CD pipeline), [ADR-023](ADR-023-image-distribution.md) (image distribution)

## Context

[ADR-024](ADR-024-ci-cd-pipeline.md) stood up the pipeline (CI gate, CodeQL, image build,
tag-driven release). After ~10 days of runs (169 across 6/6–6/16) the run history exposed three
inefficiencies worth correcting:

- **The multi-arch image rebuilds on every push to `main`, regardless of what changed.** It is
  the pipeline's long pole — averaging ~5 min (max ~9.5 min), ~10× CI's 32 s — because the arm64
  leg builds under QEMU emulation. Docs-, spec-, and `site/`-only merges paid the full build for
  an image whose bytes were unchanged.
- **`codeql.yml` had no `concurrency` group** (CI, image, and pages all did). Iterative PR pushes
  left superseded CodeQL runs (~89 s each) running to completion instead of being cancelled.
- **The release path did not gate the image on tests.** `release.yml` called `image.yml` directly
  on a `v*` tag; a tag can point at *any* commit, so a tagged build could publish `latest` without
  a single test having run. (The image build itself only catches frontend *build* breakage via the
  Dockerfile's stage 1 — not `go test`, Vitest, `svelte-check`, or the theming guard.)

The empirical case also *narrowed* what's worth optimizing: CI at 32 s is too cheap to path-filter
(the saving is dwarfed by the required-check complexity it would add), and the historical 28 %
image-build failure rate was entirely 6/12 bring-up ("Set up job" config failures, fixed by PR #11)
— 13/13 green since — so these changes are efficiency/correctness, not reliability, plays.

## Decision

Three changes, all within the existing four-workflow shape from ADR-024:

1. **Path-scope the image build's push trigger.** `image.yml`'s `push: [main]` trigger gains a
   `paths:` filter listing only what lands in the image — `cmd/**`, `internal/**`, `web/**`,
   `go.mod`, `go.sum`, `Dockerfile`, `.dockerignore`, and `image.yml` itself. `workflow_dispatch`
   and `workflow_call` carry no `paths`, so **tagged releases and manual runs always build.**
   Dependabot base-image / module bumps still rebuild (they touch `go.mod`/`Dockerfile`).

2. **Add a `concurrency` group to `codeql.yml`** — `codeql-${{ github.ref }}`,
   `cancel-in-progress: true` — matching CI/image/pages.

3. **Gate the release on CI.** `ci.yml` gains `workflow_call` so it is reusable; `release.yml`
   now runs `ci` first and makes `image` `needs: ci`. The tag thus re-runs the *exact* PR-merge
   checks (backend, frontend, theming) before any image is built or published — reusing the
   workflow rather than duplicating its steps, mirroring how `release.yml` already reuses
   `image.yml`.

### Rationale for the shape

- **Reuse over duplication.** Making `ci.yml` `workflow_call`-able keeps the release gate in
  lockstep with the merge gate automatically — if CI's checks change, the release inherits them.
  This is the same pattern ADR-024 chose for `image.yml`.
- **`paths:` is safe on the image trigger specifically** because the image build never runs as a
  PR-required check (it's `push`/tag/dispatch only). The equivalent on CI's `pull_request` leg was
  rejected: CI is a branch-protection-required check (ADR-024), so a top-level `paths:` filter
  would leave docs-only PRs with a perpetually-pending required status — and CI is too cheap to be
  worth the change-detection job that would avoid it.
- **List image inputs explicitly** (rather than `paths-ignore` of docs/site) so a *new* top-level
  source directory fails closed — it won't match the filter, so it won't silently ship unbuilt
  until someone notices. The trade-off: adding a new image-relevant path means updating the list.

## Consequences

- Docs-, spec-, and `site/`-only merges to `main` no longer trigger the ~5 min image build; the
  landing-page deploy (already `site/**`-scoped, ADR-024) remains the only workflow they fire.
- A `v*` tag now runs CI → image → release in sequence; release wall-clock grows by CI's ~30 s,
  and a failing test now blocks publication of `latest` instead of shipping it.
- Superseded CodeQL runs are cancelled on rapid PR pushes, reducing redundant Actions minutes.
- **Infrastructure/access surface is unchanged or reduced.** No new secrets; the reusable `ci`
  call runs with `contents: read` only. Path-scoping *narrows* when the `packages: write` image
  job runs. Ran through `/security-review`.
- Risk fail-closed-not-open: if the image `paths:` list ever omits a genuinely image-affecting
  path, that change would merge without rebuilding the image until a covered path also changes — a
  staleness bug, not a security one. The explicit list and `image.yml`-self-inclusion keep it
  visible in review.
