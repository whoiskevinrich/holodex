---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-294                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The People and Cast sections now show larger poster tiles, and share one consistent look across Media and Film pages.
---

# HOLODEX-294 · Reusable PeopleGrid component

Extract the Media detail page's People section into a shared `PeopleGrid.svelte`, increase its
poster tile size to match the Films section, replace the empty-state dashed box with a "+ Add
person" text CTA, and reuse the same component for the Film detail page's read-only Cast section.
Done means: one component, wired into both call sites, verified across owner/visitor/empty states
and all three skins, with design + testing gates closed.

**Design package:** no spec/ADR (pure display-markup consolidation, no new capability or schema
change — see the handoff's own "Why no spec/ADR" section) · [people-grid-handoff.md](../design/people-grid-handoff.md) · [testing-strategy.md](../testing-strategy.md)

## Gates — definition of done

- [~] spec `write-spec` — not applicable, no new user capability (handoff §"Why no spec/ADR")
- [~] architecture `architecture` — not applicable, no schema/cross-cutting decision
- [x] design `design-handoff` → `docs/design/people-grid-handoff.md`, `docs/design/people-grid-mockup.svg`
- [~] backend — not applicable, both pages already receive fully-populated `Person[]`; no query changes
- [x] frontend → `web/src/lib/components/entity/PeopleGrid.svelte`, wired into `media/[id]` and `films/[id]`
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (manual driven-browser QA, no backend to Go-test)
- [~] security `security-review` — not applicable, no auth/access/infrastructure touched, no new mutation surface

## Up next — ordered (position = priority)

1. [ ] [—] Commit, push, open PR (non-draft — all applicable gates green)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-29 · session
- skills: design-handoff, testing-strategy, simplify
- handoff: `PeopleGrid.svelte` extracted and wired into Media (owner-editable) and Film Cast
  (read-only, no attach/detach passed). Poster tiles resized to match Films' `grid-cols-3/4/6`;
  empty state swapped from a dashed box to a bare "+ Add person" text CTA via `PersonPicker`'s new
  `hasPeople` prop. `/simplify` found and fixed three real issues (see PeopleGrid.svelte comments/
  diff); one finding (attach/detach as two separately-optional props) explicitly skipped as a noted
  footgun, not worth a bigger API change yet. Verified in-browser: Media populated + empty states,
  Film Cast with real attached-film data, all 3 skins via computed-style contrast (5.7–6.3:1) and
  tile-geometry checks (worked around a `resize_window` preset viewport-zero artifact). `npm run
  check`: 0 errors. Design + testing gates now green; spec/architecture/backend/security all
  correctly not-applicable. Next session: commit, push, open PR.
