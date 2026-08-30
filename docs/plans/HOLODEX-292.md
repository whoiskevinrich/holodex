---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-292                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Tag chips now look and behave the same everywhere they appear (Media, Films).
---

# HOLODEX-292 · Shared TagLinkChip component

Extract the tag display chip (name + provenance suffix + optional remove control) into one shared
`TagLinkChip.svelte`, replacing three inconsistent inline chip styles that Media (owner and
visitor branches disagreed with each other) and Film detail pages each grew independently. Done
means: one component, wired into both existing call sites, verified across owner/visitor states
and all three skins, with design + testing gates closed. People details is explicitly out of
scope (no `person_tags` relation exists yet — tracked separately as HOLODEX-39).

**Design package:** no spec/ADR (pure display-markup consolidation, no new capability or schema
change — see the handoff's own "Why no spec/ADR" section) · [tag-link-chip-handoff.md](../design/tag-link-chip-handoff.md) · [testing-strategy.md §11](../testing-strategy.md)

## Gates — definition of done

- [~] spec `write-spec` — not applicable, no new user capability (handoff §"Why no spec/ADR")
- [~] architecture `architecture` — not applicable, no schema/cross-cutting decision
- [x] design `design-handoff` → `docs/design/tag-link-chip-handoff.md`, `docs/design/tag-link-chip-mockup.svg`
- [~] backend — not applicable, both pages already receive fully-populated `Tag` objects; no query changes
- [x] frontend → `web/src/lib/components/entity/TagLinkChip.svelte`, wired into `media/[id]` and `films/[id]`
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §11 (manual driven-browser QA, no backend to Go-test)
- [~] security `security-review` — not applicable, no auth/access/infrastructure touched, no new mutation surface

## Up next — ordered (position = priority)

1. [ ] [—] Jira sync: confirm HOLODEX-292 reflects gates above, push, open Draft PR

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-29 · session
- skills: design-handoff, testing-strategy, simplify
- handoff: Component built, wired into Media + Film detail pages, verified in-browser (owner/visitor
  states, all 3 skins) including catching and fixing a provenance-suffix visibility regression before
  it shipped. `/simplify` found and fixed two real issues (reused `f36.ts`'s `isProviderSource`/
  `providerOf` instead of re-parsing `provider:` inline; corrected the handoff's inaccurate claim that
  the `onremove`-presence owner switch matches `EntityImageSlot`'s pattern — it doesn't, that component
  uses an explicit `isOwner` boolean). Design + testing gates now green; spec/architecture/backend/
  security are all correctly not-applicable for this pure frontend consolidation. Next session: sync
  Jira, commit, push, open the Draft PR.

### 2026-08-29 · session
- skills: code-review (xhigh --fix)
- handoff: PR #270 opened ready-for-review, then ran a 10-angle `/code-review xhigh --fix` pass on it.
  Fixed two real correctness regressions: the busy-remove-button glyph (`× → …`) was driven by
  Media's page-global `tagBusy` flag, so mutating any one tag made every chip look like it was being
  removed — reverted to a static `×`. Read-only chips (Film, Media visitors) had lost their full-pill
  click/tap target when the extraction moved padding off the `<a>` onto a non-interactive wrapper —
  restructured to two branches so the read-only path's `<a>` is the whole padded pill again, matching
  prior behavior exactly. Also simplified `onremove={isOwner ? () => removeTag(t.id) : undefined}` to
  `onremove={isOwner ? removeTag : undefined}` (removeTag's signature already matches). Handoff doc and
  `npm run check` (0 errors) kept in sync; both fixes re-verified live in-browser (owner + visitor).
  Skipped four findings as deliberate/out-of-scope: provenance hidden from visitors (matches an
  explicit Resolved Decision, already reaffirmed in QA), `entity/` folder placement (mirrors
  StudioLinkCard's identical precedent — the CLAUDE.md rule text is what's stale, not this file), a
  pre-existing unrelated near-miss error-misattribution bug in `submitTagAdd`, and `categories/[id]`
  still having its own unmigrated duplicate tag-chip markup (spun off as a follow-up task instead).
  Next session: sync Jira, push the fix commit (PR already open).
