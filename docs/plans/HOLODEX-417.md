---
key: HOLODEX-417
status: in-progress
depends-on: []
release_note: The Edit Description / Notes / Bio dialog now keeps Save and Cancel on screen however long the text is — each source is clamped to four lines with a Show more toggle, and the dialog scrolls inside itself instead of growing past the window.
---

# HOLODEX-417 · Edit Description modal pushes Save/Cancel off screen when sources are long

Done means: with a 2000-character overview in any namespace, `SourceEditModal` (Video overview,
Film description, Person bio) shows every source row and the Save/Cancel footer on screen at
once, the dialog scrolls inside itself when a row is expanded, and every other `ConfirmDialog`
caller is bounded the same way.

**Design package:** spec n/a (bug, no behaviour change) · ADR n/a · [handoff](../design/source-edit-modal-overflow-handoff.md) · testing-strategy row "`ConfirmDialog` bounded + `SourceEditModal` clamped rows"

## Gates — definition of done

- [~] spec `write-spec` → `docs/specs/**` — until: a behaviour change; this is a layout bug
- [~] architecture `architecture` → `docs/architecture/ADR-*` — until: a cross-cutting decision; none here
- [x] design `design-handoff` → `docs/design/**`
- [~] backend — until: any server change; none
- [x] frontend
- [x] testing `testing-strategy`
- [~] security `security-review` — until: auth/access/infra is touched; none

## Up next — ordered (position = priority)

1. [ ] [—] Kevin's prod-skin look at `/media/<id>` Edit Notes with a long value — then mark PR ready
2. [ ] [—] On merge: nothing to sweep (single Bug, branch-keyed — CI moves it to Done)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · critique → fix, all three priorities shipped
- skills: design-critique, design-handoff, testing-strategy, code-review
- handoff: A (bounded `ConfirmDialog`), B (`SourceValueClamp` four-line rows), C (`rows="5"`) are coded and measured live on the AMV testbed at 520px and 760px in all three skins; PR is up as Draft pending Kevin's look — nothing else outstanding.
