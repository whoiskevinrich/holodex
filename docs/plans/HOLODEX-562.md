---
key: HOLODEX-562
status: in-progress
profile: full
depends-on: []
approved:
  design:
    on: 2026-10-11
    at: 35330eb
release_note: "You can now delete a film from its page. Move it to Trash to hide it (restore it any time before the grace period ends), or delete it permanently. Its videos stay in your library."
---

# HOLODEX-562 · Owner can delete a film from the Film Detail page

Relates to HOLODEX-279 (F56). Films had no way to be removed. F78 gives the film page the media
page's Manage split button (Move to Trash / Delete permanently…), with films joining media in the
one Trash. Owner's picks (2026-10-11): full parity with media; linked videos are only unlinked;
permanent delete removes the film's image files; exit is back-or-Films-list; a trashed film is
hidden but intact and keeps its name (refused + offered a restore); one Trash list with a kind
label; the media grace period; a trashed film withdraws its title/album candidate; a purged film's
standing source choice falls through and is cleared.

Design package: [spec](../specs/delete-film.md) · [handoff](../design/delete-film-handoff.md) ·
[mockup](../design/delete-film-mockup.svg).

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `docs/specs/delete-film.md` (F78), RD1–RD10, P0-1–P0-9
- [~] architecture — no technology fork (films ride the existing `deleted_at` axis and the purge ticker in media-ingest.md)
- [x] design `design-handoff` → `docs/design/**` — `delete-film-handoff.md` + `delete-film-mockup.svg`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review` — new owner-gated destructive endpoints

## Up next — ordered (position = priority)

1. [ ] [backend] Migration: `films.deleted_at` + index; film soft-delete / restore / purge (image dir), trash listing of both kinds
2. [ ] [backend] Film visibility seam on every film read surface; trashed-film name hold on create/rename/year; resolver candidate withdrawn while trashed; stale decision cleared at purge
3. [ ] [frontend] Manage split button + dialogs on the film page; Trash kind label; in-Trash verdict in rename/year/attach-create
4. [ ] [testing] Tests per P0 + `docs/testing-strategy.md`
5. [ ] [security] `/security-review`, then mark the PR ready (sweep HOLODEX-562 → In Review)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-11 · spec, design, crossing into build
- skills: design-handoff, implement
- handoff: Spec F78 and the design handoff + SVG are pushed; owner approved the mockup and both recommended answers in chat. Start at Up next 1.
