---
key: HOLODEX-528
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # docs + agent tooling; no product surface
depends-on: [HOLODEX-527]
release_note: ""
---

# HOLODEX-528 · Retire the numbered ADRs

The last task of epic HOLODEX-523. Approach agreed with the owner 2026-10-04: **archive and
redirect, don't delete and rewrite.** Rewriting all ~6,300 mentions across ~900 files was rejected:
it would lose sub-decision precision, it would break append-only worklogs, and the diff would be
unreviewable.

- All 121 `ADR-NNN` files moved to `docs/architecture/archive/`. Each now opens with a banner naming
  where its content lives (a topic doc, a reference doc, or "never adopted").
- `archive/README.md` holds the old index; the live `docs/architecture/README.md` lists topic docs only.
- Links repo-wide were rewritten by path only (317 files), and 7 links that were already broken were
  fixed. The link check is clean (1,414 links). Bare `ADR-NNN` mentions in code still resolve via
  the archive.
- The 12 process ADRs were folded into `docs/reference/`, including the new `ci-and-releases.md`
  and `dev-credentials.md`.
- `adr-claims.mjs` retired. Its shared helpers are now `scripts/claims-common.mjs`, and the dead
  `adr-numbers` CI job was dropped.
- Live guidance (CLAUDE.md, `.claude/rules`, `doc-types.md`) points at topic and reference docs. The
  doc-type guard also blocks new numbered files in the archive.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes, then close HOLODEX-528 and epic HOLODEX-523 by hand (jira-sync skips epics)
2. [ ] [—] Owner's call (found while folding the process ADRs): `main` has no branch protection, so "CI required" and the worklog gate aren't enforced; `cliff.toml` drops `Release-Note:` trailers, though the workflow doc says they reach release notes

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · archive + redirect
- skills: code-review
- handoff: Numbered ADRs archived with redirect banners; links fixed by path; process ADRs folded into `docs/reference/`; adr-claims retired. Script tests pass (130) and the link check is clean.

## Dropped — newest first (the reason is the point)

- [~] [docs] rewrite every `ADR-NNN` mention to topic docs — dropped 2026-10-04: ~6,300 mentions across ~900 files, sub-decision precision lost, worklogs are append-only. Mentions resolve via the archive banners; replace them when a file is next touched
