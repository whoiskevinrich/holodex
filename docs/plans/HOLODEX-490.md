---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-490
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: ui                  # a UX consistency fix; behaviour unchanged
depends-on: []
release_note: On a media page, "+ Add overview" and "+ Set part" now look like the page's other "+ Add …" actions instead of small dashed orange pills, and they're easier to hit.
---

# HOLODEX-490 · "+ Add overview" and "+ Set part" diverge from the page's "+ Add …" text CTA

This is done when every owner "add / set" affordance for a missing value on `/media/{id}` is the
`btn-quiet px-3 py-1.5 text-sm` text CTA, and the rule is written down where it loads at edit time, so
the next feature doesn't copy the nearest idiom again. HOLODEX-471 did exactly that for "+ Add overview".
Relates to HOLODEX-471.

## Gates — definition of done

- [x] design `design-handoff` → `docs/design/**`. The design critique chose "unify everything as text" (2026-09-29). The rule is in `.claude/rules/frontend-theming.md`, the terms *text CTA* and *ghost slot* are in `docs/reference/ui-vocabulary.md`, the overview-add and media-parts handoffs carry dated supersede notes, and the mockup is `docs/design/add-affordance-text-cta-mockup.svg`.
- [x] frontend → `web/src/**`. Both buttons now use `btn-quiet px-3 py-1.5 text-sm`. Verified on the stress fixture (video 9146): all six CTAs compute as muted 14px, 32px tall, borderless. Set part opens its input and Escape restores it; Add overview opens Edit Overview; no horizontal overflow. `npm run check`: 0 errors.
- [~] testing `testing-strategy`. Skipped deliberately: a class swap with no behaviour change. `web/` has no component-test harness, and the existing `part-pill-beside-the-title` geometry assertion still holds, because it counts `.part-pill`, which this change doesn't touch.

## Up next — ordered (position = priority)

1. [ ] [—] Open the PR, merge after CI; HOLODEX-490 moves to Done via jira-sync
2. [ ] [frontend] Follow-up (spun off): film page "+ Set edition" pill → text CTA, and "Attach film" vs "Add person" ghost-tile copy

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-29 · session
- skills: design-critique, code-review
- handoff: The critique found that "+ Add overview" had copied "+ Set part"'s dashed accent pill. You chose to unify both onto the btn-quiet text CTA and to document the rule in frontend-theming.md plus ui-vocabulary.md. Code-review skipped two findings: the Set-part input is 22px against the 32px button, which makes a small row jump; and the film-page holdouts, which are spun off.
