---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-328                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Improvement — the media detail page's Films and People sections now collapse to a plain "add" link when empty, use matching poster sizes when both are filled, and show a film's scene number as an editable pill on the poster, matching the film page's scenes grid.
---

# HOLODEX-328 · Media detail Films + People: collapse empty sections, port the Scenes pill

UX fix for the Films + People row on the media detail page. The row has four link states
(neither / people-only / film-only / both) and three of them read badly: an empty Films section
renders a heading plus a dashed poster box while an empty People section renders a bare text link
(two affordances for the same nothing); an empty section still claims a column beside populated
content; Films tiles are a fixed `w-20` while People tiles are a responsive `grid-cols-3/4/6`, so
the two poster rows are different sizes when both are populated; and the scene badge is a
below-poster block that makes film chips taller than person chips and diverges from the Film detail
page's Scenes grid, where the same value is a corner pill on the thumbnail.

One rule fixes the first three: **a section renders its heading and tiles only when it has content;
an empty side degrades to a bare `+ Add …` text CTA; sections stack vertically unless both are
populated, in which case they sit side by side at equal tile size.** The fourth is a straight port
of `VideoCard.svelte`'s scene pill onto the Films chips.

**Design package:** `docs/design/media-detail-films-people-handoff.md` + committed SVG mockup
(`media-detail-films-people-mockup.svg`, four states × owner/visitor). No new architecture — layout
and affordance only; no field, namespace, or decision-model seam is touched.

## Gates — definition of done

- [~] spec `write-spec` — not applicable; no behavior or requirement change, layout and affordance only
- [~] architecture `architecture` — not applicable; no data-model or seam change
- [x] design `design-handoff` — `docs/design/media-detail-films-people-handoff.md` + `media-detail-films-people-mockup.svg`
- [ ] frontend
- [ ] testing `testing-strategy`
- [~] security `security-review` — not applicable; no auth, access, or infrastructure change
- [ ] three-skin QA — Cinémathèque / Broadcast / Brutalist, owner **and** visitor

## Up next — ordered (position = priority)

1. [x] [design] Handoff doc + committed SVG mockup, Jira story filed against epic HOLODEX-12, branch renamed, In Progress fired — `docs/design/`
2. [ ] [frontend] Collapse rule: empty Films → text CTA (drop the heading + dashed box), state-1 CTAs inline, stack-unless-both-populated — `web/src/routes/media/[id]/+page.svelte`
3. [ ] [frontend] Shared fixed tile width: `PeopleGrid`'s `grid-cols-3/4/6` → `flex flex-wrap` at the Films tile size (watch the Film page's Cast section, same component) — `web/src/lib/components/entity/PeopleGrid.svelte`
4. [ ] [frontend] Scene pill port: overlay at `right-1.5 top-1.5` with `#N` / `—` / `Full`, `svelte:element` button-vs-span exactly as `VideoCard` does it; move the film chip's remove `×` to top-left — `web/src/routes/media/[id]/+page.svelte`
5. [ ] [testing] Coverage aligned to the handoff's state matrix, including the visitor-blank case
6. [ ] [—] `/simplify`, live three-skin QA (owner + visitor), push, open PR, sync Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-06 · Design settled from a hand sketch; handoff + mockup committed
- skills: design-handoff, graphify
- handoff: started from Kevin's hand-drawn four-panel sketch of the Films/People row. Read the
  current implementation first (`+page.svelte:1283-1385`, `PeopleGrid.svelte`, `PersonPicker.svelte`,
  `VideoCard.svelte`, `app.css` skin tokens) and rendered a today-vs-proposed mockup across all four
  link states, which is what surfaced that problem 1 is Films-side only — `PeopleGrid` already
  collapses to a text CTA when empty and Films never got the same treatment. Two choices were put to
  Kevin as cards and answered: tile parity via **one fixed shared width** (People loses its
  responsive grid; accepted that this resizes the Film page's Cast section too, since both use
  `PeopleGrid`), and state 1's two CTAs **inline on one row** rather than stacked as the sketch drew
  them. Kevin then asked for the scene badge to adopt the Film detail page's Scenes mechanism —
  ported `VideoCard.svelte:99-122` verbatim in spirit: corner pill overlaid on the poster, `#N`
  accent / `—` dim / `Full` dim, owner gets a `<button>` with a hover ring, visitor gets a
  `pointer-events-none` span so clicks fall through to the film link. That created a real collision
  the sketch didn't anticipate — the film chip's remove `×` already owns `right-1.5 top-1.5` — so
  remove moves to top-**left** on Films tiles only, leaving People's at top-right; noted in the
  handoff as an accepted asymmetry rather than papered over. Also noted the sketch omitted any add
  affordance in the populated states and kept the trailing dashed `+` tile, since otherwise a video
  with one person attached can never gain a second. Third open question (full-film attachments have
  no scene number, and Scenes' vocabulary only says `#N` or `—`) resolved to a dim, inert `Full`
  pill. Filed HOLODEX-328 under epic HOLODEX-12 (Video detail page) with the gate checkboxes in the
  description, renamed the branch off its auto-generated `claude/…` name, fired In Progress. Found
  HOLODEX-296 (extract a shared poster-tile component for Films + People chips) already open and
  overlapping — cross-linked both ways; this handoff specifies the sizing contract that extraction
  should satisfy but does not require it to land first. Mockup verified rendering in the browser
  pane (dim pills read dark against the light monogram plate; accent pills read `#4`). Next: the
  three frontend steps above, in order.

### 2026-09-06 · HOLODEX-329: film chips never showed the film's poster
- skills: graphify
- handoff: Kevin reported the media detail Films chips always drawing the monogram. Not the
  frontend oversight it looks like — `repo.FilmAttachment`, the struct the `/media/{id}` response
  serializes, carried no image field at all, so the chip had nothing to render. (Deliberately
  different from HOLODEX-318, the films *index* bug, where `Film.poster_url` is in the payload and
  simply goes unread — that one is frontend-only.) Fixed across four files: `FilmAttachment` gains
  `PosterVersion` (`json:"-"`) + `PosterURL`; `FilmsForVideos` fills the version through a new
  `attachFilmAttachmentPosters` helper doing ONE batched `filmImageVersions` query over the
  distinct film ids — the same read the film list/detail paths use, so a chip and the film's own
  page can never disagree about which image wins when a film holds both an uploaded and a
  provider-sourced poster; `setFilmAttachmentPosterURLs` builds the URL in `api/film_images.go`
  beside the existing `setFilmImageURLs`, keeping URL shapes out of the repo. Had to close the
  `FilmsForVideos` rows cursor explicitly before the second query rather than lean on the deferred
  Close. Frontend renders an `<img>` when `poster_url` is set, monogram otherwise, `object-cover`
  to match the People chips beside it. New `TestFilmsForVideoPosterVersion` covers no-image,
  provider-only, and upload-beats-provider, and pins that the repo leaves `PosterURL` empty.
  Live-verified on `backend-films` (which needed `FILMS_ENABLED=true` adding to this worktree's
  gitignored `.claude/launch.json` — the films API was 404ing): uploaded a poster, confirmed
  `poster_url` in `/api/v1/media/8`, confirmed the chip draws the real 1000×1500 image at 80×120
  `object-fit: cover` in all three skins with radius following each skin's token. Filed as
  HOLODEX-329 and shipped on this branch rather than its own, since it edits the same chip markup
  the redesign rewrites. `go build`, `go test ./internal/repo ./internal/api`, and
  `npm run check` (0 errors) all pass.
