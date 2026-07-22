# Canarying a release candidate

How a change that is merged to `main` but not yet released gets validated on a real
instance, and how that validated image becomes the release. Decision record:
[ADR-070](../architecture/ADR-070-canary-release-candidate-and-promote-by-retag.md).

## The idea in one line

Release Please stacks the version bump and CHANGELOG — not the code — so
`ghcr.io/whoiskevinrich/holodex:edge` **is** the content of the open release PR. There is
no separate release candidate to build; there is only a candidate to *pin, run, and
promote*.

## The loop

1. PRs merge to `main`. `image.yml` publishes `:edge` and `:sha-<short>`.
2. The **Release candidate** workflow refreshes a comment on the open release PR with both
   images' digests, whether each is current with its own sources, the commits covered, and
   a pinned pull command.
3. You pull those digests onto the canary and exercise them.
4. You merge the release PR. The `v*` tag fires `release.yml`, which **retags the digest
   you canaried** as `1.2.3` / `1.2` / `1` / `latest` — no rebuild.

The image published as `latest` is byte-identical to the one you ran.

## Running the canary

Pin by **digest**, never `:edge`. `:edge` moves under you, so "which bits are running" stops
being answerable and rollback stops being a thing you can name.

```yaml
services:
  holodex:
    image: ghcr.io/whoiskevinrich/holodex@sha256:...   # from the release PR comment
```

Roll forward by pasting the digest from the current comment; roll back by pasting the one
from the previous comment. Keep the previous digest until the release is cut.

## Reading the comment

The freshness column is the part to actually read:

| Verdict | Meaning |
|---|---|
| ✅ current | No commit touching this image's own sources has landed since it was built. Canarying it validates what this PR will release. |
| ⚠️ N source commits behind | A build failed, never ran, or is still running. **Canarying this validates something other than what will ship.** |
| ⚠️ no image published | Nothing to canary for that image. |
| ⚠️ freshness could not be determined | The check couldn't read the image's trigger paths or count commits — treat as unknown, not as fine. |

**Freshness is measured per image, not against `main` HEAD.** `image.yml` and
`provider-tmdb.yml` each carry their own `paths:` filter (ADR-035), so the two images
advance independently and a core-only merge leaves the sidecar's revision far behind
`main` — while its image is still perfectly correct. The check therefore counts only
commits matching *that image's* filter, read out of its workflow file. A rarely touched
sidecar reads ✅ current, and only a genuinely missed build reads ⚠️.

Measuring against `main` HEAD instead is what made the first version warn on nearly every
release (HOLODEX-208); a warning that is usually wrong is worse than none, because it
teaches you to scroll past the one that matters.

A ⚠️ immediately after a merge is usually just the build still running — the comment
updates itself in place when the image workflow finishes. If it persists, re-run the
relevant workflow (`workflow_dispatch`) before relying on the digest; an image rebuilt
that way is *newer* than required and still reads ✅ current.

The comment is **advisory**. It sets no status check and blocks no merge — see
[ADR-070](../architecture/ADR-070-canary-release-candidate-and-promote-by-retag.md) for why
that posture was chosen and what it costs.

## What protects the release

Promotion no longer rebuilds, so nothing about the tagged commit is re-derived. What
prevents an arbitrary image being published under a release version is
[`scripts/resolve-release-digest.mjs`](../../scripts/resolve-release-digest.mjs):

- it only ever considers commits reachable from the tagged commit (ancestry is structural,
  not checked after the fact);
- the resolved image's `org.opencontainers.image.revision` label must name that same commit,
  so a hand-pushed or mismatched `sha-*` tag is rejected;
- `git merge-base --is-ancestor` is then asserted explicitly;
- a multi-arch index whose platforms disagree on the revision is rejected.

Any failure aborts the release rather than publishing. The negative cases are covered in
[`scripts/resolve-release-digest.test.mjs`](../../scripts/resolve-release-digest.test.mjs).

The walk normally lands one commit back from the tag: release-please's version-bump commit
touches only `version.txt`, `CHANGELOG.md`, and the manifest, none of which appear in the
image workflows' `paths:` filters, so no image is built for it.

## Known limits

- **Freshness is reported, not enforced.** A stale sidecar can still ship if the comment is
  ignored.
- **Trivy does not re-run at promotion.** The digest was scanned when it was built as
  `edge`. A weekly `scan.yml` re-scans `:latest` and `:edge` against a fresh CVE database
  so this doesn't depend on something being rebuilt — but a CVE disclosed mid-week is
  caught by that scan, not by the release.
- **The published image's `revision` label points one commit behind the tag** — at the
  commit the bits actually came from, not the version-bump commit.
- **Retagging assumes the version-bump commit changes no runtime bytes.** True while there
  is no build-time version injection in the `Dockerfile`. If a version string is ever baked
  into the binary, ADR-070 must be revisited before the next release.
