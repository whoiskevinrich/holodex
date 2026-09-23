---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-448
status: in-progress
release_note: The TMDB provider sidecar image is now distroless — about 82% smaller (146MB → 26MB), runs as a non-root user, and carries no OS packages to patch. Its container health probe is now the sidecar's own binary rather than wget; a compose file that overrides the healthcheck with a wget command needs updating.
---

# HOLODEX-448 · TMDB sidecar on a distroless base

Child of epic [HOLODEX-446](https://whoiskevinrich.atlassian.net/browse/HOLODEX-446) (code-scanning
cleanup). The sidecar is a `CGO_ENABLED=0` static Go binary whose `debian:bookworm-slim` runtime
existed for exactly two things: `ca-certificates`, and `wget` for the `HEALTHCHECK`. The other ~90
packages were cargo it cannot link — and 9 of its 17 Trivy alerts were **unreachable** CVEs against
them, recurring every release. This moves the runtime to
`gcr.io/distroless/static-debian12:debug-nonroot` and moves the health probe into the binary.
ADR: [ADR-105](../architecture/ADR-105-sidecar-distroless-runtime-base.md).

## Gates — definition of done

- [x] architecture `architecture` — [ADR-105](../architecture/ADR-105-sidecar-distroless-runtime-base.md)
  D1–D4; index row landed. D3 records that `edge` and `latest` **cannot** carry different bases
  (ADR-070 retag promotion makes them one digest) — the question that will otherwise be re-asked.
- [~] spec `write-spec` — **deliberately skipped.** No functional or behavioural change: `/healthz`
  is byte-identical on the wire, and CLAUDE.md's routing table sends infrastructure to an ADR, not a
  spec. The two operator-facing *docs* that this invalidated were corrected instead
  (`metadata-provider-contract.md` §Operator wiring, `tmdb-provider.md` §Operator wiring).
- [~] design `design-handoff` — **deliberately skipped.** No user-facing surface; nothing renders.
- [x] backend — `-healthcheck` flag on `providers/tmdb/main.go` handled before credential validation,
  `resolvePort` shared by server and probe, `Dockerfile.provider-tmdb` runtime stage rebased.
  Verified locally (see session log).
- [ ] frontend — n/a, no `web/**` change.
- [ ] testing `testing-strategy` — pending.
- [ ] security `security-review` — pending.

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-448] testing gate — `docs/testing-strategy.md` + the container rows in
   `docs/specs/qa-tmdb-provider.md` §4 (they still describe the Debian image: add nonroot uid, the
   binary probe, and "no shell utilities")
2. [ ] [HOLODEX-448] security gate — `/security-review` on the base change (nonroot uid, CA bundle
   provenance, the busybox shell decision in ADR-105 D2)
3. [ ] [HOLODEX-448] Kevin's call on `debug-nonroot` vs plain `nonroot` once he sees the image —
   the busybox shell is the only reversible part of D2
4. [ ] [HOLODEX-448] on merge, sweep 448 to Done **by hand** and unblock
   [HOLODEX-450](https://whoiskevinrich.atlassian.net/browse/HOLODEX-450) (bookworm → trixie)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-22 · ADR + implementation
- skills: code-review (high --fix)
- handoff: **ADR-105 written first, then the code** — D1 probe-in-binary, D2
  `distroless/static-debian12:debug-nonroot`, D3 one base for edge and latest (forced by ADR-070),
  D4 the main image stays Debian. Implementation: `-healthcheck` handled immediately after
  `flag.Parse()` and **before** the credential checks (a probe must not need `TMDB_API_TOKEN`, nor
  inherit its `os.Exit(1)` — that would report every container unhealthy instead of unconfigured);
  `resolvePort` extracted so server and probe can never disagree; `ENTRYPOINT`/`HEALTHCHECK` use
  absolute paths since there is no shell to fall back on.
  **Verified live:** probe tracks the server (down→1, up→0, down→1) and works with no credential env
  and via `PORT`; image builds; uid is `65532(nonroot)`; busybox shell present; Docker reports
  `healthy`; **outbound TLS to TMDB succeeds** — a fake token returns
  `TMDB /3/search/movie returned 401`, i.e. the handshake completed against distroless's CA bundle
  rather than failing `x509`, which was ADR-105's stated risk. Size 146MB → 25.9MB. Multi-arch
  confirmed: the base publishes amd64 + arm64, matching the workflow's `platforms:`.
  Code review found two real breaks caused by this change — both operator-wiring compose snippets
  still probed with `wget`, which the image no longer contains (a copied stanza overrides the image
  HEALTHCHECK and reports unhealthy forever) — fixed in both docs. One finding skipped: the
  HEALTHCHECK cannot see a `-port` passed as a *container argument* (falls back to `PORT`/9100);
  unfixable from a build-time `CMD`, and strictly better than the old hardcoded-9100 wget probe.
  **Process note:** the backend edit landed before this worklog existed, so the design→build
  PreToolUse guard had nothing to check and `/implement` was never run. Substantively the boundary
  held (ADR first, then code) and the two design-phase gates that do not apply are recorded as
  skips above — but the crossing was not formally recorded. Next: testing + security gates.
