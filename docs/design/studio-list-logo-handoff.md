# Design handoff: `/studios` list rows draw the studio logo

**Status:** Proposed (option C recommended, 2026-09-19) — awaiting the owner's pick
**Story:** [HOLODEX-432](https://whoiskevinrich.atlassian.net/browse/HOLODEX-432) (child of F51, HOLODEX-246)
**Owner:** Project owner
**Date:** 2026-09-19
**Spec:** none — presentation-only change to an existing page (see "Why no spec/ADR")
**ADR:** none — ADR-079's image roles, storage, and serving are untouched
**Branch/PR:** `HOLODEX-432-studio-list-logos`
**Supersedes:** [studio-images-handoff.md §1](studio-images-handoff.md) ("`/studios` list — logo
well data source change only": the well reads `icon_url`) and the "`/studios` list well stays
`icon_url` → monogram" bullet of
[studio-logo-link-card-handoff.md §10](studio-logo-link-card-handoff.md)

## Overview

The Studios index (`web/src/routes/studios/+page.svelte`) renders each studio as a one-line row
— a 40×26 `bg-logo-plate` well, the name, an owner-only completeness ring, and the video count —
in a 1 / 2 / 3-column grid with an A–Z jump-nav. F51 (HOLODEX-247) pointed that well at
**`icon_url`**, reserving the `logo` role for the detail pages. Two facts make that the wrong
call today, the same two that moved `StudioLinkCard` to the logo (HOLODEX-397/411):

1. **TMDB enrichment only ever fills the `logo` role.** `icon_url` is populated only by an owner
   upload, so an enriched studio shows its monogram in the list while its logo sits in storage.
2. **`logo_url` is already on every list row.** `internal/api/studios.go` `listStudios` calls
   `setStudioImageURLs` per item; `Studio.logo_url` is on the `GET /studios` payload today. The
   page just never reads it.

This handoff changes the list well to draw the logo when there is one, with the HOLODEX-411
rules already proven on `StudioLinkCard` — **bare** (no plate) and **aspect-following** — but
with one list-specific layout decision the card never had to make: the row width is fixed and
the name column has to stay scannable for the A–Z nav.

**Why no spec/ADR:** no new capability, endpoint, field, or schema — the logo is stored, served,
and on the payload. Per the change-routing table this is `/design-handoff` +
`/testing-strategy` only. Frontend-only; `internal/` is untouched.

![Option C rows across all three skins (wordmark, square logo, long name + ring, icon on plate, monogram, hover), plus the rejected A and B strips](studio-list-logo-mockup.svg)

---

## 1. Decisions

Presented as inline mockups on 2026-09-19 at the `lg` column width (300px rows, ~140px of name
column after the slot, ring and count), with the stressed row — "Meridian Entertainment Group
International" + a wide wordmark + the ring + a count — in every option.

| Question | Decision (recommended) | Why |
|---|---|---|
| Slot geometry | **C — fixed-width slot, `w-24 h-8` (96×32), logo bare and centred, name always shown.** *Rejected A — free-width slot (`h-8 w-auto max-w-24`), name always shown. Rejected B — the `StudioLinkCard` rule verbatim (`h-12 max-w-48`, a ≥2:1 wordmark replaces the name).* | The F38 handoff chose the fixed 40×26 plate so "rows stay aligned"; the list is an alphabetised column and the eye scans the name edge. **A** lets that edge jag by up to 56px between a 32px square mark, a 96px wordmark and the 40px plate. **B** grows rows 46→68px and leaves a wordmark row with no readable name, so the A–Z jump and a nav-search match can land on a row the user can't read — the "never unnamed" objection that already ruled out logo-only on the card. **C** keeps every name on one vertical line (as today), costs 6px of row height, and a 5:1 wordmark still renders 96×19. |
| Precedence per row | `logo_url` → `icon_url` → monogram | Same order as `StudioLinkCard` (HOLODEX-397). The list stops being the one surface that ignores the role TMDB actually fills. |
| Plate | **Logo: none** — bare on `bg-surface`. **Icon / monogram: today's 40×26 `bg-logo-plate` plate, unchanged, centred inside the 96×32 slot.** | HOLODEX-411: a transparent brand mark on a cream box read as a floating badge on the dark skins. The plate stays where it earns its keep — under an arbitrary square icon or the monogram — and stays its current size so legacy rows look exactly as they do today rather than gaining a 96px cream rectangle. |
| Caption | **Name always shown** — no wordmark/aspect rule in the list. | The name is the list's primary key (sorting, jump-nav, in-place nav-search filter). The wordmark echo that HOLODEX-411 removed on the card is tolerable here because the slot is a third of the card's size and the name is doing navigational work, not captioning. |
| Slot height | `h-8` (32px), not the card's `h-12` (48px) | Rows are `py-2.5`; 32px keeps the row at 52px (was 46px) and the 3-column grid dense. 48px would make `/studios` taller than `/people` (whose `PersonAvatar size="sm"` is 32px — this matches it). |
| Slot width | `w-24` (96px) | 3:1 box. A 2:1 wordmark fills the height; a 5:1 renders 96×19; a square mark 32×32. Wider (128px) starts to eat the name column at the `lg` width where ~140px is left after ring + count; narrower (64px) makes a 5:1 wordmark 13px tall. |

## 2. Component change

Edit in place in `web/src/routes/studios/+page.svelte` — the well is ~15 lines of inline markup
and this page is its only consumer; do not extract a component for one call site (project
simplicity rule; `StudioLinkCard` has a different shape — 48px, caption rule — and is not the
same thing).

### Before

```svelte
<span class="flex h-[26px] w-10 shrink-0 items-center justify-center overflow-hidden rounded-theme bg-logo-plate">
	{#if s.icon_url}
		<img src={s.icon_url} alt={`${s.name} icon`} class="h-full w-full object-contain p-0.5" />
	{:else}
		<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">{monogram(s.name)}</span>
	{/if}
</span>
<span class="flex-1 truncate">{s.name}</span>
```

### After

```svelte
<!-- Leading image slot (HOLODEX-432): a fixed 96×32 box keeps the name column on one
     vertical line across logo / icon / monogram rows. A logo (the role TMDB fills) draws
     bare, contained, centred (HOLODEX-411 — no plate under a brand mark). An icon or the
     monogram keeps the 40×26 plate from HOLODEX-126, centred in the same slot. -->
<span class="flex h-8 w-24 shrink-0 items-center justify-center overflow-hidden">
	{#if s.logo_url}
		<img src={s.logo_url} alt="" class="h-full w-auto max-w-full object-contain" loading="lazy" />
	{:else}
		<span class="flex h-[26px] w-10 items-center justify-center overflow-hidden rounded-theme bg-logo-plate">
			{#if s.icon_url}
				<img src={s.icon_url} alt="" class="h-full w-full object-contain p-0.5" loading="lazy" />
			{:else}
				<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">{monogram(s.name)}</span>
			{/if}
		</span>
	{/if}
</span>
<span class="flex-1 truncate">{s.name}</span>
```

Notes for the implementer:

- **`alt=""` on both images.** The name is the adjacent text in the same `<a>`; today's
  `alt="{name} icon"` makes screen readers announce the name twice. Decorative, per
  `StudioLinkCard`'s caption-present state.
- **`loading="lazy"`** — the list can be hundreds of rows, each now a real image request; the
  People list eager-loads only the first six avatars (`eager={i < 6}`). Same idea: `loading={i
  < 6 ? 'eager' : 'lazy'}` if you want the first screen to paint without the lazy hop, otherwise
  plain `lazy` is fine.
- **No `p-1` inset on the bare logo** (the card has one). At 32px tall, 4px of padding is 25% of
  the height; the slot's own 96px width already leaves air beside anything narrower than 3:1.
- **No `onload` / natural-size logic.** There is no caption rule here, so nothing needs the
  image's natural aspect — `object-contain` inside the fixed box does all the fitting.
- **The `<a>` is unchanged**: `flex items-center gap-3 rounded-theme border border-rule
  bg-surface px-4 py-2.5 text-ink hover:border-accent`. Row height becomes 52px from the slot's
  `h-8`, not from padding.
- Update the F38 comment block above the well (it still says "~40×26 plate keeps rows aligned
  whether or not the studio has an icon") to the one in the snippet.
- **Stale-comment sweep:** `web/src/lib/types.ts` `Studio.icon_url` says "icon (studios list
  well)" and `logo_url` says "logo (detail page header)"; `internal/model/model.go` has the same
  two comments on `IconURL` / `LogoURL`. Both become "logo → StudioLinkCard + the /studios list
  (HOLODEX-432); icon → fallback on both when there is no logo". Comment-only Go change is
  fine — no behaviour moves.

## 3. Call sites — unchanged

`StudioLinkCard` (Film / Media detail), `EntityImageSlot` (studio detail Images section), the
studio picker, and the nav-search Studios tab are untouched. `poster_url` still has no
consumer.

## 4. Backend — unchanged

`GET /studios` already returns `logo_url` per row (`setStudioImageURLs`, `attachStudioImages`
batch query over `studio_images`). No new field, no new query, no migration. The only `internal/`
edit is the two comment lines in §2.

## 5. Design tokens used

| Token / utility | Usage |
|---|---|
| `bg-surface`, `border-rule`, `hover:border-accent`, `rounded-theme` | Row (unchanged) |
| `text-ink`, `text-muted` | Name, count (unchanged) |
| `bg-logo-plate`, `text-logo-plate-ink` | Icon / monogram plate only — never under a logo |
| `font-display` | Monogram glyph (unchanged) |
| `h-8 w-24` | Slot — spacing-scale utilities, no arbitrary values beyond the existing `h-[26px]` plate |

No literal colours, radii, or fonts (`.claude/rules/frontend-theming.md`). The slot itself has
no background and no border in any skin, so nothing new to theme.

## 6. States

| Row | Slot renders | Name |
|---|---|---|
| `logo_url` set | Logo, bare, `object-contain`, centred in 96×32 | Shown |
| `logo_url` **and** `icon_url` set | Logo (precedence) | Shown |
| `icon_url` only | 40×26 plate with the icon (`p-0.5`), centred in the slot | Shown |
| Neither | 40×26 plate with the monogram | Shown |
| Logo fails to load | Browser's broken-image glyph inside the 96×32 box, `alt=""` so no alt text renders; name beside it | Shown |
| Hover / focus | Row border → `border-accent` (unchanged); no slot change | — |
| Loading (page) | The existing "Loading…" line; rows appear all at once (unchanged) | — |
| Image loading (row) | Empty 96×32 box, name already in place — no layout shift because the slot is fixed-size | Shown |
| Selecting mode | n/a — studios have no merge-selection mode | — |

A failed logo load is not caught (no `onerror` fallback to the plate): `StudioLinkCard` doesn't
fall back either, the slot is fixed-size so nothing shifts, and a `?v=` URL served by our own API
fails only when the row is stale — the next reload heals it.

## 7. Responsive behaviour

| Breakpoint | Row | Name column (after slot + `gap-3`, ring, count) |
|---|---|---|
| `< sm` (1 col, 375px page) | ~343px wide | ~190px |
| `sm–lg` (2 cols) | ~300–360px | ~140–200px |
| `≥ lg` (3 cols) | ~300px | ~140px |

The slot is `shrink-0`; the name is `flex-1 truncate`. The row never wraps, and the slot never
shrinks below 96px, so the smallest name column is the `lg` case above (~140px ≈ 18 characters
at 14px), the same as today minus 56px. `document.documentElement.scrollWidth ===
clientWidth` at 375px (the HOLODEX-356 guard).

## 8. Edge cases

- **Very wide wordmark (12:1, e.g. a 1200×100 PNG):** rendered 96×8 — a smear. Accept; the name
  is beside it and the studio page shows the logo at size. `CheckRoleAspect` (HOLODEX-386)
  guards nothing about the `logo` role, so this can happen, and the fixed slot bounds it.
- **Portrait logo (2:3):** rendered 21×32, centred; the slot's empty sides are the cost of
  alignment. Accept.
- **Dark-on-transparent mark on a dark skin:** reads faint (HOLODEX-411's known cost). The name
  beside it is the label. Do not add a halo or a plate.
- **Long names:** `truncate` as today; ~18 characters at `lg`. Nothing new.
- **Hundreds of rows:** every row is now an `<img>` request (today only icon rows are). `loading="lazy"`
  keeps the initial burst to the first viewport; each image is a small PNG served with a `?v=`
  cache key by our own API. Watch `read_network_requests` on the stress fixture (HOLODEX-342).
- **Random sort / completeness sort / missing-facet filter:** slot markup is per-row and
  order-independent; nothing to do.
- **A–Z anchors:** `id="sl-X"` stays on the `<li>`; `scroll-mt-16` stays. Row height 52px still
  clears the sticky letter nav.

## 9. Accessibility

- One `<a>` per row, unchanged tab order.
- Both `<img>`s are `alt=""` (decorative — the name is the link's text). The monogram keeps
  `aria-hidden="true"`.
- Accessible name of each row link = the studio name (+ the count as today). Unchanged from
  before except the duplicate "{name} icon" announcement goes away.
- No colour-only information: the slot carries no state.

## 10. Out of scope (deliberately) and follow-ups

- **Studio page hero** still does not draw the logo —
  [HOLODEX-399](https://whoiskevinrich.atlassian.net/browse/HOLODEX-399), unchanged.
- **A logo-tile / poster-style grid for `/studios`** (like the People Poster View). The F38
  handoff deferred it "until logos are common"; logos are now common for enriched studios. File
  as its own story if wanted — it is a view mode, not a row change.
- **`poster_url`** still has no consumer.
- **`StudioLinkCard`** is not touched and does not adopt the fixed slot; the two surfaces have
  different jobs (caption vs. index).

## 11. QA checklist

Setup: a studio with a **wide raster logo** (TMDB-enriched or upload a ~5:1 PNG), one with a
**square logo**, one with **icon only** (upload an icon, no logo), one with **neither**, one with
**both**, and one with a **name > 30 characters**. Owner logged in with Admin mode on so the
completeness ring renders.

**Smoke**
- 11.1 `[smoke]` `cd web && npm run check` clean; `rg 'zinc-|sky-|rounded-(lg|md|sm|xl)|#[0-9a-f]{3,6}' web/src/routes/studios/+page.svelte` empty (the pre-existing `h-[26px]` is the only arbitrary value).
- 11.2 `[smoke]` `cd web && npm run test` — `studios/+page` tests (if any) still pass; add a render test asserting the three slot states (§6 rows 1, 3, 4) by `img[src]` suffix / monogram text.

**Agent (driven browser, `getBoundingClientRect` + computed styles — no screenshots, per the
three-skin QA reference)**
- 11.3 `[agent]` Wide-logo row on `/studios`: slot box is 96×32, `background-color` `rgba(0, 0, 0, 0)`, `border-width: 0px`; `<img src>` ends in `/images/logo?v=`, `alt === ''`, computed `object-fit: contain`, rendered height ≤ 32 and width ≤ 96 with the natural aspect preserved.
- 11.4 `[agent]` Both-logo-and-icon row: `<img src>` ends in `/images/logo?v=` — logo wins.
- 11.5 `[agent]` Icon-only row: outer slot 96×32 transparent; inner plate 40×26 with `background-color` = the skin's `--logo-plate`; `<img src>` ends in `/images/icon?v=`, `alt === ''`.
- 11.6 `[agent]` Neither: inner plate 40×26, no `<img>`, monogram text present with `aria-hidden="true"`.
- 11.7 `[agent]` Alignment: for every row in one grid column, the name `<span>`'s `getBoundingClientRect().left` is identical (±0.5px) across logo, icon and monogram rows.
- 11.8 `[agent]` Row height: every `<a>` is 52px tall (was 46).
- 11.9 `[agent]` Viewport 375px: `document.documentElement.scrollWidth === clientWidth`; the > 30-char name is truncated (`scrollWidth > clientWidth` on the name span) and the count is still visible.
- 11.10 `[agent]` Sort by name, click the letter of the wide-logo studio in the jump-nav: `document.getElementById('sl-X')` scrolls into view and its row's name text is non-empty.
- 11.11 `[agent]` Repeat 11.3, 11.5, 11.6 under `data-theme` = `cinematheque`, `broadcast`, `brutalist`: logo slot transparent in every skin; plate `background-color` equals that skin's `--logo-plate`; plate `border-radius` 2px / 0 / 0.
- 11.12 `[agent]` Network: on the stress fixture, `read_network_requests` filtered to `/images/logo` — requests are issued only for rows near the viewport on first paint (lazy), not for every row.

**Human**
- 11.13 `[human]` Open `/studios` in each skin. Every enriched studio should now show its logo sitting directly on the row — no cream box behind it — with the name beside it, and every name should start on the same vertical line down each column, exactly as the monogram rows do. Rows should look only slightly taller than before.
- 11.14 `[human]` A studio with an owner-uploaded icon but no logo should look **exactly** as it does on `main` (same small cream plate, same icon), just shifted right by ~28px to centre in the wider slot.
- 11.15 `[human]` Narrow to phone width: rows are one column, logos the same size, no sideways scroll.
