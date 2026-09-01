---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-303                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Person bio now shows in the header next to the name, and edits open a clearer picker.
---

# HOLODEX-303 · Person detail: move bio into the name/photo header row

Bio moves out of the Details card and into the hero header row (third column beside name/meta,
line-clamped so it never grows the row on desktop; stacks full-width on mobile). Owner editing
switches from SourceBadge's inline chip row to a new pencil-icon → "Edit bio" modal pattern,
documented as the standard for `long_text` tier-2 fields going forward.

**Design package:** — · — · [person-detail-bio-header-handoff.md](../design/person-detail-bio-header-handoff.md) · —

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — until: judged behavior-only (layout + interaction pattern change on an existing field), no new spec needed; revisit if scope grows
- [~] architecture `architecture` → `docs/architecture/ADR-*` — until: no cross-cutting/data-model change identified; the new modal pattern is a component convention, not an ADR-worthy decision
- [x] design `design-handoff` → `docs/design/**`
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [frontend] Add Bio as a third hero-row column in `hero()`, with line-clamp truncation — `web/src/routes/people/[id]/+page.svelte`
2. [ ] [frontend] Remove the `bio`-specific rendering from the `longFields` loop in `detail()` (Details section) — `web/src/routes/people/[id]/+page.svelte`
3. [ ] [frontend] Build the "Edit bio" modal component (radio source list + inline Custom textarea, `ConfirmDialog` chrome) — `web/src/lib/components/curation/`  ⛔ blocked on #1/#2
4. [ ] [frontend] Wire pencil icon (owner mode) + modal into the new Bio header column — `web/src/routes/people/[id]/+page.svelte`  ⛔ blocked on #3
5. [ ] [testing] Component/e2e coverage for truncation, empty bio, modal Save/Cancel/error states, three-skin QA
6. [ ] [—] Video `overview` adoption of the same modal pattern → HOLODEX (candidate follow-up issue, out of scope here)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-31 · design-handoff artifacts committed
- skills: design-handoff
- handoff: Mockup + handoff spec landed (`docs/design/person-detail-bio-header-mockup.svg`,
  `person-detail-bio-header-handoff.md`); design converged after 3 rounds of user review — header
  owns editing via a new pencil+modal pattern (not SourceBadge chips), bio drops from Details,
  mobile stacks full-width. Next session picks up implementation per "Up next" above.
