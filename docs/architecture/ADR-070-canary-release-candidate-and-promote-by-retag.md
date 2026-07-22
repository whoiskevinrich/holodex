# ADR-070: Canary the release candidate, then promote it by retagging the same digest

**Status**: Proposed
**Date**: 2026-07-22
**Deciders**: Project owner
**Relates to**: ADR-023 (image distribution — GHCR), ADR-024 (CI/CD — tag-driven releases),
ADR-034 (release notes + GHCR deployment linkage), ADR-035 (image path-scoping + release
test-gate), ADR-044 (Release Please release-PR in front of an unchanged `release.yml`).
Amends ADR-035's release test-gate and ADR-044's promotion step. Issue: HOLODEX-206.

---

## Context

Every merge to `main` publishes `ghcr.io/whoiskevinrich/holodex:edge` and
`:sha-<short>` (`image.yml`), with the provider sidecar mirroring that in
`provider-tmdb.yml`. Release Please (ADR-044) then accumulates the merged commits into a
release PR that carries **only** the version bump, `CHANGELOG.md`, and its manifest — the
code itself is already on `main`.

That means `:edge` **is** the content of the open release PR. There is no separate
"release candidate" to build. The owner runs a canary instance against a real production
library (~3.5k files), and wants to validate that content before merging the release PR.

Three gaps stop `:edge` from serving as that canary artifact:

1. **No handle on the release PR.** Nothing connects the release PR to a pullable digest,
   and nothing asserts `edge` was built from current `main`. A build that failed, or that
   was skipped by the ADR-035 `paths:` filters, leaves `edge` silently stale — the canary
   then validates the previous state while appearing to validate the new one. The core and
   sidecar images also advance independently, so "pull both `:edge`" is not a
   guaranteed-coherent pair.

2. **Promotion rebuilds.** Merging the release PR creates the version-bump commit; the
   `v*` tag fires `release.yml`, which calls `image.yml` again and **builds from scratch**.
   Different base layers, different digest. The image published as `latest` is not the
   image that was canaried, which defeats the point of canarying it.

3. **No rollback handle.** Pulling by the moving `:edge` tag leaves no pinned previous
   version to fall back to, and no record of which commit was actually exercised.

## Decision

### 1. An advisory release-candidate comment on the release-please PR

A workflow triggered when the release PR is opened or updated resolves, for both the core
and `provider-tmdb` images:

- the current `:edge` digest,
- the commit it was built from (the `org.opencontainers.image.revision` OCI label written
  by `docker/metadata-action`),
- whether that commit matches `main` HEAD.

It posts — and thereafter updates in place — a single comment carrying both digests, the
freshness assertion per image, the commit range the release covers, and a copy-paste pull
command pinned to those digests.

The comment is **advisory**. It does not gate merge, set a status check, or block anything.
A stale or mismatched image is reported in the comment, not enforced.

Because the comment already parses the release's Conventional Commits, it is also where a
per-release manual-QA checklist is generated (a `fix(writeback):` in the set lists the
writeback checks; a `fix(web):` lists the three-skin pass). That generation is deferred to
a follow-up; this ADR only fixes the comment as its home.

### 2. The canary pins by digest, not by tag

The canary instance's compose pins `ghcr.io/whoiskevinrich/holodex@sha256:…` rather than
`:edge`. Rolling forward is pasting the digest from the current comment; rolling back is
pasting the one from the previous comment. This also makes "which bits are running"
answerable from the instance itself rather than inferred from a moving tag.

### 3. `release.yml` promotes by retagging the canaried digest

The `image` job in `release.yml` stops calling `image.yml` on the tag ref. Instead it:

1. **Resolves the release digest** — walks back from the tagged commit to the newest
   ancestor that has a published `sha-<short>` image. In practice this is one step: the
   release-please commit touches only `version.txt`, `CHANGELOG.md`, and the manifest, none
   of which appear in the ADR-035 `paths:` filters, so no image is built for it.
2. **Asserts ancestry** — reads that image's `org.opencontainers.image.revision` label and
   requires `git merge-base --is-ancestor <revision> <tagged-commit>`. This is the
   load-bearing guard; see below.
3. **Retags** — `docker buildx imagetools create` copies the digest to the semver and
   `latest` tags. This is a manifest write, not a build: seconds, no QEMU, no rebuild.

The same three steps apply to the sidecar image.

Unchanged: the git-cliff release notes, the `prod` environment/deployment linkage
(ADR-034), and the Jira → Released sync.

### 4. The ancestry assertion replaces the tag-time CI re-run

`release.yml` today re-runs `ci.yml` on the tag with the stated rationale *"a tag can point
at any commit, so re-run CI here rather than trust that the tagged commit passed on main"*
(ADR-035's release test-gate). Retag-by-digest removes the rebuild that gate was protecting.

The replacement is the ancestry assertion in step 2: the digest being promoted must derive
from a commit that is an ancestor of the tag. Combined with the branch protection that
requires `ci.yml` green to merge to `main`, that commit is one that passed CI. `ci.yml`
continues to run on the tag as a signal; it is no longer what produces the artifact.

**Without the ancestry assertion, an arbitrary or stale digest could be published under a
new version number with nothing to catch it.** It is not optional.

## Options Considered

### Promotion model

| Option | Verdict |
|---|---|
| **Retag the canaried digest** | **Chosen.** Identical bits end to end — the only option that makes canarying meaningful. Costs the tag-time rebuild as a gate, recovered via the ancestry assertion. |
| Keep rebuilding at the tag | Rejected. Simplest, no change, but the shipped image is a different build from the validated one. Usually only base-layer drift; "usually" is exactly the word that makes a release gate worthless. |
| Retag, but keep `ci.yml` as a required check | Partially adopted — `ci.yml` still runs on the tag as a signal, but it is not the artifact gate and does not re-derive the image. |

### Gating model

| Option | Verdict |
|---|---|
| **Advisory comment** | **Chosen.** Single maintainer; the failure mode ("merged without canarying") is self-inflicted and visible. Zero mechanism to maintain or route around. |
| Required status check flipped by hand (`/canary-ok`) | Rejected for now. A real gate, but on a solo repo it is a checkbox that gates only the person who ticks it. The seam is open if collaborators appear. |
| Canary self-reports its running digest | Rejected. Strongest signal, most moving parts — needs an endpoint, a token, and network reachability from CI to a home instance. |

### Release-candidate artifact

| Option | Verdict |
|---|---|
| **Reuse `:edge` / `sha-<short>`** | **Chosen.** Already built on every merge; release-please holds no code back, so `edge` already is the candidate. |
| Build `1.13.0-rc.N` images from the release PR branch | Rejected. The conventional answer, and pure ceremony here: the PR branch's runtime code is byte-identical to `main`. It would add a build, a tag namespace, and cleanup for zero new information. |

## Trade-off Analysis

**Standing assumption: the version-bump commit changes no runtime bytes.** True today —
there is no ldflags version injection in the `Dockerfile`, so the binary is identical
either side of the bump. If a version string is ever baked into the build, retag-by-digest
would publish an image whose embedded version disagrees with its tag. That failure would be
silent. Any change that introduces build-time version injection must revisit this ADR.

**The freshness check is reported, not enforced.** A `provider-tmdb:edge` that is two
commits behind shows up in the comment as a warning; nothing stops the release. That is
consistent with the advisory posture, and it is a real gap the owner accepts in exchange
for no gate mechanism. The comment must therefore be legible enough to actually read —
a noisy comment that gets scrolled past is the failure mode to design against.

**Retagging weakens provenance in one narrow way**: the published `1.13.0` image's
`revision` label points at the pre-bump commit, not the tagged commit. That is accurate —
it is where the bits came from — but it means the release tag and the image label differ by
one commit. Preferred over the alternative, where the label is accurate and the *bits* are
unvalidated.

**Trivy scanning is not re-run on promotion.** The digest was scanned when it was built as
`edge`. Retagging publishes an already-scanned artifact; a CVE disclosed between the edge
build and the release tag is not caught. The scheduled scan path is the mitigation, not a
tag-time re-scan of identical bits.

## Consequences

### Positive

- The image published as `latest` is byte-identical to the one exercised on the canary.
- Rollback becomes "run the previous digest" instead of a `sha-*` archaeology exercise.
- Release runs get dramatically faster — a manifest copy instead of two multi-arch QEMU
  builds.
- Core/sidecar version skew becomes visible at the moment it matters, on the release PR.

### Negative / limitations

- One more workflow to maintain, and it depends on the `org.opencontainers.image.revision`
  label continuing to be written by `docker/metadata-action`.
- The ancestry assertion is now load-bearing security-relevant logic in the release path;
  it needs a test, not just a code review.
- Freshness is advisory, so a stale sidecar can still ship if the comment is ignored.
- Adds a `packages: write`-scoped retag step; the security review must confirm it cannot be
  induced to promote a digest from outside this repository's package namespace.

## Action Items

- [ ] Add the release-candidate comment workflow (core + sidecar digest, revision label,
      freshness vs `main` HEAD, commit coverage, pinned pull command).
- [ ] Replace the `image` job in `release.yml` with resolve → assert-ancestry → retag; same
      for `provider-tmdb`.
- [ ] Test the ancestry assertion, including the negative case (a digest whose revision is
      not an ancestor must fail the release).
- [ ] Repin the canary compose to digests and document the roll-forward/rollback steps in
      `docs/reference/`.
- [ ] `/security-review` before merge — CI/release infrastructure and GHCR publish scope.
- [ ] Follow-up (not this ADR): generate the per-release manual-QA checklist into the same
      comment from the release's Conventional Commits.

## References

- ADR-023 — image distribution (GHCR, pull-based compose)
- ADR-024 — CI/CD pipeline, tag-driven releases
- ADR-034 — release notes + `prod` deployment linkage
- ADR-035 — image path-scoping + release test-gate (amended here)
- ADR-044 — Release Please release-PR in front of `release.yml` (promotion step amended here)
- HOLODEX-206 — implementation issue
