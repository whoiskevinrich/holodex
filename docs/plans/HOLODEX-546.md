---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-546
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: chore               # the enforcement is a repo setting the owner applies; this branch only documents it
depends-on: []
release_note: ""             # no user-visible change
---

# HOLODEX-546 · Enforce the merge gate: activate the main ruleset with current checks, a bypass, and release/** coverage

This is done when the `main` ruleset (id 18851631) is Active. It must require the current `ci.yml` jobs plus
the worklog gate's `gate`, but not `transition`, which never reports after a PR's first push. It must block
deletion and force-push, give Repository admin a bypass, and cover `release/**`. The docs must describe the
enforced gate instead of saying "Not enforced by GitHub today". Found while answering the `release/**` ruleset
question from HOLODEX-545 (Relates).

## Gates — definition of done

Chore posture: no gates. The ruleset change is an owner action in repo settings. This branch carries the docs
and corrects the worklog-gate install comment, which named the check `worklog gate` instead of `gate`.

## Up next — ordered (position = priority)

1. [ ] [—] Owner: edit ruleset `main` per HOLODEX-546 (checks, bypass, `release/**` target) and set it Active
2. [ ] [—] Confirm the ruleset with `gh api repos/whoiskevinrich/holodex/rulesets/18851631`, then mark this PR ready and merge it
3. [ ] [—] Carry the `gate` check-name fix in the install comment upstream to the Flightplan plugin's `worklog-gate.yml` template, so a re-sync doesn't revert it
4. [ ] [—] Optional: a tag ruleset on `refs/tags/v*` (block deletion/update)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-07 · session
- skills: code-review
- handoff: Investigated why `main` is unprotected. The ruleset was created disabled on 2026-07-12 and never edited, with no recorded reason. As configured it would block PRs on `transition`. I drafted the docs for the enforced state and fixed the gate's check name in the install comment. Waiting on the owner to apply the ruleset.

## Dropped — newest first (the reason is the point)
