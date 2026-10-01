---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-58
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: full                # new persisted data (stored query + migration), visitor evaluation of a public query, UI on six pages
depends-on: []
approved:
  design:
    on: 2026-09-30
    at: 2325626f
release_note: Any video grid — browse, a person, tag, studio or film — can now be played straight through or shuffled with one press, and saved as a smart playlist that picks up newly matching videos on its own.
---

# HOLODEX-58 · Smart playlists and Play all from any query-backed video grid

This is done when every video grid backed by a `/media` query (browse, person, tag, studio, film)
offers *Play all* / *Shuffle* through the F69 next-up player, and the owner can save that grid's
query as a smart playlist that re-runs on every open. It must never cap at 500, never show a visitor
more than `/media` would, and never quietly broaden a stored query when an entity is merged or
deleted. Three stories ship in order under epic HOLODEX-16: HOLODEX-501 (entity grids through
`/media`), HOLODEX-500 (Play all + Shuffle), then HOLODEX-58 (smart playlists).

**Design package:** [spec F75](../specs/smart-playlists.md) · [ADR-121](../architecture/ADR-121-smart-playlists-stored-query-and-runs.md) · [handoff](../design/smart-playlists-handoff.md) + [mockup](../design/smart-playlists-mockup.svg) · [testing-strategy §23](../testing-strategy.md#23-smart-playlists-play-all-and-shuffle-f75-holodex-58--500--501-adr-121)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. F75 `docs/specs/smart-playlists.md` (`6a10f681`, `a3057304`). RD1–RD13: a live query re-run on read, Play all fixes the id list at the press, merges follow and deletes flag, shuffle is a playback mode, repeat starts a new pass, always-shuffle is a playlist property.
- [x] architecture `architecture` → `docs/architecture/ADR-*`. ADR-121 (`0f52af0a`) supersedes ADR-104 D3's "no `frozen_query`" only. Its decisions: a canonical `/media` string with `query_version`, live evaluation under the reader's posture, owner-only inputs can't be public, merge rewrite inside the merge transaction, stale refs flag rather than broaden, client-held runs with a seeded Fisher–Yates.
- [x] design `design-handoff` → `docs/design/**`. `smart-playlists-handoff.md` + `smart-playlists-mockup.svg` (`2325626f`). One *Save as playlist…* with a Smart / Snapshot toggle, a `smart` chip, and Play all / Shuffle as one split button. Owner-approved 2026-09-30 in the design session; recorded at `/implement` on the owner's confirmation, pinned to `2325626f` (no design file has changed since).
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy`. §23 is the plan (`2fd45cf7`): ranked risks, Go/API/Vitest/harness/live-QA rows, 11 mutation checks, and 5 default decisions in §23.8 to confirm at build. It closes when the tests exist and pass.
- [/] security `security-review`. The design review (2026-09-30) found no vulnerability. D4 covers every owner-only `/media` input, and the 501 change exposes nothing new, since `q` is a bound phrase-quoted FTS match and redaction and full-film hiding hold. Two build notes are folded into ADR-121: visibility before the stale check, and one `mediaFilterFor` builder. The implementation review before merge is still due.

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

1. [ ] [frontend] HOLODEX-500: `mediaFilterFor` + `GET /media/ids`, the run module (seeded shuffle, toggle, repeat), split button — `web/src/lib/`, `internal/api/`
2. [ ] [testing] Confirm the five default decisions in testing-strategy §23.8 with the owner — `docs/testing-strategy.md`
3. [ ] [backend] HOLODEX-58: migration, `canonicalPlaylistQuery`, live read (visibility first), merge rewrite, Freeze — `internal/api/`, `internal/repo/`
4. [ ] [security] Implementation `/security-review` of the smart-playlist read path and `/media/ids` before merge
5. [ ] [—] On merge: sweep HOLODEX-500 and HOLODEX-501 to Done by hand (CI moves only the branch's own key); squash subject is `feat!` (owner decision 2026-09-30: the person/tag/studio detail payloads dropped `items`/`total`, a breaking API change)

## Session log — newest first (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-30 · session
- skills: write-spec, architecture, design-handoff, testing-strategy, handoff, implement
- note: worklog hand-written by `/handoff` at the owner's request, because the SessionStart hook never scaffolded it on this branch.
- handoff: Crossed into build (design signed off at 2325626f, draft PR #424). HOLODEX-501 is built and live-checked: the person, tag and studio grids page through `/media` with no cap, and the title box is server-side `q`. Start at Up next item 1: run `/security-review` on ADR-121 D3/D4 before building the smart-playlist read path. HOLODEX-500 (item 3) doesn't depend on it.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
