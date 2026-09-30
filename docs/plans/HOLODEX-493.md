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
- [x] backend → `{cmd,internal,providers}/**`. `7ea487b` + `b7e9eb7`. Decision PUT `clear` + `{studio}` allowlist, cascade `clear` + `studio_id`, HTTP writeback `clear` against a stored cleared decision, a `Clear` job expanding to bare-name deletes held to `ValidClearTagName` in the worker, `markWriteTargets` cleared-row exception, first-non-empty snapshot. D4 was amended during the build: deletes are bare, because a probe showed an `XMP:Label` survives a `QuickTime:`-qualified delete.
- [x] frontend → `web/src/**`. `422b3dc`. The Linked now chip in `StudioPicker` and `FilmStudioCascadeDialog`, through one new `AttachedChip` also adopted by `PersonPicker`. The cockpit writes a clear via `writeEntry`, and after a clear the lone `·file` chip shows as the undo. Live QA passed in Cinémathèque on a scratch copy of the stress fixture. The owner compared the build with the approved mockup and confirmed it matches (2026-09-29).
- [x] testing `testing-strategy`. §22, as built. Go unit, API and queue tests all green; six of six mutations caught. Real-file clears pass inside the built image on MP4/exiftool, MKV/mkvpropedit and MKV/ffmpeg, XMP included. Vitest: 519 pass. Standing gaps (mp3/flac fixtures, end-to-end revert) are recorded in §22.4.
- [x] security `security-review`. The design review produced D4's strict validator. The implementation review found no vulnerabilities: every route is owner-gated, no request value reaches an exec argument or picks a tag, and a clear requires a stored cleared decision on an allowlisted field.

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

1. [ ] [—] Merge PR #420 once CI is green (squash, Conventional subject); jira-sync then moves HOLODEX-493 to Done
2. [ ] [frontend] Spec P1-2, optional: name the studio in the film results step ("Cleared Acme from 2 parts") — `web/src/lib/components/film/FilmStudioCascadeDialog.svelte`

## Session log — newest first (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-29 · session
- skills: design-critique, design-handoff, architecture, write-spec, testing-strategy, security-review, handoff, implement, code-review
- handoff: Built and settled. Design signed off at 49437119, the owner confirmed the build matches the mockup, and all seven gates are closed. The backend, bare-delete fix and frontend (7ea487b, b7e9eb7, 422b3dc) are pushed, with the real-file clear proven in the image and the implementation security review clean. PR #420 is marked ready. Spun out HOLODEX-494/495/496 and flightplan#46. Start at Up next item 1: merge #420 once CI is green.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
