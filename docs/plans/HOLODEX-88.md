---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-88                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: ui                  # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: When no metadata provider is configured, the owner now sees a muted "No metadata provider configured" note where the Enrich buttons would be, and the enrichment queue says so instead of "Nothing left to review."
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-88 · No metadata provider configured hint

With zero providers configured the owner sees why the Enrich chips are missing: a muted plain-text
hint on the person, studio, film and media pages, and an honest empty state on the enrichment queue
(QA checklist 3.4). Child of HOLODEX-168.

**Design package:** [metadata-enrichment-handoff.md § Edge Cases](../design/metadata-enrichment-handoff.md#edge-cases) · [no-provider-hint-mockup.svg](../design/no-provider-hint-mockup.svg) · [QA checklist 3.4](../design/metadata-enrichment-qa-checklist.md)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] design `design-handoff` → `docs/design/**`
- [/] frontend → `web/src/**` — built and live-verified; held for the owner's build-vs-mockup comparison (§3d)
- [~] testing `testing-strategy` — no new test surface: these route pages have no component tests; verified live against an empty-sources backend, QA checklist 3.4 rewritten — until: route pages gain a component-test harness

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

1. [ ] [design] Run /implement: owner signs off the mockup, Draft PR opens — `docs/design/no-provider-hint-mockup.svg`
2. [ ] [frontend] Owner compares the built hint against the mockup, then settle frontend — `web/src/routes/`
3. [ ] [frontend] Decide copy when providers exist but none serve this entity type (hint says "none configured") — `web/src/lib/components/enrichment/NoProviderHint.svelte`
4. [ ] [—] Fold the four copies of the sources-loading effect into one helper — `web/src/routes/*/[id]/+page.svelte`
5. [ ] [—] On merge: sweep HOLODEX-88 to Done by hand (child of epic HOLODEX-168)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-13 · mockups, owner choices, build
- skills: code-review, handoff
- handoff: Hint built on all four detail pages and the queue, live-verified with zero providers; next is /implement for the owner's mockup sign-off and the Draft PR.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
