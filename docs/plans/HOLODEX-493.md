---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-493
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: full                # a new decision shape + file-tag deletes (data model, owner mutation) and UI on two pages
depends-on: []
approved:
  design:
    on: 2026-09-29
    at: 49437119
release_note: You can now clear a wrongly parsed studio from a video or a film. It disappears in Holodex at once, and writing the file removes it from the file's tags.
---

# HOLODEX-493 · Owner can clear a studio from a video or a film

This is done when the owner can remove a mis-parsed studio from the Change studio modal on the Media
page, and from the Film page's studio dialog, where it clears only the videos carrying that studio.
The clear must hold across re-scans and remove the studio from every file tag it was read from, so
nothing is ever written in its place.

**Design package:** [spec F74](../specs/studio-clear.md) · [ADR-120](../architecture/ADR-120-owner-cleared-field-decision.md) · [handoff](../design/studio-detach-handoff.md) · [testing-strategy §22](../testing-strategy.md#22-clearing-a-studio-from-a-video-or-a-film-f74-holodex-493-adr-120)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. F74 `docs/specs/studio-clear.md`. The owner decided that a mixed-studio film clears only the matching videos, and approved the behaviour for a stale film dialog.
- [x] architecture `architecture` → `docs/architecture/ADR-*`. ADR-120, owner-reviewed: a cleared field is `manual` with an empty value via an explicit `clear`, not a `none` source. Films go through the ADR-087 cascade (D6). D4 deletes every tag the field reads from, with names checked by a strict `ValidClearTagName`.
- [x] design `design-handoff` → `docs/design/**`. Owner picked A1 (PersonPicker's attached chip) over the logo-row and "No studio" variants. Handoff and `studio-detach-mockup.svg` cover the Media and Film surfaces, and the anti-divergence rule is in `entity/CLAUDE.md` and `ui-vocabulary.md`.
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy`. The §22 plan is written. The tests themselves land with the build.
- [/] security `security-review`. The design-branch review found no vulnerabilities and produced D4's strict tag validator. The implementation PR needs its own review against D4.

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

1. [ ] [backend] `ValidClearTagName` + table test (§22.2); `JobField.Clear` → deletes in `buildBatch` — `internal/writeback/tagkeys.go`, `internal/writequeue/writequeue.go`
2. [ ] [backend] Decision PUT `clear` + `{studio}` allowlist; `markWriteTargets` cleared-row exception; HTTP writeback `clear` — `internal/api/decisions.go`, `internal/api/writeback.go`
3. [ ] [backend] Film cascade `clear` + `studio_id` scoping, building `Clear` jobs — `internal/api/film_studio_cascade.go`
4. [ ] [testing] Per-container clear proof under `make test-image` (§22.4) — `internal/writeback/studio_clear_integration_test.go`
5. [ ] [frontend] `isCleared`, `sourceChips`, cockpit `clear`, `api.setFieldDecision` — `web/src/lib/f36.ts`, `web/src/lib/writebackCockpit.ts`, `web/src/lib/api.ts`
6. [ ] [frontend] Linked now chip in `StudioPicker` and `FilmStudioCascadeDialog` — `web/src/lib/components/entity/StudioPicker.svelte`
7. [ ] [security] `/security-review` on the implementation diff, focused on D4

## Session log — newest first (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-29 · session
- skills: design-critique, design-handoff, architecture, write-spec, testing-strategy, security-review, handoff, implement
- handoff: Crossed into build. Design was signed off at 49437119 (the merge of main, which resolved two append-only table conflicts with the owner's OK), and the draft PR is open. Spun out HOLODEX-494/495/496 and flightplan#46. Start at Up next item 1: `ValidClearTagName` and the `Clear` job.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
