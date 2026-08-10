---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-273                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — "Select all undecided" in the writeback dialog now creates a standing field decision for every field it writes, instead of leaving the DB reporting it undecided after the write.
---

# HOLODEX-273 · Writeback dialog "Select all undecided" doesn't create a standing decision

Bug fix: checking a box in `WritebackFormDialog` (individually or via **Select all** on the
undecided group) is now itself the commit action, mirroring the Tier-2 redesign's explicit
Confirm click — it creates a standing `field_source_decisions` row before the file write, so
the DB no longer reports the field undecided/RD6-pending after a bulk write already landed the
value in the file.

**Design package:** none (bug fix, no spec/ADR/design churn) · `docs/testing-strategy.md` §Per-field source-of-truth (F36, ADR-051)

## Gates — definition of done

- [~] spec `write-spec` — not applicable; bug fix with no requirement/scope change
- [~] architecture `architecture` — not applicable; no data-model/seam change, reuses existing F36 decision endpoint verbatim
- [~] design `design-handoff` — not applicable; no new markup, no visual change (behavior-only)
- [~] backend — not applicable; no Go changes, frontend-only fix over the existing owner-gated `PUT .../fields/{canonical}/decision` endpoint
- [x] frontend
- [x] testing `testing-strategy`
- [x] security `security-review`

## Up next — ordered (position = priority)

1. [x] [frontend] add `decide` prop + `ensureDecision()` to `WritebackFormDialog.svelte`, wire from `+page.svelte` — `web/src/lib/components/writeback/WritebackFormDialog.svelte`
2. [x] [testing] document the fix + live-verification in testing-strategy.md — `docs/testing-strategy.md`
3. [x] [security] run `/security-review` (label was set on the Jira issue) — no findings
4. [x] [—] push, open PR (#228), sync Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-10 · Implemented, self-corrected, and live-verified the decision-on-checkbox fix
- skills: testing-strategy, security-review
- handoff: `ensureDecision()` in `WritebackFormDialog.svelte` commits a standing decision for
  every newly-checked replace field before the write — guarded against already-decided rows, the
  `image_url` RD5 exclusion, and merge/multi fields (Genres/Actors/Director), the last one caught
  live during QA since `fields={resolved}` is the full unfiltered array and initially fired
  spurious decisions on them. Live-verified end to end against `backend-films` video id 8 via
  direct `curl` (not the browser network log, which flags a false `ERR_ABORTED` on every request
  in this environment): provider decisions, a manual-edit decision, the multi-field exclusion,
  the image_url exclusion, and the already-decided no-op all confirmed correct.
  `/security-review` ran against the real diff (the dialog calls the same owner-gated
  `api.setFieldDecision` `SourceSelect` already uses, no new mutation surface) — no findings.
  Next: push, open the PR, sync Jira.
