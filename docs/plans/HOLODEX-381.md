---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-381
status: in-progress
release_note: The layout-invariant harness now stops and says why when the app under test dies mid-run — and the "full matrix crashes Vite" failure it had been hiding turns out to be a libuv bug in Node 24.0–24.15 on Windows, fixed by Node 24.16.0.
---

# HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`)

Root-cause the silent Vite death that the HOLODEX-380 testing gate hit twice, make the harness
report it as what it is instead of ~600 `error` rows, and record the fix where the next person
will find it.

**Design package:** `docs/testing-strategy.md` §12.5 · `web/geometry/README.md`

## Findings (2026-09-13/14)

- **Reproduced 4/4** on this machine (Windows 11, Node v24.15.0, Vite 8.2.2): runs 1 and 2 died
  at the `cinematheque/lg → narrow` boundary after exactly 150 loads; run 4 died 208 loads in,
  mid-cell; run 5 at 487 (`brutalist/wide`, where the ticket first saw it). Memory, threads
  and handles were flat the whole time — not exhaustion.
- **Ran clean under a debugger** (`cdbX64` attached, `sxe sbo`): 639/639 loads, no exception —
  the ~40 % slowdown changes the timing. A race, not an input.
- **Root cause is upstream, not this repo:** Node 24.0–24.15 bundle libuv 1.51.0, whose
  `uv__tcp_keepalive` (`src/win/tcp.c`) calls `RtlGetVersion()` on an uninitialised
  `OSVERSIONINFOW`; it writes past the buffer into the `/GS` stack cookie →
  `__report_gsfailure` → fastfail `0xC0000409`, no output. Runs on every outbound
  `uv_tcp_connect`; the Vite dev proxy opens one per `/api` request.
  [libuv#5106](https://github.com/libuv/libuv/issues/5106) ·
  [libuv#5107](https://github.com/libuv/libuv/pull/5107) ·
  [nodejs/node#62561](https://github.com/nodejs/node/pull/62561) — **fixed in Node 24.16.0**.
- The ticket's three hypotheses were all wrong for a reason worth keeping: the harness is
  strictly sequential (one context, one page at a time), so "nine parallel-ish contexts" and
  `--concurrency` never applied; and `vite preview` still proxies `/api`, so it would crash too.

## Gates — definition of done

- [~] spec `write-spec` — n/a: no behaviour change to the product; the harness's exit-2 contract
  ("could not run") already existed, this adds one more cause to it
- [~] architecture `architecture` — n/a: no stack change; the Node floor is a documented
  prerequisite of one dev-time tool, not a decision about the product
- [~] design `design-handoff` — n/a: no user-facing surface
- [~] backend — n/a
- [x] frontend — `web/geometry/run.mjs`: stop at the first `ERR_CONNECTION_REFUSED`, exit 2,
  name the cause; `web/geometry/README.md`: Node ≥ 24.16.0 prerequisite on Windows
- [x] testing `testing-strategy` — §12.5 gap rewritten as the diagnosis + the Node floor;
  47/47 vitest (pure halves) green; new path exercised live on the buggy Node twice (runs 4
  and 5: exit 2, the diagnostic, nothing scored)
- [~] security `security-review` — n/a: dev-time test harness, no auth/access/infra change

## Up next — ordered (position = priority)

1. [ ] [testing] Run the full matrix on Node ≥ 24.16.0 to close the loop (expected: 9/9 cells,
   `exit 0` or genuine layout verdicts; the six picker assertions need the `enrich-stub`
   profile on :9100) — `cd web && npm run geometry`
2. [ ] [—] §12 intro still says "Three skins × two viewport widths — 234 checks" (it is three
   widths / 765 checks since HOLODEX-363); stale, not this ticket's — `docs/testing-strategy.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-14 · root-caused, harness stops on a dead server, docs
- skills: code-review
- handoff: PR open; the only open item is a confirming full-matrix run on Node ≥ 24.16.0 (not
  installed here — v24.15.0 is the sole Node on the machine).
