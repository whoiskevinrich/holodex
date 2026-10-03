# ADR-122: The linked film decides a video's Film and Title by default

**Status:** Proposed. Supersedes **ADR-085 §4's "a film candidate never auto-wins"** clause only.
ADR-085 §1–3 and §5–7 stand, as does the rest of §4 (synthetic `film:<id>` namespace, the
`resolveDecided` film branch, the candidate chips).
**Date:** 2026-10-02
**Deciders:** Project owner
**Spec:** [F56 films-entity.md](../specs/films-entity.md) · **Jira:** HOLODEX-513 (relates HOLODEX-402)

## Context

ADR-085 §4 makes an attached film a resolver candidate for the video's `collection` (label
"Film", from the `Album` tag) and, for a full-film link, for `title`. It deliberately keeps the
film out of the undecided precedence walk: the `Album` tag wins until the owner pins
`provider:film:<id>` on each video. Two consequences:

- Attaching a video to a film changes nothing the owner can see on the media page's Film row.
  They have to make one extra decision per video.
- Edits to the film entity don't reach the video unless that decision exists. That covers a
  rename and the F60 RD9 display-name decision. The injected candidate also read the canonical
  `films.name` column, so a display-name decision never reached the video even when the
  decision did exist.

The owner asked that the linked Film entity decide the video's Film link: updates to the film
should flow to the media page.

## Decision

**D1: a sole linked film beats the file by default.** In `resolveField`, when a replace field has
no standing decision and exactly one `film:<id>` namespace offers a non-empty value for it, that
namespace goes ahead of the ordered sources (`soleFilmNamespace`). It rides the existing
`resolvePrecedence` walk. So a manual add still wins, a suppressed value is skipped, and the
winning source reads `film:<id>:<canonical>`, exactly as when decided. The implicit marker is
`provider:film:<id>` (non-standing).

**D2: two or more films stay ambiguous.** With several attached films offering a value, nothing
auto-wins. The record-first default stands, and the owner picks a chip. This matches the F56 user
story "when a video is attached to two films … I resolve it explicitly".

**D3: standing decisions still win.** A `file`, `manual` or other provider decision is
untouched. To keep the `Album` tag over a linked film, the owner pins `file`.

**D4: the film's display spelling is the injected value.** The media-detail read overlays each
attachment's `FilmName` with the film's decided display spelling (`repo.AttachFilmDisplayNames`,
which reads `DisplayNames(film, "title")`, the same SQL mirror search uses). The chip, the
`collection`/`title` candidates and the default winner then all read what the film page shows.
The canonical column stays the identity truth.

**D5: an undecided film winner reports its real sync state.** An undecided field is normally in
sync by construction. When a film wins by default, `InSync` compares its value to the file's tag
(nil when the field declares no file source), so the writeback cockpit flags it.

**D6: detach refreshes.** The media page re-fetches after a detach, so the Film/Title rows fall
back to the file immediately.

## Options considered

- **Keep record-first and only fix propagation (display name, refresh).** This is the minimal
  change, but it leaves "attach a film" visibly inert. The owner chose against it.
- **Write a `provider:film:<id>` decision at attach time.** This produces the same visible result,
  but a standing decision would outlive a detach and would need cleanup. It is state where a
  read-time rule suffices.
- **First-attached or lowest-id film wins when there are several.** Rejected because it is an
  arbitrary pick on genuinely ambiguous input (D2).

## Consequences

- Attaching a scene changes its resolved Film row, and a full-film link changes its Title.
  Neither touches the file until the owner writes back. ADR-085's asserted-link invariant is
  unchanged.
- Only the media-detail read injects film sources, so list/browse titles (`BrowseTitle`) are
  unaffected, as before. *Amended 2026-10-02 (HOLODEX-514):* list tiles now inject too
  (`applyBrowseTitles`, one batched `FilmsForVideos` lookup per page with display names), and so
  does the film picker's "Also in:". RD6 hides full-film videos from browse and the entity grids,
  so in practice a film-derived title shows on playlist tiles. Completeness, search, title sort,
  MCP and the film page's scene list still read the file title (HOLODEX-515).
  *Amended 2026-10-02 (HOLODEX-515):* completeness now scores with the same film sources.
  Migration 0058 adds the film's spelling inputs to ADR-099 D4's trigger set (`films.name`, the
  film's `name` decision, and its provider `title`), each dirtying every linked video, and
  re-dirties film-linked videos on upgrade. Search, title sort, MCP and the scene list still
  read the file title. They need a stored resolved title and their own ADR (HOLODEX-516).
- `TestResolveUndecided_FilmSourceNeverAutoWins` is replaced by tests covering the sole-film win,
  in-sync and out-of-sync, several films, a file decision, a manual add and detach. An API test
  covers rename, display-name decision and detach end to end.
