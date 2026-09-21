---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-438
status: in-progress
release_note: Video playlists — save the browse view you're looking at as a playlist, or add a video to one from its page; playlists are private until you make them public, and play through item to item (Picture-in-Picture included).
---

# HOLODEX-438 · F69 Video playlists — container, membership + sort, two producers, next-up

A playlist is an owner-made, Holodex-only container of videos with a **sort** — a first-class page,
**not** a spine entity (no BaselineSource / decisions / aliases / completeness / enrichment). Two
producers in v1: *Save as playlist* on browse (server-side **snapshot** of the whole filter result)
and *Add to playlist* on a video's page. Private by default, per-playlist `visibility`. Next-up rides
a **persistent `<video>` element** lifted above the media page's loading gate so PiP survives the
hand-off (Plex behaviour). Film playlists out of scope. Brainstormed + spiked 2026-09-20.
Spec: [`docs/specs/video-playlists.md`](../specs/video-playlists.md).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/video-playlists.md` (F69), RD1–RD9 locked; OQ1 resolved (hide
  the visitor nav item via `/capabilities.public_playlists`)
- [x] architecture `architecture` — [ADR-104](../architecture/ADR-104-video-playlists-container-and-persistent-player.md)
  D1–D5; index row landed; D3 adds the uncapped `ListVideoIDs` over the existing `build()`/`orderBy()`
- [x] design `design-handoff` — `docs/design/video-playlists-handoff.md` + mockup SVG: `/playlists`,
  `/playlists/[id]`, *Save as playlist* placement (OQ3 → A), *Add to playlist* picker, next-up surface;
  approved 2026-09-20
- [x] backend — S1 (HOLODEX-441): migration `0051_playlists`, `repo/playlists.go`, `/playlists*`
  handlers, `KindPlaylist` ref, `/capabilities.public_playlists`, `playlists` on the media detail;
  handler + snapshot-parity tests
- [/] frontend — **S3 DONE** (HOLODEX-443): persistent `<video>` on `/media/[id]` (f1491ae, own
  behaviour-neutral commit + `playerElement.test.ts`), `?playlist=` context, `NextUpStrip`, `ended` →
  next with an in-memory intent, Media Session next/prev, `n`/`p` hotkeys; three skins + 375px
  checked. **S2 open** (HOLODEX-442): pages + producers, three skins
- [ ] testing `testing-strategy` — handler tests incl. visitor-private = 404, order parity with
  browse, element identity across next-up, three-skin matrix, manual PiP + Safari rows
- [/] security `security-review` — **S1 signed off 2026-09-20** (no findings): every mutation inside
  `requireOwner`; reads visibility-filtered at the repo (`ListPlaylists`/`GetPlaylist`/
  `PlaylistsForVideo`), visitor + private = the unknown-id 404; `from_query` → `url.ParseQuery` →
  `videoFilterFromQuery` → bound args only, ORDER BY from the `orderBy()` whitelist; items pass the
  browse visitor redaction. Re-run at epic level once S2/S3 land.

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-442] S2 `/playlists`, `/playlists/[id]`, *Save as playlist* (A), *Add to playlist*
   picker, nav item gated on `public_playlists` — three skins
2. [ ] [HOLODEX-438] `testing-strategy` rows + epic-level `security-review` once S2 lands
3. [ ] [HOLODEX-438] manual Safari check of unmuted `play()` after `src` swap (spec OQ2) — recipe in
   the epic's spike comment; only P1-3's toggle changes if it fails
4. [ ] [HOLODEX-438] on merge sweep the epic + S1/S2/S3 to Done by hand (epic-keyed branch → CI
   fires nothing)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-20 · brainstorm → epic → spike → spec
- skills: product-brainstorming, write-spec, architecture, design-handoff, code-review, security-review
- handoff: brainstorm converged (container not entity; playlist = membership + sort; snapshot
  producers; not-a-tag guarantees), epic HOLODEX-438 filed with gates-as-checkboxes, branch renamed
  `HOLODEX-438-video-playlists`, In Progress fired. **Spike** (throwaway, reverted): the media page
  destroys its `<video>` per item (`{#if loading}` wraps the player) → next-up must be a `src` swap
  on a persistent element for PiP to survive; **shape A** chosen over a layout-level player (Kevin's
  PiP use = switch to another app). In-app Electron browser is autoplay-permissive — not evidence;
  Safari unverified. Spec F69 written and committed; five defaults confirmed by Kevin (random
  snapshot keeps the seen order as `manual`; private → 404; deep links never autoplay; trash hides
  and restore returns; duplicate names allowed). **ADR-104** written (D1–D5, index row; D3 found
  `ListVideos` caps at `maxListLimit` → snapshot needs an uncapped `ListVideoIDs` off an extracted
  clause builder). Draft PR #375 open. Stories **HOLODEX-441/442/443** (S1/S2/S3) filed under the epic. **Design
  handoff** written + mockup SVG committed; Kevin approved all panels, OQ3 → A. Next: S1.

### 2026-09-20 · S1 store + API (HOLODEX-441)
- skills: code-review (high --fix), security-review
- handoff: **S1 shipped** — migration 0051 (`playlists`, `playlist_videos` PK `(playlist_id, video_id)`,
  cascade both ways), `repo/playlists.go` (visibility + trash seam in one place, chunked snapshot
  insert in the create tx), `api/playlists.go` (`GET /playlists[/{id}]` visibility-filtered;
  owner-gated POST/PATCH/DELETE + `PUT|DELETE …/videos/{videoId}`), `playlist:<id>` ref,
  `/capabilities.public_playlists`, `playlists` on `GET /media/{id}`. `ListVideoIDs` needed **no**
  builder extraction — `build()`/`orderBy()` were already factored (ADR-104/spec corrected). Code
  review's one finding (ValidSort ↔ orderBy drift) closed with a pinning test; security review
  clean. HOLODEX-441 stays In Progress until the epic PR merges (hand sweep). Next: S3.

### 2026-09-20 · S3 next-up (HOLODEX-443)
- skills: code-review (high --fix)
- handoff: **S3 shipped in two commits.** (1) f1491ae behaviour-neutral: the `<video>` renders
  outside every `{#if}` on `/media/[id]` — the three gated regions carry their own gates, codec
  failure is an overlay, the box is `hidden` (never unmounted) for fresh-load / not-found;
  `playerElement.test.ts` pins depth-zero (old structure measures 2); live: same node across
  134 → 130. (2) `$lib/playlistContext` (param parse, neighbours, hrefs, per-(id, seed) cache,
  play intent keyed on the target id), `NextUpStrip`, page wiring (`ended` → next + intent, `play()`
  once when `video.id === route id`, Media Session), `n`/`p` via F62's `use:hotkey`. Live-verified:
  `ended` hand-off on the same element with `src` swapped and playing; reload never autoplays;
  last item stops; stale link → *Play from start*; unknown/private → param inert; three skins;
  375px no overflow. Code review found two real bugs, both fixed and re-verified: the echoed
  random seed was dropped from hrefs (Prev reshuffled) and a playlist switch showed the old
  context while the new one loaded (the fix first looped an effect on its own write — `untrack`).
  In-app browser is autoplay-permissive: PiP-survives and Safari rows stay manual. Next: S2.
