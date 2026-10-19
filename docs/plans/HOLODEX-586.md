---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-586                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: feature             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The merge picker, the add-tag box, and the tag page's parent and category pickers now search your library as you type instead of loading every entry first, so they stay fast in a large library.
approved:
  design:
    on: 2026-10-18
    at: ed5928ca
---

# HOLODEX-586 · Pickers and datalists search the server, not whole tables

Subtask of HOLODEX-354 (stress fixture: every entity list fetched its whole table). Done means the
merge picker, the media add-tag input, and the tag page's parent field and category picker each ask
the server for a bounded set of matches as the owner types, never a whole entity table.

**Design package:** [entity-identity RD8a](../specs/entity-identity.md) ·
[tag-governance P0-8a](../specs/tag-governance-and-video-enrichment.md) ·
[tag-detail parent/category](../specs/tag-detail-hierarchy-and-categories.md) ·
[handoff](../design/bounded-picker-search-handoff.md) · testing-strategy §4 Search / FTS5 row ·
architecture: none (no technology fork)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] design `design-handoff` → `docs/design/**`
- [x] backend → `{cmd,internal,providers}/**`
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy`

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced in the SessionStart banner, clipped if long (the trailing path
     survives; the middle is dropped) — so keep it to one line.
     The CHECKBOX settles an item and nothing else does: [x] done, [~] dropped, same states as a gate
     row. A strikethrough is emphasis, not a state — `~~half~~ now do the rest` is a live item
     (ADR-008). This queue holds LIVE work only: delete a done item, move a dropped one to
     ## Dropped at the end of this file (ADR-010). The banner counts any settled item left here. -->

1. [ ] [—] Squash-merge #539 with its clean subject (CI moves HOLODEX-586 to Done)
2. [ ] [—] Depends on HOLODEX-354: /tags still reads every category; it fetches them only for remove mode once it pages that route — `web/src/routes/tags/+page.svelte:175`
3. [ ] [backend] Category `?q=` folds ASCII case only (SQLite lower()); an FTS table would fold like the tag picker → HOLODEX-588

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-19 · design sign-off re-pinned
- skills: —
- handoff: Design sign-off re-pinned to ed5928ca (the owner chose the ranked-substring category rule on 2026-10-18, the handoff amendment's only change); #539 is ready — next is the squash-merge.

### 2026-10-18 · category search folded into GET /categories?q=
- skills: code-review high --fix
- handoff: Per HOLODEX-354's contract, category search is now optional `q` on GET /categories (ranked substring, cap 25, owner chose it); /categories/search is gone; draft PR #539 updated — start at marking it ready.

### 2026-10-18 · bounded picker search built and verified
- skills: code-review high --fix, handoff, implement
- handoff: Crossed into build — design signed off at 74697074; draft PR #539 open with every gate settled. Start at marking #539 ready.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
