---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-365
status: in-progress
release_note: The media page's Overview now reads the same for the owner as for a visitor — small muted prose, clamped with the expand chevron — with a pencil beside the heading that opens the source editor.
---

# HOLODEX-365 · Media detail Overview — owner view matches visitor typography

The owner's request: on the media detail page, the OVERVIEW block in Owner view should look the
same as in Visitor view — Visitor view is the correct one, and the collapsible chevron stays.

**The two views were two components.** A visitor got `ExpandableText` (`text-sm leading-relaxed
text-muted`, `line-clamp-5`, chevron). The owner got `SourceBadge`, whose at-rest value span is
base-size `text-ink` with no clamp and no chevron, then the inline chip row on click. The
typography gap *was* the component gap, so restyling `SourceBadge`'s span would have been the
wrong fix.

**The fix was already designed — as the named follow-up of HOLODEX-303.** The
[Person bio handoff](../design/person-detail-bio-header-handoff.md) adopted `SourceEditModal` as
"the standard interaction pattern for `long_text` tier-2 fields going forward", listed Video
`overview` as the only other such field, and put its adoption in Open Questions as "a natural
follow-up … flag as a candidate HOLODEX issue". `curation/CLAUDE.md` carries the same note. This
issue is that follow-up: both views render `ExpandableText`; the owner gets a pencil docked in the
`OVERVIEW` heading that opens `SourceEditModal`. `SourceBadge` leaves the Overview block (it stays
on every Metadata-list field).

**One thing the pattern gives up, knowingly.** `SourceBadge` rendered its own "file out of sync"
pill per field; `SourceEditModal` does not, so the Overview block no longer carries one. The Person
bio has the same gap, and the page-level `outOfSyncCount(resolved)` still counts overview — the
owner is still told, just not on the block. Not re-adding it here; if it turns out to matter it is
a `SourceEditModal` concern, not a media-page one.

## Gates — definition of done

- [~] spec `write-spec` — n/a: no new capability. Same field, same decision model, same values;
  only the owner's rendering and the control that opens the decision changed
- [~] architecture `architecture` — n/a: frontend-only, no seam touched
- [x] design `design-handoff` — covered by the existing
  [person-detail-bio-header-handoff.md](../design/person-detail-bio-header-handoff.md) +
  [mockup](../design/person-detail-bio-header-mockup.svg), which names Video overview as this
  follow-up. No new mockup: the heading-pencil + modal is that design applied to its second field
- [~] backend — n/a
- [x] frontend — `media/[id]/+page.svelte`: `h2` becomes a flex row with the owner-only pencil,
  `ExpandableText` is the single rendering, `SourceEditModal` mounted at the page root behind
  `overviewEditOpen`; the "candidate follow-up" notes in `SourceEditModal.svelte` and
  `curation/CLAUDE.md` flipped to match. Verified live on `backend-films` video 210 (447-char overview, file + TMDB):
  owner and visitor both resolve to 14px / 22.75px muted `line-clamp-5`; chevron expands to 6
  lines and collapses back; pencil opens the modal with File / TMDB (selected) / Custom; all three
  skins ≥ 4.9:1 on the prose, no horizontal overflow
- [~] testing `testing-strategy` — n/a: no logic added; the two components already carry their
  own coverage and `npm run test` (272) + `npm run check` (0 errors) pass
- [~] security `security-review` — n/a: no auth, access or infrastructure change; the pencil is
  gated on the same `isOwner && isReplaceField` term the badge was

## Up next — ordered (position = priority)

1. [ ] [—] **Mark the PR ready for review** once the owner has eyeballed the block — every gate
   is green. That is the act that moves this ticket to In Review.
2. [ ] [S] [HOLODEX-364](HOLODEX-364.md) (film page onto the media page's overview rule) now has a
   second thing to inherit: the film `description` is the other `long_text`-shaped field rendered
   as `ExpandableText` for visitors and `SourceBadge` for owners. Same fix applies.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-11 · built and verified in one pass
- skills: code-review (high --fix) — no findings
- the ask was typography; the answer was the documented component swap. Looked for the shortcut
  (restyle `SourceBadge`'s value span, or give it a prose snippet) and rejected it: the handoff
  had already ruled the chip row out for paragraph-length values, and a hybrid would have been a
  third pattern for two fields.
- pencil markup copied from the Person page's `pencilIcon` snippet rather than extracted — it is
  the second inline copy (`CurationChip`/`EntityImageSlot` carry the same path too); a shared icon
  is a reasonable follow-up but not this change's.
- handoff: implementation is complete and live-verified; the PR is Draft only pending the owner's
  eyeball, then mark ready.
