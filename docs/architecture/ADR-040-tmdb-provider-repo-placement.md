# ADR-040: TMDB provider source placement — monorepo subdirectory

**Status**: Accepted
**Date**: 2026-06-19
**Deciders**: Project owner
**Relates to**: [ADR-033](ADR-033-metadata-source-plugins.md) (sidecar provider protocol),
[ADR-039](ADR-039-provider-asset-urls.md) (asset-host allowlist), [ADR-023](ADR-023-image-distribution.md) (GHCR image distribution), [ADR-024](ADR-024-ci-cd-pipeline.md) (CI/CD pipeline).

---

## Context

ADR-033 established that providers are **separately-deployed sidecar containers** speaking a
small HTTP/JSON contract. It did not specify where a provider's *source code* should live —
whether in this repository or in a separate one. That question was implicit as long as providers
were only specified (as `docs/specs/tmdb-provider.md` was, written for an "external team in a
different repository"). Now that the first real provider (TMDB) is being built, the decision
must be made explicitly.

Two options:

| Option | Code independence | Operational overhead | Fit for this project |
|---|---|---|---|
| **A. `providers/tmdb/` in this repo (monorepo subdir)** | Container boundary maintained; runtime isolation unchanged | Low — one `git clone`, one CI run, releases coupled | Best for a single-developer project |
| B. Separate `holodex-provider-tmdb` repository | Maximum code independence | High — separate CI, separate GHCR image tagging, separate PR workflow | Justified only if the provider has external contributors or a different release cadence |

ADR-033's isolation language was about **runtime** (containers) not **source** (repos). "Add a
new source without changing core" means no core code change — it does not mandate a separate
repository. A provider in `providers/tmdb/` still builds as a fully independent container
image; core still talks to it over the contract; the provider still owns its own API key and
rate-limiting. Option B's overhead is pure friction for a single developer with no external
collaborators.

Two artifacts already presupposed in-repo placement: `docs/specs/tmdb-provider.md` references
the Node.js reference stub in `testdata/enrich-stub/` (co-located), and ADR-039's action items
reference updating `tmdb-provider.md` in place. The decision was implicit; this ADR makes it
explicit and records the reasoning.

## Decision

The TMDB provider source code lives in **`providers/tmdb/`** in this repository, as a `main`
package within the existing `holodex` Go module. It builds to a separate binary
(`holodex-provider-tmdb`) and publishes a separate container image
(`ghcr.io/whoiskevinrich/holodex-provider-tmdb`) via a dedicated CI workflow
(`.github/workflows/provider-tmdb.yml`). Releases tag both images at the same semver version.

### Structure

```
providers/
  tmdb/
    main.go        # HTTP server, config, entry point
    handler.go     # /healthz /describe /resolve /enrich handlers
    tmdb.go        # TMDB v3 API client
    tmdb_test.go   # handler + client tests against httptest servers
Dockerfile.provider-tmdb
.github/workflows/provider-tmdb.yml
```

The provider uses only the Go standard library — no import of `holodex/internal/...`. This
keeps it genuinely standalone at the source level even though it shares a `go.mod`.

### CI/CD

- **On push to main** (paths: `providers/tmdb/**`, `Dockerfile.provider-tmdb`, `go.mod`, `go.sum`): build and push `edge` + `sha-*` tags.
- **On `v*` tag** (via `release.yml`): build and push semver + `latest` tags alongside the main image.
- **Trivy scan**: separate SARIF category (`trivy-provider-tmdb`) uploaded to the Security tab.

### Operator wiring (unchanged from spec)

An operator who wants TMDB enrichment:
1. Adds the `holodex-tmdb` service to their compose file (image: `ghcr.io/whoiskevinrich/holodex-provider-tmdb:<tag>`).
2. Adds one entry to `metadata-sources.yaml` (`base_url: http://holodex-tmdb:9100`, `asset_hosts: [image.tmdb.org]`).
3. Reloads config (`POST /api/v1/admin/reload-config`). No core restart, no core code change.

## Rationale

- **Runtime isolation is preserved.** The container boundary (ADR-033) is maintained; the code
  boundary is a separate, less important concern for a project at this scale.
- **Lower operational friction.** One repo, one clone, one release pipeline. Evolving the
  provider protocol and the provider implementation atomically is cleaner than coordinating two
  repos.
- **Reference implementation value.** TMDB is the canonical worked example of the provider
  contract; co-locating it with the contract spec and the Node.js stub makes the trio
  discoverable together.
- **Standard-library-only provider.** The provider imports nothing from `holodex/internal/` —
  it is a genuinely standalone service that happens to be built from the same module.

## Consequences

- **New images to release.** Each `v*` tag triggers a second GHCR image push (more release
  surface, but the workflow is templated from the existing one and the overhead is small).
- **Shared `go.mod`.** A TMDB upstream dependency (if ever added) would land in the root
  `go.mod`. The current provider is stdlib-only, so this is not a concern in v1.
- **Future providers.** Additional providers follow the same pattern: `providers/<name>/` subdir
  with its own binary and Dockerfile. If provider count grows enough that shared `go.mod`
  becomes a burden, a follow-up ADR can split them into a workspace or separate modules — the
  container and protocol contracts are unaffected by that change.
- **Supersession.** If the project grows to warrant separate repos (external contributors,
  different release cadences), this ADR is superseded. The HTTP contract and compose wiring
  remain unchanged; only the source location moves.
