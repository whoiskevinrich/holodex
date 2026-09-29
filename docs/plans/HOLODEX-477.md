---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-477                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Going Back to a list now returns to where you were even if the page reloaded in between — after a sign-in refresh or an update, say.
                             # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-477 · List scroll position is lost across a full page reload

Done when ← Back to a scroll-only list (People, Studios, Tags, Films, Search, an entity's videos)
returns to the saved offset even after the document reloaded in between (re-auth, deploy, tab discard).

**Design package:** [ADR-118](../architecture/ADR-118-list-scroll-survives-reload.md) · [testing-strategy §21](../testing-strategy.md)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default — the list below is only what you get with no config. Once `profile:` is
     set, trim them to that posture. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — bug fix restoring intended Back behaviour (ADR-032 R7/R8); no requirement changed
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-118
- [~] design `design-handoff` → `docs/design/**` — no visible UI change, only where Back lands
- [~] backend → `{cmd,internal,providers}/**` — frontend-only
- [x] frontend → `web/src/**` — `navSnapshot.svelte.ts` registry mirrored to sessionStorage
- [x] testing `testing-strategy` — `navSnapshot.test.ts` + nav harness `back-after-reload-restores-scroll`, §21, mutation-checked
- [~] security `security-review` — no auth/access/infra change; per-tab storage of scroll offsets only

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

1. [ ] [—] Kevin merges the PR once CI is green (squash, Conventional subject) — `gh pr merge --squash`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-28 · session
- skills: code-review, handoff
- handoff: listScroll now survives a reload via sessionStorage (ADR-118); unit + nav harness green and mutation-checked; origin/main (ADR-117) merged in — PR is ready, next move is Kevin's merge.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
