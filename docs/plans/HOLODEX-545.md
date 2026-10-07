---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-545
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: infra               # release pipeline: build triggers, promotion tags, release-job gating
depends-on: []
release_note: ""             # pipeline only; v1.16.2's own notes come from its commits
---

# HOLODEX-545 · Hotfix release lane: patch a shipped minor from release/vX.Y without releasing main

This is done when a fix committed to a `release/vX.Y` branch can ship as `vX.Y.(Z+1)` through the normal
build-then-promote path. The moving tags (`X.Y`, `X`, `latest`) and the GitHub "Latest" release must go
only to the highest release in their range, and the Jira → Released sync must not run for a tag off
`main`. The first use is v1.16.2: a refreshed bookworm runtime that clears the 77 Trivy alerts on
`holodex:latest` while 2.0.0 isn't ready (Relates HOLODEX-446).

## Gates — definition of done

- [x] architecture → `docs/architecture/deployment.md` § "Hotfixes promote from a `release/vX.Y` branch". It amends the retag-promotion constraints and names the rejected alternatives (build at tag, `Release-As`). The "Decided in" line still needs the squash-commit link after merge.
- [x] backend → `{cmd,internal,providers}/**`. No Go change. The pipeline change is in `.github/workflows/{image,provider-tmdb,release}.yml` and `scripts/release-tags.mjs`.
- [x] testing `testing-strategy`. `scripts/release-tags.test.mjs` has 9 cases; its refusal cases are the point. A row is in `docs/testing-strategy.md`.
- [x] security `security-review`. 2026-10-07: no findings. Hardening note: `release/**` has no ruleset, so released bits no longer always come from code reviewed on `main`. The trust level is the same as tag pushes today.

## Up next — ordered (position = priority)

1. [ ] [—] Cut `release/v1.16` from `v1.16.1`, carry this pipeline change plus a refreshed runtime, and tag `v1.16.2`
2. [ ] [—] After v1.16.2 promotes, re-run `scan.yml` and confirm the `holodex:latest` alerts close
3. [ ] [—] Move HOLODEX-545 to Released by hand. The automatic sync is skipped off `main`
4. [ ] [—] Merge this PR, then replace "Decided in HOLODEX-545" in deployment.md with the squash-commit link
5. [ ] [—] Decide whether `release/**` gets a branch ruleset

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-07 · session
- skills: code-review, security-review
- handoff: Built the lane (release/** sha builds, highest-version moving tags, Jira sync gated on main) and documented it. The review fixed a silent ancestry-error path and a duplicated latest decision. Security review is clean. Next: cut v1.16.2.

## Dropped — newest first (the reason is the point)

- [~] [ci] Scope git-cliff `--latest` for a second hotfix after a newer major. Dropped 2026-10-07: it only over-includes entries in those release notes, and only for that case.
