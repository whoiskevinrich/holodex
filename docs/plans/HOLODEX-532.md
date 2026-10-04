---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-532
status: in-review            # chore posture = zero gate rows, so this stored value is read
profile: chore               # CI/build tooling only; the shipped image's contents are unchanged
depends-on: []
release_note: No change for users — image builds are faster; the images contain the same binaries as before.
---

# HOLODEX-532 · Speed up the multi-arch image build

Done means the `main` image build no longer runs the frontend or Go compile under QEMU, and
`image.yml` / `provider-tmdb.yml` stop overwriting each other's GHA cache. Run 37214241124 took
13.5 min: the arm64 chain (Vite 179s → `go build` 470s) ran under emulation and nothing was
cached, because both workflows shared the default `buildkit` cache scope.

**Change:** `Dockerfile` + `Dockerfile.provider-tmdb` build stages on `$BUILDPLATFORM`,
cross-compiling via `GOOS=$TARGETOS GOARCH=$TARGETARCH` (CGO-free); the ARG sits just before
`go build` so `go mod download` is shared across targets. Cache `scope=holodex` /
`scope=provider-tmdb`. Only the runtime `apt-get` stage still runs per platform.

Deferred (not filed): native arm64 runners, registry cache, persisted cache mounts. Revisit
only if builds stay slow after this lands.

## Gates — definition of done

<!-- chore posture: no gate rows. -->

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR once CI is green; jira-sync moves HOLODEX-532 to Done
2. [ ] [—] Check the next `main` "Build image" run: expect ~3–4 min, with the runtime `apt-get` layer CACHED from the second run on

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-04 · session
- skills: code-review (high --fix: ARG moved late so `go mod download` is shared; provider-tmdb cross-compiles too), security-review (no findings)
- Verified locally: arm64 images for both Dockerfiles build, binaries are ELF AArch64 (`e_machine` 0xb7) and execute; a two-platform build runs `npm run build` and `go mod download` once, and the arm64 `go build` as `amd64->arm64`.
- handoff: Cross-compile + per-workflow cache scope done and verified locally. Next: merge, then confirm the next main image build time.
