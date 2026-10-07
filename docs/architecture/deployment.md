# Deployment

This doc owns how Holodex is built into images and shipped: the shape of the core container, how
the SPA rides inside the Go binary, the media tools bundled at runtime, GHCR distribution and
promotion, where provider sidecars live and what they run on, and the Windows build manifest.
Which languages and frameworks are used is [stack.md](stack.md); runtime configuration and env
vars are [config-and-settings.md](config-and-settings.md); the sidecar's HTTP contract and
SSRF allowlist are [metadata-providers.md](metadata-providers.md) and
[security-perimeter.md](security-perimeter.md).

## One multi-stage image on a Debian trixie-slim runtime

The core ships as a single container built by a three-stage `Dockerfile`: a Node stage builds the
SPA, a Go stage builds a static binary (`CGO_ENABLED=0 go build -tags production ./cmd/holodex`),
and the runtime stage is `debian:trixie-slim` with the binary at `/usr/local/bin/holodex`.

- The runtime exposes 7800 (web UI + API) and 7801 (MCP), declares `/data` as a volume, and its
  `HEALTHCHECK` runs the binary itself (`holodex -healthcheck`, which dials loopback `/healthz`
  and exits 0/1) — no `curl`/`wget` in the image.
- Builder stages may use any Debian release; the binary is static, so only the runtime base matters.
- BuildKit cache mounts persist the npm cache, Go module cache and Go build cache across builds;
  `.dockerignore` keeps `web/node_modules`, `web/dist`, `.git`, `.claude`, `data` and `*.db` out of
  the build context.
- Development does not use the image: `go run ./cmd/holodex` serves the API and the Vite dev
  server (`:5173`) serves the SPA with HMR, proxying `/api` to 7800 and `/mcp` to 7801.

**Rejected:** Alpine — exiftool is Perl and its CPAN dependencies misbehave on musl, and Debian's
ffmpeg packaging is more complete.
**Rejected:** staying on bookworm — its exiftool (12.57) stops parsing Matroska at the first
Cluster, so it cannot read Tags that mkvpropedit relocates past the Clusters; pinning an upstream
exiftool tarball would make the project its maintainer.

Decided in [`c7d7bd1e`](https://github.com/whoiskevinrich/holodex/commit/c7d7bd1e).

## SPA embedded in the binary under the `production` build tag

The built SvelteKit SPA is compiled into the Go binary, so one process serves both UI and API.

- The embed lives in the `cmd/holodex` package: `frontend_prod.go` (`//go:build production`)
  embeds `all:web/dist`; `frontend_dev.go` (`!production`) returns a nil filesystem. The Docker
  build therefore copies the SPA output to `cmd/holodex/web/dist`, because `//go:embed` resolves
  relative to the package and cannot reach a parent directory.
- `cmd/holodex/spa.go` serves any file present in the embedded FS and falls back to `index.html`
  for every other path, so the client router owns deep links. `main.go` mounts it only when the
  embed is non-nil, routing `/api*`, `/healthz`, `/readyz` and `/metrics` to the backend.
- The `production` tag is mandatory for any binary that should serve the UI; a plain `go build`
  is API-only by design.
- The startup "listening" log line carries the `http://localhost:<port>` URL.

Decided in [`36e8bd8f`](https://github.com/whoiskevinrich/holodex/commit/36e8bd8f).

## Media tools bundled in the image: ffmpeg, exiftool, MKVToolNix

The image is the server: every external tool the app shells out to is installed in the runtime
stage, from the Debian archive, in one `apt-get install --no-install-recommends` line — `ffmpeg`,
`libimage-exiftool-perl`, `mkvtoolnix`, `ca-certificates`. They receive security updates by
rebuilding the image. There are no host dependencies.

- `LANG=C.UTF-8` is set in the image because MKVToolNix converts paths and output through the
  locale and cannot open non-ASCII paths under the C locale.
- **Tool presence is the backend selector.** `internal/writeback` uses mkvpropedit for
  Matroska/WebM when `mkvpropedit`, `mkvextract` and `mkvmerge` are all on `PATH`, else the ffmpeg
  remux path. There is no flag or config key. Removing the package is the rollback, and dev hosts
  or custom images without MKVToolNix keep working on the ffmpeg path, which stays under test.
- These tools parse untrusted media inside the container; they are invoked with argv (no shell)
  and absolute temp paths under the media root.
- Because tool versions differ between host and image, the suites that shell out to media tools
  (`writeback`, `thumbnail`, `metadata`) are verified inside the built image (`make test-image`)
  before an image or writeback change ships.

**Rejected:** a config toggle for the MKV backend — a second selector that can disagree with
what is installed.
**Rejected:** the upstream `mkvtoolnix.download` apt repo or static AppImages — a third-party
signing key, or no Debian security updates, for no feature needed.

Decided in [`c7d7bd1e`](https://github.com/whoiskevinrich/holodex/commit/c7d7bd1e).

## Prebuilt multi-arch images on GHCR, pulled by a compose file

Images are published to GitHub Container Registry, public-pull and authenticated-push:
`ghcr.io/whoiskevinrich/holodex` and, per sidecar, `ghcr.io/whoiskevinrich/holodex-provider-<name>`.
Each is built for `linux/amd64` and `linux/arm64`.

- A merge to `main` that touches an image's sources builds it and tags `edge` and `sha-<short>`.
  A push to a `release/**` hotfix branch tags `sha-<short>` only, never `edge`. Semver tags and
  `latest` are only ever produced by release promotion (next section), so `latest` always means
  the highest release.
- Operators run `docker-compose.prod.yml`, which pulls `holodex:${HOLODEX_TAG:-latest}` and needs
  no source tree or toolchain. The build-from-source `docker-compose.yml` is the developer path.
- Publishing uses the workflow's `GITHUB_TOKEN` with `packages: write`; no registry credential is
  stored.

Decided in [`8b83d2f7`](https://github.com/whoiskevinrich/holodex/commit/8b83d2f7).

## Releases promote the canaried digest by retag

A release never rebuilds. Promotion resolves the newest `sha-<short>` image whose
`org.opencontainers.image.revision` label is an ancestor of the release tag (asserted with
`git merge-base --is-ancestor`, failing closed on registry errors), then copies that digest to the
semver and `latest` tags with `docker buildx imagetools create`. Core and sidecar are both resolved
before either is published. The bits behind `latest` are therefore byte-identical to the `edge`
digest a canary ran, and a canary pins `image@sha256:…` rather than a moving tag.

Constraints this imposes:

- **No build-time version injection.** The version-bump commit must change no runtime bytes; an
  embedded version string would silently disagree with the retagged tag.
- **One runtime base per line.** A release from `main` makes `edge` and `latest` the same digest.
  A hotfix (below) is the one sanctioned divergence: `latest` runs the release branch's base until
  `main` next ships.
- Since promotion re-scans nothing, CVE scanning of published tags runs on its own schedule.

**Rejected:** rebuilding at the tag — the shipped image would not be the one that was validated.

Decided in [`e8120b9d`](https://github.com/whoiskevinrich/holodex/commit/e8120b9d).

### Hotfixes promote from a `release/vX.Y` branch

When the shipped line needs a fix and `main` is not ready to release, the fix ships from a
`release/vX.Y` branch cut at the shipped tag. The branch builds `sha-<short>` candidates, and a
`vX.Y.Z` tag on it promotes one through the same ancestry check and retag as a `main` release.
The procedure is in [ci-and-releases.md](../reference/ci-and-releases.md#hotfix-releases).

- **Moving tags follow the highest version, not the newest tag.** `X.Y`, `X`, `latest` and the
  GitHub "Latest" release go to a release only if it is the highest in that range
  (`scripts/release-tags.mjs`), so a 1.x hotfix cut after 2.0.0 cannot move `latest` back.
- **Jira → Released runs only for a tag reachable from `main`.** That sync releases the whole
  `status = Done` set, which is true of `main` and false of a branch.

**Rejected:** building the hotfix image at the tag — it would be the one release whose bits were
never published as a candidate first. **Rejected:** releasing `main` early under a `Release-As`
override — it ships everything on `main`, which is what a hotfix exists to avoid.

Decided in HOLODEX-545.

## Provider sidecars: in-repo source, separate image

Each metadata provider is its own container, but its source lives in this repo at
`providers/<name>/` as a `main` package in the root `holodex` module. TMDB is
`providers/tmdb/`, built by `Dockerfile.provider-tmdb` into `holodex-provider-tmdb` and released
at the same version as core.

- A sidecar imports only the Go standard library — never `holodex/internal/...` — so it stays
  standalone at the source level while sharing `go.mod`.
- Core talks to it over HTTP only. An operator adds the sidecar service to their compose file and
  one `metadata-sources.yaml` entry; no core rebuild or code change.

**Rejected:** a separate repository per provider — separate CI, tagging and PR flow buys code
independence the container boundary already provides at runtime.

Decided in [`66d2a04a`](https://github.com/whoiskevinrich/holodex/commit/66d2a04a).

## Sidecar runtime on distroless static, non-root, self-probing

Sidecar images run on `gcr.io/distroless/static-debian12:debug-nonroot`: a CA bundle, tzdata and a
busybox shell, as uid 65532, with no package manager. The static binary needs nothing else from
userspace. Sidecar ports stay above 1024 so binding needs no capability.

- The container health probe is in the binary (`-healthcheck`, handled right after flag parsing
  and before credential validation, dialing loopback on `-port`/`PORT`/`9100`). The `HEALTHCHECK`
  and `ENTRYPOINT` use absolute paths and depend on nothing outside the binary, so dropping to
  plain `nonroot` later is a one-line change.
- A need for an OS-level tool in a sidecar means vendoring the capability into Go, not adding a
  package layer.
- The core image cannot follow: ffmpeg and exiftool require a full Debian userspace.

**Rejected:** Alpine — reintroduces a package manager and an apk layer to keep patched.

Decided in [`4178681f`](https://github.com/whoiskevinrich/holodex/commit/4178681f).

## Windows `asInvoker` manifest as a committed `.syso`

`holodex.exe` embeds an application manifest declaring `requestedExecutionLevel="asInvoker"`, so
Windows never prompts for elevation. The source is `cmd/holodex/holodex.manifest`; the compiled
resource `cmd/holodex/holodex_windows_amd64.syso` is committed so a fresh clone builds without
extra tools, and is regenerated with the `//go:generate rsrc …` directive in `cmd/holodex/main.go`.
The `_windows_amd64` suffix keeps it out of every other build target. The binary writes only to
configured data and media paths, so it never needs elevation; a new target arch needs its own
`.syso`.

**Rejected:** `goversioninfo` — version metadata and icons aren't needed, and `rsrc` is smaller.

Decided in [`8599a801`](https://github.com/whoiskevinrich/holodex/commit/8599a801).
