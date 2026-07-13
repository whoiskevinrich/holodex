# QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)

Work through this against a running app. Enrichment testbed: start `provider-tmdb` (:9100) then
`backend-films` (:7800), and the `web` dev server (:5173) — open **http://localhost:5173/**. Needs: a
video with a resolved `release_date` and at least one cast member with a resolved `birthdate` (e.g.
Dune 1984, `tmdb:841`, already enriched per the HOLODEX-173 unblock comment); a cast member with **no**
birthdate on that same video; and a second video with **no** resolved `release_date` (but `recorded_at`
populated) for the negative-state check.

Spec [`age-in-media.md`](../specs/age-in-media.md) · design handoff
[`age-in-media-handoff.md`](age-in-media-handoff.md).

Legend: **[smoke]** = quick programmatic check · **[agent]** = verified this session
(`preview_eval` / unit tests) · **[human]** = needs a human look.

---

## 1. Setup / smoke

1.1 **[smoke]** `go test ./...` passes, including the new `ageInMedia` unit cases: happy path;
missing/unparseable `birthdate`; missing/unparseable `release_date`; `birthdate`-after-`release_date`
guard; leap-day boundary (mirrors F45's existing `deriveAge` coverage); a case with `deathdate` set but
irrelevant to the result.
1.2 **[smoke]** `npm --prefix web run check` passes with the new `ageInMedia` prop on `PersonPoster`
and the video-detail-scoped credit shape carrying `age_in_media`.

## 2. Agent-verified (this session)

2.1 **[agent]** Video with resolved `release_date = 1984-12-14` and a cast member with resolved
`birthdate = 1959-02-11` shows a badge reading `25`, bottom-right on that poster.
2.2 **[agent]** A cast member with no resolved `birthdate` on that same (otherwise-computable) video
shows **no badge and no layout gap**; the other cast members' badges are unaffected.
2.3 **[agent]** A video with no resolved `release_date` (but `recorded_at` populated) shows **zero**
badges across its entire cast grid — confirms no `recorded_at` fallback (AC2/AC3).
2.4 **[agent]** A deceased cast member (`deathdate` set) still shows a normal age badge, computed the
same as a living cast member — unaffected by `deathdate` (AC6).
2.5 **[agent]** A person with a standing field-source override on `birthdate` shows the age computed
from the **resolved** (winning) value, not the raw file baseline (AC7) — matches what that person's own
detail page shows for Age.

## 3. Human eyeball — all three skins

3.1 **[human]** **Cinémathèque**: badge corner is 2px rounded, legible against both the `--surface-2`
placeholder (no-photo state) and a real, busy poster photo.
3.2 **[human]** **Broadcast**: badge is square (0 radius); no collision with the CRT scanline flourish
(`.portrait-frame::after`).
3.3 **[human]** **Brutalist**: badge is square; reads cleanly on near-black; no collision with the mono
name caption below the poster.
3.4 **[human]** Visitor (Admin Mode off) sees badges identically to owner — no gating.
3.5 **[human]** At the widest grid density (`md:grid-cols-6`, desktop), both a 1-digit and a 2–3-digit
age fit the badge pill without wrapping or overflowing.
3.6 **[human]** With a screen reader or accessibility inspector, the badge announces "Age N at time of
release" (its `aria-label`), not a bare number floating unexplained over the image.
