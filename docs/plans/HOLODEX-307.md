---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-307                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The film detail page's poster now shows and edits in the header itself; the separate Images section (and the unused thumb role) is gone.
---

# HOLODEX-307 · Film detail: poster becomes the header image; remove the Images section

The header poster box (left of the film's title/studio/tags) now shows the film's real
poster and is the owner's upload/replace/remove control, in place of a separate "Images"
section below the header. The `thumb` role — which the old Images section also managed but
which never had a UI consumer — is dropped from the frontend. Done means: `npm run check`
clean, `npm run test` clean, live-verified across all 3 skins in both owner and visitor view.

**Design package:** no new spec/ADR — reuses ADR-085/ADR-086's existing `film_images`
poster/thumb roles verbatim (frontend-only change, backend untouched); no dedicated design
handoff — mechanical relocation of an existing upload affordance, same posture HOLODEX-280
itself took ("Images section is mechanical reuse of Studio's existing pattern") · testing-strategy:
["Film detail header poster; Images section removed"](../testing-strategy.md)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → not needed, no new behavior/requirement (mechanical UI relocation)
- [x] architecture `architecture` → not needed, reuses ADR-085/ADR-086 verbatim
- [x] design `design-handoff` → not needed, mechanical reuse (see design package note above)
- [x] frontend
- [x] testing `testing-strategy` → row added, see link above
- [~] security `security-review` → not touched (no auth/access/infra change) — deferred, not applicable

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [frontend] `EntityImageSlot.svelte` `variant="frame"` hero mode — `web/src/lib/components/entity/EntityImageSlot.svelte`
2. [x] [frontend] wire the film header to the new frame variant, remove the Images section — `web/src/routes/films/[id]/+page.svelte`
3. [ ] [—] converge the pencil-overlay pattern (`NameEditControl`, Person hero's `editBtn`, this frame variant) into one shared component → not filed as a separate issue yet, noted in this ticket's description

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-04 · full implementation + live verification
- skills: simplify
- handoff: HOLODEX-307 is implementation-complete and verified live (owner upload affordance,
  visitor monogram fallback, all 3 skins) — ready to push and open a PR. `/simplify` (4 parallel
  review agents) applied fixes: dropped an unused `monogramClass` override prop, named the two
  repeated tri-boolean conditions as `showMonogram`/`dashedEmptyPoster` derived values, and
  removed the now-dead `Film.thumb_url`/`FilmImageRole` `"thumb"` frontend surface. One finding
  deliberately deferred: the new frame-variant overlay buttons duplicate the Person hero's
  `editBtn` pattern rather than converging on one shared component — fixing that would touch the
  currently-working Person hero, out of scope here. `/security-review` not run — no auth/access/
  infra touched.
