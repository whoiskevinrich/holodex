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
- [ ] architecture `architecture` — ADR-104 (claim via `adr-claims.mjs`): D1 container-not-entity ·
  D2 membership + sort · D3 snapshot-only · D4 persistent player element · D5 visibility
- [ ] design `design-handoff` — `docs/design/video-playlists-handoff.md` + mockup SVG: `/playlists`,
  `/playlists/[id]`, *Save as playlist* placement (OQ3), *Add to playlist* picker, next-up surface
- [ ] backend — S1: migration (number claimed at implement time, 0051 on main 2026-09-20), repo,
  `/playlists*` handlers, `KindPlaylist` ref, `/capabilities.public_playlists`
- [ ] frontend — S2: pages + producers, three skins; S3: persistent-element refactor of
  `/media/[id]` (behaviour-neutral commit first, element-identity test), playlist context, Media
  Session next/prev
- [ ] testing `testing-strategy` — handler tests incl. visitor-private = 404, order parity with
  browse, element identity across next-up, three-skin matrix, manual PiP + Safari rows
- [ ] security `security-review` — new owner-gated writes; visibility as a read gate; `from_query`
  goes through the existing browse parser, never SQL text

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-438] `/architecture` → ADR-104; ADR index row
2. [ ] [HOLODEX-438] file stories S1/S2/S3 under the epic (parent field), then `/design-handoff`
3. [ ] [HOLODEX-438] manual Safari check of unmuted `play()` after `src` swap (spec OQ2) — recipe in
   the epic's spike comment; only P1-3's toggle changes if it fails
4. [ ] [HOLODEX-438] on merge sweep the epic + S1/S2/S3 to Done by hand (epic-keyed branch → CI
   fires nothing)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-20 · brainstorm → epic → spike → spec
- skills: product-brainstorming, write-spec
- handoff: brainstorm converged (container not entity; playlist = membership + sort; snapshot
  producers; not-a-tag guarantees), epic HOLODEX-438 filed with gates-as-checkboxes, branch renamed
  `HOLODEX-438-video-playlists`, In Progress fired. **Spike** (throwaway, reverted): the media page
  destroys its `<video>` per item (`{#if loading}` wraps the player) → next-up must be a `src` swap
  on a persistent element for PiP to survive; **shape A** chosen over a layout-level player (Kevin's
  PiP use = switch to another app). In-app Electron browser is autoplay-permissive — not evidence;
  Safari unverified. Spec F69 written and committed; five defaults confirmed by Kevin (random
  snapshot keeps the seen order as `manual`; private → 404; deep links never autoplay; trash hides
  and restore returns; duplicate names allowed). Next: ADR-104.
