---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-223                 # the tracker key; must match the branch key regex
status: in-review                   # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # test tooling with a stack fork: architecture + frontend + testing; the rest skipped below
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Developer-facing only — the web app now has browser-based component tests guarding the owner Status page's job views; nothing changes for someone using Holodex.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-223 · Add frontend component-test harness

Done means `web/` has a component-test harness that can mount a page and watch it after first paint,
and HOLODEX-203's two job-view invariants are under regression test as its first users. Broad
retroactive component coverage is out of scope.

**Design package:** [stack.md § Frontend component tests in Vitest browser mode](../architecture/stack.md#frontend-component-tests-in-vitest-browser-mode) · [testing-strategy §5 harness row](../testing-strategy.md)

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**` — test tooling, no product behaviour change
- [x] architecture `architecture` → `docs/architecture/**` — stack.md section: Vitest browser mode chosen over jsdom, a jsdom+browser hybrid, and SSR-only (owner picked browser mode, two-project split)
- [~] design `design-handoff` → `docs/design/**` — no UI change
- [~] backend → `{cmd,internal,providers}/**` — frontend-only
- [x] frontend → `web/src/**` — `vite.config.ts` client/server projects; `routes/owner/status/page.svelte.test.ts`; CI installs Chromium
- [x] testing `testing-strategy` — §0/§5/§8 updated; both invariants plus the log-fetched-once guard mutation-tested
- [~] security `security-review` — test-only tooling, no runtime/auth surface

## Up next — ordered (position = priority)

1. [ ] [—] Open the PR via /implement, merge, and confirm CI moves HOLODEX-223 to Done; pin stack.md's "Decided in" to the squash sha

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-14 · component-test harness landed
- skills: code-review, handoff, implement
- handoff: Crossed into build with no sign-off needed (design [~]); harness + invariant tests in 1b3efe0f, every gate settled, PR open and marked ready — next is merge, then pin stack.md's "Decided in".

## Dropped — newest first (the reason is the point)

- [~] [testing] Cache Chromium in CI — dropped 2026-10-14, `--with-deps` still runs apt each job; the owner accepted the ~1 min download
