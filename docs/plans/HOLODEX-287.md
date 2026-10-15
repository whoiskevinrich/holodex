---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-287
status: in-progress
profile: chore               # docs-only: redo the closed ADR as a stack.md topic-doc section
depends-on: []
release_note: ""
---

# HOLODEX-287 · Frontend component-reuse decision as a topic doc

The original ADR (closed PR #257, branch `HOLODEX-287-frontend-component-reuse-adr`) never merged
and kept colliding on ADR numbers. Numbered ADRs are retired (HOLODEX-523), so the decision is
redone as a section of `docs/architecture/stack.md`: one sibling component per data shape, sharing
extracted chrome, with the per-folder `CLAUDE.md` tables as the inventory.

What the old ADR carried that does **not** come back here:
- **Decision 2, a `PersonPicker`-shaped `TagPicker`**: superseded. Main closed the Tags gap with
  `TagAddInput` (HOLODEX-519, `docs/design/media-tag-input-handoff.md`), a text combobox. The old
  branch's `TagPicker.svelte` and handoff are obsolete.
- **Decision 1, a pointer bullet in `.claude/rules/frontend-theming.md`**: agent tooling, which
  `docs/reference/doc-types.md` keeps out of architecture. It landed as its own
  `chore(flightplan)` commit on this branch, linking to the `stack.md` section.

**Design package:** none (no UX change)

## Gates — definition of done

Posture `chore`: no gates. The architecture section is the whole deliverable.

## Up next — ordered (position = priority)

1. [x] Open the PR, squash-merge (#521 → `5e1da30c`), then pin `Decided in` in `stack.md` to the squash sha
2. [ ] Confirm HOLODEX-287 reached Done after merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-15 · session (3)
- skills: —
- handoff: Merged #521 as 5e1da30c and pinned stack.md's Decided-in line to it. Next: confirm HOLODEX-287 is Done.

### 2026-10-15 · session (2)
- skills: —
- handoff: Added the "reuse before you create a component" bullet to frontend-theming.md as a chore commit, pointing at the components CLAUDE.md files and the stack.md section. Next: merge #521, pin the squash sha.

### 2026-10-15 · session
- skills: code-review
- handoff: Redid the decision as stack.md's "one sibling per data shape, over shared chrome" section; TagPicker dropped as superseded by TagAddInput. Next: PR, merge, pin the squash sha.

## Dropped — newest first (the reason is the point)

- `TagPicker.svelte` (old Decision 2): superseded by `TagAddInput`, HOLODEX-519.
