---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-601                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # frontend feature with a technology fork (virtualizer); backend/security judged below
                             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: "People and Studios now scroll as one list across the whole library in any sort or search, so you can drag straight to any point; a position readout shows where you are, and the A–Z index works in every People view."
                             # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
approved:
  design:
    on: 2026-10-28
    at: 0dd0071e
---

# HOLODEX-601 · People and Studios scroll the whole list

People (about 1,500 entries) and Studios page 50 at a time behind Load more, so reaching the middle
of the list in any sort but name takes a dozen clicks. This epic makes them one whole-list scroll:
the scrollbar spans the whole total from the first load, and only the entries in view are fetched,
under every sort and search, in all three People views. It reverses F80's RD1 (HOLODEX-354).
Films, the Media grid, Playlists, the Tags page and the owner queues keep Load more.

**Design package:** spec F80 `docs/specs/entity-list-paging.md` (EL6, EL7, EL9, EL11–EL13; RD1, RD2,
RD6–RD8) · architecture `docs/architecture/stack.md` ("Long lists: TanStack Virtual's core over a sparse page cache") · design handoff `docs/design/entity-list-virtual-scroll-handoff.md` + mockup SVG ·
testing strategy §32.1

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
- [~] backend → `{cmd,internal,providers}/**` — no server change: list endpoints already page by offset under every sort and seeded Random, and return `letters`. until: the build finds a missing parameter
- [/] frontend → `web/src/**` — built and QA'd; held on the owner comparing the built screen with the approved mockup (/handoff §3d)
- [x] testing `testing-strategy` — unit, browser-mode, §12.4 harness and §20.6 nav Back checks pass; `-big` live QA done; screen-reader pass owner-accepted 2026-10-28
- [~] security `security-review` — no auth, access or infrastructure change; reads the same public list endpoints. until: a new endpoint or parameter is added

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

1. [ ] [frontend] Owner compares the built People page (List mid-scroll: placeholders, retry row, chip, A–Z) with the approved mockup, then mark PR #567 ready — `docs/design/entity-list-virtual-scroll-mockup.svg`
2. [ ] [testing] Close §32.1's gaps: contract at 375/768/1440px and each poster density, Table aria, merge selection across a far scroll — `web/src/lib/components/shared/VirtualRows.svelte.test.ts`
3. [ ] [architecture] Replace "Decided in: pending squash" with the squash SHA after merge — `docs/architecture/stack.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-28 · build: whole-list scroll on People and Studios
- skills: implement, code-review, handoff
- handoff: main merged in (20b53201, clean; 878 web tests pass) and testing ticked on the owner's screen-reader acceptance. Marking #567 ready is held on two owner checks: compare the built People page mid-scroll with the approved mockup (/handoff §3d), and re-confirm the design sign-off, which reads stale only because the merge brought other epics' docs/design files.

### 2026-10-27 · design phase: spec, architecture, design and test plan for the whole-list scroll
- skills: code-review, handoff, implement
- handoff: Crossed into build: design signed off at 30d3b16e, and Draft PR #567 is open (main was already in). Start by pinning `@tanstack/virtual-core` and building the runes wrapper and sparse page cache beside `PagedList` in `web/src/lib/`.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
