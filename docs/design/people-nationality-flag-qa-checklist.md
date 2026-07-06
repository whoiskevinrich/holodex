# QA Checklist: Nationality flag on the person page (HOLODEX-139)

Work through this against a running app. Enrichment testbed: start `provider-tmdb` (:9100) then
`backend-films` (:7800), and the `web` dev server (:5173) — open **http://localhost:5173/**. Enrich a
few people from TMDB so `nationality` (place of birth) is populated (e.g. Arnold Schwarzenegger →
Austria, Alyssa Milano → USA; someone with no `place_of_birth` for the empty state).

Spec [`people-nationality-flag.md`](../specs/people-nationality-flag.md) · design handoff
[`people-nationality-flag-handoff.md`](people-nationality-flag-handoff.md) · provider contract §4.2.

Legend: **[smoke]** = quick programmatic check · **[agent]** = verified this session
(`preview_eval` / unit tests) · **[human]** = needs a human look.

---

## 1. Setup / smoke

1.1 **[smoke]** `npm --prefix web run test` passes, including `nationality.test.ts` (value→country
derivation, synonyms, demonyms, diacritics, dedupe, empty/unknown → null).
1.2 **[smoke]** `npm --prefix web run build` succeeds and emits the flag SVGs as files (271 under
`dist/`), **not** inlined into the person-page chunk (chunk stays ~70 KB, no `data:image/svg` in it).

## 2. Agent-verified (this session)

2.1 **[agent]** Person with a place of birth ending in the country (Arnold Schwarzenegger, "Thal,
Styria, Austria") shows the **Austria** flag; `alt`/`title` = "Austria".
2.2 **[agent]** Deep place-of-birth string (Alyssa Milano, "Brooklyn, New York City, New York, USA")
resolves the **last** segment → `us.svg`; `alt`/`title` = "United States".
2.3 **[agent]** Person with no `nationality` (no `place_of_birth`) shows **no flag element** and **no
layout gap** (the name row has a single child).
2.4 **[agent]** Flag chrome tracks each skin's tokens: `rounded-theme` = 2px / 0 / 0 and `border-rule`
= `#2a2622` / `#1a2240` / `#333333` in Cinémathèque / Broadcast / Brutalist; flag height 16px.
2.5 **[agent]** The flag SVG is served locally (`/…/flag-icons/flags/4x3/xx.svg`, `image/svg+xml`) —
no external/CDN request; loads offline.

## 3. Human eyeball — all three skins

3.1 **[human]** **Cinémathèque**: the flag reads cleanly beside the serif name, corner slightly
rounded, hairline visible; a long name ellipsizes before the flag is pushed off.
3.2 **[human]** **Broadcast**: flag is square (no radius); the hairline and flag read against the deep
blue surface; the `+N` (if any) is muted, not competing with the teal accent.
3.3 **[human]** **Brutalist**: flag is square; hairline reads on near-black; nothing collides with the
mono name or the video-count subline.
3.4 **[human]** Hovering the flag shows the country name tooltip; with multiple nationalities the
tooltip lists all and a muted **+N** follows the primary flag.
3.5 **[human]** Visitor (Admin Mode off) still sees the flag — it is not owner-gated.
3.6 **[human]** A person whose place of birth is a city/region **without** a country (rare) simply
shows no flag rather than a wrong one — acceptable degrade.
