---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-448
status: in-progress
profile: infra
release_note: The TMDB provider sidecar image is now distroless — about 82% smaller (146MB → 26MB), runs as a non-root user (uid 65532), and carries almost no OS packages to patch. Its health probe is now the sidecar's own binary rather than wget. Existing compose files keep working; if yours overrides the healthcheck with a wget command, switching it to ["CMD", "/usr/local/bin/holodex-provider-tmdb", "-healthcheck"] is more durable. Anything that mounted a volume into the sidecar may need its ownership adjusted for the non-root user.
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
- [x] backend — `-healthcheck` flag on `providers/tmdb/main.go` handled before credential validation,
  `resolvePort` shared by server and probe, `Dockerfile.provider-tmdb` runtime stage rebased.
  Verified locally (see session log).
- [x] testing `testing-strategy` — `docs/testing-strategy.md` §15 + `TestResolvePort` /
  `TestRunHealthcheck` in `providers/tmdb/main_test.go` (mutation-checked), and
  `qa-tmdb-provider.md` §4 rewritten for the distroless runtime (4.5–4.8 added, all executed
  against the built image). Four standing gaps in §15.1 — chiefly that **no CI test covers the
  container at all**.
- [x] security `security-review` — **signed off 2026-09-22, no findings.** Verified: the
  `-healthcheck` branch `os.Exit`s unconditionally and can never reach `ListenAndServe`, so it is
  not a path to serving without credentials; `runHealthcheck`'s scheme and host are hardcoded
  (only the port interpolates, from trusted flag/env) so it is not SSRF; no `tls.Config`,
  `InsecureSkipVerify`, or `SSL_CERT_*` override anywhere under `providers/`; auth surface
  untouched. Net posture improves — root + full shell + ~90 packages → uid 65532 + busybox + 4.
  Accepted risk: the `debug` variant's busybox shell, strictly less than the previous base
  offered, reversible by dropping to plain `nonroot`.

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-448] Kevin's review of PR #379 (marked ready 2026-09-22; all gates green)
2. [ ] [HOLODEX-448] on merge, sweep 448 to Done **by hand** and unblock
   [HOLODEX-450](https://whoiskevinrich.atlassian.net/browse/HOLODEX-450) (bookworm → trixie)
3. [ ] [HOLODEX-454] testing-strategy §15.1's first gap — no CI test covers the container
   (nonroot, health transition, outbound TLS are all manual). **Filed 2026-09-22**, linked to
   HOLODEX-450: worth landing before the trixie base swap so that merges against a net rather
   than a checklist

> **Decided 2026-09-22 (Kevin): keep `debug-nonroot`.** The busybox shell stays. Don't re-propose
> plain `nonroot` — it was weighed against this variant and declined. If it is ever revisited, the
> thing that changes is that busybox's `wget` applet goes with it, which is what makes ADR-105 D1's
> binary probe load-bearing rather than belt-and-braces.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-22 · ADR + implementation
- skills: code-review (high --fix), security-review, testing-strategy
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

### 2026-09-22 · testing + security gates
- skills: testing-strategy, security-review
- handoff: **both remaining gates closed; all gates green.** Security review: no findings (the
  sign-off reasoning is on the gate line above). Testing: `TestResolvePort` + `TestRunHealthcheck`
  added to `providers/tmdb/main_test.go` — the probe is pinned in both directions (200→0, 503→1,
  no-listener→1) and asserts it requests `/healthz` specifically; mutation-checked by forcing
  `runHealthcheck` to always return 0, which fails the 503 case. `docs/testing-strategy.md` §15
  written with four standing gaps in §15.1, the first being that **no CI test covers the container
  at all** — nonroot, health transition and outbound TLS are manual only. `qa-tmdb-provider.md` §4
  rewritten for the distroless runtime with 4.5–4.8, every row executed against the built image.
  **Correction to the previous session.** That session's code review claimed the operator compose
  snippets would "report unhealthy forever" because the image no longer contains `wget`. **That is
  wrong for this base**: `debug-nonroot` ships busybox at `/busybox`, which is on `PATH` and
  provides a `wget` applet — the old probe was verified still working in-container. The doc change
  is still right, but the reason is brittleness, not breakage: the `wget` probe works only by
  accident of the `debug` variant and dies the moment the base drops to plain `nonroot`. The
  claim was corrected in both spec docs, the QA row that asserted "no shell utilities", and
  ADR-105 D2 (which now records why D1 is not optional). `curl` and `apt` are genuinely absent.
  Left: Kevin's look at `debug-nonroot` vs `nonroot` — the only thing holding the PR at Draft.

### 2026-09-22 · marked ready
- skills: —
- handoff: **Kevin chose `debug-nonroot`** (recorded as a decision above — do not re-propose plain
  `nonroot`). `frontend` flipped `[ ]` → `[~]`: the worklog gate treats `[ ]` and `[/]` as *open*
  and only `[x]`/`[~]` pass, so an n/a gate left unticked would have failed the required check the
  moment the PR left Draft. Branch was already level with main (`a610a33`), so no merge and no
  risk of dropping the jira-sync ready event. PR #379 marked ready for review; all gates green,
  CI green. Left: Kevin's review, then on merge sweep 448 → Done by hand and unblock HOLODEX-450.
