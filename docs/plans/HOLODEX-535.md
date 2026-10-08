---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-535                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: "Studios and tags you've aliased, given an image or curated are no longer deleted when their last video goes away; unused ones are cleaned up after 30 days, like people."  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-535 · Guarded prune-on-empty for studios and tags

Done when studios and tags follow the people orphan model: losing the last video link stamps
`orphaned_at` on every path (reconcile, tag detach, rescan, purge, bare tag creation), and one
grace-period sweep deletes only orphans with no authored data. Also resolves HOLODEX-494.

**Design package:** [entity-relationships.md § orphan sweep](../architecture/entity-relationships.md) ·
[studio-entity.md RD1](../specs/studio-entity.md) · [tag-governance § Unused tags](../specs/tag-governance-and-video-enrichment.md) ·
[studio-clear.md non-goal](../specs/studio-clear.md)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**` — studio-entity RD1, studio-clear non-goal, tag-governance "Unused tags"
- [x] architecture `architecture` → `docs/architecture/**` — entity-relationships.md "one orphan stamp and one sweep"
- [~] design `design-handoff` → `docs/design/**` — no UI surface; lists already hide/show orphans as before
- [ ] backend → `{cmd,internal,providers}/**`
- [~] frontend → `web/src/**` — no UI change; job digest renders the new kind generically
- [ ] testing `testing-strategy`
- [~] security `security-review` — no auth, access or infrastructure change; owner-gated paths untouched

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

1. [ ] [backend] Run /implement, then migration 0059: `orphaned_at` (+ index) on studios and tags — `internal/db/migrations/`
2. [ ] [backend] Generalize `personorphan` → `orphansweep` (kind `orphan-sweep`, per-kind `hasAuthoredData`, drop `entity_enrichment`) — `internal/personorphan/`
3. [ ] [backend] Stamp/clear on studio reconcile, tag detach, rescan, bare tag create, and pre-delete in `HardDelete` — `internal/repo/`
4. [ ] [testing] Fold the uncommitted repro into sweep tests per kind (pruned / authored survives / shared kept / purge stamps) — `internal/repo/studio_prune_repro_test.go`
5. [ ] [testing] Update `docs/testing-strategy.md` for the sweep — `docs/testing-strategy.md`
6. [ ] [—] After squash-merge: fill "Decided in" sha — `docs/architecture/entity-relationships.md`
7. [ ] [—] After merge: move HOLODEX-494 to Done (fixed by this epic)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-08 · reproduced the studio prune data loss; designed the shared orphan sweep
- skills: architecture, handoff
- handoff: Design settled (spec + architecture committed, owner chose the people orphan model for studios and tags, purge stamps all three); next is /implement, then Up next item 1.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
