---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-619                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Smart playlists can now filter by relative dates, other playlists, provider-enriched fields and your watch history, cap how many videos they hold, and have their rules edited right on the playlist page.
approved:                    # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
  design:                    # artifact is committed; THIS means the owner looked and said yes. Written
    on: 2026-11-02           # only by /implement (which asks) or /handoff (for a yes given this session);
    at: c8f301b1             # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-619 · Smart playlist rules (F89)

Done means a smart playlist can express relative dates, a result cap, references to other
playlists, resolved (provider-merged) fields and the owner's watch history, and its definition is
edited on the playlist page itself, all inside the SEC-1 to SEC-11 perimeter.

**Design package:** [spec F89](../specs/smart-playlist-rules.md) · architecture: [resolved_values index](../architecture/field-resolution.md), [watch-history.md](../architecture/watch-history.md) ·
[handoff](../design/smart-playlist-rules-handoff.md) · [security rules SEC-1–11](../architecture/security-perimeter.md) · testing-strategy: pending

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/**`
- [x] design `design-handoff` → `docs/design/**`
- [/] backend → `{cmd,internal,providers}/**` — slice 1 landed (added:, cap, playlist: refs, visibility); slices 3–4 to go
- [/] frontend → `web/src/**` — slice 1 chips and readout only; the on-page editor is slice 2
- [/] testing `testing-strategy` — strategy is §40 of `docs/testing-strategy.md`; its tests are written in the build
- [/] security `security-review` — design-stage rules SEC-1–11 committed; a review of the built diff is still due before merge

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

1. [ ] [frontend] Slice 2: the on-page definition editor with the Cap control (P0-4), replacing browse edit mode — `web/src/routes/playlists/[id]/+page.svelte`
2. [ ] [backend] Slice 3: `resolved_values` index, field conditions (P0-6), `released:` and `updated:` with the changed_at stamp from the drain — `internal/repo/`
3. [ ] [backend] Slice 4: plays, watch conditions and the watched mark (P0-7 to P0-9) — `docs/architecture/watch-history.md`
4. [ ] [testing] Slice 1 left for later: §40 Q6 fan-out/condition-count cases, Q8 depth-2+ transitive, A2 depth 1–4 — `internal/api/smart_playlist_rules_test.go`
5. [ ] [architecture] On the squash, pin both "Decided in: pending squash (HOLODEX-619)" lines — `docs/architecture/field-resolution.md`, `docs/architecture/watch-history.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-11-02 · architecture: resolved-value index, watch history; testing strategy §40
- skills: architecture, testing-strategy (own session), handoff, implement, code-review, security-review
- handoff: Slice 1 is in (`added:` on a new added_at column, the cap, `playlist:` references with loop/limit/visibility rules, visitors never see a definition); `updated:` and `released:` moved to slice 3 by owner ruling — next is slice 2, the on-page editor.

### 2026-11-01 · brainstorm, then spec, design and security in parallel sessions
- skills: write-spec, design-handoff (security review done by hand at design stage), handoff, architecture
- handoff: Spec F89, design handoff and SEC-1–11 are merged here and agree; the architecture gate is open on two forks (resolved-field querying, play-history storage) — start with `/architecture` on item 1.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
