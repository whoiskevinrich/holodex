---
key: HOLODEX-525
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # agent tooling: a path-scoped rule and a PreToolUse hook
depends-on: []
release_note: ""
---

# HOLODEX-525 · Teach the doc skills their "never contains" boundaries

The `/architecture`, `/write-spec` and `/design-handoff` skills are plugin-owned, so the
boundaries arrive from the repo instead:

- `.claude/rules/doc-types.md` loads when a spec, architecture or design file is opened.
- `scripts/hooks/doc-type-guard.mjs` injects the matching boundary when one of the three skills is
  invoked, and blocks creating a new numbered `ADR-NNN-*.md`. It is wired in
  `.claude/settings.json` next to feature-claims-guard.

The optional content lint (e.g. table names in specs) was deliberately skipped: high
false-positive risk for little gain over the rule plus the injected boundary.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes. Then, in a fresh session, confirm the hook fires live: invoking `/architecture` shows the boundary, and Writing a new `ADR-NNN` is refused. This session loaded its hooks before the change

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-04 · rule + guard hook
- skills: code-review
- handoff: Rule file and doc-type-guard hook landed with 4 unit tests (`make test-scripts`). The hook was verified on real Windows-path hook input (exit 2), but not live in this session, which loaded its hooks before the change. Check it live in the next session.

## Dropped — newest first (the reason is the point)

- [~] [testing] content lint flagging table names / endpoints in specs — dropped 2026-10-04: too many false positives (specs legitimately name an API in passing). The rule and the injected boundary address the authoring moment instead
