---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-543
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: backend             # a one-line defensive bound in writeback, no UI surface
depends-on: []
release_note: ""             # no user-visible change, since the out-of-range refs were already stripped
---

# HOLODEX-543 · Bound the char-ref code point before the rune conversion in stripIllegalCharRefs

This is done when `stripIllegalCharRefs` rejects a numeric character reference above `unicode.MaxRune` before
converting it to `rune`, so the result never depends on int32 wraparound, and CodeQL alert #144
(`go/incorrect-integer-conversion`) closes. Found in the 2026-10-07 code-scanning triage (Relates HOLODEX-446).

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**`. Skipped deliberately: a defensive bound with no behavior change. Values of 2^31 and above already wrapped negative and were stripped.
- [x] backend → `{cmd,internal,providers}/**`. `n > unicode.MaxRune` now guards the `rune(n)` conversion in `internal/writeback/writeback.go`.
- [~] testing `testing-strategy`. Skipped deliberately for the strategy doc: `TestMergeTagsXML_IllegalCharRefs` gains a `&#x80000041;` case that pins out-of-int32-range refs as stripped. No new test layer.

## Up next — ordered (position = priority)

1. [ ] [—] Confirm the Go CI job passes (Go isn't available locally, so it hasn't been run yet)
2. [ ] [—] Merge the PR, after which HOLODEX-543 moves to Done via jira-sync and CodeQL #144 closes on the next main analysis
3. [ ] [—] The other 77 open alerts (Trivy, bookworm packages in `holodex:latest` v1.16.1) clear when release PR #384 (2.0.0, trixie runtime) merges. That is not this branch's work

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-07 · session
- skills: code-review
- handoff: The bound and the test case are in. The code review added a comment on the guard and corrected the ticket's claim about the test. Tests haven't been run locally because there's no Go toolchain or Docker daemon, so CI is the first run.

## Dropped — newest first (the reason is the point)
