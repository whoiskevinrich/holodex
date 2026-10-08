# Spec: Clear a studio from a video or a film (F74)

**Status**: Draft
**Story**: HOLODEX-493 · spun out: HOLODEX-494 (prune-on-empty deletes a curated studio),
HOLODEX-495 (revert of an added tag), HOLODEX-496 (films get their own studio attachments)
**Owner**: Project owner
**Date**: 2026-09-29

**ADR**: [ADR-120 — the owner can clear a replace field](../architecture/archive/ADR-120-owner-cleared-field-decision.md).
A cleared field is a `manual` decision with an empty value, written only by an explicit `clear`.
Writeback deletes the tag. D6 covers films.
**Design handoff**: [`studio-detach-handoff.md`](../design/studio-detach-handoff.md), with a committed
SVG mockup. **Extends**: [ADR-087](../architecture/archive/ADR-087-film-studio-cascade-decide-and-writeback.md)'s
film-studio cascade, which gains a clear.

---

## Problem Statement

File parsing sometimes marks a value as a video's studio that isn't one: a filename token, a
publisher, or a mis-mapped tag. The owner can *replace* a studio on the Media page (`StudioPicker`)
and on the Film page (the cascade dialog), but can't **remove** one. The only escape is to pick
some other studio, which records a falsehood. `DELETE`-ing the decision reverts to the very file
value that was wrong. The mis-parsed studio then stays on the page, in the studio's video count, in
Studios-list filters, and in the file.

## Goals

1. **The owner can clear a video's studio in two clicks from the Media page**: the pencil, then the
   chip's `×`. The page shows no studio immediately.
2. **The owner can clear a studio from a film in two clicks**, affecting only the videos that carry
   that studio.
3. **The file follows the decision.** After writeback the file carries *no* studio tag, never a
   placeholder, and the field reads in sync.
4. **Nothing can clear a field by accident.** An empty Custom submit and an empty writeback entry
   are still refused. Only an explicit clear removes a value.
5. **One remove idiom across the three pickers.** People, the video's studio and the film's studio
   show attached items and remove them the same way.

## Non-Goals

- **Clearing other fields.** ADR-120 D2's allowlist starts at `{studio}`. Each further field needs
  its own per-container delete proof, and identity fields (`title`, `external_provider_id`) and
  images never qualify.
- **A film-level studio.** A film's studio stays the union of its videos' (HOLODEX-496 proposes a
  separate "released by" attachment). This spec clears *video* studios, from either page.
- **What happens to a studio after its last video is cleared.** That belongs to
  [studio-entity.md](studio-entity.md) RD1: the studio disappears from the list. It is deleted after
  30 days only if the owner never gave it an alias, image, decision or curation (HOLODEX-535,
  which also resolves HOLODEX-494). Clearing never deletes a studio at decision time.
- **Undo inside the writeback batch dialog.** Undo is re-picking the studio (R6). A batch revert
  for added tags is HOLODEX-495.
- **Bigger touch targets for the `×`.** It's the same ~20px as `PersonPicker`'s. Growing it has to
  happen in all three pickers at once, so it's a separate change.

## User Stories

**Owner, on a video (Media page)**
- As the owner, I want to remove a studio that file parsing got wrong, so that the video stops
  being credited to it.
- As the owner, I want to see which studio is linked when I open the studio picker, so that I know
  what I'm changing before I change it.
- As the owner, I want a cleared studio to stay cleared after a re-scan, so that the same wrong
  parse doesn't come back.
- As the owner, I want writing the video's metadata to remove the studio tag from the file, so that
  other players stop showing it too.
- As the owner, I want to undo a clear in one click, so that a slip costs nothing.

**Owner, on a film (Film page)**
- As the owner, I want to remove one studio from a film whose parts credit different studios, so
  that I fix the wrong parts without touching the right ones.
- As the owner, I want to see which parts were cleared, which collided and which failed, so that I
  know what landed in the files.

**Visitor**
- As a visitor, I see no studio on a video or film whose studio was cleared: no placeholder and no
  "none".

**Edge cases**
- As the owner, clearing a studio that would make the video a duplicate of another (same title,
  people and date, both with no studio) shows the same duplicate verdict as any studio change.
- As the owner, if the clear fails, the picker stays open with the error and nothing changes.

## Requirements

### P0: Must-have

**R1. The API records a cleared studio only on an explicit clear.** (ADR-120 D1, D2)
- `PUT /media/{id}/fields/studio/decision` accepts `{ "source": "manual", "clear": true }` and
  stores a standing `manual` decision with an empty value.
- It returns 400 for `clear` with any other source, `clear` with a non-empty `manual_value`, and
  `clear` on any field not in the allowlist (`{studio}`).
- A `manual` decision with an empty value and **no** `clear` is still a 400 (unchanged).

*Acceptance:*
- [ ] Given a video whose file says `Acme`, when the owner PUTs a studio clear, then the detail
      response has `studios: []`, and the studio field is still present with `decision: {source:
      "manual", standing: true}`, empty `values` and `in_sync: false`.
- [ ] Each of the three 400 cases above returns 400 and changes nothing.
- [ ] `PUT` with `source: manual, manual_value: ""` and no `clear` still returns 400.
- [ ] A re-scan of the video leaves the studio cleared.

**R2. The studio link follows the decision immediately.** (ADR-120 D3)
- A clear removes the video's `video_studios` links at decision time, through the existing relink.
  Writeback isn't involved.

*Acceptance:*
- [ ] Right after the clear, the studio's own page no longer lists the video, and the Studios list
      count drops by one.
- [ ] `DELETE …/fields/studio/decision` afterwards restores the file's studio and its link.

**R3. The duplicate check still runs.** (ADR-120 D5)
- A clear that would collide on the composite key returns the same `{conflict}` as any studio
  change. `StudioPicker` shows `CollisionOfferCard`, and "Save anyway" resubmits the clear with
  `override`.

*Acceptance:*
- [ ] Given two videos with the same title, people and date, where one has no studio, clearing the
      other's studio returns a conflict and stores nothing until "Save anyway".

**R4. Media page: the Linked now chip.** (handoff §Layout, §States)
- With a studio linked, the Change studio modal shows "Linked now" and the studio as an attached
  chip (`PersonPicker`'s markup) with a `×`, ruled off from the source chips.
- `×` commits the clear. While it runs the whole modal is inert and the `×` shows `…`.
- On success the modal closes, the page shows `+ Add studio`, and focus lands on it.
- On error the modal stays open with the message in `text-warn`.
- With no studio linked, the modal is titled "Add studio" and has no Linked now section.

*Acceptance:*
- [ ] The chip is the same markup as a `PersonPicker` attached chip (a diff of the two shows only
      the label and the `aria-label` text).
- [ ] The `×` has `aria-label="Remove {name} from this video"` and is reachable with Shift+Tab from
      the search input.
- [ ] After a clear, keyboard focus is on `+ Add studio`, not on `body`.
- [ ] No "None", "No studio" or empty Custom chip appears in any state.

**R5. Film page: clear only the videos carrying that studio.** (ADR-120 D6; owner decision
2026-09-29)
- `FilmStudioCascadeDialog` gets the same Linked now section: one chip per studio in the film's
  union.
- A chip's `×` runs the cascade as a **clear scoped to that studio**. Only the film's videos
  currently linked to that studio are cleared, and videos carrying another studio are untouched.
- The cascade request names the studio. `POST /films/{id}/studio/cascade` with `clear: true`
  requires a `studio_id`, and the handler walks `VideoIDsForFilm` ∩ the videos linked to that
  studio. `studio_id` without `clear` is a 400.
- Each cleared video is enqueued as a tag **delete** (`JobField{Clear: true}`), never an
  empty-values job. The dialog then shows its existing results step (Enqueued / Collision / Error)
  and hands off to `WritebackBatchDialog`.
- There's no confirm step, matching the cascade's existing change behaviour, which also writes
  immediately.

*Acceptance:*
- [ ] Given a film with parts 1–2 on `Acme` and part 3 on `Beta`, when the owner clears `Acme`,
      then parts 1–2 get a cleared decision and a tag-delete job, and part 3's decision, link and
      file are unchanged.
- [ ] The results step lists exactly parts 1–2. None is reported `enqueued` without a delete job
      behind it.
- [ ] After the batch writes, parts 1–2's files carry no studio tag, and the film shows `Beta`
      alone.
- [ ] A part that collides is listed under Collision and keeps its studio.

**R6. Undo is one action on either page.**
- Media: reopen and pick the `·file` chip (re-decides `source=file`), or `DELETE` the decision.
- Film: pick the studio again in the cascade dialog. That sets it on every video, as the cascade
  does today.

*Acceptance:*
- [ ] On the Media page, clear then pick `·file` restores the studio card and link, and the field
      reads in sync.

**R7. Writeback deletes the tag, and only when asked.** (ADR-120 D4)
- The writeback HTTP entry accepts `{ "field": "studio", "clear": true }` with no `values`, only
  when the video's standing studio decision is cleared. Otherwise it's a 400, as is any other
  empty entry.
- A clear deletes **every file tag `studio` reads from** (`Publisher`, `Label`, `Studio`,
  `ProductionCompany`, per the live mapping, each passing the strict `ValidClearTagName`
  check that never admits `all` or a wildcard), not just the
  `Publisher` write target. Otherwise a leftover `Label` would resolve the mis-parse again
  (ADR-120 D4).
- A cleared row carries its `write_target`, the tag that will be deleted, so the cockpit offers it.
  A blank pin to an empty layer stays unwritable, as ADR-113 requires (ADR-120 D4).
- The writeback cockpit shows a cleared row as applied `—` against the file's value and sends
  `clear`.
- Read-back afterwards makes the field `in_sync: true`.

*Acceptance:*
- [ ] Given a file carrying the studio under `Label` (not `Publisher`), a cleared studio written
      from the cockpit leaves neither tag, and read-back reports `in_sync: true`.
- [ ] A cleared studio's detail row has a non-empty `write_target`, while a blank pin's row still
      has none.
- [ ] `clear` for a video whose studio decision isn't cleared returns 400 and writes nothing.
- [ ] A cleared studio written from the cockpit leaves no studio tag, on every container `studio`
      has a `write_target` for in `metadata-mappings.yaml.example`, through each write path that
      container uses (exiftool; mkvpropedit and its ffmpeg fallback for MKV), proven under
      `make test-image`.
- [ ] No file ever receives a literal placeholder value.

**R8. The anti-divergence rule holds.**
- `web/src/lib/components/entity/CLAUDE.md` "Relationship pickers" is the contract: the attached
  chip at the top, `×` is the remove, a studio picker labels it "Linked now". What `×` commits may
  differ between pickers; how it looks may not.

*Acceptance:*
- [ ] The three pickers render attached chips from one shared snippet or component, or else a
      review confirms identical class strings.

### P1: Nice-to-have

- **P1-1. Shared attached-chip snippet.** Extract the chip that R8 requires three copies of into
  one component, so a later target-size change lands once.
- **P1-2. Studio count in the film results step.** Label the results with the studio cleared
  ("Cleared Acme from 2 parts"), not only per-video rows.

### P2: Future considerations (design for, don't build)

- **More clearable fields.** Keep the allowlist a single set in `decisions.go`, and keep the
  per-container delete test parameterised by field, so adding `collection` is a one-line change
  plus proof.
- **Film-level studio (HOLODEX-496).** Scope the film clear by `studio_id` so that a later "Released
  by" attachment can sit beside the derived "Sourced from" chips without changing this API.

## Success Metrics

This is a single-owner library, so success is measured on the owner's own data and in tests, not
by adoption rates.

- **Leading (at merge):** every R1–R7 acceptance item is green, including the per-container delete
  tests under `make test-image`.
- **Leading (first week on `:edge`):** every mis-parsed studio the owner knows about is cleared
  without a stand-in studio. Checked by a prod query: `SELECT COUNT(*) FROM field_source_decisions
  WHERE field_key='studio' AND source='manual' AND manual_value=''` matches the owner's count.
- **Lagging (one month):** no "placeholder" studios exist (studios created only to overwrite a bad
  parse). The owner reviews the Studios list and finds none.
- **Guardrail:** zero writeback jobs report `enqueued` for a studio clear without a tag delete in
  their args (job_runs detail review).

## Open Questions

None. Resolved on 2026-09-29:

- **Mixed-studio film:** clear only the matching videos (R5).
- **Stale film dialog (owner-approved):** `studio_id` scoping reads `video_studios` at request time.
  If a video's link changes between the dialog opening and the `×`, the cascade clears the videos
  linked *at click time*. The results step lists exactly what was cleared, so a stale dialog can't
  silently miss a video. There's no version check.

## Timeline Considerations

- **Dependencies:** none external. `make test-image` needs the trixie image from ADR-117 (merged).
- **Phasing:** one PR. Backend first (R1–R3, R7, then the R5 cascade), then the frontend (R4, R5
  dialog, cockpit). The media path is usable without the film path, but they share the writeback
  `Clear` plumbing, so splitting would ship a half-proven delete.
- **Gates:** testing-strategy section, then `/security-review` (a new owner mutation that deletes
  file tags), then `/implement`.
