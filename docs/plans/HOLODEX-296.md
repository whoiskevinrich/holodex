---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-296
status: in-review
release_note: ~
---

# HOLODEX-296 · Extract shared poster-tile component for Films + People chips

Flagged by the `/simplify` pass on the Films+People row reorder (HOLODEX-297): the Media detail
page's Films row and `PeopleGrid`'s People row each carried their own copy of the same
`curation-chip` `<li>` — fixed `w-20`, linked poster + two-line caption, hover-reveal remove
badge as a sibling of the `<a>`. The two rows sit side by side and must stay the same tile
(HOLODEX-328), so any size/hover change had to land twice.

**History:** the first attempt, PR #276, was built on the closed #275 and never rebased after
#277 (the reorder) and #308 (Films/People rework) landed; by 2026-09-19 it was `CONFLICTING`
and carried ~24K lines of `graphify-out/` churn. Closed as superseded; this is the redo on a
clean branch off `main`.

**Shape:** `entity/PosterTile.svelte` — `href`, `name`, a required `poster` snippet (image box
only; the caption comes from `name`), `onRemove?` + `busy` (owner vs. read-only follows the
`TagLinkChip` rule: pass `onRemove` or don't), `removeSide` (`'right'` default; Films passes
`'left'` because its scene pill owns the top-right corner) and `children` as the extra-overlay
slot the scene pill renders into. No visible or behavioral change.

## Gates — definition of done

- [~] spec — n/a: pure refactor, no behavior change
- [~] architecture — n/a
- [~] design — n/a: pixel-identical tiles; no new surface
- [~] backend — n/a
- [x] frontend — `PosterTile.svelte` new; `PeopleGrid.svelte` + `media/[id]/+page.svelte` Films
  row render through it; `entity/CLAUDE.md` row added, `PeopleGrid` row refreshed
- [x] testing — `npm run check` 0 errors (warnings all pre-existing, none in touched files);
  no unit tests cover these tiles (none did before); live parity QA below
- [~] security — n/a: presentational only
- [x] three-skin QA — 2026-09-19 on backend-films (fresh DB, video 6 seeded with 2 films +
  3 curated people via the API). Owner: Films remove docks top-left (`left: 6px`) beside the
  `#3`/`Full` pill, People remove top-right (`right: 6px`), both hidden at rest (opacity 0),
  same tile width (80 px), same remove bg/border tokens across Cinémathèque / Broadcast /
  Brutalist; scene pill still opens "Edit scene number"; person remove removes the tile; no
  interactive control nested in the `<a>`. Visitor: 12 tiles, 0 remove buttons, pills degrade
  to `<span>`, no Attach film / Add person CTA.
- [x] `/code-review high --fix` — no findings

## Up next

- Mark #<PR> ready once Kevin has had a look → CI moves HOLODEX-296 to In Review; Done on merge.

## Session log

- 2026-09-19 — Closed superseded PR #276; redid the extraction fresh on `HOLODEX-296-poster-tile`
  off current main. Skills: `/code-review high --fix`. Handoff: code + docs complete and
  browser-QA'd; PR open, waiting on Kevin's look before `gh pr ready`.
