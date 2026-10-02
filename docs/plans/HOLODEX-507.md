---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-507                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: feature             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: "A tag's page now lists its aliases so you can add, remove or merge them, search finds a tag by any of its aliases, and tag aliases are always lowercase like tag names."  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
approved:
  design:
    on: 2026-10-01
    at: df8c3b05
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-507 · Tag aliases: tag page panel, alias search, lowercase alias text

Finishes the tag half of F43 / ADR-061 D7. The alias backend for tags already existed (shared
`entity_aliases`, alias-aware resolve, `POST/DELETE /tags/{id}/aliases`, merge, rename). What was missing:
seeing and removing a tag's aliases, finding a tag by alias in search, lowercase alias text, and genre
writeback writing an alias beside its own tag. Done = all four, per the amended spec.

**Design package:** spec [entity-identity.md](../specs/entity-identity.md) (F43 amendment: revised RD7,
RD12, P0-9 for tags, P0-10) · handoff [tag-aliases-handoff.md](../design/tag-aliases-handoff.md) +
[mockup SVG](../design/tag-aliases-mockup.svg) · no ADR (completes ADR-061).

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] design `design-handoff` → `docs/design/**` — owner signed off 2026-10-01
- [x] backend → `{cmd,internal,providers}/**`
- [x] frontend → `web/src/**` — owner confirmed the build matches the approved mockup
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

1. [ ] [—] Review and merge the PR; CI moves HOLODEX-507 to Done
2. [ ] [—] Studio alias search, the other unmet half of P0-9 → HOLODEX-508
3. [ ] [—] Raw alias of an unattached tag is written in alias spelling → HOLODEX-509

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-01 · design, build and QA in one session
- skills: engineering:system-design, code-review (high --fix), handoff, implement
- note: the branch got its key mid-session, so no worklog existed and the design-phase guard never fired; backend and frontend were built **before** the design sign-off. Owner approved the scope and lowercase aliases in chat, not the committed mockup.
- note: security — no auth, access or infrastructure change (existing owner-gated endpoints, one parameterized FTS query, writeback only drops values), so no `/security-review`; the Jira `needs-security-review` label was cleared.
- handoff: Crossed into build — design signed off at df8c3b05, every gate settled, main merged in; PR open. Start at: review and merge it.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
