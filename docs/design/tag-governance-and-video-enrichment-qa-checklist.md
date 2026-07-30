# QA Checklist: Tag Governance & Video Enrichment (F50)

Companion to [tag-governance-and-video-enrichment-handoff.md](tag-governance-and-video-enrichment-handoff.md).
Verifier tags: `[smoke]` automatable/scriptable, `[agent]` an agent can drive via the browser
tools, `[human]` needs a person's eyes (contrast, "does this look right" judgment calls).

## Setup

- **0.1** `[human]` Open a video's detail page as owner (Admin mode on) and as visitor
  (Admin mode off) in two tabs — most checks below compare the two.
- **0.2** `[human]` Have at least one tag with a `provider:tmdb` source, one `file`-source tag,
  and one `manual`-source tag on the same test video (seed via enrichment apply + manual add)
  so the provenance-suffix states are all visible at once.

## Smoke

- **1.1** `[smoke]` Denying an already-denied term (case-insensitive dup) is rejected or
  no-ops cleanly — no duplicate row in the deny-list.
- **1.2** `[smoke]` Setting a tag's parent to one of its own descendants is rejected
  server-side (ADR-075 D1(b)) regardless of what the UI allows the owner to select.

## Agent-drivable

- **2.1** `[agent]` Visitor view: media-page tags render exactly as today — plain link chips,
  no remove control, no "+ Add tag" trigger. (Regression guard: owner-only branch must not leak.)
- **2.2** `[agent]` Owner view, video with zero tags: the Tags section still renders (not
  hidden) and shows the "+ Add tag" trigger.
- **2.3** `[agent]` Owner view: click a chip's remove `×` — chip disappears, no full-page
  reload, tag detaches from just this video (not deleted globally — check `/tags` still lists it
  if other videos use it).
- **2.4** `[agent]` Add-tag input: typing an exact denied term (case-insensitive, e.g. `GNOME`
  when `gnome` is denied) and submitting shows the inline "`'{term}' is on the deny-list.`"
  message — input is not silently cleared.
- **2.5** `[agent]` Add-tag input: typing a name that near-matches an existing tag surfaces the
  near-miss card with "Use existing" / "Add as new anyway" — same copy/behavior as `/tags`'s
  own near-miss card.
- **2.6** `[agent]` `/owner` tab row shows a **Deny-list** tab; navigating to it as a
  non-owner redirects home (same gate every other `/owner/*` page has).
- **2.7** `[agent]` Deny-list page: add a term, confirm it appears in the list; remove it,
  confirm it disappears; empty state copy shows when the list is empty.
- **2.8** `[agent]` `/tags` "Manage tags" pill ⋯ menu shows **Set parent…**; after setting one,
  the menu shows **Parent: {name}** + **Clear**, and re-opening shows **Change parent…** instead
  of **Set parent…**.
- **2.9** `[agent]` `/tags/{id}` for a tag with ancestors shows the `Ancestor › Ancestor ›`
  breadcrumb above the tag name; a root tag (no parent) shows no breadcrumb line at all.

## Human

Navigate to a video with mixed-provenance tags (see Setup 0.2), toggle Admin/owner mode on, and
switch skins via the header picker (Cinémathèque → Broadcast → Brutalist) for each of the
following:

- **3.1** `[human]` The `·tmdb` provenance suffix reads clearly in the skin's accent color and
  doesn't collide visually with the chip's remove `×` — check Broadcast and Brutalist
  specifically, both 0px-radius skins where a busy chip can look cramped.
- **3.2** `[human]` The manual tag (no suffix) doesn't look "broken" or like something failed to
  load next to its suffixed siblings — it should read as intentionally plain.
- **3.3** `[human]` The remove `×` is invisible until hover/focus on a chip, then appears
  without shifting layout (no jump/reflow of neighboring chips).
- **3.4** `[human]` On the Deny-list page, the `border-warn` **Deny** button is legibly distinct
  from the neutral **Remove** buttons in every skin (not just by color — check it still reads at
  a glance in Brutalist's high-contrast/low-saturation palette).
- **3.5** `[human]` The near-miss card and denied-term rejection message are both readable and
  don't overflow their containers with a long tag name (test with a 40+ character tag name).
- **3.6** `[human]` Fonts and layout hold with a deep hierarchy chain in the `/tags/{id}`
  breadcrumb (4+ ancestors) — confirm it wraps rather than overflowing the page width on mobile
  viewport widths.
