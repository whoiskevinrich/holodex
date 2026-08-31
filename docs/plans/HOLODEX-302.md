---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-302                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The banner, poster, and headshot on a person's detail page now lift to the front and enlarge slightly on hover, so you can see the whole image where it used to be partly hidden behind an overlapping neighbor.
---

# HOLODEX-302 · Person hero image hover-to-front

Ad hoc UX bug fix (no epic — filed after the user reported the poster being hard to make out
behind the headshot badge). Done means: hovering (or keyboard-focusing the Edit control inside)
any of the three hero images — banner, poster, headshot — on the person detail page raises it
above its overlapping neighbor and nudges it slightly larger via a shared `.person-hero-media`
CSS hook, without disturbing layout; verified live in the browser preview with synthetic
banner/poster/headshot images uploaded to a throwaway local person (deleted after) across the
Cinémathèque and Brutalist skins — the change is purely structural (position/z-index/transform,
no colors or tokens), so the third skin (Broadcast) was not separately screenshotted.

**Design package:** none — a pure hover/z-order fix on the existing F25 hero layout, not a new
UX pattern; no spec/ADR/design-handoff artifact judged necessary for this scope.

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → not needed; no behavior/data-model change, pure hover interaction on the existing F25 hero
- [x] architecture `architecture` → not needed; no architectural change
- [x] design `design-handoff` → not needed; contained CSS/markup fix to an already-specced layout, not a new pattern
- [x] frontend → `web/src/app.css`, `web/src/lib/components/person/PersonBanner.svelte`, `web/src/routes/people/[id]/+page.svelte`
- [x] testing `testing-strategy` → not needed; no new logic surface, verified live in the browser preview
- [x] security `security-review` → not needed; no auth/access/infra surface touched

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [frontend] `.person-hero-media` hover/focus-within z-index + scale lift — `web/src/app.css`
2. [x] [frontend] wire the class onto the banner wrapper — `web/src/lib/components/person/PersonBanner.svelte`
3. [x] [frontend] wire the class onto the poster and headshot wrappers in the hero — `web/src/routes/people/[id]/+page.svelte`
4. [x] [frontend] live-verify in the browser (synthetic images, Cinémathèque + Brutalist skins)

Nothing left.

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-31 · implementation + verification + ticket/worklog
- skills: simplify
- handoff: HOLODEX-302 is implementation-complete and live-verified (synthetic banner/poster/
  headshot images uploaded to a throwaway local person via the owner-upload API, deleted after)
  showing the hover-to-front lift working in Cinémathèque and Brutalist; ready for PR.
