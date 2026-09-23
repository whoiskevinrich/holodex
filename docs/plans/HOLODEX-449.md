---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-449
status: in-progress
release_note: No user-facing change. CodeQL no longer analyses test files, which stops a recurring class of false positives in the Security tab.
---

# HOLODEX-449 · CodeQL config — exclude test files from analysis

Child of epic [HOLODEX-446](https://whoiskevinrich.atlassian.net/browse/HOLODEX-446) (code-scanning
cleanup), Phase 4. Alert #83 (`js/incomplete-multi-character-sanitization`) flagged
`.replace(/<!--[\s\S]*?-->/g, '')` in `web/src/routes/media/[id]/playerElement.test.ts` — comment
stripping applied to a `.svelte` file read off disk inside a vitest structural assertion. No
untrusted input, nothing rendered as HTML. That was the second alert of this shape. Test files are
not attack surface; excluding the class beats dismissing them one at a time forever.

## Gates — definition of done

- [~] spec `write-spec` — **not applicable.** No functional or behavioural change; nothing ships
  to a user. CI configuration only.
- [~] architecture `architecture` — **deliberately skipped.** CLAUDE.md routes *cross-cutting*
  infrastructure decisions to an ADR; this is a two-line narrowing of one scanner's file set, and
  the reasoning lives where a maintainer will actually meet it — in the config file's own header
  comment. An ADR here would be ceremony pointing at eight lines of YAML.
- [~] design `design-handoff` — **not applicable.** No user-facing surface.
- [~] backend — **not applicable.** No `cmd/`, `internal/` or `providers/` change.
- [~] frontend — **not applicable.** No `web/**` change.
- [x] testing `testing-strategy` — `scripts/codeql-config.test.mjs`, 5 assertions, both failure
  directions mutation-checked. It runs in the existing `scripts` CI job via `make test-scripts`.
  No `docs/testing-strategy.md` section: the guard is self-describing and §15's precedent is for
  features, not a single CI config file.
- [x] security `security-review` — **signed off 2026-09-22, no findings.** This *reduces* scan
  scope, so the review is about what coverage is lost. (1) Only test files are excluded —
  asserted by the guard test, not by eye. (2) **Go is unaffected**: GitHub's docs are explicit
  that `paths-ignore` applies to interpreted languages and to compiled languages analysed
  *without* building; this repo builds Go with `build-mode: manual`, so all 205 `*_test.go` files
  remain in scope. The config says so rather than implying otherwise. (3) **Secret coverage is
  untouched** — gitleaks runs separately over full history (ADR-094 D4), which is the obvious
  objection and the real answer to it. (4) No `queries:` block, so which rules run is unchanged.
  (5) No `paths:` allowlist, which would have silently dropped any new top-level directory.

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-449] Kevin's review of PR #380
2. [ ] [HOLODEX-449] on merge, sweep to Done **by hand** (CI transitions only the branch's issue)
3. [ ] [HOLODEX-446] epic close-out once 448 + 449 are merged; 450 (trixie) remains blocked on 448

> **Post-merge verification is already done, on the PR's own CodeQL run** (job 107038580604) —
> it did not need to wait for main. The log shows `Using configuration file input from workflow`,
> the `paths-ignore:` block echoed, and then: **222 files extracted, 59 of them from `web/src`,
> and 0 matching `*.test.ts` / `*.test.mjs`.** Both halves proved on real infrastructure rather
> than assumed: the exclusion bites, and the app code is still scanned.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-22 · config + guard
- skills: testing-strategy, security-review
- handoff: **Shipped.** `.github/codeql/codeql-config.yml` with `paths-ignore` for `**/*.test.ts`
  and `**/*.test.mjs`, wired into `codeql.yml`'s init via `config-file:`.
  **The fact that shaped the change:** GitHub's docs state `paths`/`paths-ignore` apply to
  interpreted languages and to compiled languages analysed *without* building — so this is a
  **no-op for Go** (`build-mode: manual`). Rather than write a Go pattern that quietly does
  nothing, the config says so in its header. Verified by simulation before committing: 47 files
  excluded, **all** of them test files, zero non-test; `web/src` keeps 180 files, `scripts/`
  keeps its 16 tooling `.mjs`, Go keeps all 205 `_test.go`.
  Dropped the ticket's suggested `**/*.spec.ts`: the repo uses `.test.` exclusively (zero `.spec.`
  files) and a pattern matching nothing reads as load-bearing config that isn't.
  Converted the throwaway verification into `scripts/codeql-config.test.mjs` (5 assertions) —
  `paths-ignore` fails in the worst direction, where broadening it by one entry silences the
  Security tab in a way indistinguishable from "no vulnerabilities". Mutation-checked twice:
  adding `**/*.ts` fails two assertions, unwiring `config-file:` from the workflow fails the
  wiring one. Full scripts suite 139/139. Left: Kevin's review.

### 2026-09-22 · verified on a real CodeQL run
- skills: —
- handoff: PR #380 opened **ready** (all gates green, so no Draft stage). CI fully green, and the
  `analyze (javascript-typescript, none)` leg passing is itself proof the config is valid — an
  unparseable `config-file` fails the init step. Went further than that and read the extraction
  log: **222 files extracted, 59 from `web/src`, 0 matching the ignored patterns.** That closes
  the ticket's step 3 ("paths-ignore must not silently narrow the scan to nothing") with evidence
  instead of an assumption, and it did not need to wait for main. Left: Kevin's review, then a
  hand-sweep to Done on merge.
