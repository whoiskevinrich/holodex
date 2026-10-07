---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-544
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: chore               # CI scanner config: one input on three existing Trivy steps, no runtime change
depends-on: []
release_note: ""             # no user-visible change
---

# HOLODEX-544 · Make Trivy's SARIF upload honour the CRITICAL,HIGH severity filter

This is done when the Trivy steps in `scan.yml`, `image.yml` and `provider-tmdb.yml` upload only the
CRITICAL/HIGH findings their `severity:` input already asks for. Trivy's SARIF format ignores `severity:`
unless `limit-severities-for-sarif: true` is set. Found in the 2026-10-07 code-scanning triage
(Relates HOLODEX-446).

## Gates — definition of done

Chore posture: no gates. The change matches the threshold the workflows already declare, so it needs no ADR.
Side effect: on each category's next scan, the MEDIUM, LOW and UNKNOWN alerts already in the Security tab
(e.g. #143, the openssl LOWs, the #96/#97 tzdata notes) auto-close as "Fixed". They stop being reported;
the vulnerable packages are not remediated by this change.

## Up next — ordered (position = priority)

1. [ ] [—] Merge the PR. HOLODEX-544 then moves to Done via jira-sync
2. [ ] [—] Re-run `scan.yml` after merge to confirm the sub-HIGH alerts close

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-07 · session
- skills: code-review
- handoff: Added the SARIF severity limit to all three Trivy steps. The review flagged that dropped alerts read as "Fixed", which is recorded above and in the PR; no code change for it.

## Dropped — newest first (the reason is the point)
