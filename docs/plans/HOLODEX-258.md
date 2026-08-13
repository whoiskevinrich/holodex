---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-258                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Hardened metadata-provider enrichment so a compromised provider can no longer redirect a video's studio credit onto an unrelated existing studio.
---

# HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values

`_studio_external_ids` (ADR-054) is provider-authored directly as a self-describing
`"<namespace>:<id> <name>"` string, with no structured type standing between the provider and the
stored value — unlike the analogous `_person_external_ids` sidecar (F32/HOLODEX-102), which core
synthesizes itself from a structured `people[]` array. `sanitizeFields` never validated the id
token's shape, so a malicious/compromised provider could emit an arbitrary external id and
converge a video's studio credit onto an unrelated, attacker-chosen existing Studio via
`ReconcileVideoStudios`'s external-id-first resolve. This was filed as a deliberately-deferred
sibling gap while fixing the equivalent `sanitizePeople` finding on F32 (PR #219) — "done" here
means `_studio_external_ids` gets the same ingest-time shape guard, mirroring `sanitizePeople`.

**Design package:** no spec/ADR (shape of `sanitizeFields` unchanged; no UI surface) ·
[docs/testing-strategy.md](../testing-strategy.md) "Studio external-id de-dup" + F32 sections

## Gates — definition of done

- [~] spec `write-spec` — not required: no functional/behavioral scope change, a security hardening
      fix to an existing internal ingest path
- [~] architecture `architecture` — not required: `sanitizeFields`'s generic shape is deliberately
      unchanged; the new `sanitizeStudioExternalIDs` is a local ADR-055-aligned guard, not a new
      architectural seam
- [~] design `design-handoff` — not required: no UI surface
- [x] backend → `sanitizeStudioExternalIDs` + call-site wiring in
      `internal/enrich/service.go`, right after the existing `sanitizeFields` call
- [~] frontend — not applicable: no frontend surface touched
- [x] testing `testing-strategy` → pure-function table test
      (`TestSanitizeStudioExternalIDsRejectsMalformedID`) + end-to-end fake-provider/real-DB test
      (`TestEnrichVideoRejectsMalformedStudioExternalID`) in `internal/enrich/enrich_test.go`;
      `docs/testing-strategy.md` updated (Studio external-id de-dup bullet + F32 "Filed, not
      fixed" sentence flipped to Fixed)
- [x] security `security-review` → clean pass, no high-confidence findings; confirmed
      `sanitizeStudioExternalIDs` closes the finding (parameterized downstream lookup, matching
      split-on-first-space grammar between sanitize and re-parse, no SQL/injection surface)

## Up next — ordered (position = priority)

1. [ ] [—] get [PR #239](https://github.com/whoiskevinrich/holodex/pull/239) reviewed and merged
      to main (all gates green; Jira issue in **In Review**)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-13 · implementation + tests + docs sync + PR opened
- skills: (plan mode — no slash skill yet this session), simplify, security-review
- handoff: `sanitizeStudioExternalIDs` implemented and wired into `internal/enrich/service.go`
  right after `sanitizeFields`; both tests added and green
  (`go test ./internal/enrich/... ./internal/api/...`); `docs/testing-strategy.md` and
  `docs/plans/HOLODEX-102.md` (Up next #2) updated. `/simplify` (4-agent pass) found nothing
  requiring a code change. `/security-review` came back clean — no high-confidence findings,
  confirmed the fix closes HOLODEX-258. All gates green; committed (c37feb0), pushed, and opened
  [PR #239](https://github.com/whoiskevinrich/holodex/pull/239) ready for review. **Next
  session (if any):** watch for PR review feedback; merge → Jira Done → GHCR release → Jira
  Released, all automatic per ADR-058.
