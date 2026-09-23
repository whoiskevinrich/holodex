# ADR-105: The TMDB sidecar runs on a distroless base; the container health probe moves into the binary

**Status:** Proposed
**Date:** 2026-09-22
**Deciders:** Project owner

**Extends:** [ADR-023](ADR-023-image-distribution.md) (image distribution — the sidecar keeps its
own image, tags and multi-arch build; only the runtime base changes) ·
[ADR-040](ADR-040-tmdb-provider-repo-placement.md) (the sidecar stays in-repo, stdlib-only, HTTP-only).
**Constrained by:** [ADR-070](ADR-070-canary-release-candidate-and-promote-by-retag.md) — a release
promotes by *retag*, so `:edge` and `:latest` are the same digest and **cannot** carry different
bases. This ADR's base choice is forced to be a single one for both.
**Deliberately does not extend:** [ADR-033](ADR-033-metadata-source-plugins.md) /
[`docs/specs/metadata-provider-contract.md`](../specs/metadata-provider-contract.md) — no endpoint,
field, cap, or status code changes; `/healthz` is untouched on the wire.
**Does not apply to the main `holodex` image** — see D4.
**Issue:** [HOLODEX-448](https://whoiskevinrich.atlassian.net/browse/HOLODEX-448)
(epic [HOLODEX-446](https://whoiskevinrich.atlassian.net/browse/HOLODEX-446)).

---

## Context

The TMDB sidecar is a `CGO_ENABLED=0 -trimpath` static Go binary on a `debian:bookworm-slim`
runtime. That base exists for exactly two things: `ca-certificates`, and `wget` for the
`HEALTHCHECK`. The other ~90 packages are cargo — the binary is static and links none of them.

They are not free. The 2026-09-21 Trivy scan of `holodex-provider-tmdb:latest` reported 17 alerts,
split 9 OS-package (`liblzma5` ×2, `libpcre2-8-0` ×6, `ca-certificates` ×1) and 8 Go stdlib. **Every
one of the 9 is unreachable** — nothing in the sidecar can call pcre2 or liblzma; they are inventory
findings against packages that exist only because the base image ships them. They recur on a fixed
cadence: each is a CVE against a Debian package that Holodex neither invokes nor can invoke.

HOLODEX-446 established why they persist: under ADR-070 a release retags an existing digest, so
`:latest` never rebuilds and accumulates OS CVEs between releases. That epic's decision was to treat
that accumulation as a release-cadence signal rather than build rebuild machinery. This ADR attacks
the other half — **reduce what can accumulate** — for the one image where the reduction is nearly free.

Three facts shape the decisions:

1. **Only `ca-certificates` is load-bearing.** The sidecar makes outbound TLS calls to
   `api.themoviedb.org` and downloads poster/still assets. It needs a CA bundle and nothing else
   from userspace.
2. **`wget` exists solely for `HEALTHCHECK`.** The main `holodex` image already solved this
   differently: `cmd/holodex/main.go` carries a `-healthcheck` flag that dials `127.0.0.1:$PORT/healthz`
   and exits 0/1, and its `HEALTHCHECK` invokes the binary. The sidecar diverged for no recorded reason.
3. **The base cannot differ between `edge` and `latest`.** ADR-070's retag promotion means they are
   the same digest. A "debug base on edge, minimal base on production" split would require abandoning
   retag promotion, and would ship bits that were never canaried — including the uid change in D2,
   exactly the class of regression `edge` exists to catch.

## Decisions

### D1 — The container health probe moves into the binary

`providers/tmdb/main.go` gains a `-healthcheck` flag that probes `http://127.0.0.1:<port>/healthz`
with a short timeout and exits 0 on HTTP 200, 1 otherwise. It is handled **immediately after
`flag.Parse()`, before credential validation**: a health probe must not require `TMDB_API_TOKEN`, and
must not inherit the `os.Exit(1)` that a missing credential triggers for the server path.

Port resolution mirrors the server's: `-port`, then `PORT`, then `9100`. It deliberately does **not**
consult `HOST`/`-host` — the probe always dials loopback, because it runs inside the container.

This mirrors `cmd/holodex/main.go`'s `runHealthcheck()` rather than inventing a second shape.
`/healthz` itself is unchanged on the wire, so
[`docs/specs/metadata-provider-contract.md`](../specs/metadata-provider-contract.md) needs no diff —
this is a container-lifecycle concern, not a provider ability.

### D2 — The runtime base becomes `gcr.io/distroless/static-debian12:debug-nonroot`

`Dockerfile.provider-tmdb`'s runtime stage drops the `apt-get install ca-certificates wget` layer and
bases on `gcr.io/distroless/static-debian12:debug-nonroot`: four packages (`base-files`,
`ca-certificates`, `netbase`, `tzdata`) plus busybox, running as uid 65532.

**Why `debug-nonroot` and not `nonroot`.** The `debug` variants add a busybox shell. That is not a
meaningful security cost here: anyone who can `docker exec` into the sidecar already holds the Docker
socket, i.e. root on the host. It *is* a meaningful operability gain for a self-hosted product whose
operator debugs by exec'ing into containers. The `-nonroot` half is the part that matters — an
unprivileged uid on a network-listening process.

Port 9100 is above 1024, so binding needs no capability as uid 65532.

Note that busybox puts a `wget` applet on `PATH`, so the old probe would not visibly break on
*this* variant. That is precisely why D1 is not optional: the binary's own probe depends on
nothing outside the binary, so dropping to plain `nonroot` later is a one-line base change rather
than a silent container-health regression discovered in production.

**Why not Alpine.** ~15 packages versus 4, a shell either way, and no gcr.io dependency — but it
reintroduces a package manager and an apt/apk layer to keep current, which is the thing being
removed. The gcr.io registry dependency is accepted: it is a build-time dependency only, and the
published image is self-contained in GHCR.

### D3 — One base for `edge` and `latest`, always

Forced by ADR-070 (see Context 3), but recorded as a decision because it is the question that will be
asked again: there is no supported configuration in which the promoted image differs from the
canaried one. If a future need for a divergent production base appears, it must first supersede
ADR-070's retag promotion — not work around it.

### D4 — The main `holodex` image stays on Debian

Not in scope and not merely deferred. `holodex` needs `ffmpeg` and `libimage-exiftool-perl`, which
pull a large C surface (of its 30 OS-package alerts, `libexpat1` alone was 14). Distroless is not
available to it at any price. The separate question of `bookworm → trixie` — a support-tier upgrade,
not a structural one — is [HOLODEX-450](https://whoiskevinrich.atlassian.net/browse/HOLODEX-450),
blocked on this ADR landing so that the sidecar sets the precedent for base-image changes under ADR-070.

## Consequences

**Good.** The sidecar's 9 recurring OS-package alerts stop existing rather than being re-fixed each
release. Image size drops by roughly an order of magnitude. The process no longer runs as root. The
`-healthcheck` divergence between the two binaries is closed.

**Costs.** `apt-get` is gone — a future need for an OS-level tool in the sidecar means either
reverting the base or vendoring the capability into the Go binary, which is the right pressure for a
component whose rules already say stdlib-only. Debugging is busybox, not coreutils: no `curl`, no
`apt`. A gcr.io outage blocks a sidecar image build (not a release, which retags).

**Unchanged.** The 8 Go-stdlib alerts are unaffected — they live in the compiled binary and follow the
`golang:` toolchain pin, not the runtime base. Anyone expecting this change to zero the sidecar's alert
count will be wrong by exactly those 8.

**Risk.** The CA bundle now comes from distroless rather than a Debian `apt` install. If distroless's
bundle were stale or absent, every TMDB call would fail with a TLS verification error at runtime, not
at build. The testing strategy therefore requires a live outbound TLS call — not just a `/healthz` 200 —
before this is considered verified.
